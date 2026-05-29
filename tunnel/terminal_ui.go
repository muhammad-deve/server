package tunnel

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type terminalRequestLine struct {
	Time       time.Time
	Method     string
	Path       string
	StatusCode int
	StatusText string
}

type terminalUI struct {
	mu            sync.Mutex
	cfg           Config
	resp          registrationResponse
	latency       time.Duration
	dashboardPort int
	started       bool
	stopped       bool
}

var (
	terminalUIMu     sync.RWMutex
	activeTerminalUI *terminalUI
	terminalUIClosed bool
)

func startTerminalUI(cfg Config, resp registrationResponse, latency time.Duration, dashboardPort int) {
	ui := &terminalUI{
		cfg:           cfg,
		resp:          resp,
		latency:       latency,
		dashboardPort: dashboardPort,
	}

	terminalUIMu.Lock()
	activeTerminalUI = ui
	terminalUIClosed = false
	terminalUIMu.Unlock()

	ui.render()
}

func stopTerminalUI() {
	terminalUIMu.Lock()
	ui := activeTerminalUI
	activeTerminalUI = nil
	terminalUIClosed = true
	terminalUIMu.Unlock()

	if ui != nil {
		ui.clear()
	}
}

// add appends a new request line at the bottom of the stream. Because we render
// to the normal screen buffer (no alternate buffer, no in-place redraw), the
// terminal's native scrollback keeps every line and stays scrollable.
func (ui *terminalUI) add(line terminalRequestLine) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped {
		return
	}

	var out strings.Builder
	writeRequestLine(&out, line)
	out.WriteByte('\n')
	fmt.Print(out.String())
}

// render prints the dashboard header exactly once. Subsequent request lines are
// streamed below it so the whole session lives in normal scrollback.
func (ui *terminalUI) render() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped || ui.started {
		return
	}
	ui.started = true

	var out strings.Builder
	// Clear the visible screen and scrollback for a clean start, then leave the
	// terminal in the normal buffer so new output scrolls naturally.
	out.WriteString("\033[H\033[2J\033[3J")
	out.WriteString(buildTerminalDashboard(ui.cfg, ui.resp, ui.latency, ui.dashboardPort))
	fmt.Print(out.String())
}

func (ui *terminalUI) clear() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped {
		return
	}

	ui.stopped = true
	// Wipe the visible screen and the scrollback, then move the cursor home so
	// the prompt returns to a clean terminal after Ctrl+C.
	fmt.Print("\033[H\033[2J\033[3J")
}

func printRequestLine(req terminalRequestLine) {
	terminalUIMu.RLock()
	closed := terminalUIClosed
	terminalUIMu.RUnlock()
	if closed {
		return
	}

	var out strings.Builder
	writeRequestLine(&out, req)
	out.WriteByte('\n')
	fmt.Print(out.String())
}

func writeRequestLine(out *strings.Builder, req terminalRequestLine) {
	statusColor := ansiGray
	if req.StatusCode >= 200 && req.StatusCode < 300 {
		statusColor = ansiGreen
	}

	pathWidth := requestPathColumnWidth(req)
	displayPath := truncateDisplayPath(req.Path, pathWidth)
	fmt.Fprintf(
		out,
		"%s%-10s %-7s%s %s%-*s%s %s%d %s%s",
		ansiGray,
		req.Time.Format("15:04:05"),
		req.Method,
		ansiReset,
		ansiBold+ansiWhite,
		pathWidth,
		displayPath,
		ansiReset,
		statusColor,
		req.StatusCode,
		req.StatusText,
		ansiReset,
	)
}

func buildTerminalDashboard(cfg Config, resp registrationResponse, latency time.Duration, dashboardPort int) string {
	var out strings.Builder
	dashboardAddr := "http://127.0.0.1:" + strconv.Itoa(dashboardPort)
	fmt.Fprintf(&out, "%s$%s %s%s%s\n", ansiGray, ansiReset, ansiBold+ansiWhite, commandLine(cfg), ansiReset)
	out.WriteByte('\n')
	fmt.Fprintf(&out, "%s%-16s%s %s%s%s\n", ansiGray, "Dashboard", ansiReset, ansiBold+ansiWhite, dashboardAddr, ansiReset)
	fmt.Fprintf(&out, "%s%-16s%s %s%s%s\n", ansiGray, "Region", ansiReset, ansiBold+ansiWhite, regionLabel(cfg.Region), ansiReset)
	fmt.Fprintf(&out, "%s%-16s%s %sonline%s %s(%dms)%s\n", ansiGray, "Status", ansiReset, ansiGreen, ansiReset, ansiGray, latency.Milliseconds(), ansiReset)
	fmt.Fprintf(&out, "%s%-16s%s %s%s%s %s->%s %s%slocalhost:%s%s\n", ansiGray, "Forwarding", ansiReset, ansiGreen, resp.URL, ansiReset, ansiGray, ansiReset, ansiBold, ansiWhite, cfg.Port, ansiReset)
	out.WriteByte('\n')

	if cfg.Type == "http" {
		fmt.Fprintf(&out, "%s%sHTTP Requests%s\n", ansiBold, ansiWhite, ansiReset)
		fmt.Fprintf(&out, "%s-------------%s\n", ansiGray, ansiReset)
		out.WriteByte('\n')
	}
	return out.String()
}

func requestPathColumnWidth(req terminalRequestLine) int {
	width, _ := terminalSize()
	if width <= 0 {
		return pathColumnWidth
	}

	statusWidth := len(strconv.Itoa(req.StatusCode)) + 1 + len(req.StatusText)
	pathWidth := width - 20 - statusWidth - 1
	if pathWidth > pathColumnWidth {
		return pathColumnWidth
	}
	if pathWidth < 8 {
		return 8
	}
	return pathWidth
}

func truncateDisplayPath(path string, width int) string {
	if len(path) <= width {
		return path
	}
	if width <= 1 {
		return "."
	}
	if width <= 3 {
		return path[:width-1] + "."
	}
	return path[:width-3] + "..."
}
