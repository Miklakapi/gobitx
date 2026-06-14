package tcpprotocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"
)

func (c TCPClient) Run() error {
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
