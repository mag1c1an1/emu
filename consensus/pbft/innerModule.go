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

type NormalExtraInnerHandleMod struct {
	pbftNode *ConsensusNode
}

func (nhim *NormalExtraInnerHandleMod) HandleInPropose() (bool, *message.Request) {
	block := nhim.pbftNode.CurChain.GenerateBlock(int32(nhim.pbftNode.NodeID))
	r := &message.Request{
		RequestType: message.BlockRequest,
		ReqTime:     time.Now(),
	}
	r.Msg.Content = block.Encode()
	return true, r
}

// HandleInPrePrepare the DIY operation in preprepare
func (nhim *NormalExtraInnerHandleMod) HandleInPrePrepare(ppMsg *message.PrePrepare) bool {
	if nhim.pbftNode.CurChain.IsValidBlock(core.DecodeB(ppMsg.RequestMsg.Msg.Content)) != nil {
		nhim.pbftNode.pl.PLog.Printf("S%dN%d : not a valid block\n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID)
		return false
	}
	nhim.pbftNode.pl.PLog.Printf("S%dN%d : the pre-prepare message is correct, putting it into the RequestPool. \n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID)
	nhim.pbftNode.requestPool[string(ppMsg.Digest)] = ppMsg.RequestMsg
	// merge to be a prepare message
	return true
}

// HandleInPrepare the operation in prepare, and in pbft + tx relaying, this function does not need to do any.
func (nhim *NormalExtraInnerHandleMod) HandleInPrepare(pMsg *message.Prepare) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// HandleInCommit the operation in commit.
func (nhim *NormalExtraInnerHandleMod) HandleInCommit(cMsg *message.Commit) bool {
	r := nhim.pbftNode.requestPool[string(cMsg.Digest)]
	// requestType ...
	block := core.DecodeB(r.Msg.Content)
	nhim.pbftNode.pl.PLog.Printf("S%dN%d : adding the block %d...now height = %d \n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID, block.Header.Number, nhim.pbftNode.CurChain.CurrentBlock.Header.Number)
	nhim.pbftNode.CurChain.AddBlock(block)
	nhim.pbftNode.pl.PLog.Printf("S%dN%d : added the block %d... \n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID, block.Header.Number)
	nhim.pbftNode.CurChain.PrintBlockChain()

	// now try to relay txs to other shards (for main nodes)
	if nhim.pbftNode.NodeID == uint64(nhim.pbftNode.view.Load()) {
		nhim.pbftNode.pl.PLog.Printf("S%dN%d : main node is trying to send relay txs at height = %d \n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID, block.Header.Number)
		// generate relay pool and collect txs executed
		nhim.pbftNode.CurChain.Txpool.RelayPool = make(map[uint64][]*core.Transaction)
		// send txs executed in this block to the listener
		// add more message to measure more metrics
		bim := message.BlockInfoMsg{
			BlockBodyLength: len(block.Body),
			InnerShardTxs:   block.Body,
			Epoch:           0,

			SenderShardID: nhim.pbftNode.ShardID,
			ProposeTime:   r.ReqTime,
			CommitTime:    time.Now(),
		}
		bByte, err := json.Marshal(bim)
		if err != nil {
			log.Panic(err)
		}
		msgSend := message.MergeMessage(message.CBlockInfo, bByte)
		go networks.TcpDial(msgSend, nhim.pbftNode.ipNodeTable[params.SupervisorShard][0])
		nhim.pbftNode.pl.PLog.Printf("S%dN%d : sended excuted txs\n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID)
		nhim.pbftNode.CurChain.Txpool.GetLocked()
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
			strconv.Itoa(len(nhim.pbftNode.CurChain.Txpool.TxQueue)),
			strconv.Itoa(len(block.Body)),

			strconv.FormatInt(bim.ProposeTime.UnixMilli(), 10),
			strconv.FormatInt(bim.CommitTime.UnixMilli(), 10),

			strconv.FormatInt(computeTCL(block.Body, bim.CommitTime), 10),
		}
		nhim.pbftNode.writeCsvLine(metricName, metricVal)
		nhim.pbftNode.CurChain.Txpool.GetUnlocked()
	}
	return true
}

func (nhim *NormalExtraInnerHandleMod) HandleRequestForOldSeq(*message.RequestOldMessage) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// HandleForSequentialRequest the operation for sequential requests
func (nhim *NormalExtraInnerHandleMod) HandleForSequentialRequest(som *message.SendOldMessage) bool {
	if int(som.SeqEndHeight-som.SeqStartHeight+1) != len(som.OldRequest) {
		nhim.pbftNode.pl.PLog.Printf("S%dN%d : the SendOldMessage message is not enough\n", nhim.pbftNode.ShardID, nhim.pbftNode.NodeID)
	} else { // add the block into the node pbft blockchain
		for height := som.SeqStartHeight; height <= som.SeqEndHeight; height++ {
			r := som.OldRequest[height-som.SeqStartHeight]
			if r.RequestType == message.BlockRequest {
				b := core.DecodeB(r.Msg.Content)
				nhim.pbftNode.CurChain.AddBlock(b)
			}
		}
		nhim.pbftNode.sequenceID = som.SeqEndHeight + 1
		nhim.pbftNode.CurChain.PrintBlockChain()
	}
	return true
}
