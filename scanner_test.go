// scanner_test.go - Tests for JSONL scanning and cost calculation.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ── projectNameFromCwd ────────────────────────────────────────────────────────

func TestProjectNameFromCwd(t *testing.T) {
	tests := []struct {
		cwd  string
		want string
	}{
		{"/home/user/myproject", "user/myproject"},
		{"/home/user/myproject/", "user/myproject"},
		{"C:\\Users\\user\\project", "user/project"},
		{"/single", "/single"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		got := projectNameFromCwd(tt.cwd)
		if got != tt.want {
			t.Errorf("projectNameFromCwd(%q) = %q, want %q", tt.cwd, got, tt.want)
		}
	}
}

// ── Pricing / cost ────────────────────────────────────────────────────────────

func TestIsBillable(t *testing.T) {
	billable := []string{
		"claude-sonnet-4-6",
		"claude-opus-4-5",
		"claude-haiku-4-5",
		"claude-sonnet-4-6-20260401",
	}
	nonBillable := []string{
		"unknown",
		"local-model",
		"",
	}
	for _, m := range billable {
		if !isBillable(m) {
			t.Errorf("isBillable(%q) = false, want true", m)
		}
	}
	for _, m := range nonBillable {
		if isBillable(m) {
			t.Errorf("isBillable(%q) = true, want false", m)
		}
	}
}

func TestCalcCost(t *testing.T) {
	// 1M input tokens of sonnet-4-6 should cost $3.69
	cost := calcCost("claude-sonnet-4-6", 1_000_000, 0, 0, 0)
	if cost < 3.68 || cost > 3.70 {
		t.Errorf("calcCost sonnet input 1M = %.4f, want ~3.69", cost)
	}

	// Non-billable model should return 0
	cost = calcCost("unknown-model", 1_000_000, 1_000_000, 0, 0)
	if cost != 0 {
		t.Errorf("calcCost unknown model = %.4f, want 0", cost)
	}

	// Cache read is cheaper
	costInput := calcCost("claude-sonnet-4-6", 1_000_000, 0, 0, 0)
	costCR := calcCost("claude-sonnet-4-6", 0, 0, 1_000_000, 0)
	if costCR >= costInput {
		t.Errorf("cache read cost (%.4f) should be cheaper than input cost (%.4f)", costCR, costInput)
	}
}

// ── fmtTokens ─────────────────────────────────────────────────────────────────

func TestFmtTokens(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{1_000_000, "1.00M"},
		{2_500_000, "2.50M"},
		{1_000_000_000, "1.00B"},
	}
	for _, tt := range tests {
		got := fmtTokens(tt.n)
		if got != tt.want {
			t.Errorf("fmtTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// ── parseJSONLFile ────────────────────────────────────────────────────────────

func TestParseJSONLFile(t *testing.T) {
	dir := t.TempDir()
	jsonlPath := filepath.Join(dir, "session.jsonl")

	content := `{"type":"user","sessionId":"sess1","timestamp":"2026-04-07T10:00:00Z","cwd":"/a/b","gitBranch":"main"}
{"type":"assistant","sessionId":"sess1","timestamp":"2026-04-07T10:01:00Z","cwd":"/a/b","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":10,"cache_creation_input_tokens":5},"content":[{"type":"tool_use","name":"Bash"}]}}
{"type":"assistant","sessionId":"sess1","timestamp":"2026-04-07T10:02:00Z","cwd":"/a/b","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"user","sessionId":"sess2","timestamp":"2026-04-07T11:00:00Z","cwd":"/c/d"}
{"type":"assistant","sessionId":"sess2","timestamp":"2026-04-07T11:01:00Z","message":{"model":"claude-opus-4-5","usage":{"input_tokens":500,"output_tokens":200}}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	metas, turns, err := parseJSONLFile(jsonlPath, "")
	if err != nil {
		t.Fatalf("parseJSONLFile error: %v", err)
	}

	if len(metas) != 2 {
		t.Errorf("want 2 sessions, got %d", len(metas))
	}
	// Only turns with tokens > 0 are kept; the zero-token turn is skipped
	if len(turns) != 2 {
		t.Errorf("want 2 turns, got %d", len(turns))
	}

	// Verify tool name extraction
	var foundBash bool
	for _, tr := range turns {
		if tr.ToolName == "Bash" {
			foundBash = true
		}
	}
	if !foundBash {
		t.Error("expected turn with ToolName='Bash'")
	}
}

// ── aggregateSessions ─────────────────────────────────────────────────────────

func TestAggregateSessions(t *testing.T) {
	metas := []sessionMeta{
		{SessionID: "s1", ProjectName: "a/b", Model: "claude-sonnet-4-6"},
	}
	turns := []turn{
		{SessionID: "s1", InputTokens: 100, OutputTokens: 50, CacheReadTokens: 10, CacheCreationTokens: 5, Model: "claude-sonnet-4-6"},
		{SessionID: "s1", InputTokens: 200, OutputTokens: 80, Model: "claude-sonnet-4-6"},
	}

	sessions := aggregateSessions(metas, turns)
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.TotalInputTokens != 300 {
		t.Errorf("TotalInputTokens = %d, want 300", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 130 {
		t.Errorf("TotalOutputTokens = %d, want 130", s.TotalOutputTokens)
	}
	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
}

// ── Full scan integration test ────────────────────────────────────────────────

func TestScan(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "projects", "myproject")
	if err := os.MkdirAll(projDir, 0750); err != nil {
		t.Fatal(err)
	}

	jsonlContent := `{"type":"user","sessionId":"sess1","timestamp":"2026-04-07T10:00:00Z","cwd":"/x/y"}
{"type":"assistant","sessionId":"sess1","timestamp":"2026-04-07T10:01:00Z","cwd":"/x/y","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1000,"output_tokens":200}}}
`
	if err := os.WriteFile(filepath.Join(projDir, "s1.jsonl"), []byte(jsonlContent), 0600); err != nil {
		t.Fatal(err)
	}

	dbFile := filepath.Join(tmpDir, "usage.db")
	res, err := scan(filepath.Join(tmpDir, "projects"), dbFile, false)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if res.NewFiles != 1 {
		t.Errorf("NewFiles = %d, want 1", res.NewFiles)
	}
	if res.TurnsAdded != 1 {
		t.Errorf("TurnsAdded = %d, want 1", res.TurnsAdded)
	}
	if res.SessionsSeen != 1 {
		t.Errorf("SessionsSeen = %d, want 1", res.SessionsSeen)
	}

	// Re-scan should skip unchanged file
	res2, err := scan(filepath.Join(tmpDir, "projects"), dbFile, false)
	if err != nil {
		t.Fatalf("second scan error: %v", err)
	}
	if res2.SkippedFiles != 1 {
		t.Errorf("second scan SkippedFiles = %d, want 1", res2.SkippedFiles)
	}
	if res2.TurnsAdded != 0 {
		t.Errorf("second scan TurnsAdded = %d, want 0", res2.TurnsAdded)
	}
}
