//go:build !windows

package tunnel

func terminalSize() (int, int) {
	return fallbackTerminalSize()
}
