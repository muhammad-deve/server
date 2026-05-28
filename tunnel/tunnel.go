package tunnel

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiGray  = "\x1b[90m"
	ansiGreen = "\x1b[38;2;48;242;148m"
	ansiWhite = "\x1b[97m"
)

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

	req, err := http.ReadRequest(bufio.NewReader(stream))
	if err != nil {
		fmt.Fprintf(stream, "HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nfailed to read request: %v", err)
		return
	}
	defer req.Body.Close()

	localConn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		logRequest(req.Method, req.URL.RequestURI(), http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		fmt.Fprintf(stream, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nfailed to connect to localhost:%s: %v", port, err)
		return
	}
	defer localConn.Close()

	if err := req.Write(localConn); err != nil {
		logRequest(req.Method, req.URL.RequestURI(), http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		fmt.Fprintf(stream, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nfailed to forward request: %v", err)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(localConn), req)
	if err != nil {
		logRequest(req.Method, req.URL.RequestURI(), http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		fmt.Fprintf(stream, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nfailed to read local response: %v", err)
		return
	}
	defer resp.Body.Close()

	logRequest(req.Method, req.URL.RequestURI(), resp.StatusCode, http.StatusText(resp.StatusCode))
	if err := resp.Write(stream); err != nil {
		fmt.Println("error writing tunnel response:", err)
	}
}

func logRequest(method, path string, statusCode int, statusText string) {
	if statusText == "" {
		statusText = "status"
	}
	statusColor := ansiGray
	if statusCode >= 200 && statusCode < 300 {
		statusColor = ansiGreen
	}

	fmt.Printf(
		"%s%-10s %-7s%s %s%-32s%s %s%d %s%s\n",
		ansiGray,
		time.Now().Format("15:04:05"),
		method,
		ansiReset,
		ansiBold+ansiWhite,
		path,
		ansiReset,
		statusColor,
		statusCode,
		statusText,
		ansiReset,
	)
}

func printDashboard(cfg Config, resp registrationResponse, latency time.Duration) {
	fmt.Printf("\n%s$%s %sgoport %s %s%s\n", ansiGray, ansiReset, ansiBold+ansiWhite, cfg.Type, cfg.Port, ansiReset)
	fmt.Println()
	fmt.Printf("%s%-16s%s %s%s%s\n", ansiGray, "Dashboard", ansiReset, ansiBold+ansiWhite, "http://127.0.0.1:4040", ansiReset)
	fmt.Printf("%s%-16s%s %s%s%s\n", ansiGray, "Region", ansiReset, ansiBold+ansiWhite, regionLabel(cfg.Region), ansiReset)
	fmt.Printf("%s%-16s%s %sonline%s %s(%dms)%s\n", ansiGray, "Status", ansiReset, ansiGreen, ansiReset, ansiGray, latency.Milliseconds(), ansiReset)
	fmt.Printf("%s%-16s%s %s%s%s %s→%s %s%slocalhost:%s%s\n", ansiGray, "Forwarding", ansiReset, ansiGreen, resp.URL, ansiReset, ansiGray, ansiReset, ansiBold, ansiWhite, cfg.Port, ansiReset)
	fmt.Println()

	if cfg.Type == "http" {
		fmt.Printf("%s%sHTTP Requests%s\n", ansiBold, ansiWhite, ansiReset)
		fmt.Printf("%s-------------%s\n", ansiGray, ansiReset)
		fmt.Println()
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
