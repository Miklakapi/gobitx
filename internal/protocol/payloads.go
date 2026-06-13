package protocol

import "encoding/json"

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ReadyPayload struct {
	Port int `json:"port"`
}

type DownloadRequest struct {
	DurationNS int64 `json:"duration_ns"`
}

type UploadRequest struct {
	DurationNS int64 `json:"duration_ns"`
}

type QualityRequest struct {
	DurationNS int64 `json:"duration_ns"`
	PacketSize int   `json:"packet_size"`
	PacketRate int   `json:"packet_rate"`
}

type ResultPayload struct {
	Type ResultType      `json:"type"`
	Data json.RawMessage `json:"data"`
}
