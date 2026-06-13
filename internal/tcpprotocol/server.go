package tcpprotocol

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

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
		writeWithErrorLog(c, "ERR server busy\n")
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

		shouldClose := handleCommands(c, command)
		if shouldClose {
			return
		}
	}
}

func handleCommands(c net.Conn, command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		writeWithErrorLog(c, "ERR empty command\n")
		return false
	}

	switch parts[0] {
	case "HELLO":
		writeWithErrorLog(c, "OK\n")
		return false

	case "PING":
		writeWithErrorLog(c, "PONG\n")
		return false

	case "TCP_DOWNLOAD":
		if len(parts) < 2 {
			writeWithErrorLog(c, "ERR invalid duration\n")
			return false
		}

		duration, err := time.ParseDuration(parts[1])
		if err != nil {
			writeWithErrorLog(c, "ERR invalid duration\n")
			return false
		}

		writeWithErrorLog(c, "READY\n")

		_, err = sendRawData(c, duration)
		if err != nil {
			slog.Error("download send failed", "err", err)
			return true
		}

		return false

	case "RESULT":
		if len(parts) < 3 {
			writeWithErrorLog(c, "ERR invalid result\n")
			return false
		}

		showResults(parts[1], parts[2])
		writeWithErrorLog(c, "OK\n")
		return false

	case "QUIT":
		writeWithErrorLog(c, "BYE\n")
		return true

	default:
		writeWithErrorLog(c, "ERR unknown command\n")
		return false
	}
}

func showResults(resultType string, data string) {
	switch resultType {
	case "tcp_latency":
		var result LatencyResult

		err := json.Unmarshal([]byte(data), &result)
		if err != nil {
			slog.Error("failed to decode latency result", "err", err)
			return
		}

		fmt.Printf(
			"TCP latency result: samples=%d min=%s avg=%s max=%s\n",
			result.Samples,
			result.Min,
			result.Avg,
			result.Max,
		)
	case "tcp_download":
		var result TCPDownloadResult

		err := json.Unmarshal([]byte(data), &result)
		if err != nil {
			slog.Error("failed to decode download result", "err", err)
			return
		}

		fmt.Printf(
			"TCP download: bytes=%d duration=%s avg=%.2f Mbps\n",
			result.Bytes,
			result.Duration,
			result.AvgMbps,
		)

	default:
		fmt.Println("Unknown result type:", resultType)
	}
}

func sendRawData(c net.Conn, duration time.Duration) (int64, error) {
	buffer := make([]byte, 64*1024)

	var totalBytes int64

	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		n, err := c.Write(buffer)
		if err != nil {
			return totalBytes, err
		}

		totalBytes += int64(n)
	}

	return totalBytes, nil
}

func writeWithErrorLog(c net.Conn, data string) {
	_, err := c.Write([]byte(data))
	if err != nil {
		slog.Error("write failed", "err", err)
	}
}
