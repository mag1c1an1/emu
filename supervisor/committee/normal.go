package committee

import (
	"emu/core"
	"emu/message"
	"emu/networks"
	"emu/params"
	"emu/supervisor/signal"
	"emu/supervisor/supervisor_log"
	"emu/utils"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"os"
	"time"
)

type NormalCommitteeModule struct {
	csvPath      string
	dataTotalNum int
	nowDataNum   int
	batchDataNum int
	IpNodeTable  map[uint64]map[uint64]string
	sl           *supervisor_log.SupervisorLog
	Ss           *signal.StopSignal // to control the stop message sending
}

func NewNormalCommitteeModule(IpNodeTable map[uint64]map[uint64]string, Ss *signal.StopSignal, slog *supervisor_log.SupervisorLog, csvFilePath string, dataNum, batchNum int) *NormalCommitteeModule {
	return &NormalCommitteeModule{
		csvPath:      csvFilePath,
		dataTotalNum: dataNum,
		batchDataNum: batchNum,
		nowDataNum:   0,
		IpNodeTable:  IpNodeTable,
		Ss:           Ss,
		sl:           slog,
	}
}

// transform, data to transaction
// check whether it is a legal txs message. if so, read txs and put it into the txlist
func data2tx(data []string, nonce uint64) (*core.Transaction, bool) {
	if data[6] == "0" && data[7] == "0" && len(data[3]) > 16 && len(data[4]) > 16 && data[3] != data[4] {
		val, ok := new(big.Int).SetString(data[8], 10)
		if !ok {
			log.Panic("new int failed\n")
		}
		tx := core.NewTransaction(data[3][2:], data[4][2:], val, nonce, time.Now())
		return tx, true
	}
	return &core.Transaction{}, false
}

func (ncm *NormalCommitteeModule) HandleOtherMessage([]byte) {}

func (ncm *NormalCommitteeModule) txSending(txlist []*core.Transaction) {
	// the txs will be sent
	sendToShard := make(map[uint64][]*core.Transaction)

	for idx := 0; idx <= len(txlist); idx++ {
		if idx > 0 && (idx%params.InjectSpeed == 0 || idx == len(txlist)) {
			// send to shard
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				it := message.InjectTxs{
					Txs:       sendToShard[sid],
					ToShardID: sid,
				}
				itByte, err := json.Marshal(it)
				if err != nil {
					log.Panic(err)
				}
				sendMsg := message.MergeMessage(message.CInject, itByte)
				go networks.TcpDial(sendMsg, ncm.IpNodeTable[sid][0])
			}
			sendToShard = make(map[uint64][]*core.Transaction)
			time.Sleep(time.Second)
		}
		if idx == len(txlist) {
			break
		}
		tx := txlist[idx]
		sendersID := uint64(utils.Addr2Shard(tx.Sender))
		sendToShard[sendersID] = append(sendToShard[sendersID], tx)
	}
}

// MsgSendingControl read transactions, the Number of the transactions is - batchDataNum
func (ncm *NormalCommitteeModule) MsgSendingControl() {
	txFile, err := os.Open(ncm.csvPath)
	if err != nil {
		log.Panic(err)
	}
	defer func(txFile *os.File) {
		err := txFile.Close()
		if err != nil {
			panic(err)
		}
	}(txFile)
	reader := csv.NewReader(txFile)
	txList := make([]*core.Transaction, 0) // save the txs in this epoch (round)

	for {
		data, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Panic(err)
		}
		if tx, ok := data2tx(data, uint64(ncm.nowDataNum)); ok {
			txList = append(txList, tx)
			ncm.nowDataNum++
		}

		// re-shard condition, enough edges
		if len(txList) == int(ncm.batchDataNum) || ncm.nowDataNum == ncm.dataTotalNum {
			ncm.txSending(txList)
			// reset the variants about tx sending
			txList = make([]*core.Transaction, 0)
			ncm.Ss.StopGapReset()
		}

		if ncm.nowDataNum == ncm.dataTotalNum {
			break
		}
	}
}

// HandleBlockInfo no operation here
func (ncm *NormalCommitteeModule) HandleBlockInfo(b *message.BlockInfoMsg) {
	ncm.sl.Slog.Printf("received from shard %d in epoch %d.\n", b.SenderShardID, b.Epoch)
}
