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
	"github.com/Miklakapi/gobitx/internal/tcpdata"
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

	slog.Debug("connection established")

	go func() {
		<-c.ctx.Done()
		conn.Close()
	}()

	codec := protocol.NewCodec(conn)

	if err := c.handshake(codec); err != nil {
		return c.friendlyClientError(err)
	}

	latencyResult, err := c.latencyTest(codec)
	if err != nil {
		return c.friendlyClientError(err)
	}

	fmt.Println()
	showLatencyResult(latencyResult)
	fmt.Println()

	downloadResult, err := c.downloadTest(codec)
	if err != nil {
		return c.friendlyClientError(err)
	}

	showTransferResult(protocol.ResultDownload, downloadResult)
	fmt.Println()

	uploadResult, err := c.uploadTest(codec)
	if err != nil {
		return c.friendlyClientError(err)
	}
	showTransferResult(protocol.ResultUpload, uploadResult)
	fmt.Println()

	return nil
}

func (c Client) handshake(codec *protocol.Codec) error {
	slog.Debug("handshake started")

	_, err := requestFrame(codec, protocol.CommandPing, nil, protocol.CommandPong)
	if err != nil {
		return err
	}

	slog.Debug("handshake completed")
	return nil
}

func (c Client) latencyTest(codec *protocol.Codec) (protocol.LatencyResult, error) {
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

	slog.Debug("latency test completed")
	return result, nil
}

func (c Client) downloadTest(codec *protocol.Codec) (protocol.TransferResult, error) {
	slog.Debug("download test started")

	frame, err := requestFrame(
		codec,
		protocol.CommandDownload,
		protocol.TransferRequest{DurationNS: c.cfg.Duration},
		protocol.CommandReady,
	)
	if err != nil {
		return protocol.TransferResult{}, err
	}

	payload, err := decodeReadyPayload(frame)
	if err != nil {
		return protocol.TransferResult{}, err
	}

	conn, err := c.dialDataConnection(payload.Port)
	if err != nil {
		return protocol.TransferResult{}, err
	}

	defer conn.Close()

	cancelDataCtx := c.closeDataConnectionOnContext(conn)
	defer cancelDataCtx()

	slog.Debug("data receiving started")
	result, err := tcpdata.ReceiveData(conn, c.cfg.Duration, func(downloadResult protocol.TransferResult) {
		showTransferResult(protocol.ResultDownload, downloadResult)

		if err := sendProgress(codec, protocol.ResultDownload, downloadResult); err != nil {
			slog.Warn("failed to send download progress", "err", err)
			return
		}
	})
	if err != nil {
		return protocol.TransferResult{}, err
	}
	slog.Debug("data receiving completed")

	if err := conn.Close(); err != nil {
		return protocol.TransferResult{}, err
	}

	if err := sendResult(codec, protocol.ResultDownload, result); err != nil {
		return protocol.TransferResult{}, err
	}

	slog.Debug("download test completed")

	return result, nil
}

func (c Client) uploadTest(codec *protocol.Codec) (protocol.TransferResult, error) {
	slog.Debug("upload test started")

	frame, err := requestFrame(
		codec,
		protocol.CommandUpload,
		protocol.TransferRequest{DurationNS: c.cfg.Duration},
		protocol.CommandReady,
	)
	if err != nil {
		return protocol.TransferResult{}, err
	}

	payload, err := decodeReadyPayload(frame)
	if err != nil {
		return protocol.TransferResult{}, err
	}

	conn, err := c.dialDataConnection(payload.Port)
	if err != nil {
		return protocol.TransferResult{}, err
	}

	defer conn.Close()

	cancelDataCtx := c.closeDataConnectionOnContext(conn)
	defer cancelDataCtx()

	slog.Debug("data sending started")

	sendErrCh := make(chan error, 1)

	go func() {
		err := tcpdata.SendData(conn, c.cfg.Duration)
		closeErr := conn.Close()

		if err != nil {
			sendErrCh <- err
			return
		}

		if closeErr != nil {
			sendErrCh <- closeErr
			return
		}

		sendErrCh <- nil
	}()

	for {
		responseFrame, err := codec.ReadFrame()
		if err != nil {
			return protocol.TransferResult{}, err
		}

		if err := decodeErrorFrame(responseFrame); err != nil {
			return protocol.TransferResult{}, err
		}

		switch responseFrame.Command {
		case protocol.CommandProgress:
			progress, err := decodeTransferResultPayload(responseFrame, protocol.ResultUpload)
			if err != nil {
				return protocol.TransferResult{}, err
			}

			showTransferResult(protocol.ResultUpload, progress)

		case protocol.CommandResult:
			result, err := decodeTransferResultPayload(responseFrame, protocol.ResultUpload)
			if err != nil {
				return protocol.TransferResult{}, err
			}

			if err := writeProtocolFrame(codec, protocol.CommandOK, nil); err != nil {
				return protocol.TransferResult{}, err
			}

			if err := <-sendErrCh; err != nil {
				return protocol.TransferResult{}, err
			}

			slog.Debug("data sending completed")
			slog.Debug("upload test completed")

			return result, nil

		default:
			return protocol.TransferResult{}, fmt.Errorf("unexpected response: got %d, expected progress or result", responseFrame.Command)
		}
	}
}

func (c Client) dialDataConnection(port int) (net.Conn, error) {
	var d net.Dialer

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return d.DialContext(timeoutCtx, "tcp", fmt.Sprint(c.cfg.Destination, ":", port))
}

func (c Client) closeDataConnectionOnContext(conn net.Conn) context.CancelFunc {
	dataCtx, cancel := context.WithCancel(c.ctx)

	go func() {
		<-dataCtx.Done()
		conn.Close()
	}()

	return cancel
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
		Min:     minLatency,
		Avg:     avgLatency,
		Max:     maxLatency,
	}
}
