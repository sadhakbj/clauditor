// cli.go - CLI commands: scan, today, stats, dashboard.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	_ "modernc.org/sqlite"
)

// ── Colors ────────────────────────────────────────────────────────────────────

var (
	cHeader  = color.New(color.FgCyan, color.Bold)
	cTitle   = color.New(color.FgWhite, color.Bold)
	cDim     = color.New(color.FgHiBlack)
	cTotal   = color.New(color.FgWhite, color.Bold)
	cOpus    = color.New(color.FgMagenta, color.Bold)
	cSonnet  = color.New(color.FgCyan, color.Bold)
	cHaiku   = color.New(color.FgGreen, color.Bold)
	cUnknown = color.New(color.FgHiBlack)
	cToken   = color.New(color.FgYellow)
	cCost    = color.New(color.FgGreen)
	cLabel   = color.New(color.FgHiBlack)
	cInfo    = color.New(color.FgHiWhite)
	cProject = color.New(color.FgBlue, color.Bold)
)

func modelColor(model string) *color.Color {
	ml := strings.ToLower(model)
	switch {
	case strings.Contains(ml, "opus"):
		return cOpus
	case strings.Contains(ml, "sonnet"):
		return cSonnet
	case strings.Contains(ml, "haiku"):
		return cHaiku
	default:
		return cUnknown
	}
}

// ── Pricing (Anthropic API, April 2026) ───────────────────────────────────────

type modelPricing struct {
	Input      float64 // $ per million tokens
	Output     float64
	CacheWrite float64
	CacheRead  float64
}

var pricing = map[string]modelPricing{
	"claude-opus-4-6":   {6.15, 30.75, 7.69, 0.61},
	"claude-opus-4-5":   {6.15, 30.75, 7.69, 0.61},
	"claude-sonnet-4-6": {3.69, 18.45, 4.61, 0.37},
	"claude-sonnet-4-5": {3.69, 18.45, 4.61, 0.37},
	"claude-haiku-4-5":  {1.23, 6.15, 1.54, 0.12},
	"claude-haiku-4-6":  {1.23, 6.15, 1.54, 0.12},
}

var defaultPricing = modelPricing{3.69, 18.45, 4.61, 0.37}

func getPricing(model string) (modelPricing, bool) {
	if model == "" {
		return defaultPricing, false
	}
	if p, ok := pricing[model]; ok {
		return p, true
	}
	for key, p := range pricing {
		if strings.HasPrefix(model, key) {
			return p, true
		}
	}
	ml := strings.ToLower(model)
	if strings.Contains(ml, "opus") {
		return pricing["claude-opus-4-6"], true
	}
	if strings.Contains(ml, "sonnet") {
		return pricing["claude-sonnet-4-6"], true
	}
	if strings.Contains(ml, "haiku") {
		return pricing["claude-haiku-4-5"], true
	}
	return defaultPricing, false
}

func isBillable(model string) bool {
	ml := strings.ToLower(model)
	return strings.Contains(ml, "opus") ||
		strings.Contains(ml, "sonnet") ||
		strings.Contains(ml, "haiku")
}

func calcCost(model string, inp, out, cacheRead, cacheCreation int64) float64 {
	if !isBillable(model) {
		return 0
	}
	p, _ := getPricing(model)
	return float64(inp)*p.Input/1e6 +
		float64(out)*p.Output/1e6 +
		float64(cacheRead)*p.CacheRead/1e6 +
		float64(cacheCreation)*p.CacheWrite/1e6
}

// ── Formatting ────────────────────────────────────────────────────────────────

func fmtTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func fmtCost(c float64) string {
	return fmt.Sprintf("$%.4f", c)
}

func hr(char string, width int) {
	cDim.Println(strings.Repeat(char, width))
}

// ── requireDB opens (or creates) the DB, ensuring the schema exists ───────────

func requireDB() *sql.DB {
	db, err := openDB(dbPath)
	if err != nil {
		color.Red("Error opening database: %v", err)
		os.Exit(1)
	}
	if err := initDB(db); err != nil {
		color.Red("Error initializing database: %v", err)
		os.Exit(1)
	}
	return db
}

// ── Commands ──────────────────────────────────────────────────────────────────

func cmdScan() {
	cHeader.Printf("Scanning Claude projects: %s ...\n", projectsDir)
	if _, err := scan(projectsDir, dbPath, true); err != nil {
		color.Red("Scan error: %v", err)
		os.Exit(1)
	}

	cHeader.Printf("\nScanning Cursor data: %s ...\n", cursorDir)
	if _, err := scanCursor(cursorDir, dbPath, true); err != nil {
		color.Red("Cursor scan error: %v", err)
		os.Exit(1)
	}
}

func cmdToday() {
	db := requireDB()
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	type modelRow struct {
		Model string
		Inp   int64
		Out   int64
		CR    int64
		CC    int64
		Turns int64
	}

	rows, err := db.Query(`
		SELECT
			COALESCE(model, 'unknown') as model,
			SUM(input_tokens)          as inp,
			SUM(output_tokens)         as out,
			SUM(cache_read_tokens)     as cr,
			SUM(cache_creation_tokens) as cc,
			COUNT(*)                   as turns
		FROM turns
		WHERE substr(timestamp, 1, 10) = ?
		GROUP BY model
		ORDER BY inp + out DESC`, today)
	if err != nil {
		color.Red("Query error: %v", err)
		os.Exit(1)
	}
	defer rows.Close()

	var modelRows []modelRow
	for rows.Next() {
		var r modelRow
		if err := rows.Scan(&r.Model, &r.Inp, &r.Out, &r.CR, &r.CC, &r.Turns); err != nil {
			continue
		}
		modelRows = append(modelRows, r)
	}

	var sessionCount int64
	db.QueryRow(`
		SELECT COUNT(DISTINCT session_id)
		FROM turns
		WHERE substr(timestamp, 1, 10) = ?`, today).Scan(&sessionCount)

	fmt.Println()
	hr("─", 60)
	cHeader.Printf("  Today's Usage  ")
	cDim.Printf("(%s)\n", today)
	hr("─", 60)

	if len(modelRows) == 0 {
		cDim.Println("  No usage recorded today.")
		fmt.Println()
		return
	}

	var totalInp, totalOut, totalCR, totalCC, totalTurns int64
	var totalCost float64

	for _, r := range modelRows {
		cost := calcCost(r.Model, r.Inp, r.Out, r.CR, r.CC)
		totalCost += cost
		totalInp += r.Inp
		totalOut += r.Out
		totalCR += r.CR
		totalCC += r.CC
		totalTurns += r.Turns

		mc := modelColor(r.Model)
		mc.Printf("  %-30s", r.Model)
		cLabel.Print("  turns=")
		cInfo.Printf("%-4d", r.Turns)
		cLabel.Print("  in=")
		cToken.Printf("%-8s", fmtTokens(r.Inp))
		cLabel.Print("  out=")
		cToken.Printf("%-8s", fmtTokens(r.Out))
		cLabel.Print("  cost=")
		cCost.Println(fmtCost(cost))
	}

	hr("─", 60)
	cTotal.Printf("  %-30s", "TOTAL")
	cLabel.Print("  turns=")
	cTotal.Printf("%-4d", totalTurns)
	cLabel.Print("  in=")
	cToken.Printf("%-8s", fmtTokens(totalInp))
	cLabel.Print("  out=")
	cToken.Printf("%-8s", fmtTokens(totalOut))
	cLabel.Print("  cost=")
	cCost.Println(fmtCost(totalCost))

	fmt.Println()
	cLabel.Print("  Sessions today:   ")
	cInfo.Printf("%d\n", sessionCount)
	cLabel.Print("  Cache read:       ")
	cToken.Printf("%s\n", fmtTokens(totalCR))
	cLabel.Print("  Cache creation:   ")
	cToken.Printf("%s\n", fmtTokens(totalCC))
	hr("─", 60)
	fmt.Println()
}

func cmdStats() {
	db := requireDB()
	defer db.Close()

	var (
		totalInp, totalOut, totalCR, totalCC int64
		totalTurns, totalSessions            int64
		firstDate, lastDate                  string
	)
	db.QueryRow(`
		SELECT
			COALESCE(SUM(total_input_tokens), 0),
			COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cache_read), 0),
			COALESCE(SUM(total_cache_creation), 0),
			COALESCE(SUM(turn_count), 0),
			COUNT(*),
			COALESCE(MIN(first_timestamp), ''),
			COALESCE(MAX(last_timestamp), '')
		FROM sessions`).Scan(
		&totalInp, &totalOut, &totalCR, &totalCC,
		&totalTurns, &totalSessions, &firstDate, &lastDate)

	type modelRow struct {
		Model    string
		Inp      int64
		Out      int64
		CR       int64
		CC       int64
		Turns    int64
		Sessions int64
	}
	rows, _ := db.Query(`
		SELECT
			COALESCE(model, 'unknown') as model,
			COALESCE(SUM(total_input_tokens), 0)   as inp,
			COALESCE(SUM(total_output_tokens), 0)  as out,
			COALESCE(SUM(total_cache_read), 0)     as cr,
			COALESCE(SUM(total_cache_creation), 0) as cc,
			COALESCE(SUM(turn_count), 0)           as turns,
			COUNT(*)                               as sessions
		FROM sessions
		GROUP BY model
		ORDER BY inp + out DESC`)
	defer rows.Close()

	var byModel []modelRow
	for rows.Next() {
		var r modelRow
		rows.Scan(&r.Model, &r.Inp, &r.Out, &r.CR, &r.CC, &r.Turns, &r.Sessions)
		byModel = append(byModel, r)
	}

	type projRow struct {
		Name     string
		Inp      int64
		Out      int64
		Turns    int64
		Sessions int64
	}
	prows, _ := db.Query(`
		SELECT
			COALESCE(project_name, 'unknown') as project_name,
			COALESCE(SUM(total_input_tokens), 0)  as inp,
			COALESCE(SUM(total_output_tokens), 0) as out,
			COALESCE(SUM(turn_count), 0)           as turns,
			COUNT(*)                               as sessions
		FROM sessions
		GROUP BY project_name
		ORDER BY inp + out DESC
		LIMIT 5`)
	defer prows.Close()

	var topProjects []projRow
	for prows.Next() {
		var r projRow
		prows.Scan(&r.Name, &r.Inp, &r.Out, &r.Turns, &r.Sessions)
		topProjects = append(topProjects, r)
	}

	var avgInp, avgOut float64
	db.QueryRow(`
		SELECT
			COALESCE(AVG(daily_inp), 0),
			COALESCE(AVG(daily_out), 0)
		FROM (
			SELECT
				substr(timestamp, 1, 10) as day,
				SUM(input_tokens)  as daily_inp,
				SUM(output_tokens) as daily_out
			FROM turns
			WHERE timestamp >= datetime('now', '-30 days')
			GROUP BY day
		)`).Scan(&avgInp, &avgOut)

	var totalCost float64
	for _, r := range byModel {
		totalCost += calcCost(r.Model, r.Inp, r.Out, r.CR, r.CC)
	}

	if len(firstDate) > 10 {
		firstDate = firstDate[:10]
	}
	if len(lastDate) > 10 {
		lastDate = lastDate[:10]
	}

	fmt.Println()
	hr("═", 60)
	cHeader.Println("  AI Tools Usage  ─  All-Time Statistics")
	hr("═", 60)

	cLabel.Print("  Period:           ")
	cInfo.Printf("%s", firstDate)
	cDim.Print(" → ")
	cInfo.Printf("%s\n", lastDate)
	cLabel.Print("  Total sessions:   ")
	cInfo.Printf("%d\n", totalSessions)
	cLabel.Print("  Total turns:      ")
	cInfo.Printf("%s\n", fmtTokens(totalTurns))

	fmt.Println()
	cLabel.Print("  Input tokens:     ")
	cToken.Printf("%-12s", fmtTokens(totalInp))
	cDim.Println("  (raw prompt tokens)")
	cLabel.Print("  Output tokens:    ")
	cToken.Printf("%-12s", fmtTokens(totalOut))
	cDim.Println("  (generated tokens)")
	cLabel.Print("  Cache read:       ")
	cToken.Printf("%-12s", fmtTokens(totalCR))
	cDim.Println("  (90% cheaper than input)")
	cLabel.Print("  Cache creation:   ")
	cToken.Printf("%-12s", fmtTokens(totalCC))
	cDim.Println("  (25% premium on input)")

	fmt.Println()
	cLabel.Print("  Est. total cost:  ")
	cCost.Printf("$%.4f\n", totalCost)

	hr("─", 60)
	cTitle.Println("  By Model:")
	for _, r := range byModel {
		cost := calcCost(r.Model, r.Inp, r.Out, r.CR, r.CC)
		mc := modelColor(r.Model)
		fmt.Print("    ")
		mc.Printf("%-30s", r.Model)
		cLabel.Print("  sessions=")
		cInfo.Printf("%-4d", r.Sessions)
		cLabel.Print("  turns=")
		cInfo.Printf("%-6s", fmtTokens(r.Turns))
		cLabel.Print("  in=")
		cToken.Printf("%-8s", fmtTokens(r.Inp))
		cLabel.Print("  out=")
		cToken.Printf("%-8s", fmtTokens(r.Out))
		cLabel.Print("  cost=")
		cCost.Println(fmtCost(cost))
	}

	hr("─", 60)
	cTitle.Println("  Top Projects:")
	for _, r := range topProjects {
		fmt.Print("    ")
		cProject.Printf("%-40s", r.Name)
		cLabel.Print("  sessions=")
		cInfo.Printf("%-3d", r.Sessions)
		cLabel.Print("  turns=")
		cInfo.Printf("%-6s", fmtTokens(r.Turns))
		cLabel.Print("  tokens=")
		cToken.Println(fmtTokens(r.Inp + r.Out))
	}

	if avgInp > 0 {
		hr("─", 60)
		cTitle.Println("  Daily Average (last 30 days):")
		cLabel.Print("    Input:   ")
		cToken.Printf("%s\n", fmtTokens(int64(avgInp)))
		cLabel.Print("    Output:  ")
		cToken.Printf("%s\n", fmtTokens(int64(avgOut)))
	}

	hr("═", 60)
	fmt.Println()
}

func cmdDashboard(port int, noBrowser bool) {
	cHeader.Println("Running scan first...")
	cmdScan()

	cHeader.Println("\nStarting dashboard server...")
	serveDashboard(port, noBrowser)
}
