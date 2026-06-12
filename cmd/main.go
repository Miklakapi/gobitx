package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
	tcpprotocol "github.com/Miklakapi/gobitx/internal/tcpprotocl"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := config.New()
	if err != nil {
		log.Fatalln(err)
	}

	if config.Mode == "server" {
		server, err := tcpprotocol.NewTCPServer(config)
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
	runClient(config)
}

func runClient(cfg config.Config) {
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", fmt.Sprint("localhost", cfg.Port))
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("HELLO\n")); err != nil {
		log.Fatalln(err)
	}

	reader := bufio.NewReaderSize(conn, 1024)

	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Print(response)

	if _, err := conn.Write([]byte("PING\n")); err != nil {
		log.Fatalln(err)
	}

	response, err = reader.ReadString('\n')
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Print(response)
}
