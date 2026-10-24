package share

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func get(handler *Handler, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}

func TestStreamAndColonPathsAreRejected(t *testing.T) {
	handler, root := newTestHandler(t, "", time.Time{})
	if err := os.WriteFile(filepath.Join(root, "id_rsa"), []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"id_rsa::$DATA", "hello.txt::$DATA", "hello.txt:secret", "nested:x/note.txt", "C:/Windows/win.ini"} {
		for _, endpoint := range []string{"/file", "/api/list", "/zip"} {
			response := get(handler, "/s/"+testToken+endpoint+"?path="+url.QueryEscape(p))
			if response.Code == http.StatusOK {
				t.Errorf("%s?path=%s returned 200", endpoint, p)
			}
		}
	}
}

// The path parameter was decoded twice, so names with % in them failed.
func TestFileNameWithPercentDownloads(t *testing.T) {
	handler, root := newTestHandler(t, "", time.Time{})
	if err := os.WriteFile(filepath.Join(root, "100% done.txt"), []byte("percent"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := get(handler, "/s/"+testToken+"/file?path="+url.QueryEscape("100% done.txt"))
	if response.Code != http.StatusOK || response.Body.String() != "percent" {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	// A literal %2F stays part of the name instead of becoming a slash.
	response = get(handler, "/s/"+testToken+"/file?path="+url.QueryEscape("..%2Fhello.txt"))
	if response.Code == http.StatusOK {
		t.Fatal("double-decoded traversal was served")
	}
}

// fakeShortNames resolves ENV~1 and GIT~1 the way NTFS does, so the test runs
// where 8.3 names are off.
func fakeShortNames(t *testing.T) {
	t.Helper()
	previous := evalSymlinks
	evalSymlinks = func(p string) (string, error) {
		dir, base := filepath.Split(p)
		switch strings.ToUpper(base) {
		case "ENV~1":
			p = filepath.Join(dir, ".env")
		case "GIT~1":
			p = filepath.Join(dir, ".git")
		}
		return filepath.EvalSymlinks(p)
	}
	t.Cleanup(func() { evalSymlinks = previous })
}

func TestBlocklistChecksResolvedNames(t *testing.T) {
	fakeShortNames(t)
	handler, root := newTestHandler(t, "", time.Time{})
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[remote]"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/file?path=ENV~1", "/file?path=env~1", "/api/list?path=GIT~1", "/zip?path=GIT~1", "/file?path=GIT~1/config"} {
		if response := get(handler, "/s/"+testToken+target); response.Code == http.StatusOK {
			t.Errorf("%s returned 200: %q", target, response.Body.String())
		}
	}
}

func TestZipOfRootSkipsBlockedEntries(t *testing.T) {
	handler, root := newTestHandler(t, "", time.Time{})
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[remote]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "server.key"), []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := get(handler, "/s/"+testToken+"/zip?path=.")
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		if blockedPath(file.Name) {
			t.Errorf("zip contains %s", file.Name)
		}
		opened, _ := file.Open()
		_, _ = io.Copy(io.Discard, opened)
		opened.Close()
	}
}
