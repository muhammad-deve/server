//go:build !windows

package tunnel

// enableVirtualTerminal: Unix terminals understand escape codes already.
func enableVirtualTerminal() bool { return true }
