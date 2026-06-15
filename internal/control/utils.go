package control

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Miklakapi/gobitx/internal/protocol"
)

func writeProtocolError(codec *protocol.Codec, code protocol.ErrorCode, message string) {
	writeProtocol(codec, protocol.CommandError, protocol.ErrorPayload{
		Code:    code,
		Message: message,
	})
}

func writeProtocol(codec *protocol.Codec, command protocol.Command, payload any) {
	frame, err := protocol.NewFrame(command, payload)
	if err != nil {
		slog.Error("failed to build frame", "err", err)
		return
	}

	err = codec.WriteFrame(frame)
	if err != nil {
		slog.Error("failed to write frame", "err", err)
	}
}

func sendResult(codec *protocol.Codec, resultType protocol.ResultType, result any) error {
	resultData, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = requestFrame(codec, protocol.CommandResult, protocol.ResultPayload{
		Type: resultType,
		Data: resultData,
	}, protocol.CommandOK)
	if err != nil {
		return err
	}

	return nil
}

func requestFrame(codec *protocol.Codec, command protocol.Command, payload any, expected protocol.Command) (protocol.Frame, error) {
	if err := writeProtocolFrame(codec, command, payload); err != nil {
		return protocol.Frame{}, err
	}

	responseFrame, err := readExpectedFrame(codec, expected)
	if err != nil {
		return protocol.Frame{}, err
	}

	return responseFrame, nil
}

func writeProtocolFrame(codec *protocol.Codec, command protocol.Command, payload any) error {
	frame, err := protocol.NewFrame(command, payload)
	if err != nil {
		return err
	}

	return codec.WriteFrame(frame)
}

func readExpectedFrame(codec *protocol.Codec, expected protocol.Command) (protocol.Frame, error) {
	frame, err := codec.ReadFrame()
	if err != nil {
		return protocol.Frame{}, err
	}

	if err = decodeErrorFrame(frame); err != nil {
		return protocol.Frame{}, err
	}

	if frame.Command != expected {
		return protocol.Frame{}, fmt.Errorf("unexpected response: got %d, expected %d", frame.Command, expected)
	}

	return frame, nil
}

func decodeErrorFrame(frame protocol.Frame) error {
	if frame.Command == protocol.CommandError {
		var errorPayload protocol.ErrorPayload

		err := json.Unmarshal(frame.Payload, &errorPayload)
		if err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}

		return fmt.Errorf("server error: %s", errorPayload.Message)
	}
	return nil
}

func showLatencyResult(data protocol.LatencyResult) {
	fmt.Printf(
		"Latency: samples=%d min=%s avg=%s max=%s\n",
		data.Samples,
		data.MinNS,
		data.AvgNS,
		data.MaxNS,
	)
}

func showTransferResult(transferType protocol.ResultType, data protocol.TransferResult) {
	transferTypeLabel := string(transferType)

	if transferTypeLabel != "" {
		transferTypeLabel = strings.ToUpper(transferTypeLabel[:1]) + transferTypeLabel[1:]
	}

	fmt.Printf(
		"%s: bytes=%d duration=%s min=%f avg=%f max=%f stability=%f\n",
		transferTypeLabel,
		data.Bytes,
		data.DurationNS,
		data.AvgMbps,
		data.MinMbps,
		data.MaxMbps,
		data.Stability,
	)
}

func showQualityResult(data protocol.QualityResult) {
	fmt.Printf(
		"Quality: sent_packets=%d received_packets=%d lost_packets=%d loss_percent=%.2f "+
			"avg_jitter=%s max_jitter=%s out_of_order=%d received_mbps=%.2f\n",
		data.SentPackets,
		data.ReceivedPackets,
		data.LostPackets,
		data.LossPercent,
		data.AvgJitterNS,
		data.MaxJitterNS,
		data.OutOfOrder,
		data.ReceivedMbps,
	)
}
