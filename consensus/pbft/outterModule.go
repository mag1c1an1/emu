package pbft

import (
	"emu/message"
	"encoding/json"
	"log"
)

// NormalExtraOuterHandleMod This module used in the blockChain using transaction .
type NormalExtraOuterHandleMod struct {
	pbftNode *ConsensusNode
}

// HandleMessageOutsidePBFT msgType can be defined in message
func (nohm *NormalExtraOuterHandleMod) HandleMessageOutsidePBFT(msgType message.MessageType, content []byte) bool {
	switch msgType {
	case message.CInject:
		nohm.handleInjectTx(content)
	default:
	}
	return true
}

func (nohm *NormalExtraOuterHandleMod) handleInjectTx(content []byte) {
	it := new(message.InjectTxs)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	nohm.pbftNode.CurChain.Txpool.AddTxs2Pool(it.Txs)
	nohm.pbftNode.pl.Plog.Printf("S%dN%d : has handled injected txs msg, txs: %d \n", nohm.pbftNode.ShardId, nohm.pbftNode.NodeId, len(it.Txs))
}
