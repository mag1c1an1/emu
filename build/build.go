package build

import (
	"emu/consensus/pbft"
	"emu/networks"
	"emu/params"
	"emu/supervisor"
	"log"
	"time"
)

func initConfig(nid, nnm, sid, snm uint64) *params.ChainConfig {
	// Read the contents of ipTable.json
	ipMap := readIpTable("./ipTable.json")
	params.IpMapNodeTable = ipMap
	params.SupervisorAddr = params.IpMapNodeTable[params.SupervisorShard][0]

	// check the correctness of params
	if len(ipMap)-1 < int(snm) {
		log.Panicf("Input ShardNumber = %d, but only %d shards in ipTable.json.\n", snm, len(ipMap)-1)
	}
	for shardID := 0; shardID < len(ipMap)-1; shardID++ {
		if len(ipMap[uint64(shardID)]) < int(nnm) {
			log.Panicf("Input NodeNumber = %d, but only %d nodes in Shard %d.\n", nnm, len(ipMap[uint64(shardID)]), shardID)
		}
	}

	params.NodesInShard = int(nnm)
	params.ShardNum = int(snm)

	// init the network layer
	networks.InitNetworkTools()

	pcc := &params.ChainConfig{
		ChainID:       sid,
		NodeID:        nid,
		ShardID:       sid,
		NodesPerShard: uint64(params.NodesInShard),
		ShardNums:     snm,
		BlockSize:     uint64(params.MaxBlockSizeGlobal),
		BlockInterval: uint64(params.BlockInterval),
		InjectSpeed:   uint64(params.InjectSpeed),
	}
	return pcc
}

func BuildSupervisor(nnm, snm uint64) {
	methodID := params.ConsensusMethod
	var measureMod []string

	switch methodID {
	case 0, 2:
		measureMod = params.MeasureRelayMod
	case 4:
		measureMod = params.MeasureNormalMod
	default:
		measureMod = params.MeasureBrokerMod
	}

	measureMod = append(measureMod, "TxDetails")

	lsn := new(supervisor.Supervisor)
	lsn.NewSupervisor(params.SupervisorAddr, initConfig(123, nnm, 123, snm), params.CommitteeMethod[methodID], measureMod...)
	time.Sleep(10000 * time.Millisecond)
	go lsn.SupervisorTxHandling()
	lsn.TcpListen()
}

func BuildNewPbftNode(nid, nnm, sid, snm uint64) {
	methodID := params.ConsensusMethod
	worker := pbft.NewPbftNode(sid, nid, initConfig(nid, nnm, sid, snm), params.CommitteeMethod[methodID])
	go worker.TcpListen()
	worker.Propose()
}
