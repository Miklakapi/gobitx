package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
	"github.com/Miklakapi/gobitx/internal/protocol"
	"github.com/Miklakapi/gobitx/internal/tcpdata"
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
			slog.Error("accept error", "err", err)
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

		handleCommand(codec, frame)
	}
}

func handleCommand(codec *protocol.Codec, frame protocol.Frame) {
	switch frame.Command {
	case protocol.CommandPing:
		writeProtocol(codec, protocol.CommandPong, nil)
		return
	case protocol.CommandDownload:
		handleDownloadCommand(codec, frame)
		return
	case protocol.CommandUpload:
		handleUploadCommand(codec, frame)
		return
	case protocol.CommandQuality:
		return
	case protocol.CommandResult:
		if err := handleCommandResult(frame); err != nil {
			writeResultError(codec, err)
			return
		}
		writeProtocol(codec, protocol.CommandOK, nil)
		return
	default:
		writeProtocolError(codec, protocol.ErrorInvalidCommand, "unknown command")
	}
}

func handleDownloadCommand(codec *protocol.Codec, frame protocol.Frame) {
	var payload protocol.TransferRequest

	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		slog.Warn("invalid download payload", "err", err)
		writeProtocolError(codec, protocol.ErrorInvalidPayload, "invalid download payload")
		return
	}

	listener, port, err := tcpdata.Listen()
	if err != nil {
		slog.Warn("failed to open download data port", "err", err)
		writeProtocolError(codec, protocol.ErrorDataPortUnavailable, "download data port unavailable")
		return
	}
	defer listener.Close()

	writeProtocol(codec, protocol.CommandReady, protocol.ReadyPayload{Port: port})

	conn, err := listener.Accept()
	if err != nil {
		slog.Warn("failed to accept download data connection", "err", err)
		writeProtocolError(codec, protocol.ErrorDataConnectionFailed, "download data connection failed")
		return
	}
	defer conn.Close()

	if err := tcpdata.SendData(conn, payload.DurationNS+2*time.Second); err != nil {
		slog.Warn("download data transfer failed", "err", err)
		writeProtocolError(codec, protocol.ErrorDataTransferFailed, "download data transfer failed")
		return
	}
}

func handleUploadCommand(codec *protocol.Codec, frame protocol.Frame) {
	var payload protocol.TransferRequest

	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		slog.Warn("invalid upload payload", "err", err)
		writeProtocolError(codec, protocol.ErrorInvalidPayload, "invalid upload payload")
		return
	}

	listener, port, err := tcpdata.Listen()
	if err != nil {
		slog.Warn("failed to open upload data port", "err", err)
		writeProtocolError(codec, protocol.ErrorDataPortUnavailable, "upload data port unavailable")
		return
	}
	defer listener.Close()

	writeProtocol(codec, protocol.CommandReady, protocol.ReadyPayload{Port: port})

	conn, err := listener.Accept()
	if err != nil {
		slog.Warn("failed to accept upload data connection", "err", err)
		writeProtocolError(codec, protocol.ErrorDataConnectionFailed, "upload data connection failed")
		return
	}
	defer conn.Close()

	result, err := tcpdata.ReceiveData(conn, payload.DurationNS+2*time.Second)
	if err != nil {
		slog.Warn("upload data transfer failed", "err", err)
		writeProtocolError(codec, protocol.ErrorDataTransferFailed, "upload data transfer failed")
		return
	}

	showTransferResult(protocol.ResultUpload, result)

	if err := sendResult(codec, protocol.ResultUpload, result); err != nil {
		slog.Warn("failed to send upload result", "err", err)
		return
	}
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
