package udpdata

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Miklakapi/byteflow"
	"github.com/Miklakapi/gobitx/internal/protocol"
)

const (
	packetHeaderSize = 16
	packetSize       = 1200
	packetsPerSecond = 10000
	minProgressLeft  = 600 * time.Millisecond
)

type ProgressHandler func(protocol.QualityResult)

type receiveStats struct {
	receivedPackets int64
	receivedBytes   int64
	highestSequence uint64
	hasSequence     bool
	outOfOrder      int64

	previousSequence    uint64
	hasPreviousSequence bool

	previousDelay    time.Duration
	hasPreviousDelay bool

	totalJitter   time.Duration
	maxJitter     time.Duration
	jitterSamples int64
}

func Listen() (*net.UDPConn, int, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, 0, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, 0, err
	}

	port := conn.LocalAddr().(*net.UDPAddr).Port

	return conn, port, nil
}

func Dial(address string) (*net.UDPConn, error) {
	udpAddress, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	return net.DialUDP("udp", nil, udpAddress)
}

func SendPackets(conn *net.UDPConn, duration time.Duration) error {
	buffer := make([]byte, packetSize)

	start := time.Now()
	deadline := start.Add(duration)
	packetInterval := time.Second / time.Duration(packetsPerSecond)
	nextSendAt := start

	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	var sequence uint64

	for time.Now().Before(deadline) {
		now := time.Now()

		if sleepDuration := nextSendAt.Sub(now); sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}

		now = time.Now()
		if now.After(deadline) {
			break
		}

		binary.BigEndian.PutUint64(buffer[0:8], sequence)
		binary.BigEndian.PutUint64(buffer[8:16], uint64(now.UnixNano()))

		_, err := conn.Write(buffer)
		if err != nil {
			if isClosedConnectionError(err) {
				return nil
			}

			if isTimeoutError(err) {
				return nil
			}

			return err
		}

		sequence++
		nextSendAt = nextSendAt.Add(packetInterval)

		if nextSendAt.Before(now) {
			nextSendAt = now.Add(packetInterval)
		}
	}

	return nil
}

func ReceivePackets(conn *net.UDPConn, duration time.Duration, progressHandler ProgressHandler) (protocol.QualityResult, error) {
	buffer := make([]byte, 64*1024)

	start := time.Now()
	lastReadAt := start
	deadline := start.Add(duration)
	nextProgressAt := start.Add(time.Second)

	if err := conn.SetReadDeadline(deadline); err != nil {
		return protocol.QualityResult{}, err
	}

	var stats receiveStats

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		now := time.Now()

		if n >= packetHeaderSize {
			readPacket(buffer[:n], now, &stats)
			lastReadAt = now
		}

		if progressHandler != nil &&
			!now.Before(nextProgressAt) &&
			deadline.Sub(now) >= minProgressLeft {
			progressHandler(buildQualityResult(stats, start, lastReadAt))
			nextProgressAt = now.Add(time.Second)
		}

		if err != nil {
			if isTimeoutError(err) {
				break
			}

			if isClosedConnectionError(err) {
				break
			}

			return protocol.QualityResult{}, err
		}
	}

	return buildQualityResult(stats, start, lastReadAt), nil
}

func readPacket(packet []byte, receivedAt time.Time, stats *receiveStats) {
	sequence := binary.BigEndian.Uint64(packet[0:8])
	sentAt := int64(binary.BigEndian.Uint64(packet[8:16]))

	stats.receivedPackets++
	stats.receivedBytes += int64(len(packet))

	if !stats.hasSequence || sequence > stats.highestSequence {
		stats.highestSequence = sequence
		stats.hasSequence = true
	}

	if stats.hasPreviousSequence && sequence < stats.previousSequence {
		stats.outOfOrder++
	}

	stats.previousSequence = sequence
	stats.hasPreviousSequence = true

	delay := receivedAt.Sub(time.Unix(0, sentAt))

	if stats.hasPreviousDelay {
		jitter := delay - stats.previousDelay
		if jitter < 0 {
			jitter = -jitter
		}

		stats.totalJitter += jitter
		stats.jitterSamples++

		if jitter > stats.maxJitter {
			stats.maxJitter = jitter
		}
	}

	stats.previousDelay = delay
	stats.hasPreviousDelay = true
}

func buildQualityResult(stats receiveStats, start time.Time, lastReadAt time.Time) protocol.QualityResult {
	var sentPackets int64
	if stats.hasSequence {
		sentPackets = int64(stats.highestSequence) + 1
	}

	lostPackets := max(sentPackets-stats.receivedPackets, 0)

	var lossPercent float64
	if sentPackets > 0 {
		lossPercent = float64(lostPackets) / float64(sentPackets) * 100
	}

	var avgJitter time.Duration
	if stats.jitterSamples > 0 {
		avgJitter = stats.totalJitter / time.Duration(stats.jitterSamples)
	}

	elapsed := lastReadAt.Sub(start)
	if stats.receivedPackets == 0 {
		elapsed = time.Since(start)
	}

	var receivedRate byteflow.Rate
	if elapsed > 0 {
		receivedRate = byteflow.PerSecond(byteflow.Bytes(stats.receivedBytes), elapsed)
	}

	return protocol.QualityResult{
		SentPackets:     sentPackets,
		ReceivedPackets: stats.receivedPackets,
		LostPackets:     lostPackets,
		LossPercent:     lossPercent,
		AvgJitter:       avgJitter,
		MaxJitter:       stats.maxJitter,
		OutOfOrder:      stats.outOfOrder,
		ReceivedRate:    receivedRate,
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

	return strings.Contains(msg, "use of closed network connection")
}
