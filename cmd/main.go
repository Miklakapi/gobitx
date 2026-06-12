package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Miklakapi/gobitx/internal/config"
	tcpprotocol "github.com/Miklakapi/gobitx/internal/tcpprotocl"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		log.Fatalln(err)
	}

	handleTCP(ctx, cfg)
}

func handleTCP(ctx context.Context, cfg config.Config) {
	if cfg.Mode == "server" {
		server, err := tcpprotocol.NewTCPServer(cfg)
		if err != nil {
			log.Fatalln(err)
			return
		}

		go func() {
			<-ctx.Done()
			server.Close()
		}()
		server.Run()
		return
	}

	client := tcpprotocol.NewTCPClient(cfg)

	client.Run()
}
