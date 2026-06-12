package tcpprotocol

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/Miklakapi/gobitx/internal/config"
)

type TCPServer struct {
	cfg             config.Config
	listener        net.Listener
	clientConnected bool
	mu              sync.RWMutex
}

func NewTCPServer(cfg config.Config) (server *TCPServer, err error) {
	l, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		return server, err
	}

	server = &TCPServer{
		cfg:      cfg,
		listener: l,
	}

	log.Println("Server listening on", cfg.Port)

	return server, nil
}

func (s *TCPServer) Run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Server stopped")
				return
			}
			log.Println("Accept error:", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *TCPServer) Close() {
	log.Println("Stopping server...")
	s.listener.Close()
}

func (s *TCPServer) handleConnection(c net.Conn) {
	defer c.Close()

	s.mu.Lock()
	if s.clientConnected {
		s.mu.Unlock()
		log.Println("Rejected client: another client is already connected")
		writeWithErrorLog(c, "ERR server busy: another client is already connected\n")
		return
	}
	s.clientConnected = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.clientConnected = false
		s.mu.Unlock()
	}()

	log.Println("Client connected")

	reader := bufio.NewReaderSize(c, 1024)

	for {
		line, err := reader.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			log.Println("Command too long")
			return
		}

		if err == io.EOF {
			log.Println("Client disconnected")
			return
		}

		if err != nil {
			log.Println("Read error:", err)
			return
		}

		command := strings.TrimSpace(string(line))
		log.Println("Client:", command)

		handleCommands(c, command)
	}
}

func handleCommands(c net.Conn, command string) {
	switch command {
	case "PING":
		writeWithErrorLog(c, "PONG\n")
	case "QUIT":
		writeWithErrorLog(c, "BYE\n")
		return
	default:
		writeWithErrorLog(c, "ERR unknown command\n")
	}
}

func writeWithErrorLog(c net.Conn, data string) {
	_, err := c.Write([]byte(data))
	if err != nil {
		log.Println("Write error:", err)
	}
}
