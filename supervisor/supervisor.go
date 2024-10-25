package supervisor

import (
	"emu/params"
	"emu/supervisor/measure"
	"net"
	"sync"
)

type Supervisor struct {
	// basic infos
	IpAddr      string // ip address of this Supervisor
	ChainConfig *params.ChainConfig
	IpNodeTable map[uint64]map[uint64]string

	// tcp control
	listenStop bool
	tcpLn      net.Listener
	tcpLock    sync.Mutex
	// logger module
	sl *supervisor_log.SupervisorLog

	// control components
	Ss *signal.StopSignal // to control the stop message sending

	// supervisor and committee components
	comMod committee.CommitteeModule

	// measure components
	testMeasureMods []measure.MeasureModule

	// diy, add more structures or classes here ...
}
