package tcpprotocol

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/Miklakapi/gobitx/internal/config"
)

type TCPServer struct {
	cfg             config.Config
	listener        net.Listener
	clientConnected bool
	mu              sync.RWMutex
}

func NewTCPServer(cfg config.Config) (server *TCPServer, err error) {
	l, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		return server, err
	}

	server = &TCPServer{
		cfg:      cfg,
		listener: l,
	}

	fmt.Println("Server listening on", cfg.Port)

	return server, nil
}

func (s *TCPServer) Run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				fmt.Println("Server stopped")
				return
			}
			slog.Error("accept error:", "err", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *TCPServer) Close() {
	s.listener.Close()
}

func (s *TCPServer) handleConnection(c net.Conn) {
	defer c.Close()

	s.mu.Lock()
	if s.clientConnected {
		s.mu.Unlock()
		slog.Debug("client rejected", "reason", "another client is already connected")
		writeWithErrorLog(c, "ERR server busy: another client is already connected\n")
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

	reader := bufio.NewReaderSize(c, 1024)

	for {
		line, err := reader.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			slog.Warn("command too long")
			return
		}

		if err == io.EOF {
			slog.Debug("client disconnected")
			return
		}

		if err != nil {
			slog.Error("read error:", "err", err)
			return
		}

		command := strings.TrimSpace(string(line))
		slog.Debug("command triggered", "command", command)
		handleCommands(c, command)
	}
}

func handleCommands(c net.Conn, command string) {
	switch command {
	case "HELLO":
		writeWithErrorLog(c, "OK\n")
	case "PING":
		writeWithErrorLog(c, "PONG\n")
	default:
		writeWithErrorLog(c, "ERR unknown command\n")
	}
}

func writeWithErrorLog(c net.Conn, data string) {
	_, err := c.Write([]byte(data))
	if err != nil {
		slog.Error("write failed", "err", err)
	}
}
