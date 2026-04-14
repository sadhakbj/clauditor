// scanner_test.go - Tests for JSONL scanning and cost calculation.
package main

import (
	"encoding/json"
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

	metas, turns, err := parseJSONLFile(jsonlPath)
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

// ── parseJSONLFile source tagging ─────────────────────────────────────────────

func TestParseJSONLFileSourceTagging(t *testing.T) {
	dir := t.TempDir()
	jsonlPath := filepath.Join(dir, "session.jsonl")

	content := `{"type":"user","sessionId":"sess1","timestamp":"2026-04-07T10:00:00Z","cwd":"/a/b"}
{"type":"assistant","sessionId":"sess1","timestamp":"2026-04-07T10:01:00Z","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	metas, turns, err := parseJSONLFile(jsonlPath)
	if err != nil {
		t.Fatalf("parseJSONLFile error: %v", err)
	}

	for _, m := range metas {
		if m.Source != sourceClaude {
			t.Errorf("sessionMeta.Source = %q, want %q", m.Source, sourceClaude)
		}
	}
	for _, tr := range turns {
		if tr.Source != sourceClaude {
			t.Errorf("turn.Source = %q, want %q", tr.Source, sourceClaude)
		}
	}
}

// ── DB migration ──────────────────────────────────────────────────────────────

func TestMigrateDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "migrate.db")

	// Create a minimal DB without the source columns (simulating an old DB).
	db, err := openDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			project_name TEXT,
			first_timestamp TEXT,
			last_timestamp TEXT,
			git_branch TEXT,
			total_input_tokens INTEGER DEFAULT 0,
			total_output_tokens INTEGER DEFAULT 0,
			total_cache_read INTEGER DEFAULT 0,
			total_cache_creation INTEGER DEFAULT 0,
			model TEXT,
			turn_count INTEGER DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS turns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			timestamp TEXT,
			model TEXT,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			cache_read_tokens INTEGER DEFAULT 0,
			cache_creation_tokens INTEGER DEFAULT 0,
			tool_name TEXT,
			cwd TEXT
		);
		CREATE TABLE IF NOT EXISTS processed_files (path TEXT PRIMARY KEY, mtime REAL, lines INTEGER);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Now open with initDB which should migrate.
	db2, err := openDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if err := initDB(db2); err != nil {
		t.Fatalf("initDB (migrate) error: %v", err)
	}

	// Insert a row and verify the source column exists and defaults correctly.
	_, err = db2.Exec(`INSERT INTO turns
		(session_id, timestamp, model, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, tool_name, cwd)
		VALUES ('s1','2026-04-01T10:00:00Z','claude-sonnet-4-6',100,50,0,0,NULL,'/a/b')`)
	if err != nil {
		t.Fatalf("insert after migration: %v", err)
	}

	var src string
	if err := db2.QueryRow(`SELECT COALESCE(source,'claude') FROM turns WHERE session_id='s1'`).Scan(&src); err != nil {
		t.Fatalf("query source after migration: %v", err)
	}
	if src != "claude" {
		t.Errorf("migrated source = %q, want 'claude'", src)
	}
}

// ── Cursor conversation parsing ───────────────────────────────────────────────

func TestParseCursorConversations(t *testing.T) {
	raw := json.RawMessage(`[
  {
    "id": "conv1",
    "workspace": "/home/user/myproject",
    "createdAt": "2026-04-07T10:00:00Z",
    "updatedAt": "2026-04-07T10:05:00Z",
    "turns": [
      {"model": "gpt-4o", "tokens": {"inputTokens": 500, "outputTokens": 200}, "timestamp": "2026-04-07T10:01:00Z"},
      {"model": "gpt-4o", "tokens": {"inputTokens": 300, "outputTokens": 100}, "timestamp": "2026-04-07T10:03:00Z"}
    ]
  },
  {
    "id": "conv2",
    "createdAt": "2026-04-07T11:00:00Z",
    "messages": [
      {"role": "user",      "timestamp": "2026-04-07T11:00:00Z"},
      {"role": "assistant", "model": "claude-3-5-sonnet", "usage": {"input_tokens": 400, "output_tokens": 150}, "timestamp": "2026-04-07T11:01:00Z"}
    ]
  }
]`)

	blobs := []json.RawMessage{raw}
	metas, turns := parseCursorConversations(blobs, "")
	if len(metas) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(metas))
	}
	if len(turns) != 3 {
		t.Fatalf("want 3 turns, got %d", len(turns))
	}
	for _, m := range metas {
		if m.Source != sourceCursor {
			t.Errorf("meta Source = %q, want %q", m.Source, sourceCursor)
		}
	}
	for _, tr := range turns {
		if tr.Source != sourceCursor {
			t.Errorf("turn Source = %q, want %q", tr.Source, sourceCursor)
		}
	}
	// Verify project name extracted from workspace
	var projMeta *sessionMeta
	for i := range metas {
		if metas[i].SessionID == "cursor:conv1" {
			projMeta = &metas[i]
		}
	}
	if projMeta == nil {
		t.Fatal("session cursor:conv1 not found")
	}
	if projMeta.ProjectName != "user/myproject" {
		t.Errorf("ProjectName = %q, want %q", projMeta.ProjectName, "user/myproject")
	}
}

// ── Scan stores source correctly ──────────────────────────────────────────────

func TestScanSourceColumn(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "projects", "myproj")
	if err := os.MkdirAll(projDir, 0750); err != nil {
		t.Fatal(err)
	}

	content := `{"type":"user","sessionId":"s1","timestamp":"2026-04-07T10:00:00Z","cwd":"/x/y"}
{"type":"assistant","sessionId":"s1","timestamp":"2026-04-07T10:01:00Z","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1000,"output_tokens":200}}}
`
	if err := os.WriteFile(filepath.Join(projDir, "s1.jsonl"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	dbFile := filepath.Join(tmpDir, "usage.db")
	if _, err := scan(filepath.Join(tmpDir, "projects"), dbFile, false); err != nil {
		t.Fatalf("scan error: %v", err)
	}

	db, err := openDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var src string
	db.QueryRow(`SELECT COALESCE(source,'claude') FROM turns WHERE session_id='s1'`).Scan(&src)
	if src != sourceClaude {
		t.Errorf("turn source = %q, want %q", src, sourceClaude)
	}
	db.QueryRow(`SELECT COALESCE(source,'claude') FROM sessions WHERE session_id='s1'`).Scan(&src)
	if src != sourceClaude {
		t.Errorf("session source = %q, want %q", src, sourceClaude)
	}
}

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
