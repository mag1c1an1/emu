package measure

import "emu/message"

type MeasureModule interface {
	UpdateMeasureRecord(*message.BlockInfoMsg)
	HandleExtraMessage([]byte)
	OutputMetricName() string
	OutputRecord() ([]float64, float64)
}
