package supervisor

import (
	"bufio"
	"emu/message"
	"emu/networks"
	"emu/params"
	"emu/supervisor/committee"
	"emu/supervisor/measure"
	"emu/supervisor/signal"
	"emu/supervisor/supervisor_log"
	"encoding/json"
	"io"
	"log"
	"net"
	"sync"
	"time"
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

func (d *Supervisor) NewSupervisor(ip string, pcc *params.ChainConfig, committeeMethod string, measureModNames ...string) {
	d.IpAddr = ip
	d.ChainConfig = pcc
	d.IpNodeTable = params.IpMapNodeTable

	d.sl = supervisor_log.NewSupervisorLog()

	d.Ss = signal.NewStopSignal(3 * int(pcc.ShardNums))

	switch committeeMethod {
	//case "CLPA_Broker":
	//	d.comMod = committee.NewCLPACommitteeMod_Broker(d.Ip_nodeTable, d.Ss, d.sl, params.DatasetFile, params.TotalDataSize, params.TxBatchSize, params.ReConfigTimeGap)
	//case "CLPA":
	//	d.comMod = committee.NewCLPACommitteeModule(d.Ip_nodeTable, d.Ss, d.sl, params.DatasetFile, params.TotalDataSize, params.TxBatchSize, params.ReConfigTimeGap)
	//case "Broker":
	//	d.comMod = committee.NewBrokerCommitteeMod(d.Ip_nodeTable, d.Ss, d.sl, params.DatasetFile, params.TotalDataSize, params.TxBatchSize)
	//default:
	//	d.comMod = committee.NewNormalCommitteeModule(d.Ip_nodeTable, d.Ss, d.sl, params.DatasetFile, params.TotalDataSize, params.TxBatchSize)
	default:
		d.comMod = committee.NewNormalCommitteeModule(d.IpNodeTable, d.Ss, d.sl, params.DatasetFile, params.TotalDataSize, params.TxBatchSize)
	}

	d.testMeasureMods = make([]measure.MeasureModule, 0)
	for _, mModName := range measureModNames {
		switch mModName {
		case "TPSNormal":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewAvgTPSNormal())

			/*	case "TPS_Relay":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_avgTPS_Relay())
				case "TPS_Broker":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_avgTPS_Broker())
				case "TCL_Relay":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTCLNormal())
				case "TCL_Broker":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_TCL_Broker())
				case "CrossTxRate_Relay":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestCrossTxRate_Relay())
				case "CrossTxRate_Broker":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestCrossTxRate_Broker())
				case "TxNumberCount_Relay":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestTxNumCount_Relay())
				case "TxNumberCount_Broker":
					d.testMeasureMods = append(d.testMeasureMods, measure.NewTestTxNumCount_Broker())*/
		case "TxDetails":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestTxDetail())
		default:
		}
	}
}

// Supervisor received the block information from the leaders, and handle these
// message to measure the performances.
func (d *Supervisor) handleBlockInfos(content []byte) {
	bim := new(message.BlockInfoMsg)
	err := json.Unmarshal(content, bim)
	if err != nil {
		log.Panic()
	}
	// StopSignal check
	if bim.BlockBodyLength == 0 {
		d.Ss.StopGapInc()
	} else {
		d.Ss.StopGapReset()
	}

	d.comMod.HandleBlockInfo(bim)

	// measure update
	for _, measureMod := range d.testMeasureMods {
		measureMod.UpdateMeasureRecord(bim)
	}
	// add codes here ...
}

// SupervisorTxHandling read transactions from dataFile. When the number of data is enough,
// the Supervisor will do re-partition and send partitionMsg and txs to leaders.
func (d *Supervisor) SupervisorTxHandling() {
	d.comMod.MsgSendingControl()
	// TxHandling is end
	for !d.Ss.GapEnough() { // wait all txs to be handled
		time.Sleep(time.Second)
		for _, measureMod := range d.testMeasureMods {
			d.sl.Slog.Println(measureMod.OutputMetricName())
			d.sl.Slog.Println(measureMod.Res())
		}
		d.sl.Sync()
	}
	// send stop message
	stopMsg := message.MergeMessage(message.CStop, []byte("this is a stop message~"))
	d.sl.Slog.Println("Supervisor: now sending cstop message to all nodes")
	for sid := uint64(0); sid < d.ChainConfig.ShardNums; sid++ {
		for nid := uint64(0); nid < d.ChainConfig.NodesPerShard; nid++ {
			networks.TcpDial(stopMsg, d.IpNodeTable[sid][nid])
		}
	}
	// make sure all stop messages are sent.
	time.Sleep(time.Duration(params.Delay+params.JitterRange+3) * time.Millisecond)

	d.sl.Slog.Println("Supervisor: now Closing")
	d.listenStop = true
	d.CloseSupervisor()
}

// handle message. only one message to be handled now
func (d *Supervisor) handleMessage(msg []byte) {
	msgType, content := message.SplitMessage(msg)
	switch msgType {
	case message.CBlockInfo:
		d.handleBlockInfos(content)
		// add codes for more functionality
	default:
		d.comMod.HandleOtherMessage(msg)
		for _, mm := range d.testMeasureMods {
			mm.HandleExtraMessage(msg)
		}
	}
}

func (d *Supervisor) handleClientRequest(con net.Conn) {
	// close
	defer func(con net.Conn) {
		err := con.Close()
		if err != nil {
			panic(err)
		}
	}(con)

	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		switch err {
		case nil:
			d.tcpLock.Lock()
			d.handleMessage(clientRequest)
			d.tcpLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}

func (d *Supervisor) TcpListen() {
	ln, err := net.Listen("tcp", d.IpAddr)
	if err != nil {
		log.Panic(err)
	}
	d.tcpLn = ln
	for {
		conn, err := d.tcpLn.Accept()
		if err != nil {
			return
		}
		go d.handleClientRequest(conn)
	}
}

//// tcp listen for Supervisor
//func (d *Supervisor) OldTcpListen() {
//	ipaddr, err := net.ResolveTCPAddr("tcp", d.IPaddr)
//	if err != nil {
//		log.Panic(err)
//	}
//	ln, err := net.ListenTCP("tcp", ipaddr)
//	d.tcpLn = ln
//	if err != nil {
//		log.Panic(err)
//	}
//	d.sl.Slog.Printf("Supervisor begins listening：%s\n", d.IPaddr)
//
//	for {
//		conn, err := d.tcpLn.Accept()
//		if err != nil {
//			if d.listenStop {
//				return
//			}
//			log.Panic(err)
//		}
//		b, err := io.ReadAll(conn)
//		if err != nil {
//			log.Panic(err)
//		}
//		d.handleMessage(b)
//		conn.(*net.TCPConn).SetLinger(0)
//		defer conn.Close()
//	}
//}

// CloseSupervisor  and record the data in .csv file
func (d *Supervisor) CloseSupervisor() {
	d.sl.Slog.Println("Closing...")
	for _, measureMod := range d.testMeasureMods {
		d.sl.Slog.Println(measureMod.OutputMetricName())
		d.sl.Slog.Println(measureMod.OutputRecord())
		println()
	}
	networks.CloseAllConnInPool()
	err := d.tcpLn.Close()
	if err != nil {
		panic(err)
		return
	}
}
