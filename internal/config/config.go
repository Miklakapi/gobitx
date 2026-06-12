package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Mode        string
	Protocol    string
	Destination string
	Duration    time.Duration
	Port        string
}

func New() (Config, error) {
	var cfg Config

	if len(os.Args) < 2 {
		return cfg, fmt.Errorf("Missing command: use 'server' or 'client'")
	}

	if os.Args[1] != "server" && os.Args[1] != "client" {
		return cfg, fmt.Errorf("Invalid command: use 'server' or 'user'")
	}

	cfg.Mode = os.Args[1]

	flags := flag.NewFlagSet(cfg.Mode, flag.ExitOnError)

	var port int
	var protocol string

	flags.DurationVar(&cfg.Duration, "duration", 10*time.Second, "Test duration, for example 10s, 30s or 1m")
	flags.IntVar(&port, "port", 5200, "Port to listen on or connect to")
	flags.StringVar(&protocol, "protocol", "both", "Protocol to user: tcp, udp or both")

	cfg.Port = fmt.Sprint(":", port)

	if protocol != "udp" && protocol != "tcp" && protocol != "both" {
		return cfg, fmt.Errorf("Invalid protocol: use 'tcp', 'udp' or 'both'")
	}

	cfg.Protocol = protocol

	err := flags.Parse(os.Args[2:])
	if err != nil {
		return cfg, err
	}

	if cfg.Mode == "client" {
		if flags.NArg() < 1 {
			return cfg, fmt.Errorf("Missing destination address: use 'gotbitx client <address>")
		}

		cfg.Destination = flags.Arg(0)
	}

	return cfg, nil
}
