package tcpprotocol

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
)

type TCPClient struct {
	cfg config.Config
}

func NewTCPClient(cfg config.Config) (client TCPClient) {
	client.cfg = cfg
	return client
}

func (c TCPClient) Run() error {
	var d net.Dialer

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", fmt.Sprint(c.cfg.Destination, c.cfg.Port))
	if err != nil {
		return err
	}
	defer conn.Close()

	reader := bufio.NewReaderSize(conn, 512)

	ready, err := handshake(conn, reader)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("server is busy")
	}

	err = latencyTest(conn, reader)
	if err != nil {
		return err
	}

	return nil
}

func handshake(c net.Conn, reader *bufio.Reader) (bool, error) {
	_, err := c.Write([]byte("HELLO\n"))
	if err != nil {
		return false, err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	trimmedResponse := strings.TrimSpace(response)

	switch trimmedResponse {
	case "OK":
		return true, nil
	case "ERR server busy":
		return false, nil
	default:
		return false, fmt.Errorf("invalid handshake response: %q", trimmedResponse)
	}
}

func latencyTest(c net.Conn, reader *bufio.Reader) error {
	slog.Debug("latency test started")

	message := []byte("PING\n")
	measurements := make([]time.Duration, 0, 20)

	for range 20 {
		start := time.Now()

		_, err := c.Write(message)
		if err != nil {
			return err
		}

		response, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		latency := time.Since(start)

		if strings.TrimSpace(response) != "PONG" {
			return fmt.Errorf("invalid response from server")
		}

		measurements = append(measurements, latency)
	}

	return nil
}
