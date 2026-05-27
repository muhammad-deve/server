package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	printDashboard(cfg, resp, time.Since(start))

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

func printDashboard(cfg Config, resp registrationResponse, latency time.Duration) {
	domain := "goport.uz"
	if config.Domain != "" && config.Domain != "goport" {
		domain = config.Domain
	}

	fmt.Printf("You can find it in %s\n\n", domain)
	fmt.Printf("%-20s %s\n", "Dashboard", "http://127.0.0.1:4040")
	fmt.Printf("%-20s %s\n", "Region", regionLabel(cfg.Region))
	fmt.Printf("%-20s online (%dms)\n", "Status", latency.Milliseconds())
	fmt.Printf("%-20s %s -> goport:%s\n\n", "Forwarding", resp.URL, cfg.Port)

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
