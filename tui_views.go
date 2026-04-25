// tui_views.go - Render functions for each TUI view.
package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/charmbracelet/lipgloss"
)

var (
	colInput  = lipgloss.Color("#6366f1") // blue/indigo  – matches web Input
	colOutput = lipgloss.Color("#c084fc") // purple/violet – matches web Output
	// colEm    = cache read  (green,  defined in tui.go)
	// colAmber = cache creation (amber, defined in tui.go)
	colQuestionBorder = lipgloss.Color("#0ea5e9")
	colAnswerBorder   = lipgloss.Color("#10b981")
)

// ── Overview view ─────────────────────────────────────────────────────────────

// chartPanelH is the fixed total height (borders included) of each chart panel.
const chartPanelH = 22

func renderOverview(m tuiModel) string {
	if m.data == nil {
		return ""
	}

	cutoff := rangeCutoff(m.dateRange)

	var totalInp, totalOut, totalCR, totalCC, totalTurns int64
	totalSessions := 0
	totalCost := 0.0
	for _, s := range m.data.SessionsAll {
		if cutoff != "" && s.LastDate < cutoff {
			continue
		}
		totalInp += s.Input
		totalOut += s.Output
		totalCR += s.CacheRead
		totalCC += s.CacheCreation
		totalTurns += s.Turns
		totalSessions++
		totalCost += calcCost(s.Model, s.Input, s.Output, s.CacheRead, s.CacheCreation)
	}

	rangeDesc := strings.ToLower(rangeLabels[m.dateRange])

	// Row 1: Sessions | Turns | Input Tokens | Output Tokens
	row1 := renderKPIRow4(m.width, totalSessions, totalTurns, totalInp, totalOut, rangeDesc)
	// Row 2: Cache Read | Cache Creation | Est. Cost
	row2 := renderKPIRow3(m.width, totalCR, totalCC, totalCost)
	charts := renderChartsSideBySide(m, cutoff)

	return row1 + "\n\n" + row2 + "\n\n" + charts
}

// renderKPIRow4 renders four metric cards: Sessions, Turns, Input, Output.
func renderKPIRow4(totalW, sessions int, turns, inp, out int64, rangeDesc string) string {
	innerW := totalW - 4
	cardW := (innerW - 3) / 4

	cards := []struct{ label, value, desc string }{
		{"SESSIONS", fmt.Sprintf("%d", sessions), rangeDesc},
		{"TURNS", fmtTokens(turns), "total API calls"},
		{"INPUT TOKENS", fmtTokens(inp), "prompt tokens"},
		{"OUTPUT TOKENS", fmtTokens(out), "generated tokens"},
	}

	var cells []string
	for _, c := range cards {
		label := lipgloss.NewStyle().Foreground(colDim).Bold(true).Render(c.label)
		value := lipgloss.NewStyle().Foreground(colText).Bold(true).Render(c.value)
		desc := styleSub.Render(c.desc)
		cell := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(0, 1).
			Width(cardW).
			Render(label + "\n" + value + "\n" + desc)
		cells = append(cells, cell)
	}
	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, cells...)
}

// renderKPIRow3 renders three wide cards: Cache Read, Cache Creation, Cost.
func renderKPIRow3(totalW int, cr, cc int64, cost float64) string {
	innerW := totalW - 4
	cardW := (innerW - 2) / 3

	crContent := lipgloss.NewStyle().Foreground(colAmber).Render("⚡") + " " +
		lipgloss.NewStyle().Foreground(colText).Bold(true).Render(fmtTokens(cr)) + "\n" +
		styleSub.Render("Cache Read · 90% cheaper")

	ccContent := styleSub.Render("✦") + " " +
		lipgloss.NewStyle().Foreground(colText).Bold(true).Render(fmtTokens(cc)) + "\n" +
		styleSub.Render("Cache Creation · 25% premium")

	costContent := styleDim.Render("$ ") +
		styleKPICost.Render(fmt.Sprintf("$%.2f", cost)) + "\n" +
		styleSub.Render("Est. Cost · API pricing")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(cardW)

	return "  " + lipgloss.JoinHorizontal(lipgloss.Top,
		boxStyle.Render(crContent),
		" ",
		boxStyle.Render(ccContent),
		" ",
		boxStyle.Render(costContent),
	)
}

// ── Charts side-by-side ───────────────────────────────────────────────────────

func renderChartsSideBySide(m tuiModel, cutoff string) string {
	innerW := m.width - 4

	rightW := innerW * 35 / 100
	if rightW < 28 {
		rightW = 28
	}
	if rightW > 52 {
		rightW = 52
	}
	leftW := innerW - rightW - 1

	left := renderDailyChartNT(m, cutoff, leftW, chartPanelH)
	right := renderModelDonut(m, cutoff, rightW, chartPanelH)

	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

// ── Daily stacked bar chart (ntcharts) ────────────────────────────────────────

func renderDailyChartNT(m tuiModel, cutoff string, panelW, panelH int) string {
	type dayTok struct{ inp, out, cr, cc int64 }
	dayMap := map[string]*dayTok{}

	for _, r := range m.data.DailyByModel {
		if cutoff != "" && r.Day < cutoff {
			continue
		}
		d := dayMap[r.Day]
		if d == nil {
			d = &dayTok{}
			dayMap[r.Day] = d
		}
		d.inp += r.Input
		d.out += r.Output
		d.cr += r.CacheRead
		d.cc += r.CacheCreation
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(panelW).
		Height(panelH - 2)

	title := styleTitle.Render("Daily Token Usage — " + strings.ToUpper(rangeLabels[m.dateRange]))

	if len(dayMap) == 0 {
		return boxStyle.Render(title + "\n\n" + styleDim.Render("No data"))
	}

	// Sort days, keep last 14
	days := make([]string, 0, len(dayMap))
	for d := range dayMap {
		days = append(days, d)
	}
	sort.Strings(days)
	if len(days) > 14 {
		days = days[len(days)-14:]
	}

	// Legend line
	type legItem struct {
		label string
		col   lipgloss.Color
	}
	legItems := []legItem{
		{"Input", colInput},
		{"Output", colOutput},
		{"Cache Read", colEm},
		{"Cache Creation", colAmber},
	}
	var legParts []string
	for _, li := range legItems {
		dot := lipgloss.NewStyle().Foreground(li.col).Render("■")
		legParts = append(legParts, dot+" "+styleSub.Render(li.label))
	}
	legend := strings.Join(legParts, "  ")

	// Chart area: box border+padding = 4 width overhead; title+legend+blank+borders = 7 height overhead
	chartW := panelW - 4
	chartH := panelH - 7
	if chartH < 5 {
		chartH = 5
	}

	// Build BarData
	barData := make([]barchart.BarData, 0, len(days))
	for _, day := range days {
		d := dayMap[day]
		barData = append(barData, barchart.BarData{
			Label: day[5:], // MM-DD
			Values: []barchart.BarValue{
				{Name: "Input", Value: float64(d.inp), Style: lipgloss.NewStyle().Foreground(colInput).Background(colInput)},
				{Name: "Output", Value: float64(d.out), Style: lipgloss.NewStyle().Foreground(colOutput).Background(colOutput)},
				{Name: "Cache Read", Value: float64(d.cr), Style: lipgloss.NewStyle().Foreground(colEm).Background(colEm)},
				{Name: "Cache Creation", Value: float64(d.cc), Style: lipgloss.NewStyle().Foreground(colAmber).Background(colAmber)},
			},
		})
	}

	bc := barchart.New(chartW, chartH,
		barchart.WithDataSet(barData),
		barchart.WithStyles(
			lipgloss.NewStyle().Foreground(colDim),
			lipgloss.NewStyle().Foreground(colSub),
		),
	)
	bc.Draw()

	content := title + "\n" + legend + "\n\n" + bc.View()
	return boxStyle.Render(content)
}

// ── By-Model donut chart (ASCII) ──────────────────────────────────────────────

func renderModelDonut(m tuiModel, cutoff string, panelW, panelH int) string {
	type modelAgg struct {
		model            string
		inp, out, cr, cc int64
	}
	aggMap := map[string]*modelAgg{}
	var totalTokens int64

	for _, s := range m.data.SessionsAll {
		if cutoff != "" && s.LastDate < cutoff {
			continue
		}
		a := aggMap[s.Model]
		if a == nil {
			a = &modelAgg{model: s.Model}
			aggMap[s.Model] = a
		}
		a.inp += s.Input
		a.out += s.Output
		a.cr += s.CacheRead
		a.cc += s.CacheCreation
		totalTokens += s.Input + s.Output + s.CacheRead + s.CacheCreation
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(panelW).
		Height(panelH - 2)

	title := styleTitle.Render("By Model")

	if totalTokens == 0 {
		return boxStyle.Render(title + "\n\n" + styleDim.Render("No data"))
	}

	aggs := make([]*modelAgg, 0, len(aggMap))
	for _, a := range aggMap {
		aggs = append(aggs, a)
	}
	sort.Slice(aggs, func(i, j int) bool {
		ai := aggs[i].inp + aggs[i].out + aggs[i].cr + aggs[i].cc
		aj := aggs[j].inp + aggs[j].out + aggs[j].cr + aggs[j].cc
		return ai > aj
	})

	// Build segments
	type seg struct {
		start, end float64
		col        lipgloss.Color
	}
	var segments []seg
	var cum float64
	for _, a := range aggs {
		tokens := a.inp + a.out + a.cr + a.cc
		pct := float64(tokens) / float64(totalTokens)
		end := cum + pct*2*math.Pi
		segments = append(segments, seg{cum, end, modelFamilyColor(a.model)})
		cum = end
	}

	// Donut dimensions (W ≈ 2H for circular appearance in typical terminals)
	contentW := panelW - 4 // 2 border + 2 padding
	// content = title(1) + "\n\n"(2) + donut + "\n"(1) + legend(len(aggs)) = donut + len(aggs) + 4
	// must fit in box inner height = panelH - 2 (borders), so donut = panelH - 6 - len(aggs)
	donutH := panelH - 6 - len(aggs)
	if donutH > contentW/2 {
		donutH = contentW / 2
	}
	if donutH < 4 {
		donutH = 4
	}
	donutW := donutH * 2
	if donutW > contentW {
		donutW = contentW &^ 1 // round down to even
		donutH = donutW / 2
	}

	cx := float64(donutW-1) / 2.0
	cy := float64(donutH-1) / 2.0
	// maxR: radius in chars such that the donut looks circular
	// (scaling dy by 2 to account for 2:1 pixel aspect ratio of terminal chars)
	maxR := math.Min(cx, cy*2)

	var donutRows []string
	for y := 0; y < donutH; y++ {
		var line strings.Builder
		dy := float64(y) - cy
		for x := 0; x < donutW; x++ {
			dx := float64(x) - cx
			dist := math.Sqrt(dx*dx+dy*dy*4) / maxR
			if dist >= 0.44 && dist <= 1.0 {
				angle := math.Atan2(dy*2, dx)
				if angle < 0 {
					angle += 2 * math.Pi
				}
				col := segments[len(segments)-1].col
				for _, s := range segments {
					if angle >= s.start && angle < s.end {
						col = s.col
						break
					}
				}
				line.WriteString(lipgloss.NewStyle().Foreground(col).Background(col).Render("█"))
			} else {
				line.WriteByte(' ')
			}
		}
		donutRows = append(donutRows, line.String())
	}

	// Center donut horizontally
	leftPad := strings.Repeat(" ", (contentW-donutW)/2)
	var donutLines []string
	for _, row := range donutRows {
		donutLines = append(donutLines, leftPad+row)
	}
	donutStr := strings.Join(donutLines, "\n")

	// Legend
	var legLines []string
	for _, a := range aggs {
		tokens := a.inp + a.out + a.cr + a.cc
		pct := float64(tokens) / float64(totalTokens)
		col := modelFamilyColor(a.model)
		dot := lipgloss.NewStyle().Foreground(col).Render("●")
		name := styleSub.Render(truncate(a.model, contentW-9))
		pctS := styleDim.Render(fmt.Sprintf(" %.0f%%", pct*100))
		legLines = append(legLines, dot+" "+name+pctS)
	}

	content := title + "\n\n" + donutStr + "\n" + strings.Join(legLines, "\n")
	return boxStyle.Render(content)
}

// ── Sessions view ─────────────────────────────────────────────────────────────

func renderSessions(m tuiModel, _ int) string {
	var header strings.Builder
	header.WriteString("  ")
	if m.filterMode || m.filterText != "" {
		filterStyle := lipgloss.NewStyle().Foreground(colAmber).Bold(true)
		header.WriteString(filterStyle.Render("filter: /" + m.filterText))
		if m.filterMode {
			header.WriteString(filterStyle.Render("▌"))
		}
		header.WriteString(styleDim.Render("  (esc to clear)"))
	} else {
		header.WriteString(styleDim.Render("Press / to filter by project or model"))
	}
	header.WriteString("\n")

	tbl := lipgloss.NewStyle().MarginLeft(2).Render(m.sessionTable.View())
	return header.String() + "\n" + tbl
}

// ── Models view ───────────────────────────────────────────────────────────────

func renderModels(m tuiModel, _ int) string {
	tbl := lipgloss.NewStyle().MarginLeft(2).Render(m.modelTable.View())
	return "\n" + tbl
}

func renderSessionRequests(m tuiModel, _ int) string {
	if m.selectedSession == nil {
		return center(styleDim.Render("No session selected"), m.width, m.height-2)
	}
	if m.detailLoading {
		return center(m.spinner.View()+"  Loading session requests…", m.width, m.height-2)
	}
	if m.detailErr != "" {
		return center(lipgloss.NewStyle().Foreground(colRed).Render("✗ "+m.detailErr), m.width, m.height-2)
	}
	if m.selectedSessionReqs == nil {
		return center(styleDim.Render("No request data"), m.width, m.height-2)
	}

	var parts []string
	parts = append(parts, renderSessionHeader(*m.selectedSessionReqs))
	parts = append(parts, "")

	for i, group := range m.selectedSessionReqs.Groups {
		parts = append(parts, renderRequestSummaryItem(group, i, m.selectedRequestIndex == i, m.width))
	}
	if len(m.selectedSessionReqs.Groups) == 0 {
		parts = append(parts, styleDim.Render("  No requests found for this session"))
	}
	return strings.Join(parts, "\n")
}

func renderRequestDetail(m tuiModel, _ int) string {
	req := m.selectedRequest()
	if req == nil || m.selectedSessionReqs == nil {
		return styleDim.Render("No request selected")
	}

	total := len(m.selectedSessionReqs.Groups)
	index := total - m.selectedRequestIndex
	metaParts := []string{
		fmt.Sprintf("Request %d/%d", index, total),
		fmt.Sprintf("%d calls", len(req.Turns)),
		fmtCost(req.CostUSD),
	}
	if req.ElapsedSec > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%ds elapsed", req.ElapsedSec))
	}
	if req.FirstTs > 0 {
		metaParts = append(metaParts, time.Unix(req.FirstTs, 0).Local().Format("2006-01-02 15:04"))
	}

	question := requestQuestion(req)
	answer := requestAnswer(req)
	if question == "" {
		question = "No user question captured."
	}
	if answer == "" {
		answer = "No assistant answer captured."
	}

	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}
	question = wrapBlock(question, contentWidth)
	answer = wrapBlock(answer, contentWidth)

	questionBox := renderDetailBlock("Question", question, contentWidth, colQuestionBorder)
	answerBox := renderDetailBlock("Answer", answer, contentWidth, colAnswerBorder)

	var parts []string
	parts = append(parts, "  "+styleSub.Render(strings.Join(metaParts, "  ·  ")))
	parts = append(parts, "")
	parts = append(parts, "  "+questionBox)
	parts = append(parts, "")
	parts = append(parts, "  "+answerBox)
	return strings.Join(parts, "\n")
}

func renderSessionHeader(data sessionRequestsResponse) string {
	session := data.Session
	meta := []string{
		fmt.Sprintf("%d requests", data.TotalGroups),
		fmt.Sprintf("%d turns", session.Turns),
		fmt.Sprintf("%.0fm", session.DurationMin),
	}
	if session.Model != "" {
		meta = append(meta, session.Model)
	}
	if session.Tool != "" {
		meta = append(meta, session.Tool)
	}

	idSuffix := session.SessionID
	if len(idSuffix) > 8 {
		idSuffix = idSuffix[len(idSuffix)-8:]
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Render(
			styleTitle.Render(session.Project) + "\n" +
				styleSub.Render("session "+idSuffix) + "\n" +
				styleDim.Render(strings.Join(meta, "  ·  ")),
		)
	return "  " + box
}

func renderRequestSummaryItem(group requestGroup, idx int, selected bool, totalW int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)
	if selected {
		boxStyle = boxStyle.BorderForeground(colEm)
	}

	width := totalW - 4
	if width < 40 {
		width = 40
	}
	boxStyle = boxStyle.Width(width)

	label := fmt.Sprintf("#%d", idx+1)
	if selected {
		label = "▶ " + label
	}
	metaParts := []string{
		fmt.Sprintf("%d calls", len(group.Turns)),
		fmtCost(group.CostUSD),
	}
	if group.ElapsedSec > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%ds elapsed", group.ElapsedSec))
	}

	question := truncatePreview(requestQuestion(&group), width-6, 2)
	answer := truncatePreview(requestAnswer(&group), width-6, 3)

	var content []string
	content = append(content, lipgloss.NewStyle().Foreground(colEm).Bold(true).Render(label)+"  "+styleSub.Render(strings.Join(metaParts, "  ·  ")))
	content = append(content, "")
	content = append(content, styleTitle.Render("Question"))
	content = append(content, indentBlock(question, ""))
	content = append(content, "")
	content = append(content, styleTitle.Render("Answer"))
	content = append(content, indentBlock(answer, ""))
	return "  " + boxStyle.Render(strings.Join(content, "\n"))
}

func requestQuestion(group *requestGroup) string {
	if group == nil || group.Message == nil {
		return ""
	}
	return sanitizeText(group.Message.Content)
}

func requestAnswer(group *requestGroup) string {
	if group == nil {
		return ""
	}
	answer := sanitizeText(group.AssistantResponse)
	if answer != "" {
		return answer
	}
	return sanitizeText(group.PreActionText)
}

func sanitizeText(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	lines := strings.Split(s, "\n")
	var cleaned []string
	lastBlank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " ")
		if strings.TrimSpace(line) == "" {
			if !lastBlank {
				cleaned = append(cleaned, "")
			}
			lastBlank = true
			continue
		}
		cleaned = append(cleaned, strings.TrimSpace(line))
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func truncatePreview(s string, width, maxLines int) string {
	if s == "" {
		return styleDim.Render("No text")
	}
	lines := wrapText(s, width)
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	lines = lines[:maxLines]
	last := []rune(lines[maxLines-1])
	if len(last) > 0 {
		if len(last) >= 1 {
			last = last[:len(last)-1]
		}
		lines[maxLines-1] = string(last) + "…"
	}
	return strings.Join(lines, "\n")
}

func wrapText(s string, width int) []string {
	if width < 10 {
		width = 10
	}
	var out []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		var current []string
		for _, word := range words {
			current = append(current, splitLongWord(word, width)...)
		}
		line := current[0]
		for _, word := range current[1:] {
			if lipgloss.Width(line)+1+lipgloss.Width(word) > width {
				out = append(out, line)
				line = word
				continue
			}
			line += " " + word
		}
		out = append(out, line)
	}
	return out
}

func wrapBlock(s string, width int) string {
	return strings.Join(wrapText(s, width), "\n")
}

func splitLongWord(word string, width int) []string {
	if lipgloss.Width(word) <= width {
		return []string{word}
	}

	runes := []rune(word)
	var parts []string
	var chunk []rune
	chunkWidth := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		if chunkWidth+rw > width && len(chunk) > 0 {
			parts = append(parts, string(chunk))
			chunk = nil
			chunkWidth = 0
		}
		chunk = append(chunk, r)
		chunkWidth += rw
	}
	if len(chunk) > 0 {
		parts = append(parts, string(chunk))
	}
	return parts
}

func indentBlock(s, indent string) string {
	if s == "" {
		return indent
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderDetailBlock(title, body string, width int, border lipgloss.Color) string {
	boxWidth := width
	if boxWidth < 20 {
		boxWidth = 20
	}
	titleStyle := lipgloss.NewStyle().Foreground(border).Bold(true)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(boxWidth)
	return boxStyle.Render(titleStyle.Render(title) + "\n\n" + body)
}
