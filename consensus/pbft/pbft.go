package pbft

import (
	"emu/message"
	"emu/shard"
	"github.com/ethereum/go-ethereum/ethdb"
	"net"
	"sync"
	"sync/atomic"
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
	pbftStage              atomic.Int32 // 1->Preprepare, 2->Prepare, 3->Commit, 4->Done
	pbftLock               sync.Mutex
	conditionalVarPbftLock sync.Cond

	// locks about pbft
	sequenceLock sync.Mutex // the lock of sequence
	lock         sync.Mutex // lock the stage
	askForLock   sync.Mutex // lock for asking for a serise of requests

	// seqID of other Shards, to synchronize
	seqIDMap   map[uint64]uint64
	seqMapLock sync.Mutex

	// logger
	pl *pbft_log.PbftLog
	// tcp control
	tcpln       net.Listener
	tcpPoolLock sync.Mutex

	// to handle the message in the pbft
	ihm ExtraOpInConsensus

	// to handle the message outside pbft
	ohm OpInterShards
}
