package tcpdata

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Miklakapi/gobitx/internal/protocol"
)

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

func ReceiveData(conn net.Conn, duration time.Duration) (protocol.TransferResult, error) {
	buffer := make([]byte, 64*1024)

	var totalBytes int64

	start := time.Now()
	deadline := start.Add(duration)

	if err := conn.SetReadDeadline(deadline); err != nil {
		return protocol.TransferResult{}, err
	}

	for {
		n, err := conn.Read(buffer)

		if n > 0 {
			totalBytes += int64(n)
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

	return calculateTransferResult(totalBytes, elapsed), nil
}

func calculateTransferResult(bytes int64, duration time.Duration) protocol.TransferResult {
	if duration <= 0 {
		return protocol.TransferResult{
			Bytes:      bytes,
			DurationNS: 0,
		}
	}

	avgMbps := float64(bytes*8) / duration.Seconds() / 1_000_000

	return protocol.TransferResult{
		Bytes:      bytes,
		DurationNS: duration,
		AvgMbps:    avgMbps,
		MinMbps:    0,
		MaxMbps:    0,
		Stability:  0,
	}
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
