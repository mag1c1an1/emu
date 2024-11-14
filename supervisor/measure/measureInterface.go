package measure

import "emu/message"

type MeasureModule interface {
	UpdateMeasureRecord(*message.BlockInfoMsg)
	HandleExtraMessage([]byte)
	OutputMetricName() string
	// OutputRecord Write CSV
	OutputRecord() ([]float64, float64)
	Res() ([]float64, float64)
}
