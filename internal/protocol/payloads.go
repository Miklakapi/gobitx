package protocol

import (
	"encoding/json"
	"time"
)

type ErrorPayload struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type ReadyPayload struct {
	Port int `json:"port"`
}

type TransferRequest struct {
	DurationNS time.Duration `json:"duration_ns"`
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
