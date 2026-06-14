package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Miklakapi/gobitx/internal/config"
	"github.com/Miklakapi/gobitx/internal/control"
	"github.com/Miklakapi/gobitx/internal/logger"
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

	if cfg.Mode == "server" {
		server, err := control.NewServer(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		go func() {
			<-ctx.Done()
			server.Close()
		}()

		fmt.Println("Application started on port:", cfg.Port)
		server.Run()
		fmt.Println("Application stopped")

		return
	}

	client := control.NewClient(ctx, cfg)

	err = client.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
