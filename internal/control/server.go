package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/Miklakapi/gobitx/internal/config"
	"github.com/Miklakapi/gobitx/internal/protocol"
)

type Server struct {
	cfg             config.Config
	listener        net.Listener
	clientConnected bool
	mu              sync.Mutex
}

func NewServer(cfg config.Config) (*Server, error) {
	listener, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:             cfg,
		listener:        listener,
		clientConnected: false,
	}, nil
}

func (s *Server) Run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			slog.Error("accept error: ", "err", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	codec := protocol.NewCodec(conn)

	s.mu.Lock()
	if s.clientConnected {
		s.mu.Unlock()

		slog.Debug("client rejected", "reason", "another client is already connected")

		writeProtocolError(codec, protocol.ErrorServerBusy, "server is busy")

		return
	}
	s.clientConnected = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.clientConnected = false
		s.mu.Unlock()
	}()

	slog.Debug("client connected")

	for {
		frame, err := codec.ReadFrame()
		if err == io.EOF {
			slog.Debug("client disconnected")
			return
		}

		if errors.Is(err, protocol.ErrInvalidMagic) {
			slog.Warn("invalid protocol magic")
			writeProtocolError(codec, protocol.ErrorInvalidMagic, "invalid protocol magic")
			return
		}

		if errors.Is(err, protocol.ErrUnsupportedVersion) {
			slog.Warn("unsupported protocol version", "err", err)
			writeProtocolError(codec, protocol.ErrorUnsupportedVersion, "unsupported protocol version")
			return
		}

		if errors.Is(err, protocol.ErrPayloadTooLarge) {
			slog.Warn("payload too large", "err", err)
			writeProtocolError(codec, protocol.ErrorInvalidPayload, "payload too large")
			return
		}

		if err != nil {
			slog.Error("failed to read frame", "err", err)
			return
		}

		shouldClose := handleCommand(codec, frame)
		if shouldClose {
			return
		}
	}
}

func handleCommand(codec *protocol.Codec, frame protocol.Frame) bool {
	switch frame.Command {
	case protocol.CommandPing:
		writeProtocol(codec, protocol.CommandPong, nil)
		return false
	case protocol.CommandDownload:
		return false
	case protocol.CommandUpload:
		return false
	case protocol.CommandQuality:
		return false
	case protocol.CommandResult:
		if err := handleCommandResult(frame); err != nil {
			writeResultError(codec, err)
			return false
		}
		writeProtocol(codec, protocol.CommandOK, nil)
		return false
	case protocol.CommandQuit:
		writeProtocol(codec, protocol.CommandOK, nil)
		return true
	default:
		writeProtocolError(codec, protocol.ErrorInvalidCommand, "unknown command")
	}
	return false
}

func handleCommandResult(frame protocol.Frame) error {
	var result protocol.ResultPayload

	if err := json.Unmarshal(frame.Payload, &result); err != nil {
		return err
	}

	switch result.Type {
	case protocol.ResultLatency:
		var data protocol.LatencyResult

		if err := json.Unmarshal(result.Data, &data); err != nil {
			return err
		}

		showLatencyResult(data)

		return nil

	case protocol.ResultUpload, protocol.ResultDownload:
		var data protocol.TransferResult

		if err := json.Unmarshal(result.Data, &data); err != nil {
			return err
		}

		showTransferResult(result.Type, data)

		return nil

	case protocol.ResultQuality:
		var data protocol.QualityResult

		if err := json.Unmarshal(result.Data, &data); err != nil {
			return err
		}

		showQualityResult(data)

		return nil

	default:
		return fmt.Errorf("unknown result type")
	}
}

func writeResultError(codec *protocol.Codec, err error) {
	slog.Warn("invalid result received", "err", err)

	writeProtocolError(codec, protocol.ErrorInvalidResult, "invalid result")
}
