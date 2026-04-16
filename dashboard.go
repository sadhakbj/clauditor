// dashboard.go - Local web dashboard served on localhost:8080.
package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed all:frontend/dist
var distFS embed.FS

// ── Dashboard data ────────────────────────────────────────────────────────────

type dailyModelRow struct {
	Day           string `json:"day"`
	Model         string `json:"model"`
	Tool          string `json:"tool"`
	SessionID     string `json:"session_id"`
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cache_read"`
	CacheCreation int64  `json:"cache_creation"`
	Turns         int64  `json:"turns"`
}

type sessionRow struct {
	SessionID     string  `json:"session_id"`
	Project       string  `json:"project"`
	FirstTs       int64   `json:"first_ts"`
	LastTs        int64   `json:"last_ts"`
	LastDate      string  `json:"last_date"`
	DurationMin   float64 `json:"duration_min"`
	Model         string  `json:"model"`
	Tool          string  `json:"tool"`
	Turns         int64   `json:"turns"`
	Input         int64   `json:"input"`
	Output        int64   `json:"output"`
	CacheRead     int64   `json:"cache_read"`
	CacheCreation int64   `json:"cache_creation"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

type toolSummaryRow struct {
	Tool          string `json:"tool"`
	Sessions      int64  `json:"sessions"`
	Turns         int64  `json:"turns"`
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cache_read"`
	CacheCreation int64  `json:"cache_creation"`
}

type dashboardData struct {
	Error        string           `json:"error,omitempty"`
	AllModels    []string         `json:"all_models"`
	AllTools     []string         `json:"all_tools"`
	DailyByModel []dailyModelRow  `json:"daily_by_model"`
	SessionsAll  []sessionRow     `json:"sessions_all"`
	ToolSummary  []toolSummaryRow `json:"tool_summary"`
	GeneratedAt  string           `json:"generated_at"`
}

func getDashboardData() dashboardData {
	db, err := openDB(dbPath)
	if err != nil {
		return dashboardData{Error: "Cannot open database. Run: claude-usage scan"}
	}
	defer db.Close()

	// All models
	modelRows, _ := db.Query(`
		SELECT COALESCE(model, 'unknown') as model
		FROM turns
		GROUP BY model
		ORDER BY SUM(input_tokens + output_tokens) DESC`)
	defer modelRows.Close()

	var allModels []string
	for modelRows.Next() {
		var m string
		modelRows.Scan(&m)
		allModels = append(allModels, m)
	}

	// All tools
	toolRows, _ := db.Query(`
		SELECT COALESCE(tool, 'claude_code') as tool
		FROM turns
		GROUP BY tool
		ORDER BY tool`)
	defer toolRows.Close()

	var allTools []string
	for toolRows.Next() {
		var t string
		toolRows.Scan(&t)
		allTools = append(allTools, t)
	}

	// Per-tool summary
	tsrows, _ := db.Query(`
		SELECT
			COALESCE(tool, 'claude_code') as tool,
			COUNT(DISTINCT session_id)    as sessions,
			COUNT(*)                      as turns,
			SUM(input_tokens)             as input,
			SUM(output_tokens)            as output,
			SUM(cache_read_tokens)        as cache_read,
			SUM(cache_creation_tokens)    as cache_creation
		FROM turns
		GROUP BY tool
		ORDER BY tool`)
	defer tsrows.Close()

	var toolSummary []toolSummaryRow
	for tsrows.Next() {
		var r toolSummaryRow
		tsrows.Scan(&r.Tool, &r.Sessions, &r.Turns, &r.Input, &r.Output, &r.CacheRead, &r.CacheCreation)
		toolSummary = append(toolSummary, r)
	}

	// Daily per-model (local date, includes session_id for frontend filtered cost)
	drows, _ := db.Query(`
		SELECT
			substr(datetime(timestamp, 'localtime'), 1, 10) as day,
			COALESCE(model, 'unknown')        as model,
			COALESCE(tool, 'claude_code')     as tool,
			session_id,
			SUM(input_tokens)                 as input,
			SUM(output_tokens)                as output,
			SUM(cache_read_tokens)            as cache_read,
			SUM(cache_creation_tokens)        as cache_creation,
			COUNT(*)                          as turns
		FROM turns
		GROUP BY day, model, tool, session_id
		ORDER BY day, model`)
	defer drows.Close()

	var dailyByModel []dailyModelRow
	for drows.Next() {
		var r dailyModelRow
		drows.Scan(&r.Day, &r.Model, &r.Tool, &r.SessionID, &r.Input, &r.Output, &r.CacheRead, &r.CacheCreation, &r.Turns)
		dailyByModel = append(dailyByModel, r)
	}

	// Per-session cost: sum calcCost per (session_id, model) group
	costBySession := map[string]float64{}
	crows, _ := db.Query(`
		SELECT session_id, COALESCE(model,'unknown'),
			SUM(input_tokens), SUM(output_tokens),
			SUM(cache_read_tokens), SUM(cache_creation_tokens)
		FROM turns
		GROUP BY session_id, model`)
	if crows != nil {
		defer crows.Close()
		for crows.Next() {
			var csid, cmodel string
			var cinp, cout, ccr, ccc int64
			crows.Scan(&csid, &cmodel, &cinp, &cout, &ccr, &ccc)
			costBySession[csid] += calcCost(cmodel, cinp, cout, ccr, ccc)
		}
	}

	// All sessions (includes tool)
	srows, err := db.Query(`
		SELECT
			session_id, COALESCE(project_name,'unknown'), first_timestamp, last_timestamp,
			total_input_tokens, total_output_tokens,
			total_cache_read, total_cache_creation,
			COALESCE(model,'unknown'), turn_count,
			COALESCE(tool,'claude_code')
		FROM sessions
		ORDER BY last_timestamp DESC`)
	if err != nil {
		return dashboardData{Error: "Query error: " + err.Error()}
	}
	defer srows.Close()

	var sessionsAll []sessionRow
	for srows.Next() {
		var (
			sid, project, first, last string
			inp, out, cr, cc          int64
			model, tool               string
			turns                     int64
		)
		srows.Scan(&sid, &project, &first, &last, &inp, &out, &cr, &cc, &model, &turns, &tool)

		durationMin := sessionDurationMin(first, last)
		lastTs := parseTs(last)
		lastDate := ""
		if lastTs > 0 {
			lastDate = time.Unix(lastTs, 0).Local().Format("2006-01-02")
		} else if len(last) >= 10 {
			lastDate = last[:10]
		}

		sessionsAll = append(sessionsAll, sessionRow{
			SessionID:     sid, // full ID — display layer truncates
			Project:       project,
			FirstTs:       parseTs(first),
			LastTs:        parseTs(last),
			LastDate:      lastDate,
			DurationMin:   durationMin,
			Model:         model,
			Tool:          tool,
			Turns:         turns,
			Input:         inp,
			Output:        out,
			CacheRead:     cr,
			CacheCreation: cc,
			TotalCostUSD:  costBySession[sid],
		})
	}

	return dashboardData{
		AllModels:    allModels,
		AllTools:     allTools,
		DailyByModel: dailyByModel,
		SessionsAll:  sessionsAll,
		ToolSummary:  toolSummary,
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
}

func sessionDurationMin(first, last string) float64 {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	var t1, t2 time.Time
	var err1, err2 error
	for _, layout := range layouts {
		t1, err1 = time.Parse(layout, first)
		if err1 == nil {
			break
		}
	}
	for _, layout := range layouts {
		t2, err2 = time.Parse(layout, last)
		if err2 == nil {
			break
		}
	}
	if err1 != nil || err2 != nil {
		return 0
	}
	d := t2.Sub(t1).Minutes()
	if d < 0 {
		return 0
	}
	// Round to 1 decimal
	return float64(int(d*10+0.5)) / 10
}

func parseTs(s string) int64 {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// ── Session detail ────────────────────────────────────────────────────────────

type turnDetailRow struct {
	ID                  int64   `json:"id"`
	SessionID           string  `json:"session_id"`
	TimestampTs         int64   `json:"timestamp_ts"`
	Model               string  `json:"model"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	ToolName            *string `json:"tool_name"`
	Cwd                 *string `json:"cwd"`
	Tool                string  `json:"tool"`
	CostUSD             float64 `json:"cost_usd"`
}

type sessionDetailResponse struct {
	Session sessionRow      `json:"session"`
	Turns   []turnDetailRow `json:"turns"`
}

func getSessionDetail(sessionID string) (sessionDetailResponse, error) {
	db, err := openDB(dbPath)
	if err != nil {
		return sessionDetailResponse{}, err
	}
	defer db.Close()

	row := db.QueryRow(`
		SELECT
			session_id, COALESCE(project_name,'unknown'), first_timestamp, last_timestamp,
			total_input_tokens, total_output_tokens, total_cache_read, total_cache_creation,
			COALESCE(model,'unknown'), turn_count, COALESCE(tool,'claude_code')
		FROM sessions
		WHERE session_id = ?`, sessionID)

	var (
		sid, project, first, last string
		inp, out, cr, cc          int64
		model, tool               string
		turns                     int64
	)
	if err := row.Scan(&sid, &project, &first, &last, &inp, &out, &cr, &cc, &model, &turns, &tool); err != nil {
		return sessionDetailResponse{}, fmt.Errorf("session not found: %w", err)
	}

	durationMin := sessionDurationMin(first, last)
	lastDate := ""
	if len(last) >= 10 {
		lastDate = last[:10]
	}

	sess := sessionRow{
		SessionID:     sid,
		Project:       project,
		FirstTs:       parseTs(first),
		LastTs:        parseTs(last),
		LastDate:      lastDate,
		DurationMin:   durationMin,
		Model:         model,
		Tool:          tool,
		Turns:         turns,
		Input:         inp,
		Output:        out,
		CacheRead:     cr,
		CacheCreation: cc,
	}

	trows, err := db.Query(`
		SELECT
			id, session_id, timestamp, COALESCE(model,'unknown'),
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			tool_name, cwd, COALESCE(tool,'claude_code')
		FROM turns
		WHERE session_id = ?
		ORDER BY timestamp DESC`, sessionID)
	if err != nil {
		return sessionDetailResponse{}, fmt.Errorf("turns query error: %w", err)
	}
	defer trows.Close()

	var turnsList []turnDetailRow
	for trows.Next() {
		var t turnDetailRow
		var tsStr string
		trows.Scan(&t.ID, &t.SessionID, &tsStr, &t.Model,
			&t.InputTokens, &t.OutputTokens, &t.CacheReadTokens, &t.CacheCreationTokens,
			&t.ToolName, &t.Cwd, &t.Tool)
		t.TimestampTs = parseTs(tsStr)
		t.CostUSD = calcCost(t.Model, t.InputTokens, t.OutputTokens, t.CacheReadTokens, t.CacheCreationTokens)
		turnsList = append(turnsList, t)
	}

	return sessionDetailResponse{Session: sess, Turns: turnsList}, nil
}

// ── Session requests (turns grouped by user message) ─────────────────────────

type requestGroup struct {
	Message           *userMessage    `json:"message"`
	PreActionText     string          `json:"pre_action_text"`
	ThinkingTokens    int             `json:"thinking_tokens"`
	AssistantResponse string          `json:"assistant_response"`
	Turns             []turnDetailRow `json:"turns"`
	TotalTokens       int64           `json:"total_tokens"`
	FirstTs           int64           `json:"first_ts"`
	ElapsedSec        int64           `json:"elapsed_sec"`
	CostUSD           float64         `json:"cost_usd"`
}

type conversationEntry struct {
	Role           string
	Timestamp      int64
	Content        string
	ThinkingLen    int // byte length of thinking text (for token estimation)
	HasThinking    bool
}

type sessionRequestsResponse struct {
	Session      sessionRow     `json:"session"`
	Groups       []requestGroup `json:"groups"`
	TotalGroups  int            `json:"total_groups"`
	Page         int            `json:"page"`
	Limit        int            `json:"limit"`
	TotalCostUSD float64        `json:"total_cost_usd"` // sum of per-turn costs across all models
}

// isSyntheticMessage returns true for user messages injected by Claude Code
// (task notifications, local command outputs, image path references, context
// summaries, interruption notices). These are NOT real user-typed messages and
// should be excluded when finding the triggering message for a request group.
func isSyntheticMessage(content string) bool {
	synthetic := []string{
		"<",                        // XML-like injected context (<task-notification>, <local-command-*>, etc.)
		"[Request interrupted",     // "Request interrupted by user for tool use"
		"[Image: source:",          // image path reference injected alongside the real message
		"[Precompacted",            // precompacted context summary
		"This session is being continued from a previous conversation",
	}
	for _, prefix := range synthetic {
		if strings.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}

func findMsgForTurn(msgs []userMessage, turnTs int64) *userMessage {
	var best *userMessage
	for i := range msgs {
		if msgs[i].TimestampTs <= turnTs {
			best = &msgs[i]
		} else {
			break
		}
	}
	return best
}

func getSessionRequests(sessionID string, page, limit int) (sessionRequestsResponse, error) {
	db, err := openDB(dbPath)
	if err != nil {
		return sessionRequestsResponse{}, err
	}
	defer db.Close()

	// Session info
	row := db.QueryRow(`
		SELECT session_id, COALESCE(project_name,'unknown'), first_timestamp, last_timestamp,
			total_input_tokens, total_output_tokens, total_cache_read, total_cache_creation,
			COALESCE(model,'unknown'), turn_count, COALESCE(tool,'claude_code')
		FROM sessions WHERE session_id = ?`, sessionID)
	var sid, project, first, last, model, tool string
	var inp, out, cr, cc, turns int64
	if err := row.Scan(&sid, &project, &first, &last, &inp, &out, &cr, &cc, &model, &turns, &tool); err != nil {
		return sessionRequestsResponse{}, fmt.Errorf("session not found: %w", err)
	}
	lastDate := ""
	if len(last) >= 10 {
		lastDate = last[:10]
	}
	sess := sessionRow{
		SessionID: sid, Project: project,
		FirstTs: parseTs(first), LastTs: parseTs(last), LastDate: lastDate,
		DurationMin: sessionDurationMin(first, last),
		Model: model, Tool: tool, Turns: turns,
		Input: inp, Output: out, CacheRead: cr, CacheCreation: cc,
	}

	// All turns DESC
	trows, err := db.Query(`
		SELECT id, session_id, timestamp, COALESCE(model,'unknown'),
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			tool_name, cwd, COALESCE(tool,'claude_code')
		FROM turns WHERE session_id = ? ORDER BY timestamp DESC`, sessionID)
	if err != nil {
		return sessionRequestsResponse{}, fmt.Errorf("turns query: %w", err)
	}
	defer trows.Close()
	var allTurns []turnDetailRow
	for trows.Next() {
		var t turnDetailRow
		var tsStr string
		trows.Scan(&t.ID, &t.SessionID, &tsStr, &t.Model,
			&t.InputTokens, &t.OutputTokens, &t.CacheReadTokens, &t.CacheCreationTokens,
			&t.ToolName, &t.Cwd, &t.Tool)
		t.TimestampTs = parseTs(tsStr)
		t.CostUSD = calcCost(t.Model, t.InputTokens, t.OutputTokens, t.CacheReadTokens, t.CacheCreationTokens)
		allTurns = append(allTurns, t)
	}

	// Full conversation from JSONL (ASC by timestamp): user + assistant entries
	convo, _ := getSessionConversation(sessionID)

	// Build user-message slice (for turn→message matching)
	var msgs []userMessage
	for _, e := range convo {
		if e.Role == "user" {
			msgs = append(msgs, userMessage{TimestampTs: e.Timestamp, Content: e.Content})
		}
	}

	// Group consecutive turns sharing the same user message
	var groups []requestGroup
	var cur *requestGroup
	var curMsgTs int64 = -1 << 62
	for _, turn := range allTurns {
		msg := findMsgForTurn(msgs, turn.TimestampTs)
		var key int64
		if msg != nil {
			key = msg.TimestampTs
		} else {
			key = -turn.TimestampTs // unique — each ungrouped turn is its own group
		}
		if cur == nil || key != curMsgTs {
			groups = append(groups, requestGroup{
				Message:     msg,
				Turns:       []turnDetailRow{turn},
				TotalTokens: turn.InputTokens + turn.OutputTokens,
				FirstTs:     turn.TimestampTs,
				CostUSD:     turn.CostUSD,
			})
			cur = &groups[len(groups)-1]
			curMsgTs = key
		} else {
			cur.Turns = append(cur.Turns, turn)
			cur.TotalTokens += turn.InputTokens + turn.OutputTokens
			cur.CostUSD += turn.CostUSD
		}
	}

	// Attach pre-action text, thinking tokens, final response, and elapsed time to each group.
	// Groups are DESC; convo is ASC.
	for i := range groups {
		groupTs := groups[i].FirstTs
		var nextTs int64 = 0
		if i+1 < len(groups) {
			nextTs = groups[i+1].FirstTs
		}
		var preActionText, finalResponse string
		var thinkingTokens int
		firstAssistantIdx := -1
		for j, e := range convo {
			if e.Role != "assistant" {
				continue
			}
			if e.Timestamp <= nextTs || e.Timestamp > groupTs+300 {
				continue
			}
			if firstAssistantIdx < 0 {
				// First assistant entry in this window: captures pre-action text + thinking
				firstAssistantIdx = j
				preActionText = e.Content
				thinkingTokens = e.ThinkingLen / 4 // ~4 chars per token
			}
			// Last assistant entry = final response
			if e.Content != "" {
				finalResponse = e.Content
			}
		}
		groups[i].PreActionText = preActionText
		groups[i].ThinkingTokens = thinkingTokens
		// If there's only one assistant entry, pre-action and final are the same — avoid duplication
		if finalResponse != preActionText {
			groups[i].AssistantResponse = finalResponse
		}

		// Compute elapsed time using positional matching: look backwards from the first
		// assistant entry to find the real triggering user message, skipping synthetics.
		// This is more accurate than timestamp-based matching because synthetic user
		// messages (task notifications, image refs, context summaries, etc.) can pollute
		// timestamp-based lookups.
		if firstAssistantIdx > 0 {
			for j := firstAssistantIdx - 1; j >= 0; j-- {
				e := convo[j]
				if e.Role != "user" || e.Content == "" || isSyntheticMessage(e.Content) {
					continue
				}
				groups[i].Message = &userMessage{TimestampTs: e.Timestamp, Content: e.Content}
				if groupTs > e.Timestamp {
					groups[i].ElapsedSec = groupTs - e.Timestamp
				}
				break
			}
		}
		// If positional match found nothing, fall back to the timestamp-based message
		// already assigned during grouping (Message may already be set).
		if groups[i].ElapsedSec == 0 && groups[i].Message != nil && groupTs > groups[i].Message.TimestampTs {
			groups[i].ElapsedSec = groupTs - groups[i].Message.TimestampTs
		}
	}

	// Compute total cost by summing per-turn costs across all models.
	// session.model is the dominant model but sessions can use multiple models,
	// so applying one model's price to all tokens would give a wrong total.
	var totalCostUSD float64
	for _, t := range allTurns {
		totalCostUSD += calcCost(t.Model, t.InputTokens, t.OutputTokens, t.CacheReadTokens, t.CacheCreationTokens)
	}

	totalGroups := len(groups)
	offset := (page - 1) * limit
	if offset > totalGroups {
		offset = totalGroups
	}
	end := offset + limit
	if end > totalGroups {
		end = totalGroups
	}

	return sessionRequestsResponse{
		Session:      sess,
		Groups:       groups[offset:end],
		TotalGroups:  totalGroups,
		Page:         page,
		Limit:        limit,
		TotalCostUSD: totalCostUSD,
	}, nil
}

// ── User messages (read from JSONL) ──────────────────────────────────────────

type userMessage struct {
	TimestampTs int64  `json:"timestamp_ts"`
	Content     string `json:"content"`
}

func getSessionMessages(sessionID string) ([]userMessage, error) {
	// Find the JSONL file by searching project dirs
	jsonlPath := ""
	filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, sessionID+".jsonl") {
			jsonlPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if jsonlPath == "" {
		return nil, fmt.Errorf("conversation file not found")
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type rawRecord struct {
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		Message   struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}

	var msgs []userMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for sc.Scan() {
		var rec rawRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		if rec.Type != "user" || rec.Message.Role != "user" {
			continue
		}
		text := extractText(rec.Message.Content)
		if text == "" {
			continue
		}
		msgs = append(msgs, userMessage{
			TimestampTs: parseTs(rec.Timestamp),
			Content:     text,
		})
	}
	return msgs, sc.Err()
}

// getSessionConversation returns all user + assistant entries from the JSONL, sorted ASC.
func getSessionConversation(sessionID string) ([]conversationEntry, error) {
	jsonlPath := ""
	filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, sessionID+".jsonl") {
			jsonlPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if jsonlPath == "" {
		return nil, fmt.Errorf("conversation file not found")
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type rawRecord struct {
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		Message   struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}

	var raw []conversationEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for sc.Scan() {
		var rec rawRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		role := rec.Message.Role
		if role != "user" && role != "assistant" {
			continue
		}
		text := extractText(rec.Message.Content)
		thinkingLen := 0
		hasThinking := false
		if role == "assistant" {
			t := extractThinking(rec.Message.Content)
			thinkingLen = len(t)
			hasThinking = thinkingLen > 0
		}
		if text == "" && !hasThinking {
			continue
		}
		raw = append(raw, conversationEntry{
			Role:        role,
			Timestamp:   parseTs(rec.Timestamp),
			Content:     text,
			ThinkingLen: thinkingLen,
			HasThinking: hasThinking,
		})
	}

	// Claude Code emits thinking and the following text as two separate assistant
	// messages milliseconds apart. Merge them so preActionText is populated correctly.
	var entries []conversationEntry
	for i := 0; i < len(raw); i++ {
		e := raw[i]
		if e.Role == "assistant" && e.HasThinking && e.Content == "" &&
			i+1 < len(raw) && raw[i+1].Role == "assistant" && !raw[i+1].HasThinking {
			// Absorb the next entry's text into this thinking entry
			e.Content = raw[i+1].Content
			i++ // skip the now-merged next entry
		}
		entries = append(entries, e)
	}

	return entries, sc.Err()
}

func extractThinking(raw json.RawMessage) string {
	var blocks []struct {
		Type     string `json:"type"`
		Thinking string `json:"thinking"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "thinking" && b.Thinking != "" {
				parts = append(parts, b.Thinking)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Plain string
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// Array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// ── CORS middleware ───────────────────────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── HTTP handler ──────────────────────────────────────────────────────────────

func newDashboardMux() http.Handler {
	mux := http.NewServeMux()

	// Serve embedded frontend/dist with SPA fallback for client-side routing
	distSub, err := fs.Sub(distFS, "frontend/dist")
	if err != nil {
		log.Fatalf("Failed to sub distFS: %v", err)
	}
	fileServer := http.FileServer(http.FS(distSub))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Don't intercept /api paths
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Try to open the requested file; fall back to index.html for SPA routing
		f, err := distSub.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			// Serve index.html for all unmatched paths (SPA client-side routing)
			idx, idxErr := distSub.Open("index.html")
			if idxErr != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			defer idx.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.Copy(w, idx) //nolint:errcheck
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		scan(projectsDir, dbPath, false)   // rescan Claude transcripts
		scanCodex(codexDir, dbPath, false) // rescan Codex sessions
		data := getDashboardData()
		body, err := json.Marshal(data)
		if err != nil {
			http.Error(w, "JSON error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		path = strings.TrimSuffix(path, "/")

		// /api/sessions/{id}/requests
		if strings.HasSuffix(path, "/requests") {
			id := strings.TrimSuffix(path, "/requests")
			page, limit := 1, 20
			if v := r.URL.Query().Get("page"); v != "" {
				fmt.Sscanf(v, "%d", &page)
			}
			if v := r.URL.Query().Get("limit"); v != "" {
				fmt.Sscanf(v, "%d", &limit)
			}
			if page < 1 {
				page = 1
			}
			if limit < 1 || limit > 100 {
				limit = 20
			}
			resp, err := getSessionRequests(id, page, limit)
			if err != nil {
				if strings.Contains(err.Error(), "session not found") {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			body, _ := json.Marshal(resp)
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
			return
		}

		// /api/sessions/{id}/messages
		if strings.HasSuffix(path, "/messages") {
			id := strings.TrimSuffix(path, "/messages")
			msgs, err := getSessionMessages(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			body, _ := json.Marshal(msgs)
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
			return
		}

		// /api/sessions/{id}
		id := path
		if id == "" {
			http.Error(w, "session id required", http.StatusBadRequest)
			return
		}

		detail, err := getSessionDetail(id)
		if err != nil {
			if strings.Contains(err.Error(), "session not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		body, err := json.Marshal(detail)
		if err != nil {
			http.Error(w, "JSON error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	return corsMiddleware(mux)
}

func serveDashboard(port int, noBrowser bool) {
	addr := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("Dashboard running at http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop.")

	// Open browser after short delay (unless --no-browser)
	if !noBrowser {
		go func() {
			time.Sleep(time.Second)
			openBrowser(fmt.Sprintf("http://%s", addr))
		}()
	}

	server := &http.Server{
		Addr:    addr,
		Handler: newDashboardMux(),
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

