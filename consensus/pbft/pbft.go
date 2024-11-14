package pbft

import (
	"bufio"
	"emu/chain"
	"emu/consensus/pbft/pbft_log"
	"emu/message"
	"emu/networks"
	"emu/params"
	"emu/shard"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type ConsensusNode struct {
	// the local config about pbft
	RunningNode *shard.Node // the node information
	ShardID     uint64      // denote the ID of the shard (or pbft), only one pbft consensus in a shard
	NodeID      uint64      // denote the ID of the node in the pbft (shard)

	// the data structure for blockchain
	CurChain *chain.BlockChain // all node in the shard maintain the same blockchain
	db       ethdb.Database    // to save the mpt

	// the global config about pbft
	pbftChainConfig *params.ChainConfig          // the chain config in this pbft
	ipNodeTable     map[uint64]map[uint64]string // denote the ip of the specific node
	nodeNums        uint64                       // the number of nodes in this pbft, denoted by N
	maliciousNums   uint64                       // f, 3f + 1 = N

	// view change
	view           atomic.Int32 // denote the view of this pbft, the main node can be inferred from this variant
	lastCommitTime atomic.Int64 // the time since last commit.
	viewChangeMap  map[ViewChangeData]map[uint64]bool
	newViewMap     map[ViewChangeData]map[uint64]bool

	// the control message and message checking utils in pbft
	sequenceID        uint64                          // the message sequence id of the pbft
	stopSignal        atomic.Bool                     // send stop signal
	pStop             chan uint64                     // channel for stopping consensus
	requestPool       map[string]*message.Request     // RequestHash to Request
	cntPrepareConfirm map[string]map[*shard.Node]bool // count the prepare confirm message, [messageHash][Node]bool
	cntCommitConfirm  map[string]map[*shard.Node]bool // count the commit confirm message, [messageHash][Node]bool
	isCommitBroadcast map[string]bool                 // denote whether the commit is broadcast
	isReply           map[string]bool                 // denote whether the message is a reply
	height2Digest     map[uint64]string               // sequence (block height) -> request, fast read

	// pbft stage wait
	pbftStage              atomic.Int32 // 1->Preprepare, 2->Prepare, 3->Commit, 4->Done, 5 -> view change
	pbftLock               sync.Mutex
	conditionalVarPbftLock sync.Cond

	// locks about pbft
	sequenceLock sync.Mutex // the lock of sequence
	lock         sync.Mutex // lock the stage
	askForLock   sync.Mutex // lock for asking for a series of requests

	// seqID of other Shards, to synchronize
	seqIDMap   map[uint64]uint64
	seqMapLock sync.Mutex

	// logger
	pl *pbft_log.PbftLog
	// tcp control
	tcpLn       net.Listener
	tcpPoolLock sync.Mutex

	// to handle the message in the pbft
	ihm ExtraOpInConsensus

	// to handle the message outside pbft
	ohm OpInterShards
}

func NewPbftNode(shardID, nodeID uint64, pcc *params.ChainConfig, messageHandleType string) *ConsensusNode {
	p := new(ConsensusNode)
	p.ipNodeTable = params.IpMapNodeTable
	p.nodeNums = pcc.NodesPerShard
	p.ShardID = shardID
	p.NodeID = nodeID
	p.pbftChainConfig = pcc
	fp := params.DatabaseWritePath + "mptDB/ldb/s" + strconv.FormatUint(shardID, 10) + "/n" + strconv.FormatUint(nodeID, 10)
	var err error
	p.db, err = rawdb.NewLevelDBDatabase(fp, 0, 1, "accountState", false)
	if err != nil {
		log.Panic(err)
	}
	p.CurChain, err = chain.NewBlockChain(pcc, p.db)
	if err != nil {
		log.Panic("cannot new a blockchain")
	}

	p.RunningNode = &shard.Node{
		NodeID:  nodeID,
		ShardID: shardID,
		IpAddr:  p.ipNodeTable[shardID][nodeID],
	}

	p.stopSignal.Store(false)
	p.sequenceID = p.CurChain.CurrentBlock.Header.Number + 1
	p.pStop = make(chan uint64)
	p.requestPool = make(map[string]*message.Request)
	p.cntPrepareConfirm = make(map[string]map[*shard.Node]bool)
	p.cntCommitConfirm = make(map[string]map[*shard.Node]bool)
	p.isCommitBroadcast = make(map[string]bool)
	p.isReply = make(map[string]bool)
	p.height2Digest = make(map[uint64]string)
	p.maliciousNums = (p.nodeNums - 1) / 3

	// init view & last commit time
	p.view.Store(0)
	p.lastCommitTime.Store(time.Now().Add(time.Second * 5).UnixMilli())
	p.viewChangeMap = make(map[ViewChangeData]map[uint64]bool)
	p.newViewMap = make(map[ViewChangeData]map[uint64]bool)

	p.seqIDMap = make(map[uint64]uint64)

	p.pl = pbft_log.NewPbftLog(shardID, nodeID)

	// choose how to handle the messages in pbft or beyond pbft
	switch string(messageHandleType) {
	default:
		p.ihm = &NormalExtraInnerHandleMod{
			pbftNode: p,
		}
		p.ohm = &NormalExtraOuterHandleMod{
			pbftNode: p,
		}
	}

	// set pbft stage now
	p.conditionalVarPbftLock = *sync.NewCond(&p.pbftLock)
	p.pbftStage.Store(1)

	return p
}

func (p *ConsensusNode) handleMessage(msg []byte) {
	msgType, content := message.SplitMessage(msg)
	switch msgType {
	case message.CPrePrepare:
		// use "go" to start a go routine to handle this message, so that a pre-arrival message will not be aborted.
		go p.handlePrePrepare(content)
	case message.CPrepare:
		go p.handlePrepare(content)
	case message.CCommit:
		go p.handleCommit(content)
	case message.CRequestOldRequest:
		p.handleRequestOldSeq(content)
	case message.CSendOldRequest:
		p.handleSendOldSeq(content)
	case message.ViewChangePropose:
		p.handleViewChangeMsg(content)
	case message.NewChange:
		p.handleNewViewMsg(content)
	case message.CStop:
		p.WaitToStop()
	default:
		p.ohm.HandleMessageOutsidePBFT(msgType, content)
	}
}
func (p *ConsensusNode) handleClientRequest(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Panic(err)
		}
	}(conn)
	clientReader := bufio.NewReader(conn)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		if p.stopSignal.Load() {
			return
		}
		switch err {
		case nil:
			// why use this only one go routine can handle message?
			p.tcpPoolLock.Lock()
			p.handleMessage(clientRequest)
			p.tcpPoolLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}
func (p *ConsensusNode) TcpListen() {
	ln, err := net.Listen("tcp", p.RunningNode.IpAddr)
	p.tcpLn = ln
	if err != nil {
		log.Panic(err)
	}
	for {
		conn, err := p.tcpLn.Accept()
		if err != nil {
			return
		}
		go p.handleClientRequest(conn)
	}
}
func (p *ConsensusNode) WaitToStop() {
	p.pl.PLog.Println("handling stop message")
	p.stopSignal.Store(true)
	networks.CloseAllConnInPool()
	err := p.tcpLn.Close()
	if err != nil {
		log.Panic(err)
	}
	p.closePbft()
	p.pl.PLog.Println("handled stop message in TCPListen Routine")
	p.pStop <- 1

}
func (p *ConsensusNode) closePbft() {
	p.CurChain.CloseBlockChain()
}
