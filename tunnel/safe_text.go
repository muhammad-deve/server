package tunnel

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// TerminalSafe escapes control characters so text from the network (server
// replies, visitor paths) can't move the cursor, recolor the terminal or set
// its title when printed. Bytes below 0x20, DEL, the C1 range U+0080-U+009F
// and invalid UTF-8 are shown as \xNN or \uNNNN.
func TerminalSafe(s string) string {
	if !needsEscape(s) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r >= 0x80 && r <= 0x9f:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

func needsEscape(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
		i += size
	}
	return false
}
