package share

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const testToken = "abcdefghijklmnopqrstuvwxyz123456"

func newTestHandler(t *testing.T, password string, expires time.Time) (*Handler, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "note.txt"), []byte("nested"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=value"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(Config{Root: root, Token: testToken, Password: password, ExpiresAt: expires, Preview: true})
	if err != nil {
		t.Fatal(err)
	}
	return handler, root
}

func TestListNestedAndHideSensitiveFiles(t *testing.T) {
	handler, _ := newTestHandler(t, "", time.Time{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/api/list?path=.", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), ".env") {
		t.Fatal("hidden file leaked in directory listing")
	}
	var result struct {
		Entries []entry `json:"entries"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || !result.Entries[0].Directory {
		t.Fatalf("entries = %#v", result.Entries)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/api/list?path=nested", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "note.txt") {
		t.Fatalf("nested listing failed: %d %s", response.Code, response.Body.String())
	}
}

func TestTraversalAndHiddenDownloadsAreDenied(t *testing.T) {
	handler, _ := newTestHandler(t, "", time.Time{})
	for _, target := range []string{"/s/" + testToken + "/file?path=..%252Foutside.txt", "/s/" + testToken + "/file?path=%2e%2e%2foutside.txt", "/s/" + testToken + "/file?path=.env", "/s/wrong-token/file?path=hello.txt"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code == http.StatusOK {
			t.Fatalf("%s unexpectedly returned 200", target)
		}
	}
}

func TestSymlinkEscapeIsDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires optional Windows privilege")
	}
	handler, root := newTestHandler(t, "", time.Time{})
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/file?path=escape.txt", nil))
	if response.Code == http.StatusOK {
		t.Fatal("symlink outside the shared root was served")
	}
}

func TestFileRangeAndStreamingZip(t *testing.T) {
	handler, _ := newTestHandler(t, "", time.Time{})
	request := httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/file?path=hello.txt", nil)
	request.Header.Set("Range", "bytes=0-4")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "hello" {
		t.Fatalf("range response = %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/zip?path=.", nil))
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
		opened, _ := file.Open()
		_, _ = io.Copy(io.Discard, opened)
		opened.Close()
	}
	if !names["hello.txt"] || !names["nested/note.txt"] || names[".env"] {
		t.Fatalf("zip entries = %v", names)
	}
}

func TestPasswordAndExpiration(t *testing.T) {
	handler, _ := newTestHandler(t, "correct horse", time.Time{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/api/list", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unprotected status = %d", response.Code)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/s/"+testToken+"/api/unlock", strings.NewReader(`{"password":"correct horse"}`)))
	if response.Code != http.StatusNoContent || len(response.Result().Cookies()) != 1 {
		t.Fatalf("unlock status = %d", response.Code)
	}
	expired, _ := newTestHandler(t, "", time.Now().Add(-time.Minute))
	response = httptest.NewRecorder()
	expired.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+testToken+"/api/list", nil))
	if response.Code != http.StatusGone {
		t.Fatalf("expired status = %d", response.Code)
	}
}
