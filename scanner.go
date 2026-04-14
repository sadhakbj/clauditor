// scanner.go - Scans Claude Code JSONL transcript files and stores data in SQLite.
package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var projectsDir = filepath.Join(homeDir(), ".claude", "projects")
var dbPath = filepath.Join(homeDir(), ".claude", "usage.db")

const sourceClaude = "claude"
const sourceCursor = "cursor"

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic("cannot determine home directory: " + err.Error())
	}
	return h
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func openDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func initDB(db *sql.DB) error {
	// Step 1: create tables and indexes that don't depend on the source column.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			session_id              TEXT PRIMARY KEY,
			project_name            TEXT,
			first_timestamp         TEXT,
			last_timestamp          TEXT,
			git_branch              TEXT,
			total_input_tokens      INTEGER DEFAULT 0,
			total_output_tokens     INTEGER DEFAULT 0,
			total_cache_read        INTEGER DEFAULT 0,
			total_cache_creation    INTEGER DEFAULT 0,
			model                   TEXT,
			turn_count              INTEGER DEFAULT 0,
			source                  TEXT DEFAULT 'claude'
		);

		CREATE TABLE IF NOT EXISTS turns (
			id                      INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id              TEXT,
			timestamp               TEXT,
			model                   TEXT,
			input_tokens            INTEGER DEFAULT 0,
			output_tokens           INTEGER DEFAULT 0,
			cache_read_tokens       INTEGER DEFAULT 0,
			cache_creation_tokens   INTEGER DEFAULT 0,
			tool_name               TEXT,
			cwd                     TEXT,
			source                  TEXT DEFAULT 'claude'
		);

		CREATE TABLE IF NOT EXISTS processed_files (
			path    TEXT PRIMARY KEY,
			mtime   REAL,
			lines   INTEGER
		);

		CREATE INDEX IF NOT EXISTS idx_turns_session    ON turns(session_id);
		CREATE INDEX IF NOT EXISTS idx_turns_timestamp  ON turns(timestamp);
		CREATE INDEX IF NOT EXISTS idx_sessions_first   ON sessions(first_timestamp);
	`)
	if err != nil {
		return err
	}

	// Step 2: migrate existing databases (adds source column if absent).
	if err := migrateDB(db); err != nil {
		return err
	}

	// Step 3: create source-dependent indexes now that the column is guaranteed.
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_turns_source    ON turns(source);
		CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source);
	`)
	return err
}

// migrateDB adds new columns to existing databases that were created before
// the multi-source feature was introduced.
func migrateDB(db *sql.DB) error {
	for _, stmt := range []string{
		`ALTER TABLE sessions ADD COLUMN source TEXT DEFAULT 'claude'`,
		`ALTER TABLE turns    ADD COLUMN source TEXT DEFAULT 'claude'`,
	} {
		_, err := db.Exec(stmt)
		// SQLite returns "duplicate column name" if the column already exists;
		// that is not an error we need to surface.
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return nil
}

// ── Data types ────────────────────────────────────────────────────────────────

type sessionMeta struct {
	SessionID      string
	ProjectName    string
	FirstTimestamp string
	LastTimestamp  string
	GitBranch      string
	Model          string
	Source         string
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
	Source              string
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
		Model   string `json:"model"`
		Usage   struct {
			InputTokens          int64 `json:"input_tokens"`
			OutputTokens         int64 `json:"output_tokens"`
			CacheReadInputTokens int64 `json:"cache_read_input_tokens"`
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
func parseJSONLFile(filepath string) ([]sessionMeta, []turn, error) {
	return parseJSONLFileWithSource(filepath, sourceClaude)
}

// parseJSONLFileWithSource parses a JSONL file and tags every record with the
// provided source identifier.
func parseJSONLFileWithSource(filePath, source string) ([]sessionMeta, []turn, error) {
	f, err := os.Open(filePath)
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
				Source:         source,
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
			Source:              source,
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
func parseJSONLFileFromLine(filePath string, startLine int) ([]turn, error) {
	return parseJSONLFileFromLineWithSource(filePath, startLine, sourceClaude)
}

// parseJSONLFileFromLineWithSource parses lines starting at startLine and tags
// each turn with the provided source.
func parseJSONLFileFromLineWithSource(filePath string, startLine int, source string) ([]turn, error) {
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
			Source:              source,
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
		totalInput   int64
		totalOutput  int64
		totalCR      int64
		totalCC      int64
		turnCount    int64
		model        string
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
		src := s.Source
		if src == "" {
			src = sourceClaude
		}
		var existing bool
		err := db.QueryRow(
			"SELECT 1 FROM sessions WHERE session_id = ?", s.SessionID,
		).Scan(&existing)

		if err == sql.ErrNoRows {
			_, err = db.Exec(`
				INSERT INTO sessions
					(session_id, project_name, first_timestamp, last_timestamp,
					 git_branch, total_input_tokens, total_output_tokens,
					 total_cache_read, total_cache_creation, model, turn_count, source)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				s.SessionID, s.ProjectName, s.FirstTimestamp, s.LastTimestamp,
				s.GitBranch, s.TotalInputTokens, s.TotalOutputTokens,
				s.TotalCacheRead, s.TotalCacheCreation, s.Model, s.TurnCount, src,
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
			 cache_read_tokens, cache_creation_tokens, tool_name, cwd, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, t := range turns {
		src := t.Source
		if src == "" {
			src = sourceClaude
		}
		_, err = stmt.Exec(
			t.SessionID, t.Timestamp, nilIfEmpty(t.Model),
			t.InputTokens, t.OutputTokens,
			t.CacheReadTokens, t.CacheCreationTokens,
			nilIfEmpty(t.ToolName), t.Cwd, src,
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

func scan(projDir, dbP string, verbose bool) (scanResult, error) {
	db, err := openDB(dbP)
	if err != nil {
		return scanResult{}, err
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		return scanResult{}, err
	}

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
			metas, turns, err := parseJSONLFile(filePath)
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
			metas, _, err := parseJSONLFile(filePath)
			if err != nil {
				fmt.Printf("  Warning: error reading %s: %v\n", filePath, err)
				continue
			}

			// Parse only new lines for turns
			newTurns, err = parseJSONLFileFromLine(filePath, oldLines)
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
