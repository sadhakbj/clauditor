// codex_scanner_test.go - Tests for Codex CLI JSONL parsing and scan.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ── parseCodexJSONLFile ───────────────────────────────────────────────────────

func TestParseCodexJSONLFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rollout-2026-04-15T11-54-50-abc123.jsonl")

	// A minimal two-turn Codex session: session_meta, turn_context, two token_count events.
	content := `{"timestamp":"2026-04-15T11:54:50.291Z","type":"session_meta","payload":{"id":"abc123","cwd":"/home/user/myproject","git":{"branch":"main","commit_hash":"deadbeef","repository_url":"git@github.com:user/repo.git"}}}
{"timestamp":"2026-04-15T11:54:54.493Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T11:54:57.282Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":2638,"cached_input_tokens":0,"output_tokens":30,"reasoning_output_tokens":0,"total_tokens":2668},"last_token_usage":{"input_tokens":2638,"cached_input_tokens":0,"output_tokens":30,"reasoning_output_tokens":0,"total_tokens":2668},"model_context_window":272000}}}
{"timestamp":"2026-04-15T11:55:02.704Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T11:55:25.931Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":5299,"cached_input_tokens":2560,"output_tokens":61,"reasoning_output_tokens":0,"total_tokens":5360},"last_token_usage":{"input_tokens":2661,"cached_input_tokens":2560,"output_tokens":31,"reasoning_output_tokens":0,"total_tokens":2692},"model_context_window":272000}}}
`
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	metas, turns, err := parseCodexJSONLFile(p)
	if err != nil {
		t.Fatalf("parseCodexJSONLFile error: %v", err)
	}

	if len(metas) != 1 {
		t.Fatalf("want 1 session, got %d", len(metas))
	}
	m := metas[0]
	if m.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want %q", m.SessionID, "abc123")
	}
	if m.ProjectName != "user/myproject" {
		t.Errorf("ProjectName = %q, want %q", m.ProjectName, "user/myproject")
	}
	if m.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", m.GitBranch, "main")
	}
	if m.Model != "gpt-5-codex" {
		t.Errorf("Model = %q, want %q", m.Model, "gpt-5-codex")
	}

	if len(turns) != 2 {
		t.Fatalf("want 2 turns, got %d", len(turns))
	}

	// Turn 1: no cached tokens — non-cached input = 2638 - 0 = 2638
	t1 := turns[0]
	if t1.InputTokens != 2638 {
		t.Errorf("turn1 InputTokens = %d, want 2638 (non-cached)", t1.InputTokens)
	}
	if t1.CacheReadTokens != 0 {
		t.Errorf("turn1 CacheReadTokens = %d, want 0", t1.CacheReadTokens)
	}
	if t1.OutputTokens != 30 {
		t.Errorf("turn1 OutputTokens = %d, want 30", t1.OutputTokens)
	}
	if t1.Model != "gpt-5-codex" {
		t.Errorf("turn1 Model = %q, want %q", t1.Model, "gpt-5-codex")
	}
	if t1.CacheCreationTokens != 0 {
		t.Errorf("turn1 CacheCreationTokens = %d, want 0", t1.CacheCreationTokens)
	}

	// Turn 2: input_tokens=2661 total, cached=2560 → non-cached = 2661-2560 = 101
	t2 := turns[1]
	if t2.InputTokens != 101 {
		t.Errorf("turn2 InputTokens = %d, want 101 (non-cached: 2661-2560)", t2.InputTokens)
	}
	if t2.CacheReadTokens != 2560 {
		t.Errorf("turn2 CacheReadTokens = %d, want 2560", t2.CacheReadTokens)
	}
	if t2.OutputTokens != 31 {
		t.Errorf("turn2 OutputTokens = %d, want 31", t2.OutputTokens)
	}
}

// TestParseCodexJSONLFile_FallbackDelta verifies that when last_token_usage is
// absent (older log format), per-turn usage is derived by subtracting the
// previous cumulative total.
func TestParseCodexJSONLFile_FallbackDelta(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "session.jsonl")

	// Only total_token_usage provided — no last_token_usage field.
	content := `{"timestamp":"2026-04-15T12:00:00.000Z","type":"session_meta","payload":{"id":"fallback1","cwd":"/a/b"}}
{"timestamp":"2026-04-15T12:00:01.000Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T12:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":50,"reasoning_output_tokens":0,"total_tokens":1050}}}}
{"timestamp":"2026-04-15T12:00:03.000Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T12:00:04.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":2100,"cached_input_tokens":500,"output_tokens":120,"reasoning_output_tokens":0,"total_tokens":2220}}}}
`
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, turns, err := parseCodexJSONLFile(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns, got %d", len(turns))
	}

	// Turn 1: total input=1000, cached=0 → non-cached = 1000-0 = 1000
	if turns[0].InputTokens != 1000 {
		t.Errorf("turn1 InputTokens = %d, want 1000 (non-cached)", turns[0].InputTokens)
	}
	if turns[0].OutputTokens != 50 {
		t.Errorf("turn1 OutputTokens = %d, want 50", turns[0].OutputTokens)
	}

	// Turn 2: total delta input=2100-1000=1100, cached delta=500-0=500 → non-cached = 1100-500 = 600
	if turns[1].InputTokens != 600 {
		t.Errorf("turn2 InputTokens = %d, want 600 (non-cached: 1100-500)", turns[1].InputTokens)
	}
	if turns[1].CacheReadTokens != 500 {
		t.Errorf("turn2 CacheReadTokens = %d, want 500", turns[1].CacheReadTokens)
	}
	if turns[1].OutputTokens != 70 {
		t.Errorf("turn2 OutputTokens = %d, want 70", turns[1].OutputTokens)
	}
}

// TestParseCodexJSONLFile_SkipsZeroTokenTurns verifies that token_count events
// where all usage fields are zero are not emitted as turns.
func TestParseCodexJSONLFile_SkipsZeroTokenTurns(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "session.jsonl")

	content := `{"timestamp":"2026-04-15T12:00:00.000Z","type":"session_meta","payload":{"id":"zerotest","cwd":"/x/y"}}
{"timestamp":"2026-04-15T12:00:01.000Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T12:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"total_tokens":0},"last_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"total_tokens":0}}}}
{"timestamp":"2026-04-15T12:00:03.000Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T12:00:04.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":20,"total_tokens":520},"last_token_usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":20,"total_tokens":520}}}}
`
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, turns, err := parseCodexJSONLFile(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(turns) != 1 {
		t.Errorf("want 1 turn (zero-token skipped), got %d", len(turns))
	}
}

// TestParseCodexJSONLFile_NoGit verifies that sessions without git metadata parse cleanly.
func TestParseCodexJSONLFile_NoGit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "session.jsonl")

	content := `{"timestamp":"2026-04-15T12:00:00.000Z","type":"session_meta","payload":{"id":"nogit1","cwd":"/home/user/proj"}}
{"timestamp":"2026-04-15T12:00:01.000Z","type":"turn_context","payload":{"model":"gpt-4o"}}
{"timestamp":"2026-04-15T12:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":300,"cached_input_tokens":0,"output_tokens":15,"total_tokens":315}}}}
`
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	metas, turns, err := parseCodexJSONLFile(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("want 1 session, got %d", len(metas))
	}
	if metas[0].GitBranch != "" {
		t.Errorf("GitBranch = %q, want empty", metas[0].GitBranch)
	}
	if len(turns) != 1 {
		t.Errorf("want 1 turn, got %d", len(turns))
	}
}

// ── parseCodexJSONLFileFromLine ───────────────────────────────────────────────

func TestParseCodexJSONLFileFromLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "session.jsonl")

	// 5-line file: session_meta (0), turn_context (1), token_count (2),
	// turn_context (3), token_count (4).
	content := `{"timestamp":"2026-04-15T12:00:00.000Z","type":"session_meta","payload":{"id":"inc1","cwd":"/a/b"}}
{"timestamp":"2026-04-15T12:00:01.000Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T12:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":40,"total_tokens":1040}}}}
{"timestamp":"2026-04-15T12:00:03.000Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T12:00:04.000Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":900,"cached_input_tokens":200,"output_tokens":35,"total_tokens":935}}}}
`
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Only parse turns from line 3 onwards (should see only turn 2).
	_, turns, err := parseCodexJSONLFileFromLine(p, 3)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn (incremental), got %d", len(turns))
	}
	// input_tokens=900 total, cached=200 → non-cached = 900-200 = 700
	if turns[0].InputTokens != 700 {
		t.Errorf("turn InputTokens = %d, want 700 (non-cached: 900-200)", turns[0].InputTokens)
	}
	if turns[0].CacheReadTokens != 200 {
		t.Errorf("turn CacheReadTokens = %d, want 200", turns[0].CacheReadTokens)
	}
}

// ── scanCodex integration ─────────────────────────────────────────────────────

func TestScanCodex(t *testing.T) {
	tmpDir := t.TempDir()
	// Layout: sessions/2026/04/15/rollout-<ts>-<id>.jsonl
	sessDir := filepath.Join(tmpDir, "sessions", "2026", "04", "15")
	if err := os.MkdirAll(sessDir, 0750); err != nil {
		t.Fatal(err)
	}

	content := `{"timestamp":"2026-04-15T11:54:50.291Z","type":"session_meta","payload":{"id":"scantest1","cwd":"/home/user/myproject","git":{"branch":"main","commit_hash":"abc","repository_url":""}}}
{"timestamp":"2026-04-15T11:54:54.493Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-15T11:54:57.282Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":2638,"cached_input_tokens":0,"output_tokens":30,"total_tokens":2668}}}}
`
	filePath := filepath.Join(sessDir, "rollout-2026-04-15T11-54-50-scantest1.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	dbFile := filepath.Join(tmpDir, "usage.db")
	res, err := scanCodex(filepath.Join(tmpDir, "sessions"), dbFile, false)
	if err != nil {
		t.Fatalf("scanCodex error: %v", err)
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

	// Re-scan should skip unchanged file.
	res2, err := scanCodex(filepath.Join(tmpDir, "sessions"), dbFile, false)
	if err != nil {
		t.Fatalf("second scanCodex error: %v", err)
	}
	if res2.SkippedFiles != 1 {
		t.Errorf("second scan SkippedFiles = %d, want 1", res2.SkippedFiles)
	}
	if res2.TurnsAdded != 0 {
		t.Errorf("second scan TurnsAdded = %d, want 0", res2.TurnsAdded)
	}
}

// TestScanCodex_MissingDir verifies that a missing Codex sessions directory
// causes no error (Codex not installed / never used).
func TestScanCodex_MissingDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "usage.db")

	res, err := scanCodex(filepath.Join(tmpDir, "nonexistent", "sessions"), dbFile, false)
	if err != nil {
		t.Errorf("expected no error for missing dir, got: %v", err)
	}
	if res.NewFiles != 0 || res.TurnsAdded != 0 {
		t.Errorf("expected empty result for missing dir, got %+v", res)
	}
}
