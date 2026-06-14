package control

import (
	"encoding/json"
	"fmt"
	"log/slog"

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
	frame, err := protocol.NewFrame(command, payload)
	if err != nil {
		return protocol.Frame{}, err
	}

	err = codec.WriteFrame(frame)
	if err != nil {
		return protocol.Frame{}, err
	}

	responseFrame, err := readExpectedFrame(codec, expected)
	if err != nil {
		return protocol.Frame{}, err
	}

	return responseFrame, nil
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
