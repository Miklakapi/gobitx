package tcpprotocol

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Miklakapi/gobitx/internal/config"
)

type TCPClient struct {
	cfg config.Config
}

func NewTCPClient(cfg config.Config) (client TCPClient) {
	client.cfg = cfg
	return client
}

func (c TCPClient) Run() error {
	var d net.Dialer

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", fmt.Sprint(c.cfg.Destination, c.cfg.Port))
	if err != nil {
		return err
	}
	defer conn.Close()

	reader := bufio.NewReaderSize(conn, 1024)

	writeWithErrorLog(conn, "PING\n")

	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	fmt.Print(response)

	return nil
}
