package pbft

import (
	"emu/message"
	"emu/networks"
	"emu/params"
	"emu/shard"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Propose this func is only invoked by main node
func (p *ConsensusNode) Propose() {
	// wait other nodes to start Tcp listen, sleep 5 sec.
	time.Sleep(5 * time.Second)

	nextRoundBeginSignal := make(chan bool)

	go func() {
		// go into the next round
		for {
			time.Sleep(time.Duration(int64(p.pbftChainConfig.BlockInterval)) * time.Millisecond)
			// send a signal to another GO-Routine. It will block until a GO-Routine try to fetch data from this channel.
			for p.pbftStage.Load() != 1 {
				time.Sleep(time.Millisecond * 100)
			}
			nextRoundBeginSignal <- true
		}
	}()

	go func() {
		// check whether to view change
		for {
			time.Sleep(time.Second)
			if time.Now().UnixMilli()-p.lastCommitTime.Load() > int64(params.PbftViewChangeTimeOut) {
				p.lastCommitTime.Store(time.Now().UnixMilli())
				go p.viewChangePropose()
			}
		}
	}()

	for {
		select {
		case <-nextRoundBeginSignal:
			go func() {
				// if this node is not leader, do not propose.
				if uint64(p.view.Load()) != p.NodeId {
					return
				}

				p.sequenceLock.Lock()
				p.pl.Plog.Printf("S%dN%d get sequenceLock locked, now trying to propose...\n", p.ShardId, p.NodeId)
				// propose
				// implement interface to generate propose
				_, r := p.ihm.HandleInPropose()

				digest := getDigest(r)
				p.requestPool[string(digest)] = r
				p.pl.Plog.Printf("S%dN%d put the request into the pool ...\n", p.ShardId, p.NodeId)

				ppMsg := message.PrePrepare{
					RequestMsg: r,
					Digest:     digest,
					SeqID:      p.sequenceID,
				}
				p.height2Digest[p.sequenceID] = string(digest)
				// marshal and broadcast
				ppByte, err := json.Marshal(ppMsg)
				if err != nil {
					log.Panic(err)
				}
				msgSend := message.MergeMessage(message.CPrePrepare, ppByte)
				networks.Broadcast(p.RunningNode.IpAddr, p.getNeighborNodes(), msgSend)
				// send to self
				networks.TcpDial(msgSend, p.RunningNode.IpAddr)
				p.pbftStage.Store(2)
			}()

		case <-p.pStop:
			p.pl.Plog.Printf("S%dN%d get stopSignal in Propose Routine, now stop...\n", p.ShardId, p.NodeId)
			return
		}
	}
}

// Handle pre-prepare messages here.
// If you want to do more operations in the pre-prepare stage, you can implement the interface "ExtraOpInConsensus",
// and call the function: **ExtraOpInConsensus.HandleInPrePrepare**
func (p *ConsensusNode) handlePrePrepare(content []byte) {
	p.RunningNode.PrintNode()
	fmt.Println("received the PrePrepare ...")
	// decode the message
	ppMsg := new(message.PrePrepare)
	err := json.Unmarshal(content, ppMsg)
	if err != nil {
		log.Panic(err)
	}

	curView := p.view.Load()
	p.pbftLock.Lock()
	defer p.pbftLock.Unlock()

	for p.pbftStage.Load() < 1 && ppMsg.SeqID >= p.sequenceID && p.view.Load() == curView {
		p.conditionalVarPbftLock.Wait()
	}
	defer p.conditionalVarPbftLock.Broadcast()

	// if this message is out of date, return.
	if ppMsg.SeqID < p.sequenceID || p.view.Load() != curView {
		return
	}

	flag := false
	if digest := getDigest(ppMsg.RequestMsg); string(digest) != string(ppMsg.Digest) {
		p.pl.Plog.Printf("S%dN%d : the digest is not consistent, so refuse to prepare. \n", p.ShardId, p.NodeId)
	} else if p.sequenceID < ppMsg.SeqID {
		// may have sync issue?
		p.requestPool[string(getDigest(ppMsg.RequestMsg))] = ppMsg.RequestMsg
		p.height2Digest[ppMsg.SeqID] = string(getDigest(ppMsg.RequestMsg))
		p.pl.Plog.Printf("S%dN%d : the Sequence id is not consistent, so refuse to prepare. \n", p.ShardId, p.NodeId)
	} else {
		// do your operation in this interface
		flag = p.ihm.HandleInPrePrepare(ppMsg)
		p.requestPool[string(getDigest(ppMsg.RequestMsg))] = ppMsg.RequestMsg
		p.height2Digest[ppMsg.SeqID] = string(getDigest(ppMsg.RequestMsg))
	}
	// if the message is true, broadcast the prepare message
	if flag {
		pre := message.Prepare{
			Digest:     ppMsg.Digest,
			SeqID:      ppMsg.SeqID,
			SenderNode: p.RunningNode,
		}
		prepareByte, err := json.Marshal(pre)
		if err != nil {
			log.Panic()
		}
		// broadcast
		msgSend := message.MergeMessage(message.CPrepare, prepareByte)
		networks.Broadcast(p.RunningNode.IpAddr, p.getNeighborNodes(), msgSend)
		networks.TcpDial(msgSend, p.RunningNode.IpAddr)
		p.pl.Plog.Printf("S%dN%d : has broadcast the prepare message \n", p.ShardId, p.NodeId)

		// Pbft stage add 1. It means that this round of pbft goes into the next stage, i.e., Prepare stage.
		p.pbftStage.Add(1)
	}
}

// Handle prepare messages here.
// If you want to do more operations in the prepare stage, you can implement the interface "ExtraOpInConsensus",
// and call the function: **ExtraOpInConsensus.HandleInPrepare**
func (p *ConsensusNode) handlePrepare(content []byte) {
	p.pl.Plog.Printf("S%dN%d : received the Prepare ...\n", p.ShardId, p.NodeId)
	// decode the message
	pMsg := new(message.Prepare)
	err := json.Unmarshal(content, pMsg)
	if err != nil {
		log.Panic(err)
	}

	curView := p.view.Load()
	p.pbftLock.Lock()
	defer p.pbftLock.Unlock()
	for p.pbftStage.Load() < 2 && pMsg.SeqID >= p.sequenceID && p.view.Load() == curView {
		p.conditionalVarPbftLock.Wait()
	}
	defer p.conditionalVarPbftLock.Broadcast()

	// if this message is out of date, return.
	if pMsg.SeqID < p.sequenceID || p.view.Load() != curView {
		return
	}

	if _, ok := p.requestPool[string(pMsg.Digest)]; !ok {
		p.pl.Plog.Printf("S%dN%d : doesn't have the digest in the requst pool, refuse to commit\n", p.ShardId, p.NodeId)
	} else if p.sequenceID < pMsg.SeqID {
		p.pl.Plog.Printf("S%dN%d : inconsistent sequence ID, refuse to commit\n", p.ShardId, p.NodeId)
	} else {
		// if needed more operations, implement interfaces
		p.ihm.HandleInPrepare(pMsg)

		p.set2DMap(true, string(pMsg.Digest), pMsg.SenderNode)
		cnt := len(p.cntPrepareConfirm[string(pMsg.Digest)])

		// if the node has received 2f messages (itself included), and it haven't committed, then it commit
		p.lock.Lock()
		defer p.lock.Unlock()
		if uint64(cnt) >= 2*p.maliciousNums+1 && !p.isCommitBroadcast[string(pMsg.Digest)] {
			p.pl.Plog.Printf("S%dN%d : is going to commit\n", p.ShardId, p.NodeId)
			// generate commit and broadcast
			c := message.Commit{
				Digest:     pMsg.Digest,
				SeqID:      pMsg.SeqID,
				SenderNode: p.RunningNode,
			}
			commitByte, err := json.Marshal(c)
			if err != nil {
				log.Panic()
			}
			msgSend := message.MergeMessage(message.CCommit, commitByte)
			networks.Broadcast(p.RunningNode.IpAddr, p.getNeighborNodes(), msgSend)
			networks.TcpDial(msgSend, p.RunningNode.IpAddr)
			p.isCommitBroadcast[string(pMsg.Digest)] = true
			p.pl.Plog.Printf("S%dN%d : commit is broadcast\n", p.ShardId, p.NodeId)

			p.pbftStage.Add(1)
		}
	}
}

// Handle commit messages here.
// If you want to do more operations in the commit stage, you can implement the interface "ExtraOpInConsensus",
// and call the function: **ExtraOpInConsensus.HandleInCommit**
func (p *ConsensusNode) handleCommit(content []byte) {
	// decode the message
	cMsg := new(message.Commit)
	err := json.Unmarshal(content, cMsg)
	if err != nil {
		log.Panic(err)
	}

	curView := p.view.Load()
	p.pbftLock.Lock()
	defer p.pbftLock.Unlock()
	for p.pbftStage.Load() < 3 && cMsg.SeqID >= p.sequenceID && p.view.Load() == curView {
		p.conditionalVarPbftLock.Wait()
	}
	defer p.conditionalVarPbftLock.Broadcast()

	if cMsg.SeqID < p.sequenceID || p.view.Load() != curView {
		return
	}

	// stage >= 3 and seqId >= p.seqId

	p.pl.Plog.Printf("S%dN%d received the Commit from ...%d\n", p.ShardId, p.NodeId, cMsg.SenderNode.NodeId)
	p.set2DMap(false, string(cMsg.Digest), cMsg.SenderNode)
	cnt := len(p.cntCommitConfirm[string(cMsg.Digest)])

	p.lock.Lock()
	defer p.lock.Unlock()

	if uint64(cnt) >= 2*p.maliciousNums+1 && !p.isReply[string(cMsg.Digest)] {
		p.pl.Plog.Printf("S%dN%d : has received 2f + 1 commits ... \n", p.ShardId, p.NodeId)
		// if this node is left behind, so it need to request blocks
		if _, ok := p.requestPool[string(cMsg.Digest)]; !ok {
			p.isReply[string(cMsg.Digest)] = true
			p.askForLock.Lock()
			// request the block
			sn := &shard.Node{
				NodeId:  uint64(p.view.Load()),
				ShardId: p.ShardId,
				IpAddr:  p.ipNodeTable[p.ShardId][uint64(p.view.Load())],
			}
			oldRequest := message.RequestOldMessage{
				SeqStartHeight: p.sequenceID + 1,
				SeqEndHeight:   cMsg.SeqID,
				ServerNode:     sn,
				SenderNode:     p.RunningNode,
			}
			b, err := json.Marshal(oldRequest)
			if err != nil {
				log.Panic()
			}

			p.pl.Plog.Printf("S%dN%d : is now requesting message (seq %d to %d) ... \n", p.ShardId, p.NodeId, oldRequest.SeqStartHeight, oldRequest.SeqEndHeight)
			msgSend := message.MergeMessage(message.CRequestOldRequest, b)
			networks.TcpDial(msgSend, oldRequest.ServerNode.IpAddr)
		} else {
			// implement interface
			p.ihm.HandleInCommit(cMsg)
			p.isReply[string(cMsg.Digest)] = true
			p.pl.Plog.Printf("S%dN%d: this round of pbft %d is end \n", p.ShardId, p.NodeId, p.sequenceID)
			p.sequenceID += 1
		}

		p.pbftStage.Store(1)
		p.lastCommitTime.Store(time.Now().UnixMilli())

		// if this node is a main node, then unlock the sequencelock
		if p.NodeId == uint64(p.view.Load()) {
			p.sequenceLock.Unlock()
			p.pl.Plog.Printf("S%dN%d get sequenceLock unlocked...\n", p.ShardId, p.NodeId)
		}
	}
}

// this func is only invoked by the main node,
// if the request is correct, the main node will send
// block back to the message sender.
// now this function can send both block and partition
func (p *PbftConsensusNode) handleRequestOldSeq(content []byte) {
	if uint64(p.view.Load()) != p.NodeID {
		return
	}

	rom := new(message.RequestOldMessage)
	err := json.Unmarshal(content, rom)
	if err != nil {
		log.Panic()
	}
	p.pl.Plog.Printf("S%dN%d : received the old message requst from ...", p.ShardID, p.NodeID)
	rom.SenderNode.PrintNode()

	oldR := make([]*message.Request, 0)
	for height := rom.SeqStartHeight; height <= rom.SeqEndHeight; height++ {
		if _, ok := p.height2Digest[height]; !ok {
			p.pl.Plog.Printf("S%dN%d : has no this digest to this height %d\n", p.ShardID, p.NodeID, height)
			break
		}
		if r, ok := p.requestPool[p.height2Digest[height]]; !ok {
			p.pl.Plog.Printf("S%dN%d : has no this message to this digest %d\n", p.ShardID, p.NodeID, height)
			break
		} else {
			oldR = append(oldR, r)
		}
	}
	p.pl.Plog.Printf("S%dN%d : has generated the message to be sent\n", p.ShardID, p.NodeID)

	p.ihm.HandleReqestforOldSeq(rom)

	// send the block back
	sb := message.SendOldMessage{
		SeqStartHeight: rom.SeqStartHeight,
		SeqEndHeight:   rom.SeqEndHeight,
		OldRequest:     oldR,
		SenderNode:     p.RunningNode,
	}
	sbByte, err := json.Marshal(sb)
	if err != nil {
		log.Panic()
	}
	msg_send := message.MergeMessage(message.CSendOldRequest, sbByte)
	networks.TcpDial(msg_send, rom.SenderNode.IPaddr)
	p.pl.Plog.Printf("S%dN%d : send blocks\n", p.ShardID, p.NodeID)
}

// node request blocks and receive blocks from the main node
func (p *PbftConsensusNode) handleSendOldSeq(content []byte) {
	som := new(message.SendOldMessage)
	err := json.Unmarshal(content, som)
	if err != nil {
		log.Panic()
	}
	p.pl.Plog.Printf("S%dN%d : has received the SendOldMessage message\n", p.ShardID, p.NodeID)

	// implement interface for new consensus
	p.ihm.HandleforSequentialRequest(som)
	beginSeq := som.SeqStartHeight
	for idx, r := range som.OldRequest {
		p.requestPool[string(getDigest(r))] = r
		p.height2Digest[uint64(idx)+beginSeq] = string(getDigest(r))
		p.isReply[string(getDigest(r))] = true
		p.pl.Plog.Printf("this round of pbft %d is end \n", uint64(idx)+beginSeq)
	}
	p.sequenceID = som.SeqEndHeight + 1
	if rDigest, ok1 := p.height2Digest[p.sequenceID]; ok1 {
		if r, ok2 := p.requestPool[rDigest]; ok2 {
			ppmsg := &message.PrePrepare{
				RequestMsg: r,
				SeqID:      p.sequenceID,
				Digest:     getDigest(r),
			}
			flag := false
			flag = p.ihm.HandleinPrePrepare(ppmsg)
			if flag {
				pre := message.Prepare{
					Digest:     ppmsg.Digest,
					SeqID:      ppmsg.SeqID,
					SenderNode: p.RunningNode,
				}
				prepareByte, err := json.Marshal(pre)
				if err != nil {
					log.Panic()
				}
				// broadcast
				msg_send := message.MergeMessage(message.CPrepare, prepareByte)
				networks.Broadcast(p.RunningNode.IPaddr, p.getNeighborNodes(), msg_send)
				p.pl.Plog.Printf("S%dN%d : has broadcast the prepare message \n", p.ShardID, p.NodeID)
			}
		}
	}

	p.askForLock.Unlock()
}
