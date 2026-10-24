//go:build windows

package share

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func shortName(t *testing.T, path string) string {
	t.Helper()
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Fatalf("GetShortPathName(%s): %v", path, err)
	}
	return filepath.Base(windows.UTF16ToString(buf[:n]))
}

// On NTFS with 8.3 names, .env is also reachable as ENV~1 and .git as GIT~1.
// Run with TMP on a volume that has 8.3 names (usually C:) to exercise it.
func TestWindowsShortNamesCannotReachBlockedFiles(t *testing.T) {
	handler, root := newTestHandler(t, "", time.Time{})
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[remote]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "id_ed25519"), []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	envShort := shortName(t, filepath.Join(root, ".env"))
	if strings.EqualFold(envShort, ".env") {
		t.Skip("8.3 short names are disabled on this volume")
	}
	gitShort := shortName(t, filepath.Join(root, ".git"))
	keyShort := shortName(t, filepath.Join(root, "id_ed25519"))
	t.Logf("short names: %s %s %s", envShort, gitShort, keyShort)

	targets := []string{
		"/file?path=" + url.QueryEscape(envShort),
		"/file?path=" + url.QueryEscape(keyShort),
		"/file?path=" + url.QueryEscape(gitShort+"/config"),
		"/api/list?path=" + url.QueryEscape(gitShort),
		"/zip?path=" + url.QueryEscape(gitShort),
	}
	for _, target := range targets {
		if response := get(handler, "/s/"+testToken+target); response.Code == http.StatusOK {
			t.Errorf("%s returned 200: %q", target, response.Body.String())
		}
	}
}

// Windows drops trailing dots and spaces, so "id_rsa." opens id_rsa.
func TestWindowsTrailingDotsCannotReachBlockedFiles(t *testing.T) {
	handler, root := newTestHandler(t, "", time.Time{})
	if err := os.WriteFile(filepath.Join(root, "id_rsa"), []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "server.pem"), []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"id_rsa.", "id_rsa..", "id_rsa. .", "server.pem.", ".env.", "ID_RSA."} {
		if response := get(handler, "/s/"+testToken+"/file?path="+url.QueryEscape(name)); response.Code == http.StatusOK {
			t.Errorf("%q returned 200: %q", name, response.Body.String())
		}
	}
}
