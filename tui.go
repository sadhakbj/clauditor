// tui.go - k9s-style terminal UI for clauditor.
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Colours / styles ─────────────────────────────────────────────────────────

var (
	colEm     = lipgloss.Color("#10b981")
	colDim    = lipgloss.Color("#52525b")
	colSub    = lipgloss.Color("#a1a1aa")
	colText   = lipgloss.Color("#f4f4f5")
	colBorder = lipgloss.Color("#27272a")
	colOpus   = lipgloss.Color("#a78bfa")
	colSonnet = lipgloss.Color("#0ea5e9")
	colHaiku  = lipgloss.Color("#10b981")
	colAmber  = lipgloss.Color("#f59e0b")
	colRed    = lipgloss.Color("#ef4444")

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("#111113"))

	styleTab = lipgloss.NewStyle().
			Foreground(colDim).
			Padding(0, 2)

	styleTabActive = lipgloss.NewStyle().
			Foreground(colEm).
			Bold(true).
			Padding(0, 2)

	styleFooter = lipgloss.NewStyle().
			Background(lipgloss.Color("#111113")).
			Foreground(colDim)

	styleTitle = lipgloss.NewStyle().
			Foreground(colEm).
			Bold(true)

	styleDim = lipgloss.NewStyle().Foreground(colDim)
	styleSub = lipgloss.NewStyle().Foreground(colSub)

	styleKPICost = lipgloss.NewStyle().
			Foreground(colEm).
			Bold(true)

	styleHelp = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(1, 3)
)

// ── Model states ──────────────────────────────────────────────────────────────

type viewID int

const (
	viewOverview viewID = iota
	viewSessions
	viewModels
)

type sessionScreen int

const (
	sessionScreenList sessionScreen = iota
	sessionScreenRequests
	sessionScreenRequestDetail
)

type rangeKey int

const (
	rangeToday rangeKey = iota
	range7d
	range30d
	range90d
	rangeAll
)

var rangeLabels = map[rangeKey]string{
	rangeToday: "Today",
	range7d:    "Last 7 days",
	range30d:   "Last 30 days",
	range90d:   "Last 90 days",
	rangeAll:   "All time",
}

// ── Messages ──────────────────────────────────────────────────────────────────

type dataLoadedMsg struct{ data dashboardData }
type tickMsg struct{}
type sessionRequestsLoadedMsg struct {
	sessionID string
	data      sessionRequestsResponse
	err       error
}

// ── Main model ────────────────────────────────────────────────────────────────

type tuiModel struct {
	width, height int
	view          viewID
	dateRange     rangeKey

	loading     bool
	scanning    bool
	data        *dashboardData
	err         string
	lastScanned string
	showHelp    bool

	spinner      spinner.Model
	sessionTable table.Model
	modelTable   table.Model
	viewport     viewport.Model

	filterMode bool
	filterText string

	sessionRows          []sessionRow
	sessionScreen        sessionScreen
	selectedSession      *sessionRow
	selectedSessionReqs  *sessionRequestsResponse
	selectedRequestIndex int
	detailViewport       viewport.Model
	detailLoading        bool
	detailErr            string

	confirmQuit  bool
	quitFocusYes bool
}

func newTUIModel() tuiModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colEm)

	return tuiModel{
		loading:        true,
		scanning:       true,
		spinner:        sp,
		dateRange:      range30d,
		viewport:       viewport.New(0, 0),
		detailViewport: viewport.New(0, 0),
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		doScanAndLoad(),
		tea.SetWindowTitle("clauditor"),
	)
}

func doScanAndLoad() tea.Cmd {
	return func() tea.Msg {
		scan(projectsDir, dbPath, false)
		data := getDashboardData()
		return dataLoadedMsg{data: data}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTables()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case dataLoadedMsg:
		m.loading = false
		m.scanning = false
		d := msg.data
		m.data = &d
		m.lastScanned = time.Now().Format("15:04:05")
		m.err = d.Error
		m.rebuildTables()
		return m, tickCmd()

	case sessionRequestsLoadedMsg:
		m.detailLoading = false
		if msg.err != nil {
			m.detailErr = msg.err.Error()
			m.selectedSessionReqs = nil
			return m, nil
		}
		m.detailErr = ""
		data := msg.data
		m.selectedSessionReqs = &data
		m.sessionScreen = sessionScreenRequests
		m.selectedRequestIndex = 0
		m.rebuildDetailViewport()
		return m, nil

	case tickMsg:
		m.scanning = true
		return m, tea.Batch(m.spinner.Tick, doScanAndLoad())

	case tea.KeyMsg:
		if m.confirmQuit {
			return m.handleQuitConfirmKey(msg)
		}
		// filter mode captures all keys
		if m.filterMode {
			return m.handleFilterKey(msg)
		}
		return m.handleKey(msg)
	}

	// Propagate to sub-components
	var cmd tea.Cmd
	switch m.view {
	case viewSessions:
		m.sessionTable, cmd = m.sessionTable.Update(msg)
	case viewModels:
		m.modelTable, cmd = m.modelTable.Update(msg)
	}
	return m, cmd
}

func (m tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.view == viewSessions && m.sessionScreen != sessionScreenList {
		return m.handleSessionDetailKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.confirmQuit = true
		m.quitFocusYes = false
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "esc":
		if m.showHelp {
			m.showHelp = false
		}
		m.filterText = ""
		return m, nil

	// Switch views
	case "1":
		m.view = viewOverview
		m.showHelp = false
		m.syncTableFocus()
	case "2":
		m.view = viewSessions
		m.showHelp = false
		m.syncTableFocus()
	case "3":
		m.view = viewModels
		m.showHelp = false
		m.syncTableFocus()
	case "tab":
		m.view = (m.view + 1) % 3
		m.showHelp = false
		m.syncTableFocus()

	// Date range
	case "t":
		m.dateRange = rangeToday
		m.buildOverviewViewport()
	case "d":
		m.dateRange = range7d
		m.buildOverviewViewport()
	case "w":
		m.dateRange = range30d
		m.buildOverviewViewport()
	case "m":
		m.dateRange = range90d
		m.buildOverviewViewport()
	case "a":
		m.dateRange = rangeAll
		m.buildOverviewViewport()

	// Refresh
	case "r":
		m.scanning = true
		return m, tea.Batch(m.spinner.Tick, doScanAndLoad())

	// Filter (sessions view only)
	case "/":
		if m.view == viewSessions && m.sessionScreen == sessionScreenList {
			m.filterMode = true
			m.filterText = ""
		}
		return m, nil

	case "enter":
		if m.view == viewSessions && m.sessionScreen == sessionScreenList {
			session := m.selectedSessionRow()
			if session == nil {
				return m, nil
			}
			m.selectedSession = session
			m.selectedSessionReqs = nil
			m.selectedRequestIndex = 0
			m.detailErr = ""
			m.detailLoading = true
			m.sessionScreen = sessionScreenRequests
			m.rebuildDetailViewport()
			return m, loadSessionRequests(session.SessionID)
		}
	}

	// Navigation
	var cmd tea.Cmd
	switch m.view {
	case viewOverview:
		m.viewport, cmd = m.viewport.Update(msg)
	case viewSessions:
		m.sessionTable, cmd = m.sessionTable.Update(msg)
	case viewModels:
		m.modelTable, cmd = m.modelTable.Update(msg)
	}
	return m, cmd
}

func (m tuiModel) handleSessionDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.confirmQuit = true
		m.quitFocusYes = false
		return m, nil
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "esc", "backspace":
		switch m.sessionScreen {
		case sessionScreenRequestDetail:
			m.sessionScreen = sessionScreenRequests
			m.rebuildDetailViewport()
		case sessionScreenRequests:
			m.sessionScreen = sessionScreenList
			m.selectedSessionReqs = nil
			m.selectedSession = nil
			m.detailErr = ""
			m.detailLoading = false
			m.syncTableFocus()
		}
		return m, nil
	case "enter":
		if m.sessionScreen == sessionScreenRequests && m.selectedRequest() != nil {
			m.sessionScreen = sessionScreenRequestDetail
			m.rebuildDetailViewport()
		}
		return m, nil
	case "g":
		if m.sessionScreen == sessionScreenRequests {
			m.selectedRequestIndex = 0
			m.rebuildDetailViewport()
			return m, nil
		}
	case "G":
		if m.sessionScreen == sessionScreenRequests && m.selectedSessionReqs != nil && len(m.selectedSessionReqs.Groups) > 0 {
			m.selectedRequestIndex = len(m.selectedSessionReqs.Groups) - 1
			m.rebuildDetailViewport()
			return m, nil
		}
	}

	switch m.sessionScreen {
	case sessionScreenRequests:
		switch msg.String() {
		case "j", "down":
			if m.selectedSessionReqs != nil && m.selectedRequestIndex < len(m.selectedSessionReqs.Groups)-1 {
				m.selectedRequestIndex++
				m.rebuildDetailViewport()
			}
			return m, nil
		case "k", "up":
			if m.selectedRequestIndex > 0 {
				m.selectedRequestIndex--
				m.rebuildDetailViewport()
			}
			return m, nil
		}
	case sessionScreenRequestDetail:
		var cmd tea.Cmd
		m.detailViewport, cmd = m.detailViewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m tuiModel) handleQuitConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		m.quitFocusYes = true
		return m, nil
	case "right", "l":
		m.quitFocusYes = false
		return m, nil
	case "tab", "shift+tab":
		m.quitFocusYes = !m.quitFocusYes
		return m, nil
	case "y", "Y":
		m.quitFocusYes = true
		return m, tea.Quit
	case "n", "N":
		m.confirmQuit = false
		m.quitFocusYes = false
		return m, nil
	case "enter":
		if m.quitFocusYes {
			return m, tea.Quit
		}
		m.confirmQuit = false
		m.quitFocusYes = false
		return m, nil
	case "q", "ctrl+c", "esc":
		m.confirmQuit = false
		m.quitFocusYes = false
		return m, nil
	}
	return m, nil
}

func (m tuiModel) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.filterMode = false
		m.rebuildTables()
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.rebuildTables()
		}
	default:
		if len(msg.Runes) == 1 {
			m.filterText += string(msg.Runes)
			m.rebuildTables()
		}
	}
	return m, nil
}

// ── Table builders ────────────────────────────────────────────────────────────

func (m *tuiModel) rebuildTables() {
	if m.data == nil {
		return
	}
	m.buildSessionTable()
	m.buildModelTable()
	m.syncTableFocus()
	m.buildOverviewViewport()
	m.rebuildDetailViewport()
}

func (m *tuiModel) buildOverviewViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	bodyH := m.height - 2 // header(1) + footer(1)
	if bodyH < 1 {
		bodyH = 1
	}
	m.viewport = viewport.New(m.width, bodyH)
	m.viewport.SetContent(renderOverview(*m))
}

func (m *tuiModel) rebuildDetailViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	headerH := lipgloss.Height(m.renderHeader())
	footerH := lipgloss.Height(m.renderFooter())
	bodyH := m.height - headerH - footerH
	if bodyH < 1 {
		bodyH = 1
	}

	m.detailViewport = viewport.New(m.width, bodyH)
	switch m.sessionScreen {
	case sessionScreenRequests:
		m.detailViewport.SetContent(renderSessionRequests(*m, bodyH))
		m.detailViewport.SetYOffset(m.selectedRequestOffset())
	case sessionScreenRequestDetail:
		m.detailViewport.SetContent(renderRequestDetail(*m, bodyH))
	}
}

func (m *tuiModel) buildSessionTable() {
	cols := []table.Column{
		{Title: "PROJECT", Width: 22},
		{Title: "MODEL", Width: 20},
		{Title: "TURNS", Width: 7},
		{Title: "INPUT", Width: 9},
		{Title: "OUTPUT", Width: 9},
		{Title: "COST", Width: 9},
		{Title: "DURATION", Width: 10},
	}

	cutoff := rangeCutoff(m.dateRange)
	filter := strings.ToLower(m.filterText)

	var rows []table.Row
	var sessionRows []sessionRow
	for _, s := range m.data.SessionsAll {
		if cutoff != "" && s.LastDate < cutoff {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(s.Project), filter) &&
			!strings.Contains(strings.ToLower(s.Model), filter) {
			continue
		}
		cost := calcCost(s.Model, s.Input, s.Output, s.CacheRead, s.CacheCreation)
		dur := fmt.Sprintf("%.0fm", s.DurationMin)
		rows = append(rows, table.Row{
			truncate(s.Project, 22),
			truncate(s.Model, 20),
			fmt.Sprintf("%d", s.Turns),
			fmtTokens(s.Input),
			fmtTokens(s.Output),
			fmtCost(cost),
			dur,
		})
		sessionRows = append(sessionRows, s)
	}

	tableHeight := m.height - 8 // header + footer + table header
	if tableHeight < 5 {
		tableHeight = 5
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(m.view == viewSessions),
		table.WithHeight(tableHeight),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(colSub)
	ts.Selected = ts.Selected.
		Foreground(colText).
		Background(lipgloss.Color("#1c1c1f")).
		Bold(false)
	t.SetStyles(ts)
	m.sessionTable = t
	m.sessionRows = sessionRows
}

func (m *tuiModel) buildModelTable() {
	// Aggregate per-model from sessions filtered by date
	cutoff := rangeCutoff(m.dateRange)
	type agg struct {
		inp, out, cr, cc int64
		turns, sessions  int64
	}
	aggMap := map[string]*agg{}
	for _, s := range m.data.SessionsAll {
		if cutoff != "" && s.LastDate < cutoff {
			continue
		}
		a := aggMap[s.Model]
		if a == nil {
			a = &agg{}
			aggMap[s.Model] = a
		}
		a.inp += s.Input
		a.out += s.Output
		a.cr += s.CacheRead
		a.cc += s.CacheCreation
		a.turns += s.Turns
		a.sessions++
	}

	cols := []table.Column{
		{Title: "MODEL", Width: 28},
		{Title: "SESSIONS", Width: 10},
		{Title: "TURNS", Width: 8},
		{Title: "INPUT", Width: 10},
		{Title: "OUTPUT", Width: 10},
		{Title: "CACHE READ", Width: 12},
		{Title: "COST", Width: 10},
	}

	var rows []table.Row
	for model, a := range aggMap {
		cost := calcCost(model, a.inp, a.out, a.cr, a.cc)
		rows = append(rows, table.Row{
			truncate(model, 28),
			fmt.Sprintf("%d", a.sessions),
			fmtTokens(a.turns),
			fmtTokens(a.inp),
			fmtTokens(a.out),
			fmtTokens(a.cr),
			fmtCost(cost),
		})
	}

	tableHeight := m.height - 8
	if tableHeight < 5 {
		tableHeight = 5
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(m.view == viewModels),
		table.WithHeight(tableHeight),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(colSub)
	ts.Selected = ts.Selected.
		Foreground(colText).
		Background(lipgloss.Color("#1c1c1f")).
		Bold(false)
	t.SetStyles(ts)
	m.modelTable = t
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m tuiModel) View() string {
	if m.width == 0 {
		return ""
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	bodyH := m.height - headerH - footerH

	var body string
	if m.loading {
		body = m.renderLoading(bodyH)
	} else if m.err != "" {
		body = m.renderError(bodyH)
	} else if m.showHelp {
		body = m.renderHelp(bodyH)
	} else {
		switch m.view {
		case viewOverview:
			body = m.viewport.View()
		case viewSessions:
			if m.sessionScreen == sessionScreenList {
				body = renderSessions(m, bodyH)
			} else {
				body = m.detailViewport.View()
			}
		case viewModels:
			body = renderModels(m, bodyH)
		}
	}

	if m.confirmQuit {
		body = m.renderQuitConfirm(body)
	}

	return header + "\n" + body + "\n" + footer
}

func (m tuiModel) renderHeader() string {
	tabs := []string{}
	labels := []string{"Overview", "Sessions", "Models"}
	for i, label := range labels {
		num := styleDim.Render(fmt.Sprintf("%d", i+1))
		if viewID(i) == m.view {
			tabs = append(tabs, styleTabActive.Render(num+" "+label))
		} else {
			tabs = append(tabs, styleTab.Render(num+" "+label))
		}
	}

	brand := lipgloss.NewStyle().Foreground(colEm).Bold(true).Padding(0, 2).Render("◈ clauditor")
	tabRow := strings.Join(tabs, "")

	rangeStr := lipgloss.NewStyle().Foreground(colDim).Padding(0, 2).Render(rangeLabels[m.dateRange])
	scanIndicator := ""
	if m.scanning {
		scanIndicator = " " + m.spinner.View()
	}

	left := brand + tabRow
	right := rangeStr + scanIndicator + "  "

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	line := left + strings.Repeat(" ", gap) + right

	return styleHeader.Width(m.width).Render(line)
}

func (m tuiModel) renderFooter() string {
	keys := []string{
		keyHint("1-3", "switch view"),
		keyHint("t/d/w/m/a", "date range"),
		keyHint("r", "refresh"),
		keyHint("?", "help"),
		keyHint("q", "quit"),
	}
	if m.view == viewSessions && m.sessionScreen == sessionScreenList {
		keys = append(keys[:3], append([]string{keyHint("/", "filter")}, keys[3:]...)...)
	}
	if m.view == viewSessions && m.sessionScreen != sessionScreenList {
		keys = []string{
			keyHint("enter", "open"),
			keyHint("esc", "back"),
			keyHint("j/k", "move"),
			keyHint("?", "help"),
			keyHint("q", "quit"),
		}
		if m.sessionScreen == sessionScreenRequestDetail {
			keys = []string{
				keyHint("↑/↓", "scroll"),
				keyHint("esc", "back"),
				keyHint("?", "help"),
				keyHint("q", "quit"),
			}
		}
	}
	if m.filterMode {
		keys = []string{keyHint("filter:", "/"+m.filterText+"▌"), keyHint("enter/esc", "done")}
	}
	if m.confirmQuit {
		keys = []string{
			keyHint("tab/←/→", "toggle"),
			keyHint("enter", "select"),
			keyHint("esc", "cancel"),
		}
	}

	left := "  " + strings.Join(keys, "  ")
	right := ""
	if m.lastScanned != "" {
		right = styleDim.Render("scanned "+m.lastScanned) + "  "
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	line := left + strings.Repeat(" ", gap) + right
	return styleFooter.Width(m.width).Render(line)
}

func (m tuiModel) renderLoading(h int) string {
	msg := m.spinner.View() + "  Scanning and loading data…"
	return center(msg, m.width, h)
}

func (m tuiModel) renderError(h int) string {
	msg := lipgloss.NewStyle().Foreground(colRed).Render("✗ " + m.err)
	return center(msg, m.width, h)
}

func (m tuiModel) renderHelp(h int) string {
	bindings := [][]string{
		{"1 / 2 / 3", "Switch to Overview / Sessions / Models"},
		{"tab", "Cycle to next view"},
		{"t / d / w / m / a", "Date range: today / 7d / 30d / 90d / all"},
		{"r", "Rescan JSONL files and refresh data"},
		{"j / k  ↑ / ↓", "Navigate table rows"},
		{"g / G", "Jump to top / bottom of table"},
		{"enter (sessions)", "Open session or request detail"},
		{"/ (sessions list)", "Enter filter mode"},
		{"esc", "Close filter or this help"},
		{"?", "Toggle this help overlay"},
		{"q / ctrl+c", "Quit"},
	}

	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Keyboard Shortcuts") + "\n\n")
	for _, b := range bindings {
		key := lipgloss.NewStyle().Foreground(colEm).Bold(true).Width(22).Render(b[0])
		sb.WriteString("  " + key + "  " + styleSub.Render(b[1]) + "\n")
	}

	box := styleHelp.Render(sb.String())
	return center(box, m.width, h)
}

func (m tuiModel) renderQuitConfirm(body string) string {
	_ = body

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#d4d4d8")).
		Padding(1, 3).
		Width(58)

	title := lipgloss.NewStyle().
		Foreground(colRed).
		Bold(true).
		Render("Quit clauditor?")

	message := styleSub.Render("Use tab or arrow keys to choose, then press enter.")
	yes := m.renderQuitButton("Yes", m.quitFocusYes, true)
	no := m.renderQuitButton("No", !m.quitFocusYes, false)

	rowWidth := max(lipgloss.Width(message), 32)
	gap := rowWidth - lipgloss.Width(yes) - lipgloss.Width(no)
	if gap < 6 {
		gap = 6
	}
	buttons := yes + strings.Repeat(" ", gap) + no
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", message, "", buttons)
	dialog := dialogStyle.Render(content)

	bodyHeight := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.renderFooter())
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	overlay := lipgloss.Place(
		m.width,
		bodyHeight,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)

	return overlay
}

func (m tuiModel) renderQuitButton(label string, focused bool, destructive bool) string {
	style := lipgloss.NewStyle().
		Padding(0, 2)

	if destructive {
		if focused {
			style = style.Background(colRed).Foreground(lipgloss.Color("#111113")).Bold(true)
		} else {
			style = style.Foreground(colRed).Bold(true)
		}
	} else {
		if focused {
			style = style.Background(lipgloss.Color("#27272a")).Foreground(colText).Bold(true)
		} else {
			style = style.Foreground(colText).Bold(true)
		}
	}

	return style.Render(label)
}

func loadSessionRequests(sessionID string) tea.Cmd {
	return func() tea.Msg {
		data, err := getSessionRequests(sessionID, 1, 10000)
		return sessionRequestsLoadedMsg{
			sessionID: sessionID,
			data:      data,
			err:       err,
		}
	}
}

func (m tuiModel) selectedSessionRow() *sessionRow {
	if len(m.sessionRows) == 0 {
		return nil
	}
	idx := m.sessionTable.Cursor()
	if idx < 0 || idx >= len(m.sessionRows) {
		return nil
	}
	s := m.sessionRows[idx]
	return &s
}

func (m tuiModel) selectedRequest() *requestGroup {
	if m.selectedSessionReqs == nil || len(m.selectedSessionReqs.Groups) == 0 {
		return nil
	}
	if m.selectedRequestIndex < 0 || m.selectedRequestIndex >= len(m.selectedSessionReqs.Groups) {
		return nil
	}
	return &m.selectedSessionReqs.Groups[m.selectedRequestIndex]
}

func (m tuiModel) selectedRequestOffset() int {
	if m.selectedSessionReqs == nil || m.selectedRequestIndex <= 0 {
		return 0
	}

	offset := lipgloss.Height(renderSessionHeader(*m.selectedSessionReqs)) + 1
	for i := 0; i < m.selectedRequestIndex; i++ {
		offset += lipgloss.Height(renderRequestSummaryItem(
			m.selectedSessionReqs.Groups[i],
			i,
			false,
			m.width,
		)) + 1
	}
	return offset
}

func (m *tuiModel) syncTableFocus() {
	if m.view == viewSessions && m.sessionScreen == sessionScreenList {
		m.sessionTable.Focus()
	} else {
		m.sessionTable.Blur()
	}

	if m.view == viewModels {
		m.modelTable.Focus()
	} else {
		m.modelTable.Blur()
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func keyHint(key, desc string) string {
	k := lipgloss.NewStyle().Foreground(colEm).Bold(true).Render(key)
	d := styleDim.Render(" " + desc)
	return k + d
}

func center(content string, w, h int) string {
	cw := lipgloss.Width(content)
	ch := lipgloss.Height(content)
	padX := (w - cw) / 2
	padY := (h - ch) / 2
	if padX < 0 {
		padX = 0
	}
	if padY < 0 {
		padY = 0
	}
	top := strings.Repeat("\n", padY)
	left := strings.Repeat(" ", padX)
	lines := strings.Split(content, "\n")
	var out []string
	for _, l := range lines {
		out = append(out, left+l)
	}
	return top + strings.Join(out, "\n")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func rangeCutoff(r rangeKey) string {
	now := time.Now()
	switch r {
	case rangeToday:
		return now.Format("2006-01-02")
	case range7d:
		return now.AddDate(0, 0, -7).Format("2006-01-02")
	case range30d:
		return now.AddDate(0, 0, -30).Format("2006-01-02")
	case range90d:
		return now.AddDate(0, 0, -90).Format("2006-01-02")
	default:
		return ""
	}
}

func modelFamilyColor(model string) lipgloss.Color {
	ml := strings.ToLower(model)
	switch {
	case strings.Contains(ml, "opus"):
		return colOpus
	case strings.Contains(ml, "sonnet"):
		return colSonnet
	case strings.Contains(ml, "haiku"):
		return colHaiku
	default:
		return colAmber
	}
}

// ── Entry point ───────────────────────────────────────────────────────────────

func runTUI() error {
	m := newTUIModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
