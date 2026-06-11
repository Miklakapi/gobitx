package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

type Config struct {
	Mode        string
	Destination string
	Duration    time.Duration
	Port        string
}

func main() {
	config := parseConfig()

	if config.Mode == "server" {
		runServer(config)
		return
	}
	runClient(config)
}

func parseConfig() Config {
	if len(os.Args) < 2 {
		log.Fatalln("Missing command: use 'server' or 'client'")
	}

	mode := os.Args[1]
	if mode != "server" && mode != "client" {
		log.Fatalln("Invalid command: use 'server' or 'client'")
	}

	flags := flag.NewFlagSet(mode, flag.ExitOnError)

	var duration time.Duration
	var port int

	flags.DurationVar(&duration, "duration", 10*time.Second, "Test duration, for example 10s, 30s or 1m")
	flags.IntVar(&port, "port", 5200, "TCP port to listen on or connect to")

	err := flags.Parse(os.Args[2:])
	if err != nil {
		log.Fatalln(err)
	}

	destination := ""

	if mode == "client" {
		if flags.NArg() < 1 {
			log.Fatalln("Missing destination address: use 'gobitx client <host>'")
		}

		destination = flags.Arg(0)
	}

	return Config{
		Mode:        mode,
		Destination: destination,
		Duration:    duration,
		Port:        fmt.Sprint(":", port),
	}
}

func runServer(cfg Config) {
	l, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalln(err)
		}

		go func(c net.Conn) {
			defer c.Close()

			reader := io.TeeReader(c, os.Stdout)

			_, err := io.Copy(c, reader)
			if err != nil {
				log.Println("Connection error: ", err)
			}
		}(conn)
	}
}

func runClient(cfg Config) {
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", fmt.Sprint("localhost", cfg.Port))
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("Test 123 312 Test")); err != nil {
		log.Fatalln(err)
	}
}
