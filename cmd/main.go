package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := config.New()
	if err != nil {
		log.Fatalln(err)
	}

	if config.Mode == "server" {
		runServer(ctx, config)
		return
	}
	runClient(config)
}

func runServer(ctx context.Context, cfg config.Config) {
	l, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	log.Println("Server listening on", cfg.Port)

	go func() {
		<-ctx.Done()
		log.Println("Stopping server...")
		l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Println("Server stopped")
				return
			default:
				log.Println("Accept error:", err)
				continue
			}
		}

		go func(c net.Conn) {
			defer c.Close()

			log.Println("client connected")

			reader := bufio.NewReaderSize(c, 1024)

			for {
				line, err := reader.ReadSlice('\n')
				if err == bufio.ErrBufferFull {
					log.Println("command too long")
					return
				}

				if err == io.EOF {
					log.Println("client disconnected")
					return
				}

				if err != nil {
					log.Println("read error:", err)
					return
				}

				command := strings.TrimSpace(string(line))

				log.Println("client:", command)
				switch command {
				case "HELLO":
					_, err = c.Write([]byte("OK\n"))
				case "PING":
					_, err = c.Write([]byte("PONG\n"))
				case "QUIT":
					_, err = c.Write([]byte("BYE\n"))
					return
				default:
					_, err = c.Write([]byte("ERR unknown command\n"))
				}

				if err != nil {
					log.Println("write error:", err)
					return
				}
			}
		}(conn)
	}
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
