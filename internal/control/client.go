package control

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
	"github.com/Miklakapi/gobitx/internal/protocol"
)

type Client struct {
	cfg config.Config
}

func NewClient(cfg config.Config) Client {
	return Client{
		cfg: cfg,
	}
}

func (c Client) Run() error {
	var d net.Dialer

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", fmt.Sprint(c.cfg.Destination, c.cfg.Port))
	if err != nil {
		return err
	}
	defer conn.Close()

	codec := protocol.NewCodec(conn)

	if err := handshake(codec); err != nil {
		return err
	}

	latencyResult, err := latencyTest(codec)
	if err != nil {
		return err
	}

	fmt.Printf(
		"TCP latency: samples=%d min=%s avg=%s max=%s\n",
		latencyResult.Samples,
		latencyResult.MinNS,
		latencyResult.AvgNS,
		latencyResult.MaxNS,
	)

	return nil
}

func handshake(codec *protocol.Codec) error {
	slog.Debug("handshake started")

	frame, err := protocol.NewFrame(protocol.CommandPing, nil)
	if err != nil {
		return err
	}

	err = codec.WriteFrame(frame)
	if err != nil {
		return err
	}

	responseFrame, err := codec.ReadFrame()
	if err != nil {
		return err
	}

	switch responseFrame.Command {
	case protocol.CommandPong:
		return nil

	case protocol.CommandError:
		var errorPayload protocol.ErrorPayload

		err := json.Unmarshal(responseFrame.Payload, &errorPayload)
		if err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}

		return fmt.Errorf("server error: %s", errorPayload.Message)

	default:
		return fmt.Errorf("invalid handshake response: %d", responseFrame.Command)
	}
}

func latencyTest(codec *protocol.Codec) (protocol.LatencyResult, error) {
	slog.Debug("latency test started")

	const Iterations = 20

	frame, err := protocol.NewFrame(protocol.CommandPing, nil)
	if err != nil {
		return protocol.LatencyResult{}, err
	}

	measurements := make([]time.Duration, 0, Iterations)

	for range Iterations {
		start := time.Now()

		err = codec.WriteFrame(frame)
		if err != nil {
			return protocol.LatencyResult{}, err
		}

		responseFrame, err := codec.ReadFrame()
		if err != nil {
			return protocol.LatencyResult{}, err
		}

		latency := time.Since(start)

		if responseFrame.Command == protocol.CommandError {
			var errorPayload protocol.ErrorPayload

			err := json.Unmarshal(responseFrame.Payload, &errorPayload)
			if err != nil {
				return protocol.LatencyResult{}, fmt.Errorf("failed to decode error response: %w", err)
			}

			return protocol.LatencyResult{}, fmt.Errorf("server error: %s", errorPayload.Message)
		} else if responseFrame.Command != protocol.CommandPong {
			return protocol.LatencyResult{}, fmt.Errorf("invalid response from server: %q", responseFrame.Command)
		}

		measurements = append(measurements, latency)
	}

	result := calculateLatencyResult(measurements)

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
