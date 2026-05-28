package tunnel

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/muhammad-deve/server/cmd/config"
)

type Config struct {
	Port      string
	Subdomain string
	Region    string
	Type      string
}

type registrationRequest struct {
	Type string `json:"type"`
	Port string `json:"port"`
}

type registrationResponse struct {
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
}

func Start(cfg Config) {
	start := time.Now()
	conn, err := net.Dial("tcp", config.ServerAddr)
	if err != nil {
		fmt.Println("error connecting to server:", err)
		return
	}
	defer conn.Close()

	req := registrationRequest{
		Type: cfg.Type,
		Port: cfg.Port,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		fmt.Println("error sending tunnel request:", err)
		return
	}

	var resp registrationResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		fmt.Println("error reading tunnel response:", err)
		return
	}
	if resp.Error != "" {
		fmt.Println("server error:", resp.Error)
		return
	}
	if resp.URL == "" {
		fmt.Println("server error: empty tunnel url")
		return
	}

	printDashboard(cfg, resp, time.Since(start))

	session, err := yamux.Client(conn, nil)
	if err != nil {
		fmt.Println("error starting tunnel session:", err)
		return
	}
	defer session.Close()

	done := make(chan error, 1)
	go func() {
		for {
			stream, err := session.Accept()
			if err != nil {
				done <- err
				return
			}
			go handleStream(stream, cfg.Port)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		fmt.Println("\nstopping tunnel")
		session.Close()
	case err := <-done:
		fmt.Println("server closed connection:", err)
	}
}

func handleStream(stream net.Conn, port string) {
	defer stream.Close()

	localConn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		fmt.Fprintf(stream, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nfailed to connect to localhost:%s: %v", port, err)
		return
	}
	defer localConn.Close()

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(localConn, stream)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(stream, localConn)
		errCh <- err
	}()
	<-errCh
}

func printDashboard(cfg Config, resp registrationResponse, latency time.Duration) {
	domain := "goport.uz"
	if config.Domain != "" && config.Domain != "goport" {
		domain = config.Domain
	}

	fmt.Printf("You can find it in %s\n\n", domain)
	fmt.Printf("%-20s %s\n", "Dashboard", "http://127.0.0.1:4040")
	fmt.Printf("%-20s %s\n", "Region", regionLabel(cfg.Region))
	fmt.Printf("%-20s online (%dms)\n", "Status", latency.Milliseconds())
	fmt.Printf("%-20s %s -> localhost:%s\n\n", "Forwarding", resp.URL, cfg.Port)

	if cfg.Type == "http" {
		fmt.Println("HTTP Requests")
		fmt.Println("-------------")
	}
}

func regionLabel(region string) string {
	switch region {
	case "eu":
		return "Europe (eu)"
	case "":
		return "Europe (eu)"
	default:
		return region
	}
}
