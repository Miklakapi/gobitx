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
	"github.com/Miklakapi/gobitx/internal/udpdata"
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
		handleQualityCommand(codec, frame)
		return
	case protocol.CommandResult:
		if err := handleCommandResult(frame); err != nil {
			writeResultError(codec, err)
			return
		}
		writeProtocol(codec, protocol.CommandOK, nil)
		return
	default:
		slog.Debug("unknown command", "command", frame.Command)
		writeProtocolError(codec, protocol.ErrorInvalidCommand, "unknown command")
	}
}

func handleDownloadCommand(codec *protocol.Codec, frame protocol.Frame) {
	slog.Debug("download test started")

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

	sendErrCh := make(chan error, 1)

	go func() {
		err := tcpdata.SendData(conn, payload.DurationNS+2*time.Second)
		closeErr := conn.Close()

		if err != nil {
			sendErrCh <- err
			return
		}

		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			sendErrCh <- closeErr
			return
		}

		sendErrCh <- nil
	}()

	for {
		responseFrame, err := codec.ReadFrame()
		if err != nil {
			slog.Warn("failed to read download progress/result", "err", err)
			return
		}

		if err := decodeErrorFrame(responseFrame); err != nil {
			slog.Warn("download failed", "err", err)
			return
		}

		switch responseFrame.Command {
		case protocol.CommandProgress:
			var progressPayload protocol.ResultPayload

			if err := json.Unmarshal(responseFrame.Payload, &progressPayload); err != nil {
				writeProtocolError(codec, protocol.ErrorInvalidPayload, "invalid download progress payload")
				return
			}

			if progressPayload.Type != protocol.ResultDownload {
				writeProtocolError(codec, protocol.ErrorInvalidResult, "invalid download progress type")
				return
			}

			var progress protocol.TransferResult
			if err := json.Unmarshal(progressPayload.Data, &progress); err != nil {
				writeProtocolError(codec, protocol.ErrorInvalidPayload, "invalid download progress data")
				return
			}

			showTransferResult(protocol.ResultDownload, progress)

		case protocol.CommandResult:
			if err := handleCommandResult(responseFrame); err != nil {
				writeResultError(codec, err)
				return
			}

			if err := writeProtocolFrame(codec, protocol.CommandOK, nil); err != nil {
				slog.Warn("failed to write download OK", "err", err)
				return
			}

			if err := <-sendErrCh; err != nil {
				slog.Warn("download data transfer failed", "err", err)
				writeProtocolError(codec, protocol.ErrorDataTransferFailed, "download data transfer failed")
				return
			}

			slog.Debug("download test completed")
			return

		default:
			writeProtocolError(codec, protocol.ErrorInvalidCommand, "unexpected command during download")
			return
		}
	}
}

func handleUploadCommand(codec *protocol.Codec, frame protocol.Frame) {
	slog.Debug("upload test started")

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

	result, err := tcpdata.ReceiveData(conn, payload.DurationNS+2*time.Second, func(uploadResult protocol.TransferResult) {
		showTransferResult(protocol.ResultUpload, uploadResult)

		if err := sendProgress(codec, protocol.ResultUpload, uploadResult); err != nil {
			slog.Warn("failed to send upload progress", "err", err)
			return
		}
	})

	if err != nil {
		slog.Warn("upload data transfer failed", "err", err)
		writeProtocolError(codec, protocol.ErrorDataTransferFailed, "upload data transfer failed")
		return
	}

	showTransferResult(protocol.ResultUpload, result)
	fmt.Println()

	slog.Debug("upload test completed")

	if err := sendResult(codec, protocol.ResultUpload, result); err != nil {
		slog.Warn("failed to send upload result", "err", err)
		return
	}
}

func handleQualityCommand(codec *protocol.Codec, frame protocol.Frame) {
	slog.Debug("quality test started")

	var payload protocol.TransferRequest

	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		slog.Warn("invalid quality payload", "err", err)
		writeProtocolError(codec, protocol.ErrorInvalidPayload, "invalid quality payload")
		return
	}

	conn, port, err := udpdata.Listen()
	if err != nil {
		slog.Warn("failed to open quality data port", "err", err)
		writeProtocolError(codec, protocol.ErrorDataPortUnavailable, "quality data port unavailable")
		return
	}
	defer conn.Close()

	writeProtocol(codec, protocol.CommandReady, protocol.ReadyPayload{Port: port})

	result, err := udpdata.ReceivePackets(conn, payload.DurationNS+2*time.Second, func(qualityResult protocol.QualityResult) {
		showQualityResult(qualityResult)

		if err := sendProgress(codec, protocol.ResultQuality, qualityResult); err != nil {
			slog.Warn("failed to send quality progress", "err", err)
			return
		}
	})
	if err != nil {
		slog.Warn("quality data transfer failed", "err", err)
		writeProtocolError(codec, protocol.ErrorDataTransferFailed, "quality data transfer failed")
		return
	}

	showQualityResult(result)
	fmt.Println()

	slog.Debug("quality test completed")

	if err := sendResult(codec, protocol.ResultQuality, result); err != nil {
		slog.Warn("failed to send quality result", "err", err)
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

		fmt.Println()
		showLatencyResult(data)
		fmt.Println()

		return nil

	case protocol.ResultUpload, protocol.ResultDownload:
		var data protocol.TransferResult

		if err := json.Unmarshal(result.Data, &data); err != nil {
			return err
		}

		showTransferResult(result.Type, data)
		fmt.Println()

		return nil

	case protocol.ResultQuality:
		var data protocol.QualityResult

		if err := json.Unmarshal(result.Data, &data); err != nil {
			return err
		}

		showQualityResult(data)
		fmt.Println()

		return nil

	default:
		return fmt.Errorf("unknown result type")
	}
}

func writeResultError(codec *protocol.Codec, err error) {
	slog.Warn("invalid result received", "err", err)

	writeProtocolError(codec, protocol.ErrorInvalidResult, "invalid result")
}
