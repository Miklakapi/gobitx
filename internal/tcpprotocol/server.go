package tcpprotocol

import (
	"net"
	"time"
)

func SendRawData(c net.Conn, duration time.Duration) (int64, error) {
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
