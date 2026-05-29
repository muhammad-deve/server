package tunnel

import (
	"os"
	"strconv"
)

const (
	fallbackTerminalWidth  = 80
	fallbackTerminalHeight = 24
)

func fallbackTerminalSize() (int, int) {
	width := envInt("COLUMNS")
	height := envInt("LINES")
	if width > 0 && height > 0 {
		return width, height
	}
	return fallbackTerminalWidth, fallbackTerminalHeight
}

func envInt(name string) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return 0
	}
	return value
}
