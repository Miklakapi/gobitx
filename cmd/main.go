package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Miklakapi/gobitx/internal/config"
	"github.com/Miklakapi/gobitx/internal/logger"
	"github.com/Miklakapi/gobitx/internal/tcpprotocol"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger.SetupLogger(cfg.Verbose)

	if cfg.Protocol == "tcp" {
		err := handleTCP(ctx, cfg)
		if err != nil {
			slog.Error("fatal error", "err", err)
			os.Exit(1)
		}
	}
}

func handleTCP(ctx context.Context, cfg config.Config) error {
	if cfg.Mode == "server" {
		server, err := tcpprotocol.NewTCPServer(cfg)
		if err != nil {
			return err
		}

		go func() {
			<-ctx.Done()
			server.Close()
		}()

		server.Run()

		return nil
	}

	client := tcpprotocol.NewTCPClient(cfg)

	return client.Run()
}
