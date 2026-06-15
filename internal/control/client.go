package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
	"github.com/Miklakapi/gobitx/internal/protocol"
)

type Client struct {
	cfg config.Config
	ctx context.Context
}

func NewClient(ctx context.Context, cfg config.Config) Client {
	return Client{
		cfg: cfg,
		ctx: ctx,
	}
}

func (c Client) Run() error {
	var d net.Dialer

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := d.DialContext(timeoutCtx, "tcp", fmt.Sprint(c.cfg.Destination, c.cfg.Port))
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-c.ctx.Done()
		conn.Close()
	}()

	codec := protocol.NewCodec(conn)

	if err := handshake(codec); err != nil {
		return c.friendlyClientError(err)
	}

	latencyResult, err := latencyTest(codec)
	if err != nil {
		return c.friendlyClientError(err)
	}

	showLatencyResult(latencyResult)
	// TODO: Download test
	// TODO: Show results

	// TODO: Upload test
	// TODO: Show results

	return nil
}

func handshake(codec *protocol.Codec) error {
	slog.Debug("handshake started")

	_, err := requestFrame(codec, protocol.CommandPing, nil, protocol.CommandPong)
	if err != nil {
		return err
	}

	return nil
}

func latencyTest(codec *protocol.Codec) (protocol.LatencyResult, error) {
	slog.Debug("latency test started")

	const Iterations = 20

	measurements := make([]time.Duration, 0, Iterations)

	for range Iterations {
		start := time.Now()

		_, err := requestFrame(codec, protocol.CommandPing, nil, protocol.CommandPong)
		if err != nil {
			return protocol.LatencyResult{}, err
		}

		latency := time.Since(start)

		measurements = append(measurements, latency)
	}

	result := calculateLatencyResult(measurements)

	err := sendResult(codec, protocol.ResultLatency, result)
	if err != nil {
		return protocol.LatencyResult{}, err
	}

	return result, nil
}

func calculateLatencyResult(measurements []time.Duration) protocol.LatencyResult {
	if len(measurements) == 0 {
		return protocol.LatencyResult{}
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

	return protocol.LatencyResult{
		Samples: len(measurements),
		MinNS:   minLatency,
		AvgNS:   avgLatency,
		MaxNS:   maxLatency,
	}
}

func (c Client) friendlyClientError(err error) error {
	if err == nil {
		return nil
	}

	if c.ctx.Err() != nil {
		return nil
	}

	if errors.Is(err, io.EOF) {
		return fmt.Errorf("server closed connection")
	}

	if errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("connection closed")
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("connection timed out")
	}

	return err
}
