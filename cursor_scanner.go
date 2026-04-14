// cursor_scanner.go - Scans Cursor AI editor usage data from local SQLite storage.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// cursorDir is the root of Cursor's user-data directory. It is set at startup
// and can be overridden via the --cursor-dir flag.
var cursorDir = defaultCursorDir()

// defaultCursorDir returns the platform-specific default location of Cursor's
// user-data directory.
func defaultCursorDir() string {
	home := homeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Cursor")
		}
		return filepath.Join(home, "AppData", "Roaming", "Cursor")
	default: // linux and others
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "Cursor")
		}
		return filepath.Join(home, ".config", "Cursor")
	}
}

// ── Cursor data types ─────────────────────────────────────────────────────────

// cursorConversation represents a single conversation record stored by Cursor.
type cursorConversation struct {
	ID        string                `json:"id"`
	Workspace string                `json:"workspace,omitempty"`
	CreatedAt string                `json:"createdAt"`
	UpdatedAt string                `json:"updatedAt,omitempty"`
	// Cursor stores per-request records under "turns" or "messages" depending
	// on the version.
	Turns    []cursorTurnRecord    `json:"turns,omitempty"`
	Messages []cursorMessageRecord `json:"messages,omitempty"`
}

// cursorTurnRecord is the format used in newer Cursor versions.
type cursorTurnRecord struct {
	Model     string `json:"model"`
	Timestamp string `json:"timestamp,omitempty"`
	Tokens    struct {
		InputTokens  int64 `json:"inputTokens"`
		OutputTokens int64 `json:"outputTokens"`
	} `json:"tokens,omitempty"`
	// Some versions expose OpenAI-style usage fields.
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

// cursorMessageRecord is the format used in older Cursor versions.
type cursorMessageRecord struct {
	Role  string `json:"role"` // "assistant" | "user"
	Model string `json:"model,omitempty"`
	Usage *struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		// OpenAI-compatible naming
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ── SQLite state.vscdb reader ─────────────────────────────────────────────────

// readCursorStateDB opens a VS Code/Cursor state.vscdb SQLite file and returns
// the raw JSON values for any keys that look like conversation data.
func readCursorStateDB(dbFile string) ([]json.RawMessage, error) {
	db, err := sql.Open("sqlite", dbFile+"?_pragma=journal_mode(WAL)&mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Cursor stores key-value data in ItemTable (VS Code storage format).
	rows, err := db.Query(`
		SELECT value FROM ItemTable
		WHERE key LIKE '%conversation%'
		   OR key LIKE '%aiChat%'
		   OR key LIKE '%composer%'
		   OR key LIKE '%chat%'
		ORDER BY key`)
	if err != nil {
		// Table may not exist in all Cursor versions.
		return nil, nil
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if len(raw) > 0 && (raw[0] == '[' || raw[0] == '{') {
			results = append(results, json.RawMessage(raw))
		}
	}
	return results, rows.Err()
}

// ── Conversation parsing ──────────────────────────────────────────────────────

// parseCursorConversations converts raw JSON blobs read from Cursor's state DB
// into normalised sessions and turns.
func parseCursorConversations(blobs []json.RawMessage, workspace string) ([]sessionMeta, []turn) {
	var metas []sessionMeta
	var turns []turn

	for _, blob := range blobs {
		if len(blob) == 0 {
			continue
		}
		// The value may be a single conversation object or an array of them.
		var convs []cursorConversation
		if blob[0] == '[' {
			if err := json.Unmarshal(blob, &convs); err != nil {
				continue
			}
		} else {
			var c cursorConversation
			if err := json.Unmarshal(blob, &c); err != nil {
				continue
			}
			convs = []cursorConversation{c}
		}

		for _, conv := range convs {
			if conv.ID == "" {
				continue
			}
			sessionID := "cursor:" + conv.ID
			ts := conv.CreatedAt
			lastTS := conv.UpdatedAt
			if lastTS == "" {
				lastTS = ts
			}
			proj := projectNameFromCwd(conv.Workspace)
			if proj == "unknown" && workspace != "" {
				proj = projectNameFromCwd(workspace)
			}

			var model string
			var sessionTurns []turn

			// Normalise turns from the "turns" field.
			for _, t := range conv.Turns {
				inp := t.Tokens.InputTokens
				out := t.Tokens.OutputTokens
				if inp == 0 && out == 0 {
					inp = t.Usage.PromptTokens
					out = t.Usage.CompletionTokens
				}
				if inp+out == 0 {
					continue
				}
				tm := t.Timestamp
				if tm == "" {
					tm = ts
				}
				if t.Model != "" {
					model = t.Model
				}
				sessionTurns = append(sessionTurns, turn{
					SessionID:    sessionID,
					Timestamp:    tm,
					Model:        t.Model,
					InputTokens:  inp,
					OutputTokens: out,
					Cwd:          conv.Workspace,
					Source:       sourceCursor,
				})
			}

			// Normalise turns from the "messages" field.
			for _, msg := range conv.Messages {
				if msg.Role != "assistant" || msg.Usage == nil {
					continue
				}
				inp := msg.Usage.InputTokens
				out := msg.Usage.OutputTokens
				if inp == 0 && out == 0 {
					inp = msg.Usage.PromptTokens
					out = msg.Usage.CompletionTokens
				}
				if inp+out == 0 {
					continue
				}
				tm := msg.Timestamp
				if tm == "" {
					tm = ts
				}
				if msg.Model != "" {
					model = msg.Model
				}
				sessionTurns = append(sessionTurns, turn{
					SessionID:    sessionID,
					Timestamp:    tm,
					Model:        msg.Model,
					InputTokens:  inp,
					OutputTokens: out,
					Cwd:          conv.Workspace,
					Source:       sourceCursor,
				})
			}

			if len(sessionTurns) == 0 {
				continue
			}

			turns = append(turns, sessionTurns...)
			metas = append(metas, sessionMeta{
				SessionID:      sessionID,
				ProjectName:    proj,
				FirstTimestamp: ts,
				LastTimestamp:  lastTS,
				Model:          model,
				Source:         sourceCursor,
			})
		}
	}
	return metas, turns
}

// ── Main Cursor scan ──────────────────────────────────────────────────────────

// scanCursor walks Cursor's user-data directory, reads conversation data from
// every state.vscdb it finds, and stores the normalised records in the shared
// SQLite database.
func scanCursor(curDataDir, dbP string, verbose bool) (scanResult, error) {
	if _, err := os.Stat(curDataDir); os.IsNotExist(err) {
		if verbose {
			fmt.Printf("  Cursor data directory not found (%s) — skipping\n", curDataDir)
		}
		return scanResult{}, nil
	}

	db, err := openDB(dbP)
	if err != nil {
		return scanResult{}, err
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		return scanResult{}, err
	}

	// Walk Cursor's User directory looking for state.vscdb files.
	userDir := filepath.Join(curDataDir, "User")
	var stateDBs []string
	_ = filepath.WalkDir(userDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == "state.vscdb" {
			stateDBs = append(stateDBs, path)
		}
		return nil
	})

	if len(stateDBs) == 0 {
		if verbose {
			fmt.Printf("  No Cursor state databases found in %s\n", userDir)
		}
		return scanResult{}, nil
	}

	var res scanResult
	seenSessions := map[string]struct{}{}

	for _, stateDB := range stateDBs {
		// Determine workspace from the path (workspaceStorage/<hash>/)
		workspace := ""
		parts := strings.Split(stateDB, string(os.PathSeparator))
		for i, p := range parts {
			if p == "workspaceStorage" && i+2 < len(parts) {
				workspace = parts[i+2]
				break
			}
		}

		blobs, err := readCursorStateDB(stateDB)
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: reading %s: %v\n", stateDB, err)
			}
			continue
		}
		if len(blobs) == 0 {
			continue
		}

		metas, newTurns := parseCursorConversations(blobs, workspace)
		if len(newTurns) == 0 {
			continue
		}

		sessions := aggregateSessions(metas, newTurns)
		// Tag sessions with Source so aggregateSessions result inherits it.
		for i := range sessions {
			if sessions[i].Source == "" {
				sessions[i].Source = sourceCursor
			}
		}

		if err := upsertSessions(db, sessions); err != nil {
			if verbose {
				fmt.Printf("  Warning: upsert cursor sessions: %v\n", err)
			}
		}
		if err := insertTurns(db, newTurns); err != nil {
			if verbose {
				fmt.Printf("  Warning: insert cursor turns: %v\n", err)
			}
		}

		for _, s := range sessions {
			seenSessions[s.SessionID] = struct{}{}
		}
		res.TurnsAdded += len(newTurns)
		res.NewFiles++
	}

	res.SessionsSeen = len(seenSessions)

	if verbose {
		fmt.Printf("\nCursor scan complete:\n")
		fmt.Printf("  State DBs scanned: %d\n", len(stateDBs))
		fmt.Printf("  Turns added:       %d\n", res.TurnsAdded)
		fmt.Printf("  Sessions seen:     %d\n", res.SessionsSeen)
	}

	return res, nil
}
