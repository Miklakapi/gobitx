package protocol

import (
	"time"

	"github.com/Miklakapi/byteflow"
)

type ResultType string

const (
	ResultDownload ResultType = "download"
	ResultUpload   ResultType = "upload"
	ResultLatency  ResultType = "latency"
	ResultQuality  ResultType = "quality"
)

type LatencyResult struct {
	Samples int           `json:"samples"`
	Min     time.Duration `json:"min"`
	Avg     time.Duration `json:"avg"`
	Max     time.Duration `json:"max"`
}

type TransferResult struct {
	Bytes     byteflow.Size `json:"bytes"`
	Duration  time.Duration `json:"duration"`
	AvgRate   byteflow.Rate `json:"avg_rate"`
	MinRate   byteflow.Rate `json:"min_rate"`
	MaxRate   byteflow.Rate `json:"max_rate"`
	Stability float64       `json:"stability"`
}

type QualityResult struct {
	SentPackets     int64         `json:"sent_packets"`
	ReceivedPackets int64         `json:"received_packets"`
	LostPackets     int64         `json:"lost_packets"`
	LossPercent     float64       `json:"loss_percent"`
	AvgJitter       time.Duration `json:"avg_jitter"`
	MaxJitter       time.Duration `json:"max_jitter"`
	OutOfOrder      int64         `json:"out_of_order"`
	ReceivedRate    byteflow.Rate `json:"received_rate"`
}
