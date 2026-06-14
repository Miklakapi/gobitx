package protocol

import "time"

type ResultType string

const (
	ResultDownload ResultType = "download"
	ResultUpload   ResultType = "upload"
	ResultLatency  ResultType = "latency"
	ResultQuality  ResultType = "quality"
)

type LatencyResult struct {
	Samples int           `json:"samples"`
	MinNS   time.Duration `json:"min_ns"`
	AvgNS   time.Duration `json:"avg_ns"`
	MaxNS   time.Duration `json:"max_ns"`
}

type TransferResult struct {
	Bytes      int64   `json:"bytes"`
	DurationNS int64   `json:"duration_ns"`
	AvgMbps    float64 `json:"avg_mbps"`
	MinMbps    float64 `json:"min_mbps"`
	MaxMbps    float64 `json:"max_mbps"`
	Stability  float64 `json:"stability"`
}

type QualityResult struct {
	SentPackets     int64   `json:"sent_packets"`
	ReceivedPackets int64   `json:"received_packets"`
	LostPackets     int64   `json:"lost_packets"`
	LossPercent     float64 `json:"loss_percent"`
	AvgJitterNS     int64   `json:"avg_jitter_ns"`
	MaxJitterNS     int64   `json:"max_jitter_ns"`
	OutOfOrder      int64   `json:"out_of_order"`
	ReceivedMbps    float64 `json:"received_mbps"`
}
