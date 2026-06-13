package tcpprotocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
)

type LatencyResult struct {
	Samples int
	Min     time.Duration
	Avg     time.Duration
	Max     time.Duration
}

type TCPDownloadResult struct {
	Bytes     int64
	Duration  time.Duration
	AvgMbps   float64
	MinMbps   float64
	MaxMbps   float64
	Stability float64
}

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

	reader := bufio.NewReader(conn)

	ready, err := handshake(conn, reader)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("server is busy")
	}

	latencyResult, err := latencyTest(conn, reader)
	if err != nil {
		return err
	}

	fmt.Printf(
		"TCP latency: samples=%d min=%s avg=%s max=%s\n",
		latencyResult.Samples,
		latencyResult.Min,
		latencyResult.Avg,
		latencyResult.Max,
	)

	downloadResult, err := downloadTest(conn, reader, c.cfg.Duration)
	if err != nil {
		return err
	}

	fmt.Printf(
		"TCP download: bytes=%d duration=%s avg=%.2f Mbps\n",
		downloadResult.Bytes,
		downloadResult.Duration,
		downloadResult.AvgMbps,
	)

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

func latencyTest(c net.Conn, reader *bufio.Reader) (LatencyResult, error) {
	slog.Debug("latency test started")

	message := []byte("PING\n")
	measurements := make([]time.Duration, 0, 20)

	for range 20 {
		start := time.Now()

		_, err := c.Write(message)
		if err != nil {
			return LatencyResult{}, err
		}

		response, err := reader.ReadString('\n')
		if err != nil {
			return LatencyResult{}, err
		}

		latency := time.Since(start)

		if strings.TrimSpace(response) != "PONG" {
			return LatencyResult{}, fmt.Errorf("invalid response from server: %q", strings.TrimSpace(response))
		}

		measurements = append(measurements, latency)
	}

	result := calculateLatencyResult(measurements)

	err := sendResult(c, reader, "tcp_latency", result)
	if err != nil {
		return result, err
	}

	return result, nil
}

func calculateLatencyResult(measurements []time.Duration) LatencyResult {
	if len(measurements) == 0 {
		return LatencyResult{}
	}

	minLatency := measurements[0]
	maxLatency := measurements[0]
	var totalLatency time.Duration

	for _, latency := range measurements {
		if latency < minLatency {
			minLatency = latency
		}

		if latency > maxLatency {
			maxLatency = latency
		}

		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(len(measurements))

	return LatencyResult{
		Samples: len(measurements),
		Min:     minLatency,
		Avg:     avgLatency,
		Max:     maxLatency,
	}
}

func downloadTest(c net.Conn, reader *bufio.Reader, duration time.Duration) (TCPDownloadResult, error) {
	slog.Debug("download test started")

	command := fmt.Sprintf("TCP_DOWNLOAD %s\n", duration)

	_, err := c.Write([]byte(command))
	if err != nil {
		return TCPDownloadResult{}, err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return TCPDownloadResult{}, err
	}
	if strings.TrimSpace(response) != "READY" {
		return TCPDownloadResult{}, fmt.Errorf("invalid response from server: %q", strings.TrimSpace(response))
	}

	buffer := make([]byte, 64*1024)

	start := time.Now()
	deadline := start.Add(duration)

	var totalBytes int64

	for time.Now().Before(deadline) {
		n, err := reader.Read(buffer)
		if err != nil {
			return TCPDownloadResult{}, err
		}

		totalBytes += int64(n)
	}

	elapsed := time.Since(start)
	avgMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	result := TCPDownloadResult{
		Bytes:    totalBytes,
		Duration: elapsed,
		AvgMbps:  avgMbps,
	}

	err = sendResult(c, reader, "tcp_download", result)
	if err != nil {
		return result, err
	}

	return result, nil
}

func sendResult(c net.Conn, reader *bufio.Reader, testType string, result any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("RESULT %s %s\n", testType, data)

	_, err = c.Write([]byte(message))
	if err != nil {
		return err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	if strings.TrimSpace(response) != "OK" {
		return fmt.Errorf("invalid result response from server: %q", strings.TrimSpace(response))
	}

	return nil
}
