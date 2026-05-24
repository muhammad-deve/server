package tunnel

import (
	"fmt"
	"net"
)

type Config struct {
	Port      string
	Subdomain string
	Region    string
	Type      string
}

func Start(cfg Config) {
	fmt.Printf("connecting to goport server...\n")

	conn, err := net.Dial("tcp", "localhost:7000")
	if err != nil {
		fmt.Println("error connecting to server:", err)
		return
	}
	defer conn.Close()

	fmt.Println("connected to server successfully")
}
