// dashboard.go - Local web dashboard served on localhost:8080.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)


//go:embed web/index.html
var indexHTML string

// ── Dashboard data ────────────────────────────────────────────────────────────

type dailyModelRow struct {
	Day           string `json:"day"`
	Model         string `json:"model"`
	Source        string `json:"source"`
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cache_read"`
	CacheCreation int64  `json:"cache_creation"`
	Turns         int64  `json:"turns"`
}

type sessionRow struct {
	SessionID     string  `json:"session_id"`
	Project       string  `json:"project"`
	Last          string  `json:"last"`
	LastDate      string  `json:"last_date"`
	DurationMin   float64 `json:"duration_min"`
	Model         string  `json:"model"`
	Source        string  `json:"source"`
	Turns         int64   `json:"turns"`
	Input         int64   `json:"input"`
	Output        int64   `json:"output"`
	CacheRead     int64   `json:"cache_read"`
	CacheCreation int64   `json:"cache_creation"`
}

// toolSummary holds aggregated metrics for a single source tool (e.g. "claude"
// or "cursor").
type toolSummary struct {
	Source        string  `json:"source"`
	Sessions      int64   `json:"sessions"`
	Turns         int64   `json:"turns"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	CacheRead     int64   `json:"cache_read"`
	CacheCreation int64   `json:"cache_creation"`
}

type dashboardData struct {
	Error        string          `json:"error,omitempty"`
	AllModels    []string        `json:"all_models"`
	AllSources   []string        `json:"all_sources"`
	ToolsSummary []toolSummary   `json:"tools_summary"`
	DailyByModel []dailyModelRow `json:"daily_by_model"`
	SessionsAll  []sessionRow    `json:"sessions_all"`
	GeneratedAt  string          `json:"generated_at"`
}

func getDashboardData() dashboardData {
	db, err := openDB(dbPath)
	if err != nil {
		return dashboardData{Error: "Cannot open database. Run: clauditor scan"}
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

	// All sources
	sourceRows, _ := db.Query(`
		SELECT COALESCE(source, 'claude') as source
		FROM turns
		GROUP BY source
		ORDER BY source`)
	defer sourceRows.Close()

	var allSources []string
	for sourceRows.Next() {
		var s string
		sourceRows.Scan(&s)
		allSources = append(allSources, s)
	}

	// Per-tool summary
	tsRows, _ := db.Query(`
		SELECT
			COALESCE(source, 'claude')          as source,
			COUNT(DISTINCT session_id)          as sessions,
			COUNT(*)                            as turns,
			SUM(input_tokens)                   as input_tokens,
			SUM(output_tokens)                  as output_tokens,
			SUM(cache_read_tokens)              as cache_read,
			SUM(cache_creation_tokens)          as cache_creation
		FROM turns
		GROUP BY source
		ORDER BY source`)
	defer tsRows.Close()

	var toolsSummary []toolSummary
	for tsRows.Next() {
		var ts toolSummary
		tsRows.Scan(&ts.Source, &ts.Sessions, &ts.Turns,
			&ts.InputTokens, &ts.OutputTokens, &ts.CacheRead, &ts.CacheCreation)
		toolsSummary = append(toolsSummary, ts)
	}

	// Daily per-model (include source)
	drows, _ := db.Query(`
		SELECT
			substr(timestamp, 1, 10)             as day,
			COALESCE(model, 'unknown')           as model,
			COALESCE(source, 'claude')           as source,
			SUM(input_tokens)                    as input,
			SUM(output_tokens)                   as output,
			SUM(cache_read_tokens)               as cache_read,
			SUM(cache_creation_tokens)           as cache_creation,
			COUNT(*)                             as turns
		FROM turns
		GROUP BY day, model, source
		ORDER BY day, source, model`)
	defer drows.Close()

	var dailyByModel []dailyModelRow
	for drows.Next() {
		var r dailyModelRow
		drows.Scan(&r.Day, &r.Model, &r.Source, &r.Input, &r.Output, &r.CacheRead, &r.CacheCreation, &r.Turns)
		dailyByModel = append(dailyByModel, r)
	}

	// All sessions (include source)
	srows, err := db.Query(`
		SELECT
			session_id, COALESCE(project_name,'unknown'), first_timestamp, last_timestamp,
			total_input_tokens, total_output_tokens,
			total_cache_read, total_cache_creation,
			COALESCE(model,'unknown'), turn_count,
			COALESCE(source,'claude')
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
			model                     string
			turns                     int64
			source                    string
		)
		srows.Scan(&sid, &project, &first, &last, &inp, &out, &cr, &cc, &model, &turns, &source)

		durationMin := sessionDurationMin(first, last)
		lastShort := last
		if len(lastShort) > 16 {
			lastShort = lastShort[:16]
		}
		lastShort = replaceT(lastShort)
		lastDate := ""
		if len(last) >= 10 {
			lastDate = last[:10]
		}
		sessionIDShort := sid
		if len(sessionIDShort) > 8 {
			sessionIDShort = sessionIDShort[:8]
		}

		sessionsAll = append(sessionsAll, sessionRow{
			SessionID:     sessionIDShort,
			Project:       project,
			Last:          lastShort,
			LastDate:      lastDate,
			DurationMin:   durationMin,
			Model:         model,
			Source:        source,
			Turns:         turns,
			Input:         inp,
			Output:        out,
			CacheRead:     cr,
			CacheCreation: cc,
		})
	}

	return dashboardData{
		AllModels:    allModels,
		AllSources:   allSources,
		ToolsSummary: toolsSummary,
		DailyByModel: dailyByModel,
		SessionsAll:  sessionsAll,
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

func replaceT(s string) string {
	for i, c := range s {
		if c == 'T' {
			b := []byte(s)
			b[i] = ' '
			return string(b)
		}
	}
	return s
}

// ── HTTP handler ──────────────────────────────────────────────────────────────

func newDashboardMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, indexHTML)
	})

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		scan(projectsDir, dbPath, false)       // rescan Claude on every poll
		scanCursor(cursorDir, dbPath, false)   // rescan Cursor on every poll
		data := getDashboardData()
		body, err := json.Marshal(data)
		if err != nil {
			http.Error(w, "JSON error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	return mux
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

