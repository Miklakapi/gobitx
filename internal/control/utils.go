package control

import (
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
