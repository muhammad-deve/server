package tunnel

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/muhammad-deve/server/cmd/config"
)

type Config struct {
	Port      string
	Subdomain string
	Region    string
	Type      string
}

func Start(cfg Config) {
	fmt.Printf("connecting to goport server...\n")

	conn, err := net.Dial("tcp", config.ServerAddr)
	if err != nil {
		fmt.Println("error connecting to server:", err)
		return
	}
	defer conn.Close()

	fmt.Println("connected to server successfully:", config.ServerAddr)
	fmt.Println("tunnel is running. Press Ctrl+C to stop.")

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := conn.Read(buf)
		done <- err
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		fmt.Println("\nstopping tunnel")
	case err := <-done:
		fmt.Println("server closed connection:", err)
	}
}
