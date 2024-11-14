package measure

import (
	"emu/message"
	"strconv"
)

// TCLNormal to test average Transaction_Confirm_Latency (TCL)  in this system
type TCLNormal struct {
	epochID int

	totTxLatencyEpoch []float64 // record the Transaction_Confirm_Latency in each epoch, only for executed txs (normal txs)

	normalTxCommitLatency []int64

	normalTxNum []int

	txNum []float64 // record the txNumber in each epoch
}

func NewTCLNormal() *TCLNormal {
	return &TCLNormal{
		epochID:           -1,
		totTxLatencyEpoch: make([]float64, 0),

		normalTxCommitLatency: make([]int64, 0),

		normalTxNum: make([]int, 0),
		txNum:       make([]float64, 0),
	}
}

func (tml *TCLNormal) OutputMetricName() string {
	return "Transaction_Confirm_Latency"
}

// UpdateMeasureRecord modified latency
func (tml *TCLNormal) UpdateMeasureRecord(b *message.BlockInfoMsg) {
	if b.BlockBodyLength == 0 { // empty block
		return
	}

	epochID := b.Epoch
	mTime := b.CommitTime

	// extend
	for tml.epochID < epochID {
		tml.txNum = append(tml.txNum, 0)
		tml.totTxLatencyEpoch = append(tml.totTxLatencyEpoch, 0)

		tml.normalTxCommitLatency = append(tml.normalTxCommitLatency, 0)

		tml.normalTxNum = append(tml.normalTxNum, 0)

		tml.epochID++
	}

	tml.normalTxNum[epochID] += len(b.InnerShardTxs)
	tml.txNum[epochID] += float64(len(b.InnerShardTxs))

	// normal tx
	for _, ntx := range b.InnerShardTxs {
		tml.totTxLatencyEpoch[epochID] += mTime.Sub(ntx.Time).Seconds()

		tml.normalTxCommitLatency[epochID] += int64(mTime.Sub(ntx.Time).Milliseconds())
	}
}

func (tml *TCLNormal) HandleExtraMessage([]byte) {}

func (tml *TCLNormal) OutputRecord() (perEpochLatency []float64, totLatency float64) {
	tml.writeToCSV()

	// calculate the simple result
	perEpochLatency = make([]float64, 0)
	latencySum := 0.0
	totTxNum := 0.0
	for eid, totLatency := range tml.totTxLatencyEpoch {
		perEpochLatency = append(perEpochLatency, totLatency/tml.txNum[eid])
		latencySum += totLatency
		totTxNum += tml.txNum[eid]
	}
	totLatency = latencySum / totTxNum
	return
}

func (tml *TCLNormal) writeToCSV() {
	fileName := tml.OutputMetricName()
	measureName := []string{
		"EpochID",
		"Total tx # in this epoch",
		"Normal tx # in this epoch",
		"Sum of Normal Tx TCL (ms)",
		"Sum of All Tx TCL (sec.)"}
	measureVals := make([][]string, 0)

	for eid, totTxInE := range tml.txNum {
		csvLine := []string{
			strconv.Itoa(eid),
			strconv.FormatFloat(totTxInE, 'f', '8', 64),
			strconv.Itoa(tml.normalTxNum[eid]),
			strconv.FormatInt(tml.normalTxCommitLatency[eid], 10),
			strconv.FormatFloat(tml.totTxLatencyEpoch[eid], 'f', '8', 64),
		}
		measureVals = append(measureVals, csvLine)
	}
	WriteMetricsToCSV(fileName, measureName, measureVals)
}
