package control

import (
	"errors"
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

		errorFrame, err := protocol.NewFrame(protocol.CommandError, protocol.ErrorPayload{
			Code:    protocol.ErrorServerBusy,
			Message: "server is busy",
		})
		if err != nil {
			slog.Error("failed to build error frame", "err", err)
			return
		}

		err = codec.WriteFrame(errorFrame)
		if err != nil {
			slog.Error("failed to write error frame", "err", err)
			return
		}

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
}
