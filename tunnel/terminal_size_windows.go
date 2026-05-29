//go:build windows

package tunnel

import (
	"syscall"
	"unsafe"
)

type consoleCoord struct {
	x int16
	y int16
}

type consoleSmallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              consoleCoord
	cursorPosition    consoleCoord
	attributes        uint16
	window            consoleSmallRect
	maximumWindowSize consoleCoord
}

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle               = kernel32.NewProc("GetStdHandle")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

func terminalSize() (int, int) {
	const stdOutputHandle = ^uintptr(10)

	handle, _, _ := procGetStdHandle.Call(stdOutputHandle)
	if handle == 0 || handle == uintptr(syscall.InvalidHandle) {
		return fallbackTerminalSize()
	}

	var info consoleScreenBufferInfo
	ok, _, _ := procGetConsoleScreenBufferInfo.Call(handle, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return fallbackTerminalSize()
	}

	width := int(info.window.right - info.window.left + 1)
	height := int(info.window.bottom - info.window.top + 1)
	if width <= 0 || height <= 0 {
		return fallbackTerminalSize()
	}
	return width, height
}
