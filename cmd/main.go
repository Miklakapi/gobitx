package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

type Config struct {
	Mode        string
	Destination string
	Duration    time.Duration
	Port        int
}

func main() {
	config := parseConfig()

	fmt.Println("mode:", config.Mode)
	fmt.Println("duration:", config.Duration)
	fmt.Println("port:", config.Port)
	fmt.Println("destination:", config.Destination)
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
		Port:        port,
	}
}
