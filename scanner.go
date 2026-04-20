// scanner.go - Scans Claude Code JSONL transcript files and stores data in SQLite.
package main

import (
	"bufio"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var projectsDir = filepath.Join(homeDir(), ".claude", "projects")
var dbPath = filepath.Join(homeDir(), ".claude", "usage.db")

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic("cannot determine home directory: " + err.Error())
	}
	return h
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func openDB(path string) (*sql.DB, error) {
	return openDBWithMigrations(path, true)
}

func openDBWithMigrations(path string, runMigrations bool) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if runMigrations {
		// Run migrations automatically on DB open
		if err := initDB(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}
	}
	return db, nil
}

func initDB(db *sql.DB) error {
	// Create schema_migrations table for tracking
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Run pending migrations
	if err := runMigrations(db); err != nil {
		return err
	}

	return nil
}

func runMigrations(db *sql.DB) error {
	// Get current migration version
	var currentVersion int
	row := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&currentVersion); err != nil && err != sql.ErrNoRows {
		return err
	}

	// Load migration files
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	// Run pending migrations
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
		}
	}

	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	var migrations []migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse version from filename (e.g., 00001_initial_schema.sql)
		parts := strings.Split(entry.Name(), "_")
		if len(parts) < 1 {
			continue
		}
		versionStr := parts[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, filepath.Join("migrations", entry.Name()))
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, migration{
			version: version,
			name:    strings.TrimSuffix(entry.Name(), ".sql"),
			sql:     string(content),
		})
	}

	// Sort by version
	for i := 1; i < len(migrations); i++ {
		for j := i; j > 0 && migrations[j].version < migrations[j-1].version; j-- {
			migrations[j], migrations[j-1] = migrations[j-1], migrations[j]
		}
	}

	return migrations, nil
}

func applyMigration(db *sql.DB, m migration) error {
	// Run migration in transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.Exec(m.sql); err != nil {
		return err
	}

	// Record migration
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
		return err
	}

	return tx.Commit()
}

// ── Data types ────────────────────────────────────────────────────────────────

type sessionMeta struct {
	SessionID      string
	ProjectName    string
	FirstTimestamp string
	LastTimestamp  string
	GitBranch      string
	Model          string
	Tool           string // "claude_code" or "codex"
}

type turn struct {
	SessionID           string
	Timestamp           string
	Model               string
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	ToolName            string
	Cwd                 string
	Tool                string // "claude_code" or "codex"
}

type session struct {
	sessionMeta
	TotalInputTokens   int64
	TotalOutputTokens  int64
	TotalCacheRead     int64
	TotalCacheCreation int64
	TurnCount          int64
}

// ── JSONL parsing ─────────────────────────────────────────────────────────────

// jsonRecord mirrors the structure of each line in a Claude JSONL transcript.
type jsonRecord struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Content []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

func projectNameFromCwd(cwd string) string {
	if cwd == "" {
		return "unknown"
	}
	cwd = strings.TrimRight(strings.ReplaceAll(cwd, "\\", "/"), "/")
	parts := strings.Split(cwd, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) == 1 && parts[0] != "" {
		return parts[0]
	}
	return "unknown"
}

// parseJSONLFile parses a JSONL file and returns session metadata and turns.
// sinceDate, if non-empty (format "2006-01-02"), skips turns before that date.
func parseJSONLFile(filepath, sinceDate string) ([]sessionMeta, []turn, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var turns []turn
	metaMap := map[string]*sessionMeta{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var rec jsonRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}

		if rec.Type != "assistant" && rec.Type != "user" {
			continue
		}
		if rec.SessionID == "" {
			continue
		}

		// Update/create session metadata
		if meta, ok := metaMap[rec.SessionID]; !ok {
			metaMap[rec.SessionID] = &sessionMeta{
				SessionID:      rec.SessionID,
				ProjectName:    projectNameFromCwd(rec.Cwd),
				FirstTimestamp: rec.Timestamp,
				LastTimestamp:  rec.Timestamp,
				GitBranch:      rec.GitBranch,
				Tool:           "claude_code",
			}
		} else {
			if rec.Timestamp != "" {
				if meta.FirstTimestamp == "" || rec.Timestamp < meta.FirstTimestamp {
					meta.FirstTimestamp = rec.Timestamp
				}
				if meta.LastTimestamp == "" || rec.Timestamp > meta.LastTimestamp {
					meta.LastTimestamp = rec.Timestamp
				}
			}
			if rec.GitBranch != "" && meta.GitBranch == "" {
				meta.GitBranch = rec.GitBranch
			}
		}

		if rec.Type != "assistant" {
			continue
		}

		// Skip turns before sinceDate (compare in local time — JSONL timestamps are UTC)
		if sinceDate != "" {
			if t, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
				if t.Local().Format("2006-01-02") < sinceDate {
					continue
				}
			}
		}

		u := rec.Message.Usage
		inp := u.InputTokens
		out := u.OutputTokens
		cr := u.CacheReadInputTokens
		cc := u.CacheCreationInputTokens

		if inp+out+cr+cc == 0 {
			continue
		}

		// Extract tool name from first tool_use content item
		var toolName string
		for _, item := range rec.Message.Content {
			if item.Type == "tool_use" {
				toolName = item.Name
				break
			}
		}

		if rec.Message.Model != "" {
			metaMap[rec.SessionID].Model = rec.Message.Model
		}

		turns = append(turns, turn{
			SessionID:           rec.SessionID,
			Timestamp:           rec.Timestamp,
			Model:               rec.Message.Model,
			InputTokens:         inp,
			OutputTokens:        out,
			CacheReadTokens:     cr,
			CacheCreationTokens: cc,
			ToolName:            toolName,
			Cwd:                 rec.Cwd,
			Tool:                "claude_code",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	metas := make([]sessionMeta, 0, len(metaMap))
	for _, m := range metaMap {
		metas = append(metas, *m)
	}
	return metas, turns, nil
}

// parseJSONLFileFromLine parses only lines at index >= startLine (0-based).
// sinceDate, if non-empty, skips turns before that date.
func parseJSONLFileFromLine(filePath string, startLine int, sinceDate string) ([]turn, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var turns []turn
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	lineIdx := 0
	for sc.Scan() {
		if lineIdx < startLine {
			lineIdx++
			continue
		}
		lineIdx++

		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		var rec jsonRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Type != "assistant" || rec.SessionID == "" {
			continue
		}

		if sinceDate != "" {
			if t, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
				if t.Local().Format("2006-01-02") < sinceDate {
					continue
				}
			}
		}

		u := rec.Message.Usage
		inp := u.InputTokens
		out := u.OutputTokens
		cr := u.CacheReadInputTokens
		cc := u.CacheCreationInputTokens

		if inp+out+cr+cc == 0 {
			continue
		}

		var toolName string
		for _, item := range rec.Message.Content {
			if item.Type == "tool_use" {
				toolName = item.Name
				break
			}
		}

		turns = append(turns, turn{
			SessionID:           rec.SessionID,
			Timestamp:           rec.Timestamp,
			Model:               rec.Message.Model,
			InputTokens:         inp,
			OutputTokens:        out,
			CacheReadTokens:     cr,
			CacheCreationTokens: cc,
			ToolName:            toolName,
			Cwd:                 rec.Cwd,
			Tool:                "claude_code",
		})
	}
	return turns, sc.Err()
}

// countLines counts the number of lines in a file.
func countLines(filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

// ── Session aggregation ───────────────────────────────────────────────────────

func aggregateSessions(metas []sessionMeta, turns []turn) []session {
	type stats struct {
		totalInput  int64
		totalOutput int64
		totalCR     int64
		totalCC     int64
		turnCount   int64
		model       string
	}
	statsMap := map[string]*stats{}
	for _, t := range turns {
		s, ok := statsMap[t.SessionID]
		if !ok {
			s = &stats{}
			statsMap[t.SessionID] = s
		}
		s.totalInput += t.InputTokens
		s.totalOutput += t.OutputTokens
		s.totalCR += t.CacheReadTokens
		s.totalCC += t.CacheCreationTokens
		s.turnCount++
		if t.Model != "" {
			s.model = t.Model
		}
	}

	result := make([]session, 0, len(metas))
	for _, m := range metas {
		s := statsMap[m.SessionID]
		sess := session{sessionMeta: m}
		if s != nil {
			sess.TotalInputTokens = s.totalInput
			sess.TotalOutputTokens = s.totalOutput
			sess.TotalCacheRead = s.totalCR
			sess.TotalCacheCreation = s.totalCC
			sess.TurnCount = s.turnCount
			if s.model != "" {
				sess.Model = s.model
			}
		}
		result = append(result, sess)
	}
	return result
}

// ── DB writes ─────────────────────────────────────────────────────────────────

func upsertSessions(db *sql.DB, sessions []session) error {
	for _, s := range sessions {
		var existing bool
		err := db.QueryRow(
			"SELECT 1 FROM sessions WHERE session_id = ?", s.SessionID,
		).Scan(&existing)

		if err == sql.ErrNoRows {
			_, err = db.Exec(`
				INSERT INTO sessions
					(session_id, project_name, first_timestamp, last_timestamp,
					 git_branch, total_input_tokens, total_output_tokens,
					 total_cache_read, total_cache_creation, model, turn_count, tool)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				s.SessionID, s.ProjectName, s.FirstTimestamp, s.LastTimestamp,
				s.GitBranch, s.TotalInputTokens, s.TotalOutputTokens,
				s.TotalCacheRead, s.TotalCacheCreation, s.Model, s.TurnCount,
				toolOrDefault(s.Tool),
			)
		} else if err == nil {
			_, err = db.Exec(`
				UPDATE sessions SET
					last_timestamp      = MAX(last_timestamp, ?),
					total_input_tokens  = total_input_tokens  + ?,
					total_output_tokens = total_output_tokens + ?,
					total_cache_read    = total_cache_read    + ?,
					total_cache_creation= total_cache_creation+ ?,
					turn_count          = turn_count          + ?,
					model               = COALESCE(?, model)
				WHERE session_id = ?`,
				s.LastTimestamp,
				s.TotalInputTokens, s.TotalOutputTokens,
				s.TotalCacheRead, s.TotalCacheCreation,
				s.TurnCount, nilIfEmpty(s.Model),
				s.SessionID,
			)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func toolOrDefault(t string) string {
	if t == "" {
		return "claude_code"
	}
	return t
}

func insertTurns(db *sql.DB, turns []turn) error {
	if len(turns) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO turns
			(session_id, timestamp, model, input_tokens, output_tokens,
			 cache_read_tokens, cache_creation_tokens, tool_name, cwd, tool)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, t := range turns {
		_, err = stmt.Exec(
			t.SessionID, t.Timestamp, nilIfEmpty(t.Model),
			t.InputTokens, t.OutputTokens,
			t.CacheReadTokens, t.CacheCreationTokens,
			nilIfEmpty(t.ToolName), t.Cwd, toolOrDefault(t.Tool),
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ── Main scan ─────────────────────────────────────────────────────────────────

type scanResult struct {
	NewFiles     int
	UpdatedFiles int
	SkippedFiles int
	TurnsAdded   int
	SessionsSeen int
}

func scan(projDir, dbP string, verbose bool, sinceDate ...string) (scanResult, error) {
	since := ""
	if len(sinceDate) > 0 {
		since = sinceDate[0]
	}
	db, err := openDB(dbP)
	if err != nil {
		return scanResult{}, err
	}
	defer db.Close()

	var jsonlFiles []string
	err = filepath.WalkDir(projDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			jsonlFiles = append(jsonlFiles, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return scanResult{}, err
	}

	// Sort for deterministic order
	sortStrings(jsonlFiles)

	var res scanResult
	seenSessions := map[string]struct{}{}

	for _, filePath := range jsonlFiles {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		mtime := float64(info.ModTime().UnixNano()) / 1e9

		var oldMtime float64
		var oldLines int
		rowErr := db.QueryRow(
			"SELECT mtime, lines FROM processed_files WHERE path = ?", filePath,
		).Scan(&oldMtime, &oldLines)

		isNew := rowErr == sql.ErrNoRows
		if !isNew && abs64(mtime-oldMtime) < 0.01 {
			res.SkippedFiles++
			continue
		}

		if verbose {
			rel, _ := filepath.Rel(projDir, filePath)
			status := "NEW"
			if !isNew {
				status = "UPD"
			}
			fmt.Printf("  [%s] %s\n", status, rel)
		}

		var newTurns []turn
		var sessions []session

		if isNew {
			metas, turns, err := parseJSONLFile(filePath, since)
			if err != nil {
				fmt.Printf("  Warning: error reading %s: %v\n", filePath, err)
				continue
			}
			sessions = aggregateSessions(metas, turns)
			newTurns = turns
			res.NewFiles++
		} else {
			// Incremental: only process new lines
			currentLines, err := countLines(filePath)
			if err != nil {
				fmt.Printf("  Warning: %v\n", err)
				continue
			}
			if currentLines <= oldLines {
				// mtime changed but no new lines — just update mtime
				db.Exec("UPDATE processed_files SET mtime = ? WHERE path = ?", mtime, filePath)
				res.SkippedFiles++
				continue
			}

			// Get full session metadata from full parse (for timestamps)
			metas, _, err := parseJSONLFile(filePath, since)
			if err != nil {
				fmt.Printf("  Warning: error reading %s: %v\n", filePath, err)
				continue
			}

			// Parse only new lines for turns
			newTurns, err = parseJSONLFileFromLine(filePath, oldLines, since)
			if err != nil {
				fmt.Printf("  Warning: %v\n", err)
			}

			sessions = aggregateSessions(metas, newTurns)
			// Ensure sessions with no new turns still get timestamp updates
			sessionIDsWithTurns := map[string]struct{}{}
			for _, s := range sessions {
				sessionIDsWithTurns[s.SessionID] = struct{}{}
			}
			for _, m := range metas {
				if _, ok := sessionIDsWithTurns[m.SessionID]; !ok {
					sessions = append(sessions, session{sessionMeta: m})
				}
			}
			res.UpdatedFiles++
		}

		if err := upsertSessions(db, sessions); err != nil {
			fmt.Printf("  Warning: upsert sessions: %v\n", err)
		}
		if err := insertTurns(db, newTurns); err != nil {
			fmt.Printf("  Warning: insert turns: %v\n", err)
		}

		for _, s := range sessions {
			seenSessions[s.SessionID] = struct{}{}
		}
		res.TurnsAdded += len(newTurns)

		// Record file as processed
		lineCount, _ := countLines(filePath)
		db.Exec(`
			INSERT OR REPLACE INTO processed_files (path, mtime, lines)
			VALUES (?, ?, ?)`, filePath, mtime, lineCount)
	}

	res.SessionsSeen = len(seenSessions)

	if verbose {
		fmt.Printf("\nScan complete:\n")
		fmt.Printf("  New files:     %d\n", res.NewFiles)
		fmt.Printf("  Updated files: %d\n", res.UpdatedFiles)
		fmt.Printf("  Skipped files: %d\n", res.SkippedFiles)
		fmt.Printf("  Turns added:   %d\n", res.TurnsAdded)
		fmt.Printf("  Sessions seen: %d\n", res.SessionsSeen)
	}

	return res, nil
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
