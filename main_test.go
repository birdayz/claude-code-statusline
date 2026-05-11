package main

import (
	"encoding/json"
	"strings"
	"testing"
)

var testInput = statusInput{
	SessionName: "Fix auth middleware",
	Model: struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	}{
		DisplayName: "Opus 4.6 (1M context)",
		ID:          "claude-opus-4-6[1m]",
	},
	Cwd: "/home/user/projects/myapp",
	Workspace: struct {
		CurrentDir  string `json:"current_dir"`
		GitWorktree string `json:"git_worktree"`
	}{
		CurrentDir: "/home/user/projects/myapp",
	},
	Cost: struct {
		TotalCostUSD       float64 `json:"total_cost_usd"`
		TotalDurationMs    int64   `json:"total_duration_ms"`
		TotalAPIDurationMs int64   `json:"total_api_duration_ms"`
		TotalLinesAdded    int     `json:"total_lines_added"`
		TotalLinesRemoved  int     `json:"total_lines_removed"`
	}{
		TotalCostUSD:    0.972,
		TotalDurationMs: 775508,
	},
	ContextWindow: struct {
		TotalInputTokens  int `json:"total_input_tokens"`
		TotalOutputTokens int `json:"total_output_tokens"`
		ContextWindowSize int `json:"context_window_size"`
		UsedPercentage    int `json:"used_percentage"`
	}{
		TotalInputTokens:  152300,
		ContextWindowSize: 1000000,
	},
	SessionID: "2f7e1784-abcd-1234-5678-abcdef012345",
}

func init() {
	gitBranchFunc = func(_ statusInput) string { return "main" }
}

func TestFmtTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{27800, "27.8k"},
		{152300, "152.3k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tc := range tests {
		got := fmtTokens(tc.n)
		if got != tc.want {
			t.Errorf("fmtTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFmtDuration(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{0, ""},
		{30000, "0m"},
		{120000, "2m"},
		{775508, "12m"},
		{3661000, "1h1m"},
		{7200000, "2h0m"},
	}

	for _, tc := range tests {
		input := statusInput{}
		input.Cost.TotalDurationMs = tc.ms
		got := duration(input)
		if got != tc.want {
			t.Errorf("duration(%d ms) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func TestContextWindow(t *testing.T) {
	tests := []struct {
		used  int
		total int
		want  string
	}{
		{0, 0, ""},
		{0, 1000000, ""},
		{152300, 1000000, "152.3k/1.0M"},
		{41865, 0, "41.9k total"},
		{500, 200000, "500/200.0k"},
	}

	for _, tc := range tests {
		input := statusInput{}
		input.ContextWindow.TotalInputTokens = tc.used
		input.ContextWindow.ContextWindowSize = tc.total
		got := contextWindow(input)
		if got != tc.want {
			t.Errorf("contextWindow(%d, %d) = %q, want %q", tc.used, tc.total, got, tc.want)
		}
	}
}

func TestCost(t *testing.T) {
	tests := []struct {
		usd  float64
		want string
	}{
		{0, ""},
		{0.05, "$0.05"},
		{0.972, "$0.97"},
		{12.5, "$12.50"},
	}

	for _, tc := range tests {
		input := statusInput{}
		input.Cost.TotalCostUSD = tc.usd
		got := cost(input)
		if got != tc.want {
			t.Errorf("cost(%.3f) = %q, want %q", tc.usd, got, tc.want)
		}
	}
}

func TestSessionID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"2f7e1784-abcd-1234", "2f7e1784"},
	}

	for _, tc := range tests {
		input := statusInput{}
		input.SessionID = tc.id
		got := sessionID(input)
		if got != tc.want {
			t.Errorf("sessionID(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestModelName(t *testing.T) {
	tests := []struct {
		display string
		id      string
		want    string
	}{
		{"Opus 4.6 (1M context)", "claude-opus-4-6", "claude-opus-4-6"},
		{"", "claude-opus-4-6", "claude-opus-4-6"},
		{"Opus 4.6", "", "Opus 4.6"},
		{"", "", ""},
	}

	for _, tc := range tests {
		input := statusInput{}
		input.Model.DisplayName = tc.display
		input.Model.ID = tc.id
		got := modelName(input)
		if got != tc.want {
			t.Errorf("modelName(%q, %q) = %q, want %q", tc.display, tc.id, got, tc.want)
		}
	}
}

func TestBuildSegments(t *testing.T) {
	segments := buildSegments(testInput)

	if len(segments) == 0 {
		t.Fatal("expected segments, got none")
	}

	// First segment should be session name
	if !strings.Contains(segments[0].text, "Fix auth middleware") {
		t.Errorf("first segment should be session name, got %q", segments[0].text)
	}

	// Should have: session name, model, branch, context, cost, session id, duration
	if len(segments) < 6 {
		t.Errorf("expected at least 6 segments, got %d", len(segments))
	}

	texts := make([]string, len(segments))
	for i, s := range segments {
		texts[i] = strings.TrimSpace(s.text)
	}
	t.Logf("segments: %v", texts)
}

func TestBuildSegmentsWithWorktree(t *testing.T) {
	gitBranchFunc = func(input statusInput) string {
		branch := "feature-x"
		if wt := input.Workspace.GitWorktree; wt != "" {
			branch += "@" + wt
		}
		return branch
	}
	defer func() { gitBranchFunc = func(_ statusInput) string { return "main" } }()

	input := testInput
	input.Workspace.GitWorktree = "my-worktree"

	segments := buildSegments(input)
	found := false
	for _, s := range segments {
		if strings.Contains(s.text, "feature-x@my-worktree") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected worktree suffix in branch segment")
	}
}

func TestRenderPowerlineEmpty(t *testing.T) {
	got := renderPowerline(nil)
	if got != "" {
		t.Errorf("expected empty string for nil segments, got %q", got)
	}
}

func TestRenderPowerlineContainsSeparators(t *testing.T) {
	segments := []segment{
		{" A ", 255, 25},
		{" B ", 255, 22},
	}
	got := renderPowerline(segments)

	if !strings.Contains(got, powerlineSep) {
		t.Error("expected powerline separator in output")
	}
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Error("expected segment text in output")
	}
}

func TestFullJSONParsing(t *testing.T) {
	raw := `{
		"session_id": "abc12345-def6-7890",
		"session_name": "Debug login flow",
		"model": {"display_name": "Opus 4.6", "id": "claude-opus-4-6"},
		"cwd": "/tmp",
		"cost": {"total_cost_usd": 1.5, "total_duration_ms": 600000},
		"context_window": {"total_input_tokens": 50000, "context_window_size": 200000},
		"rate_limits": {"five_hour": {"used_percentage": 45.5, "resets_at": 0}}
	}`

	var input statusInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if input.SessionName != "Debug login flow" {
		t.Errorf("session_name = %q, want %q", input.SessionName, "Debug login flow")
	}
	if input.Cost.TotalCostUSD != 1.5 {
		t.Errorf("cost = %f, want 1.5", input.Cost.TotalCostUSD)
	}
	if input.RateLimits.FiveHour.UsedPercentage != 45.5 {
		t.Errorf("rate limit = %f, want 45.5", input.RateLimits.FiveHour.UsedPercentage)
	}
}
