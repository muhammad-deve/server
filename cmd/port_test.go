package cmd

import "testing"

func TestParsePort(t *testing.T) {
	for in, want := range map[string]string{"1": "1", "3000": "3000", " 8080 ": "8080", "65535": "65535", "03000": "3000"} {
		if got, err := parsePort(in); err != nil || got != want {
			t.Errorf("parsePort(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"0", "65536", "-1", "abc", "", "localhost:3000", "3000/tcp"} {
		if _, err := parsePort(in); err == nil {
			t.Errorf("parsePort(%q) succeeded, want an error", in)
		}
	}
}
