//go:build windows

package tunnel

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVirtualTerminal asks the console to interpret escape codes; the legacy
// console (conhost without VT) prints them as junk otherwise. It reports
// whether escape codes can be used.
func enableVirtualTerminal() bool {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		// Not a console: a pipe or file, or a pty such as mintty (Git Bash),
		// which sets TERM and understands escape codes.
		term := os.Getenv("TERM")
		return term != "" && term != "dumb"
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
