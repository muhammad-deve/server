package config

import "os"

var (
	ServerAddr string
	Domain     string
)

func Load() {
	ServerAddr = os.Getenv("GOPORT_SERVER_ADDR")
	if ServerAddr == "" {
		ServerAddr = "goport.uz:7000"
	}

	Domain = os.Getenv("GOPORT_DOMAIN")
	if Domain == "" {
		Domain = "goport.uz"
	}
}
