package tunnel

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const terminalRequestRows = 14

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
	requests      []terminalRequestLine
	requestStart  int
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
		requests:      make([]terminalRequestLine, 0, terminalRequestRows),
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

func (ui *terminalUI) add(line terminalRequestLine) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped {
		return
	}

	ui.requests = append(ui.requests, terminalRequestLine{})
	copy(ui.requests[1:], ui.requests[:len(ui.requests)-1])
	ui.requests[0] = line
	if len(ui.requests) > terminalRequestRows {
		ui.requests = ui.requests[:terminalRequestRows]
	}

	ui.renderLocked()
}

func (ui *terminalUI) render() {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.renderLocked()
}

func (ui *terminalUI) renderLocked() {
	if ui.stopped {
		return
	}

	var out strings.Builder
	if ui.requestStart == 0 {
		header := buildTerminalDashboard(ui.cfg, ui.resp, ui.latency, ui.dashboardPort)
		ui.requestStart = strings.Count(header, "\n") + 1
		out.WriteString("\033[?1049h\033[?25l\033[H\033[2J")
		out.WriteString(header)
	} else {
		fmt.Fprintf(&out, "\033[%d;1H\033[J", ui.requestStart)
	}
	writeRequestLines(&out, ui.visibleRequests())
	out.WriteString("\033[H")
	fmt.Print(out.String())
}

func (ui *terminalUI) visibleRequests() []terminalRequestLine {
	_, height := terminalSize()
	maxRows := terminalRequestRows
	if ui.requestStart > 0 && height > 0 {
		if rows := height - ui.requestStart; rows < maxRows {
			maxRows = rows
		}
	}
	if maxRows < 0 {
		maxRows = 0
	}
	if len(ui.requests) <= maxRows {
		return ui.requests
	}
	return ui.requests[:maxRows]
}

func (ui *terminalUI) clear() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped {
		return
	}

	ui.stopped = true
	fmt.Print("\033[?25h\033[H\033[2J\033[?1049l\033[H\033[2J\033[3J")
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

func writeRequestLines(out *strings.Builder, requests []terminalRequestLine) {
	for i, req := range requests {
		if i > 0 {
			out.WriteByte('\n')
		}
		writeRequestLine(out, req)
	}
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
