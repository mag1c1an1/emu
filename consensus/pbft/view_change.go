package pbft

import (
	"emu/message"
	"emu/networks"
	"encoding/json"
	"log"
	"time"
)

type ViewChangeData struct {
	NextView, SeqID int
}

// propose a view change request
func (p *ConsensusNode) viewChangePropose() {
	// load pbftStage as 5, i.e., making a view change
	p.pbftStage.Store(5)

	p.pl.Plog.Println("Main node is time out, now trying to view change. ")

	vcMsg := message.ViewChangeMsg{
		CurView:  int(p.view.Load()),
		NextView: int(p.view.Load()+1) % int(p.nodeNums),
		SeqID:    int(p.sequenceID),
		FromNode: p.NodeID,
	}
	// marshal and broadcast
	vcByte, err := json.Marshal(vcMsg)
	if err != nil {
		log.Panic(err)
	}
	msgSend := message.MergeMessage(message.ViewChangePropose, vcByte)
	networks.Broadcast(p.RunningNode.IpAddr, p.getNeighborNodes(), msgSend)
	networks.TcpDial(msgSend, p.RunningNode.IpAddr)

	p.pl.Plog.Println("View change message has been broadcast. ")
}

// handle view change messages.
func (p *ConsensusNode) handleViewChangeMsg(content []byte) {
	vcMsg := new(message.ViewChangeMsg)
	err := json.Unmarshal(content, vcMsg)
	if err != nil {
		log.Panic(err)
	}
	vcData := ViewChangeData{vcMsg.NextView, vcMsg.SeqID}
	if _, ok := p.viewChangeMap[vcData]; !ok {
		p.viewChangeMap[vcData] = make(map[uint64]bool)
	}
	p.viewChangeMap[vcData][vcMsg.FromNode] = true

	p.pl.Plog.Println("Received view change message from Node", vcMsg.FromNode)

	// if cnt = 2*f+1, then broadcast newView msg
	if len(p.viewChangeMap[vcData]) == 2*int(p.maliciousNums)+1 {
		nvMsg := message.NewViewMsg{
			CurView:  int(p.view.Load()),
			NextView: int(p.view.Load()+1) % int(p.nodeNums),
			NewSeqID: int(p.sequenceID),
			FromNode: p.NodeID,
		}
		nvByte, err := json.Marshal(nvMsg)
		if err != nil {
			log.Panic()
		}
		msgSend := message.MergeMessage(message.NewChange, nvByte)
		networks.Broadcast(p.RunningNode.IpAddr, p.getNeighborNodes(), msgSend)
		networks.TcpDial(msgSend, p.RunningNode.IpAddr)
	}
}

func (p *ConsensusNode) handleNewViewMsg(content []byte) {
	nvMsg := new(message.NewViewMsg)
	err := json.Unmarshal(content, nvMsg)
	if err != nil {
		log.Panic(err)
	}
	vcData := ViewChangeData{nvMsg.NextView, nvMsg.NewSeqID}
	if _, ok := p.newViewMap[vcData]; !ok {
		p.newViewMap[vcData] = make(map[uint64]bool)
	}
	p.newViewMap[vcData][nvMsg.FromNode] = true

	p.pl.Plog.Println("Received new view message from Node", nvMsg.FromNode)

	// if cnt = 2*f+1, then step into the next view.
	if len(p.newViewMap[vcData]) == 2*int(p.maliciousNums)+1 {
		p.view.Store(int32(vcData.NextView))
		p.sequenceID = uint64(nvMsg.NewSeqID)
		p.pbftStage.Store(1)
		p.lastCommitTime.Store(time.Now().UnixMilli())
		p.pl.Plog.Println("New view is updated.")
	}
}
