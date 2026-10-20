package share

import (
	"archive/zip"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/safeopen"
	"golang.org/x/crypto/bcrypt"
)

const maxConcurrentTransfers = 8

type Config struct {
	Root      string
	Token     string
	Password  string
	ExpiresAt time.Time
	Preview   bool
}

type Handler struct {
	root         string
	name         string
	token        string
	prefix       string
	passwordHash []byte
	expiresAt    time.Time
	preview      bool
	cookieKey    []byte
	transfers    chan struct{}
}

type entry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Directory   bool   `json:"directory"`
	Size        int64  `json:"size"`
	Modified    string `json:"modified"`
	Previewable bool   `json:"previewable"`
}

func New(cfg Config) (*Handler, error) {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve shared folder: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("shared path must be a directory")
	}
	if len(cfg.Token) < 24 {
		return nil, fmt.Errorf("share token is too short")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	var passwordHash []byte
	if cfg.Password != "" {
		passwordHash, err = bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
	}
	return &Handler{
		root: root, name: filepath.Base(root), token: cfg.Token, prefix: "/s/" + cfg.Token,
		passwordHash: passwordHash, expiresAt: cfg.ExpiresAt, preview: cfg.Preview,
		cookieKey: key, transfers: make(chan struct{}, maxConcurrentTransfers),
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.securityHeaders(w)
	if !strings.HasPrefix(r.URL.Path, h.prefix) || (len(r.URL.Path) > len(h.prefix) && r.URL.Path[len(h.prefix)] != '/') {
		http.NotFound(w, r)
		return
	}
	if !h.expiresAt.IsZero() && !time.Now().Before(h.expiresAt) {
		http.Error(w, "This share has expired.", http.StatusGone)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, h.prefix)
	switch {
	case rel == "" || rel == "/":
		h.serveBrowser(w, r)
	case rel == "/api/unlock" && r.Method == http.MethodPost:
		h.unlock(w, r)
	case rel == "/api/list" && r.Method == http.MethodGet:
		if h.authorize(w, r) {
			h.list(w, r)
		}
	case rel == "/file" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		if h.authorize(w, r) {
			h.file(w, r)
		}
	case rel == "/zip" && r.Method == http.MethodGet:
		if h.authorize(w, r) {
			h.zip(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) securityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'")
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if len(h.passwordHash) == 0 {
		return true
	}
	cookie, err := r.Cookie("goport_share")
	if err == nil && h.validCookie(cookie.Value) {
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = io.WriteString(w, `{"error":"password required"}`)
	return false
}

func (h *Handler) unlock(w http.ResponseWriter, r *http.Request) {
	if len(h.passwordHash) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&request) != nil || bcrypt.CompareHashAndPassword(h.passwordHash, []byte(request.Password)) != nil {
		time.Sleep(150 * time.Millisecond)
		http.Error(w, "Invalid password.", http.StatusUnauthorized)
		return
	}
	expires := time.Now().Add(12 * time.Hour)
	if !h.expiresAt.IsZero() && h.expiresAt.Before(expires) {
		expires = h.expiresAt
	}
	http.SetCookie(w, &http.Cookie{Name: "goport_share", Value: h.cookie(expires), Path: h.prefix, Expires: expires, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) cookie(expires time.Time) string {
	payload := fmt.Sprintf("%d.%s", expires.Unix(), h.token)
	mac := hmac.New(sha256.New, h.cookieKey)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) validCookie(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	payload := string(payloadBytes)
	var expiry int64
	if _, err := fmt.Sscanf(payload, "%d."+h.token, &expiry); err != nil || time.Now().Unix() >= expiry {
		return false
	}
	mac := hmac.New(sha256.New, h.cookieKey)
	_, _ = mac.Write([]byte(payload))
	want := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(want), []byte(parts[1])) == 1
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rel, err := cleanRelative(r.URL.Query().Get("path"))
	if err != nil || blockedPath(rel) {
		http.NotFound(w, r)
		return
	}
	directory, err := h.openDirectory(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer directory.Close()
	info, err := directory.Stat()
	if err != nil || !info.IsDir() {
		http.NotFound(w, r)
		return
	}
	items, err := directory.ReadDir(-1)
	if err != nil {
		http.Error(w, "Could not read folder.", http.StatusInternalServerError)
		return
	}
	entries := make([]entry, 0, len(items))
	for _, item := range items {
		child := path.Join(rel, item.Name())
		if blockedName(item.Name()) || item.Type()&os.ModeSymlink != 0 {
			continue
		}
		childInfo, statErr := item.Info()
		if statErr != nil {
			continue
		}
		if !childInfo.IsDir() {
			file, openErr := safeopen.OpenBeneath(h.root, filepath.FromSlash(child))
			if openErr != nil {
				continue
			}
			childInfo, statErr = file.Stat()
			_ = file.Close()
			if statErr != nil {
				continue
			}
		}
		entries = append(entries, entry{Name: item.Name(), Path: child, Directory: childInfo.IsDir(), Size: childInfo.Size(), Modified: childInfo.ModTime().UTC().Format(time.RFC3339), Previewable: h.preview && previewMIME(item.Name()) != ""})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Directory != entries[j].Directory {
			return entries[i].Directory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"name": h.name, "path": rel, "entries": entries, "passwordRequired": len(h.passwordHash) > 0})
}

func (h *Handler) file(w http.ResponseWriter, r *http.Request) {
	if !h.acquire(w, r) {
		return
	}
	defer h.release()
	rel, err := cleanRelative(r.URL.Query().Get("path"))
	if err != nil || rel == "." || blockedPath(rel) {
		http.NotFound(w, r)
		return
	}
	file, err := h.open(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	preview := r.URL.Query().Get("preview") == "1" && h.preview
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	if preview {
		contentType = previewMIME(info.Name())
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if !preview || previewMIME(info.Name()) == "" {
		w.Header().Set("Content-Disposition", contentDisposition("attachment", info.Name()))
	} else {
		w.Header().Set("Content-Disposition", contentDisposition("inline", info.Name()))
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (h *Handler) zip(w http.ResponseWriter, r *http.Request) {
	if !h.acquire(w, r) {
		return
	}
	defer h.release()
	rel, err := cleanRelative(r.URL.Query().Get("path"))
	if err != nil || blockedPath(rel) {
		http.NotFound(w, r)
		return
	}
	name := h.name
	if rel != "." {
		name = path.Base(rel)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", name+".zip"))
	archive := zip.NewWriter(w)
	err = h.addDirectory(archive, rel, "")
	closeErr := archive.Close()
	if err != nil || closeErr != nil {
		return
	}
}

func (h *Handler) addDirectory(archive *zip.Writer, rel, zipPrefix string) error {
	directory, err := h.openDirectory(rel)
	if err != nil {
		return err
	}
	defer directory.Close()
	items, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, item := range items {
		if blockedName(item.Name()) || item.Type()&os.ModeSymlink != 0 {
			continue
		}
		child := path.Join(rel, item.Name())
		archiveName := path.Join(zipPrefix, item.Name())
		if item.IsDir() {
			if err := h.addDirectory(archive, child, archiveName); err != nil {
				return err
			}
			continue
		}
		opened, err := safeopen.OpenBeneath(h.root, filepath.FromSlash(child))
		if err != nil {
			continue
		}
		info, err := opened.Stat()
		if err != nil {
			opened.Close()
			continue
		}
		if !info.Mode().IsRegular() {
			opened.Close()
			continue
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			opened.Close()
			return err
		}
		header.Name = archiveName
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err == nil {
			_, err = io.Copy(writer, opened)
		}
		opened.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) acquire(w http.ResponseWriter, r *http.Request) bool {
	select {
	case h.transfers <- struct{}{}:
		return true
	case <-r.Context().Done():
		return false
	default:
		http.Error(w, "Too many simultaneous downloads.", http.StatusTooManyRequests)
		return false
	}
}
func (h *Handler) release() { <-h.transfers }

func (h *Handler) open(rel string) (*os.File, error) {
	if rel == "." {
		return os.Open(h.root)
	}
	return safeopen.OpenBeneath(h.root, filepath.FromSlash(rel))
}

func (h *Handler) openDirectory(rel string) (*os.File, error) {
	if rel == "." {
		return os.Open(h.root)
	}
	candidate := filepath.Join(h.root, filepath.FromSlash(rel))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, err
	}
	within, err := filepath.Rel(h.root, resolved)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return nil, os.ErrPermission
	}
	return os.Open(resolved)
}

func cleanRelative(value string) (string, error) {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return "", err
	}
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	if strings.ContainsRune(decoded, 0) || path.IsAbs(decoded) || filepath.VolumeName(decoded) != "" {
		return "", errors.New("invalid path")
	}
	clean := path.Clean(decoded)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("invalid path")
	}
	return clean, nil
}

func blockedPath(rel string) bool {
	if rel == "." || rel == "" {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if blockedName(part) {
			return true
		}
	}
	return false
}
func blockedName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(lower, ".") {
		return true
	}
	switch lower {
	case "id_rsa", "id_ed25519", "credentials", "credentials.json", "config.json", "secrets", "secrets.json":
		return true
	}
	ext := strings.ToLower(filepath.Ext(lower))
	return ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx"
}

func previewMIME(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".csv", ".log", ".json", ".yaml", ".yml":
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}

func contentDisposition(kind, filename string) string {
	safe := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '"' {
			return -1
		}
		return r
	}, filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, kind, safe, url.PathEscape(filename))
}
