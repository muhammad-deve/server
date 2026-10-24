package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

// parsePort accepts a local port number from 1 to 65535.
func parsePort(value string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid port %q: use a number from 1 to 65535", value)
	}
	return strconv.Itoa(n), nil
}
