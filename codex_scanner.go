// codex_scanner.go - Parses Codex CLI JSONL session logs and stores data in SQLite.
package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// codexDir is the default Codex sessions directory.
// Overridden by the --codex-dir flag or the CODEX_HOME environment variable.
var codexDir = codexDefaultDir()

func codexDefaultDir() string {
	if env := os.Getenv("CODEX_HOME"); env != "" {
		return filepath.Join(env, "sessions")
	}
	return filepath.Join(homeDir(), ".codex", "sessions")
}

// ── JSONL record types ────────────────────────────────────────────────────────

type codexRecord struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMetaPayload struct {
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
	Git *struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

type codexTurnContextPayload struct {
	Model string `json:"model"`
}

type codexTokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type codexEventMsgPayload struct {
	Type string `json:"type"`
	Info *struct {
		LastTokenUsage  *codexTokenUsage `json:"last_token_usage"`
		TotalTokenUsage *codexTokenUsage `json:"total_token_usage"`
	} `json:"info"`
}

// ── Parsing ───────────────────────────────────────────────────────────────────

// parseCodexJSONLFile parses a Codex CLI session JSONL file into session metadata
// and per-turn usage records.
//
// Token counts come from event_msg[token_count] events. Codex provides both
// last_token_usage (per-turn delta) and total_token_usage (cumulative); we
// prefer last_token_usage and fall back to delta-from-totals for older logs
// where last_token_usage may be absent or zero.
func parseCodexJSONLFile(filePath string) ([]sessionMeta, []turn, error) {
	return parseCodexJSONLFileFromLine(filePath, 0)
}

// parseCodexJSONLFileFromLine parses a Codex JSONL file, emitting turns only
// for token_count events found at line index >= startLine (0-based). Session
// metadata and turn_context records are always read from the full file so that
// model and session ID are available even in incremental updates.
func parseCodexJSONLFileFromLine(filePath string, startLine int) ([]sessionMeta, []turn, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var (
		meta         *sessionMeta
		currentModel string
		turns        []turn
		prevTotal    codexTokenUsage
	)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	lineIdx := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			lineIdx++
			continue
		}

		var rec codexRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			lineIdx++
			continue
		}

		switch rec.Type {
		case "session_meta":
			var p codexSessionMetaPayload
			if err := json.Unmarshal(rec.Payload, &p); err != nil {
				break
			}
			branch := ""
			if p.Git != nil {
				branch = p.Git.Branch
			}
			meta = &sessionMeta{
				SessionID:      p.ID,
				ProjectName:    projectNameFromCwd(p.Cwd),
				FirstTimestamp: rec.Timestamp,
				LastTimestamp:  rec.Timestamp,
				GitBranch:      branch,
				Tool:           "codex",
			}

		case "turn_context":
			var p codexTurnContextPayload
			if err := json.Unmarshal(rec.Payload, &p); err != nil {
				break
			}
			if p.Model != "" {
				currentModel = p.Model
			}

		case "event_msg":
			// Always update prevTotal so delta calculations stay correct even
			// when we skip turns before startLine.
			var p codexEventMsgPayload
			if err := json.Unmarshal(rec.Payload, &p); err != nil {
				break
			}
			if p.Type != "token_count" || p.Info == nil {
				break
			}

			var totalInp, cached, out int64

			if ltu := p.Info.LastTokenUsage; ltu != nil && ltu.TotalTokens > 0 {
				// Prefer per-turn delta provided directly by Codex CLI.
				// input_tokens here is the TOTAL input (non-cached + cached combined).
				totalInp = ltu.InputTokens
				cached = ltu.CachedInputTokens
				out = ltu.OutputTokens
			} else if ttu := p.Info.TotalTokenUsage; ttu != nil {
				// Fallback: derive delta from cumulative totals (older log format).
				totalInp = ttu.InputTokens - prevTotal.InputTokens
				cached = ttu.CachedInputTokens - prevTotal.CachedInputTokens
				out = ttu.OutputTokens - prevTotal.OutputTokens
				if totalInp < 0 {
					totalInp = 0
				}
				if cached < 0 {
					cached = 0
				}
				if out < 0 {
					out = 0
				}
			}

			// Codex reports input_tokens as total (non-cached + cached combined).
			// Normalise to non-cached only so calcCost matches Claude's semantics:
			//   InputTokens   = tokens actually processed at full price
			//   CacheReadTokens = tokens served from cache at discount price
			inp := totalInp - cached
			if inp < 0 {
				inp = 0
			}

			// Keep cumulative totals in sync for subsequent delta computations.
			if ttu := p.Info.TotalTokenUsage; ttu != nil {
				prevTotal = *ttu
			}

			// Only emit turns for lines at or after startLine.
			if lineIdx >= startLine && totalInp+out > 0 {
				cwd := ""
				sessionID := ""
				if meta != nil {
					cwd = meta.ProjectName
					sessionID = meta.SessionID
					meta.LastTimestamp = rec.Timestamp
					if currentModel != "" {
						meta.Model = currentModel
					}
				}
				turns = append(turns, turn{
					SessionID:           sessionID,
					Timestamp:           rec.Timestamp,
					Model:               currentModel,
					InputTokens:         inp,
					CacheReadTokens:     cached,
					OutputTokens:        out,
					CacheCreationTokens: 0, // no equivalent in Codex
					Cwd:                 cwd,
					Tool:                "codex",
				})
			}
		}

		lineIdx++
	}

	if err := sc.Err(); err != nil {
		return nil, nil, err
	}

	if meta == nil {
		return nil, turns, nil
	}
	return []sessionMeta{*meta}, turns, nil
}

// ── Scan ──────────────────────────────────────────────────────────────────────

// scanCodex walks the Codex sessions directory, parses JSONL files, and writes
// usage data to the shared SQLite database. It reuses the same processed_files
// table for incremental-update tracking.
func scanCodex(codexSessionsDir, dbP string, verbose bool) (scanResult, error) {
	if _, err := os.Stat(codexSessionsDir); os.IsNotExist(err) {
		return scanResult{}, nil // directory absent — Codex not installed, skip silently
	}

	db, err := openDB(dbP)
	if err != nil {
		return scanResult{}, err
	}
	defer db.Close()

	var jsonlFiles []string
	err = filepath.WalkDir(codexSessionsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			jsonlFiles = append(jsonlFiles, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return scanResult{}, err
	}

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
			rel := filePath
			if r, err := filepath.Rel(codexSessionsDir, filePath); err == nil {
				rel = r
			}
			status := "NEW"
			if !isNew {
				status = "UPD"
			}
			fmt.Printf("  [%s] codex/%s\n", status, rel)
		}

		var newTurns []turn
		var sessions []session

		if isNew {
			metas, ts, err := parseCodexJSONLFile(filePath)
			if err != nil {
				fmt.Printf("  Warning: error reading %s: %v\n", filePath, err)
				continue
			}
			sessions = aggregateSessions(metas, ts)
			newTurns = ts
			res.NewFiles++
		} else {
			currentLines, err := countLines(filePath)
			if err != nil {
				fmt.Printf("  Warning: %v\n", err)
				continue
			}
			if currentLines <= oldLines {
				db.Exec("UPDATE processed_files SET mtime = ? WHERE path = ?", mtime, filePath)
				res.SkippedFiles++
				continue
			}

			// Full parse for session metadata, incremental parse for new turns.
			metas, _, err := parseCodexJSONLFile(filePath)
			if err != nil {
				fmt.Printf("  Warning: error reading %s: %v\n", filePath, err)
				continue
			}
			_, newTurns, err = parseCodexJSONLFileFromLine(filePath, oldLines)
			if err != nil {
				fmt.Printf("  Warning: %v\n", err)
			}
			sessions = aggregateSessions(metas, newTurns)

			// Ensure sessions with no new turns still get timestamp updates.
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

		lineCount, _ := countLines(filePath)
		db.Exec(`
			INSERT OR REPLACE INTO processed_files (path, mtime, lines)
			VALUES (?, ?, ?)`, filePath, mtime, lineCount)
	}

	res.SessionsSeen = len(seenSessions)

	if verbose && (res.NewFiles+res.UpdatedFiles+res.SkippedFiles) > 0 {
		fmt.Printf("\nCodex scan complete:\n")
		fmt.Printf("  New files:     %d\n", res.NewFiles)
		fmt.Printf("  Updated files: %d\n", res.UpdatedFiles)
		fmt.Printf("  Skipped files: %d\n", res.SkippedFiles)
		fmt.Printf("  Turns added:   %d\n", res.TurnsAdded)
		fmt.Printf("  Sessions seen: %d\n", res.SessionsSeen)
	}

	return res, nil
}
