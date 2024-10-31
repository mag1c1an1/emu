package params

import "math/big"

type ChainConfig struct {
	ChainID       uint64
	NodeID        uint64
	ShardID       uint64
	NodesPerShard uint64
	ShardNums     uint64
	BlockSize     uint64
	BlockInterval uint64
	InjectSpeed   uint64

	// used in transaction relaying, useless in brokerchain mechanism
	MaxRelayBlockSize uint64
}

var (
	SupervisorShard  = uint64(2147483647)
	InitBalance, _   = new(big.Int).SetString("100000000000000000000000000000000000000000000", 10)
	IpMapNodeTable   = make(map[uint64]map[uint64]string)
	CommitteeMethod  = []string{"CLPA_Broker", "CLPA", "Broker", "Relay", "Normal"}
	MeasureBrokerMod = []string{"TPS_Broker", "TCL_Broker", "CrossTxRate_Broker", "TxNumberCount_Broker"}
	MeasureRelayMod  = []string{"TPS_Relay", "TCL_Relay", "CrossTxRate_Relay", "TxNumberCount_Relay"}
)
