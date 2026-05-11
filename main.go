package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	powerlineSep = ""
	gitBranchIcon = "\ue0a0"
	resetSeq     = "\033[0m"
)

func fg256(c int) string { return fmt.Sprintf("\033[38;5;%dm", c) }
func bg256(c int) string { return fmt.Sprintf("\033[48;5;%dm", c) }

type segment struct {
	text string
	fg   int
	bg   int
}

func (s segment) visibleWidth() int {
	return len([]rune(s.text))
}

type statusInput struct {
	Model struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	} `json:"model"`
	Cwd       string `json:"cwd"`
	Workspace struct {
		CurrentDir  string `json:"current_dir"`
		GitWorktree string `json:"git_worktree"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD       float64 `json:"total_cost_usd"`
		TotalDurationMs    int64   `json:"total_duration_ms"`
		TotalAPIDurationMs int64   `json:"total_api_duration_ms"`
		TotalLinesAdded    int     `json:"total_lines_added"`
		TotalLinesRemoved  int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		TotalInputTokens  int `json:"total_input_tokens"`
		TotalOutputTokens int `json:"total_output_tokens"`
		ContextWindowSize int `json:"context_window_size"`
		UsedPercentage    int `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       int64   `json:"resets_at"`
		} `json:"five_hour"`
	} `json:"rate_limits"`
	SessionID   string `json:"session_id"`
	SessionName string `json:"session_name"`
}

var gitBranchFunc = gitBranchExec

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}

	var input statusInput
	if err := json.Unmarshal(data, &input); err != nil {
		return
	}

	segments := buildSegments(input)
	width := terminalWidth()

	if width > 0 {
		fmt.Print(renderResponsive(segments, width))
	} else {
		fmt.Print(renderPowerline(segments))
	}
}

func buildSegments(input statusInput) []segment {
	var segments []segment

	if name := input.SessionName; name != "" {
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		segments = append(segments, segment{" " + name + " ", 255, 62})
	}

	if model := modelName(input); model != "" {
		segments = append(segments, segment{" " + model + " ", 255, 25})
	}

	if branch := gitBranchFunc(input); branch != "" {
		segments = append(segments, segment{" " + gitBranchIcon + " " + branch + " ", 255, 22})
	}

	if ctx := contextWindow(input); ctx != "" {
		segments = append(segments, segment{" " + ctx + " ", 16, 178})
	}

	if c := cost(input); c != "" {
		segments = append(segments, segment{" " + c + " ", 255, 88})
	}

	if rl := rateLimit(input); rl != "" {
		segments = append(segments, segment{" " + rl + " ", 16, 214})
	}

	if sid := sessionID(input); sid != "" {
		segments = append(segments, segment{" " + sid + " ", 250, 238})
	}

	if dur := duration(input); dur != "" {
		segments = append(segments, segment{" " + dur + " ", 255, 97})
	}

	return segments
}

func segmentsWidth(segments []segment) int {
	w := 0
	for _, s := range segments {
		w += s.visibleWidth() + 1 // +1 for separator
	}
	return w
}

func renderResponsive(segments []segment, width int) string {
	total := segmentsWidth(segments)
	if total <= width {
		return renderPowerline(segments)
	}

	// Split into two lines at the point where first line fits
	split := len(segments)
	running := 0
	for i, s := range segments {
		running += s.visibleWidth() + 1
		if running > width && i > 0 {
			split = i
			break
		}
	}

	line1 := renderPowerline(segments[:split])
	line2 := renderPowerline(segments[split:])
	return line1 + "\n" + line2
}

func renderPowerline(segments []segment) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder

	for i, seg := range segments {
		b.WriteString(fg256(seg.fg))
		b.WriteString(bg256(seg.bg))
		b.WriteString(seg.text)

		if i < len(segments)-1 {
			next := segments[i+1]
			b.WriteString(fg256(seg.bg))
			b.WriteString(bg256(next.bg))
			b.WriteString(powerlineSep)
		} else {
			b.WriteString(resetSeq)
			b.WriteString(fg256(seg.bg))
			b.WriteString(powerlineSep)
			b.WriteString(resetSeq)
		}
	}

	return b.String()
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func terminalWidth() int {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return 0
	}
	defer f.Close()

	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0
	}
	return int(ws.Col)
}

func modelName(input statusInput) string {
	if input.Model.ID != "" {
		return input.Model.ID
	}
	return input.Model.DisplayName
}

func gitBranchExec(input statusInput) string {
	cwd := input.Cwd
	if cwd == "" {
		cwd = input.Workspace.CurrentDir
	}
	if cwd == "" {
		return ""
	}

	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}

	branch := strings.TrimSpace(string(out))
	if wt := input.Workspace.GitWorktree; wt != "" {
		branch += "@" + wt
	}
	return branch
}

func contextWindow(input statusInput) string {
	used := input.ContextWindow.TotalInputTokens
	total := input.ContextWindow.ContextWindowSize

	if used > 0 && total > 0 {
		return fmtTokens(used) + "/" + fmtTokens(total)
	}
	if used > 0 {
		return fmtTokens(used) + " total"
	}
	return ""
}

func cost(input statusInput) string {
	c := input.Cost.TotalCostUSD
	if c <= 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", c)
}

func rateLimit(input statusInput) string {
	pct := input.RateLimits.FiveHour.UsedPercentage
	if pct <= 0 {
		return ""
	}

	resetAt := input.RateLimits.FiveHour.ResetsAt
	remaining := ""
	if resetAt > 0 {
		d := time.Until(time.Unix(resetAt, 0))
		if d > 0 {
			remaining = fmt.Sprintf(" ↻%s", fmtDuration(d))
		}
	}

	return fmt.Sprintf("%.0f%%%s", pct, remaining)
}

func sessionID(input statusInput) string {
	sid := input.SessionID
	if len(sid) > 8 {
		return sid[:8]
	}
	return sid
}

func duration(input statusInput) string {
	ms := input.Cost.TotalDurationMs
	if ms <= 0 {
		return ""
	}
	return fmtDuration(time.Duration(ms) * time.Millisecond)
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func fmtTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
