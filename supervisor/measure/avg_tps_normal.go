package measure

import (
	"emu/message"
	"strconv"
	"time"
)

// AvgTPSNormal to test average TPS in this system
type AvgTPSNormal struct {
	epochID       int
	executedTxNum []float64 // record how many executed txs in an epoch, maybe the cross shard tx will be calculated as a 0.5 tx
	normalTxNum   []int

	startTime []time.Time // record when the epoch starts
	endTime   []time.Time // record when the epoch ends
}

func NewAvgTPSNormal() *AvgTPSNormal {
	return &AvgTPSNormal{
		epochID:       -1,
		executedTxNum: make([]float64, 0),
		startTime:     make([]time.Time, 0),
		endTime:       make([]time.Time, 0),

		normalTxNum: make([]int, 0),
	}
}

func (tat *AvgTPSNormal) OutputMetricName() string {
	return "Average_TPS"
}

// UpdateMeasureRecord add the number of executed txs, and change the time records
func (tat *AvgTPSNormal) UpdateMeasureRecord(b *message.BlockInfoMsg) {
	if b.BlockBodyLength == 0 { // empty block
		return
	}

	epochID := b.Epoch
	earliestTime := b.ProposeTime
	latestTime := b.CommitTime

	// extend
	for tat.epochID < epochID {
		tat.executedTxNum = append(tat.executedTxNum, 0)
		tat.startTime = append(tat.startTime, time.Time{})
		tat.endTime = append(tat.endTime, time.Time{})

		tat.normalTxNum = append(tat.normalTxNum, 0)

		tat.epochID++
	}

	// modify the local epoch data
	tat.executedTxNum[epochID] += float64(len(b.InnerShardTxs))

	tat.normalTxNum[epochID] += len(b.InnerShardTxs)

	if tat.startTime[epochID].IsZero() || tat.startTime[epochID].After(earliestTime) {
		tat.startTime[epochID] = earliestTime
	}
	if tat.endTime[epochID].IsZero() || latestTime.After(tat.endTime[epochID]) {
		tat.endTime[epochID] = latestTime
	}
}

func (tat *AvgTPSNormal) HandleExtraMessage([]byte) {}

func (tat *AvgTPSNormal) Res() (perEpochTPS []float64, totalTPS float64) {
	perEpochTPS = make([]float64, tat.epochID+1)
	totalTxNum := 0.0
	eTime := time.Now()
	lTime := time.Time{}
	for eid, exTxNum := range tat.executedTxNum {
		timeGap := tat.endTime[eid].Sub(tat.startTime[eid]).Seconds()
		perEpochTPS[eid] = exTxNum / timeGap
		totalTxNum += exTxNum
		if eTime.After(tat.startTime[eid]) {
			eTime = tat.startTime[eid]
		}
		if tat.endTime[eid].After(lTime) {
			lTime = tat.endTime[eid]
		}
	}
	totalTPS = totalTxNum / (lTime.Sub(eTime).Seconds())
	return
}

// OutputRecord output the average TPS
func (tat *AvgTPSNormal) OutputRecord() (perEpochTPS []float64, totalTPS float64) {
	tat.writeToCSV()

	// calculate the simple result
	return tat.Res()
}

func (tat *AvgTPSNormal) writeToCSV() {
	fileName := tat.OutputMetricName()
	measureName := []string{"EpochID", "Total tx # in this epoch", "Normal tx # in this epoch", "Epoch start time", "Epoch end time", "Avg. TPS of this epoch"}
	measureVals := make([][]string, 0)

	for eid, exTxNum := range tat.executedTxNum {
		timeGap := tat.endTime[eid].Sub(tat.startTime[eid]).Seconds()
		csvLine := []string{
			strconv.Itoa(eid),
			strconv.FormatFloat(exTxNum, 'f', '8', 64),
			strconv.Itoa(tat.normalTxNum[eid]),
			strconv.FormatInt(tat.startTime[eid].UnixMilli(), 10),
			strconv.FormatInt(tat.endTime[eid].UnixMilli(), 10),
			strconv.FormatFloat(exTxNum/timeGap, 'f', '8', 64),
		}
		measureVals = append(measureVals, csvLine)
	}
	WriteMetricsToCSV(fileName, measureName, measureVals)
}
