package pbft

import (
	"emu/core"
	"emu/message"
	"emu/networks"
	"emu/params"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

type NormalExtraHandleMod struct {
	pbftNode *ConsensusNode
}

func (nhm *NormalExtraHandleMod) HandleInPropose() (bool, *message.Request) {
	block := nhm.pbftNode.CurChain.GenerateBlock(int32(nhm.pbftNode.NodeId))
	r := &message.Request{
		RequestType: message.BlockRequest,
		ReqTime:     time.Now(),
	}
	r.Msg.Content = block.Encode()
	return true, r
}

// HandleInPrePrepare the DIY operation in preprepare
func (nhm *NormalExtraHandleMod) HandleInPrePrepare(ppMsg *message.PrePrepare) bool {
	if nhm.pbftNode.CurChain.IsValidBlock(core.DecodeB(ppMsg.RequestMsg.Msg.Content)) != nil {
		nhm.pbftNode.pl.Plog.Printf("S%dN%d : not a valid block\n", nhm.pbftNode.ShardId, nhm.pbftNode.NodeId)
		return false
	}
	nhm.pbftNode.pl.Plog.Printf("S%dN%d : the pre-prepare message is correct, putting it into the RequestPool. \n", nhm.pbftNode.ShardId, nhm.pbftNode.NodeId)
	nhm.pbftNode.requestPool[string(ppMsg.Digest)] = ppMsg.RequestMsg
	// merge to be a prepare message
	return true
}

// HandleInPrepare the operation in prepare, and in pbft + tx relaying, this function does not need to do any.
func (nhm *NormalExtraHandleMod) HandleInPrepare(pMsg *message.Prepare) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// HandleInCommit the operation in commit.
func (nhm *NormalExtraHandleMod) HandleInCommit(cMsg *message.Commit) bool {
	r := nhm.pbftNode.requestPool[string(cMsg.Digest)]
	// requestType ...
	block := core.DecodeB(r.Msg.Content)
	nhm.pbftNode.pl.Plog.Printf("S%dN%d : adding the block %d...now height = %d \n", nhm.pbftNode.ShardId, nhm.pbftNode.NodeId, block.Header.Number, nhm.pbftNode.CurChain.CurrentBlock.Header.Number)
	nhm.pbftNode.CurChain.AddBlock(block)
	nhm.pbftNode.pl.Plog.Printf("S%dN%d : added the block %d... \n", nhm.pbftNode.ShardId, nhm.pbftNode.NodeId, block.Header.Number)
	nhm.pbftNode.CurChain.PrintBlockChain()

	// now try to relay txs to other shards (for main nodes)
	if nhm.pbftNode.NodeId == uint64(nhm.pbftNode.view.Load()) {
		nhm.pbftNode.pl.Plog.Printf("S%dN%d : main node is trying to send relay txs at height = %d \n", nhm.pbftNode.ShardId, nhm.pbftNode.NodeId, block.Header.Number)
		// generate relay pool and collect txs executed
		nhm.pbftNode.CurChain.Txpool.RelayPool = make(map[uint64][]*core.Transaction)
		// send txs executed in this block to the listener
		// add more message to measure more metrics
		bim := message.BlockInfoMsg{
			BlockBodyLength: len(block.Body),
			InnerShardTxs:   block.Body,
			Epoch:           0,

			SenderShardID: nhm.pbftNode.ShardId,
			ProposeTime:   r.ReqTime,
			CommitTime:    time.Now(),
		}
		bByte, err := json.Marshal(bim)
		if err != nil {
			log.Panic(err)
		}
		msgSend := message.MergeMessage(message.CBlockInfo, bByte)
		go networks.TcpDial(msgSend, nhm.pbftNode.ipNodeTable[params.SupervisorShard][0])
		nhm.pbftNode.pl.Plog.Printf("S%dN%d : sended excuted txs\n", nhm.pbftNode.ShardId, nhm.pbftNode.NodeId)
		nhm.pbftNode.CurChain.Txpool.GetLocked()
		metricName := []string{
			"Block Height",
			"EpochID of this block",
			"TxPool Size",
			"# of all Txs in this block",
			"TimeStamp - Propose (unixMill)",
			"TimeStamp - Commit (unixMill)",

			"SUM of confirm latency (ms, All Txs)",
		}
		metricVal := []string{
			strconv.Itoa(int(block.Header.Number)),
			strconv.Itoa(bim.Epoch),
			strconv.Itoa(len(nhm.pbftNode.CurChain.Txpool.TxQueue)),
			strconv.Itoa(len(block.Body)),

			strconv.FormatInt(bim.ProposeTime.UnixMilli(), 10),
			strconv.FormatInt(bim.CommitTime.UnixMilli(), 10),

			strconv.FormatInt(computeTCL(block.Body, bim.CommitTime), 10),
		}
		nhm.pbftNode.writeCsvLine(metricName, metricVal)
		nhm.pbftNode.CurChain.Txpool.GetUnlocked()
	}
	return true
}

//func (rphm *RawRelayPbftExtraHandleMod) HandleReqestforOldSeq(*message.RequestOldMessage) bool {
//	fmt.Println("No operations are performed in Extra handle mod")
//	return true
//}
//
//// the operation for sequential requests
//func (rphm *RawRelayPbftExtraHandleMod) HandleforSequentialRequest(som *message.SendOldMessage) bool {
//	if int(som.SeqEndHeight-som.SeqStartHeight+1) != len(som.OldRequest) {
//		rphm.pbftNode.pl.Plog.Printf("S%dN%d : the SendOldMessage message is not enough\n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID)
//	} else { // add the block into the node pbft blockchain
//		for height := som.SeqStartHeight; height <= som.SeqEndHeight; height++ {
//			r := som.OldRequest[height-som.SeqStartHeight]
//			if r.RequestType == message.BlockRequest {
//				b := core.DecodeB(r.Msg.Content)
//				rphm.pbftNode.CurChain.AddBlock(b)
//			}
//		}
//		rphm.pbftNode.sequenceID = som.SeqEndHeight + 1
//		rphm.pbftNode.CurChain.PrintBlockChain()
//	}
//	return true
//}
