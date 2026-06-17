package tcpdata

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Miklakapi/byteflow"
	"github.com/Miklakapi/gobitx/internal/protocol"
)

type ProgressHandler func(protocol.TransferResult)

func Listen() (net.Listener, int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, 0, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	return listener, port, nil
}

func SendData(conn net.Conn, duration time.Duration) error {
	buffer := make([]byte, 64*1024)

	deadline := time.Now().Add(duration)

	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	for time.Now().Before(deadline) {
		_, err := conn.Write(buffer)
		if err != nil {
			if isClosedConnectionError(err) {
				slog.Debug("data connection closed while sending")
				return nil
			}

			if isTimeoutError(err) {
				slog.Debug("data sending deadline reached")
				return nil
			}

			return err
		}
	}

	return nil
}

func ReceiveData(conn net.Conn, duration time.Duration, progressHandler ProgressHandler) (protocol.TransferResult, error) {
	buffer := make([]byte, 64*1024)

	var totalBytes int64
	var sampleBytes int64
	var samples []byteflow.Rate

	start := time.Now()
	sampleStart := start
	deadline := start.Add(duration)

	if err := conn.SetReadDeadline(deadline); err != nil {
		return protocol.TransferResult{}, err
	}

	for {
		n, err := conn.Read(buffer)
		now := time.Now()

		if n > 0 {
			readBytes := int64(n)

			totalBytes += readBytes
			sampleBytes += readBytes
		}

		sampleDuration := now.Sub(sampleStart)
		if sampleDuration >= time.Second {
			if sampleBytes > 0 {
				sampleRate := byteflow.PerSecond(byteflow.Bytes(sampleBytes), sampleDuration)
				samples = append(samples, sampleRate)

				if progressHandler != nil {
					progressHandler(protocol.TransferResult{
						Bytes:     byteflow.Bytes(sampleBytes),
						Duration:  sampleDuration,
						AvgRate:   sampleRate,
						MinRate:   sampleRate,
						MaxRate:   sampleRate,
						Stability: 100,
					})
				}
			}

			sampleBytes = 0
			sampleStart = now
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			if isTimeoutError(err) {
				break
			}

			if isClosedConnectionError(err) {
				break
			}

			return protocol.TransferResult{}, err
		}
	}

	elapsed := time.Since(start)

	if sampleBytes > 0 {
		sampleDuration := time.Since(sampleStart)
		if sampleDuration > 0 {
			sampleRate := byteflow.PerSecond(byteflow.Bytes(sampleBytes), sampleDuration)
			samples = append(samples, sampleRate)

			if progressHandler != nil && sampleDuration >= 600*time.Millisecond {
				progressHandler(protocol.TransferResult{
					Bytes:     byteflow.Bytes(sampleBytes),
					Duration:  sampleDuration,
					AvgRate:   sampleRate,
					MinRate:   sampleRate,
					MaxRate:   sampleRate,
					Stability: 100,
				})
			}
		}
	}

	return calculateTransferResult(totalBytes, elapsed, samples), nil
}

func calculateTransferResult(bytes int64, duration time.Duration, samples []byteflow.Rate) protocol.TransferResult {
	size := byteflow.Bytes(bytes)

	if duration <= 0 {
		return protocol.TransferResult{
			Bytes:    size,
			Duration: 0,
		}
	}

	avgRate := byteflow.PerSecond(size, duration)
	minRate, maxRate, stability := calculateRateStats(avgRate, samples)

	return protocol.TransferResult{
		Bytes:     size,
		Duration:  duration,
		AvgRate:   avgRate,
		MinRate:   minRate,
		MaxRate:   maxRate,
		Stability: stability,
	}
}

func calculateRateStats(avgRate byteflow.Rate, samples []byteflow.Rate) (byteflow.Rate, byteflow.Rate, float64) {
	if len(samples) == 0 {
		return avgRate, avgRate, 100
	}

	minRate := samples[0]
	maxRate := samples[0]

	for _, sample := range samples {
		if sample < minRate {
			minRate = sample
		}

		if sample > maxRate {
			maxRate = sample
		}
	}

	if avgRate <= 0 {
		return minRate, maxRate, 0
	}

	stability := float64(minRate) / float64(avgRate) * 100
	if stability > 100 {
		stability = 100
	}

	return minRate, maxRate, stability
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

func isClosedConnectionError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	msg := err.Error()

	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "use of closed network connection")
}
