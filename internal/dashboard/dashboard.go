package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lanesket/llm.log/internal/daemon"
	"github.com/lanesket/llm.log/internal/export"
	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/storage"
)

type tickMsg time.Time
type clearCopyMsg struct{}

type tab int

const (
	tabOverview tab = iota
	tabChart
	tabCost
	tabRequests
	tabCount
)

var (
	tabNames = [tabCount]string{"Overview", "Chart", "Cost", "Requests"}
	periods  = []string{"today", "week", "month", "all"}
)

type Model struct {
	store   storage.Store
	dataDir string
	width   int
	height  int

	activeTab      tab
	period         string
	source         string // "" = all, "cc:" = Claude Code, "cc:sub", "cc:key"
	providerFilter string // "" = all, "anthropic", "openai", etc.
	costGroupBy    string // "provider" or "model"
	showHelp       bool

	providerStats     []storage.StatRow
	modelStats        []storage.StatRow
	dailyStats        []storage.StatRow
	prevProviderStats []storage.StatRow

	daemonRunning bool

	// Heatmap cursor (always shows all-time data, independent of period)
	hmRow  int               // 0-6 (Mon-Sun)
	hmCol  int               // 0..totalWeeks-1
	hmGrid *heatmapGrid      // computed grid metadata
	hmDay  []storage.StatRow // model breakdown for selected day

	requests     []storage.Record
	reqCursor    int
	reqOffset    int
	reqDetail    *storage.Record
	viewport     viewport.Model
	copyNotice   string
	availSources []string // source filters available for current period
}

func New(store storage.Store, dataDir string) Model {
	return Model{store: store, dataDir: dataDir, period: "month", costGroupBy: "provider"}
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.reqDetail != nil {
			m.viewport.Width = m.width - 4
			m.viewport.Height = m.contentHeight()
		}
		m.loadData()
		return m, nil

	case clearCopyMsg:
		m.copyNotice = ""
		return m, nil

	case tickMsg:
		m.loadData()
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })

	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			if msg.Y == 0 {
				m.handleTabClick(msg.X)
			} else if m.activeTab == tabOverview && m.hmGrid != nil {
				m.handleHeatmapClick(msg.X, msg.Y)
			} else if m.activeTab == tabRequests && m.reqDetail == nil {
				m.handleRequestClick(msg.Y)
			}
		case msg.Button == tea.MouseButtonWheelUp:
			if m.activeTab == tabRequests && m.reqDetail != nil {
				m.viewport, _ = m.viewport.Update(msg)
			} else {
				m.navigateUp()
			}
		case msg.Button == tea.MouseButtonWheelDown:
			if m.activeTab == tabRequests && m.reqDetail != nil {
				m.viewport, _ = m.viewport.Update(msg)
			} else {
				m.navigateDown()
			}
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.reqDetail != nil {
			return m.updateDetail(msg)
		}

		switch msg.String() {
		case "?":
			m.showHelp = true
		case "tab":
			m.switchTab((m.activeTab + 1) % tabCount)
		case "shift+tab":
			m.switchTab((m.activeTab - 1 + tabCount) % tabCount)
		case "l", "right":
			if m.activeTab == tabOverview && m.hmGrid != nil {
				m.hmCol = min(m.hmCol+1, m.hmGrid.TotalWeeks-1)
				m.loadHmDay()
			} else {
				m.switchTab((m.activeTab + 1) % tabCount)
			}
		case "h", "left":
			if m.activeTab == tabOverview && m.hmGrid != nil {
				m.hmCol = max(m.hmCol-1, 0)
				m.loadHmDay()
			} else {
				m.switchTab((m.activeTab - 1 + tabCount) % tabCount)
			}
		case "1", "2", "3", "4":
			m.switchTab(tab(msg.String()[0] - '1'))
		case "p":
			m.cyclePeriod()
			m.loadData()
		case "s":
			m.cycleSource()
			m.loadData()
		case "f":
			m.cycleProvider()
			m.loadData()
		case "m":
			if m.activeTab == tabCost {
				if m.costGroupBy == "provider" {
					m.costGroupBy = "model"
				} else {
					m.costGroupBy = "provider"
				}
			}
		case "e":
			return m.exportCurrent()
		case "j", "down":
			m.navigateDown()
		case "k", "up":
			m.navigateUp()
		case "enter":
			if m.activeTab == tabRequests && len(m.requests) > 0 {
				if rec, err := m.store.Get(m.requests[m.reqCursor].ID); err == nil {
					m.reqDetail = rec
					m.viewport = viewport.New(m.width-4, m.contentHeight())
					m.viewport.SetContent(m.renderDetailContent(rec))
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.reqDetail = nil
		m.copyNotice = ""
		return m, nil
	case "c":
		return m.copyToClipboard("all")
	case "p":
		return m.copyToClipboard("prompt")
	case "r":
		return m.copyToClipboard("response")
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

func (m Model) copyToClipboard(what string) (tea.Model, tea.Cmd) {
	if m.reqDetail == nil {
		return m, nil
	}
	var data []byte
	switch what {
	case "prompt":
		data = m.reqDetail.RequestBody
	case "response":
		data = m.reqDetail.ResponseBody
	case "all":
		all := map[string]json.RawMessage{}
		if len(m.reqDetail.RequestBody) > 0 {
			all["prompt"] = m.reqDetail.RequestBody
		}
		if len(m.reqDetail.ResponseBody) > 0 {
			all["response"] = m.reqDetail.ResponseBody
		}
		data, _ = json.MarshalIndent(all, "", "  ")
	}
	if len(data) == 0 {
		m.copyNotice = "nothing to copy"
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearCopyMsg{} })
	}
	var v any
	if json.Unmarshal(data, &v) == nil {
		data, _ = json.MarshalIndent(v, "", "  ")
	}
	cmd := exec.Command(clipboardCmd())
	cmd.Stdin = strings.NewReader(string(data))
	if err := cmd.Run(); err != nil {
		m.copyNotice = "copy failed"
	} else {
		m.copyNotice = what + " copied!"
	}
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearCopyMsg{} })
}

func (m Model) exportCurrent() (tea.Model, tea.Cmd) {
	notice, err := m.doExport()
	if err != nil {
		m.copyNotice = "export failed: " + err.Error()
	} else {
		m.copyNotice = notice
	}
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearCopyMsg{} })
}

func (m Model) doExport() (string, error) {
	filename := fmt.Sprintf("llm-log-export-%s.csv", time.Now().Format("20060102-150405"))

	from, to := storage.PeriodToTimeRange(m.period)
	records, err := m.store.Recent(0, from, to, m.providerFilter, m.source)
	if err != nil {
		return "", err
	}

	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := export.Write(f, records, export.Options{Format: export.CSV}, nil); err != nil {
		return "", err
	}

	path := filename
	if abs, err := filepath.Abs(filename); err == nil {
		path = abs
	}
	return "exported to " + path, nil
}

func (m Model) contentHeight() int {
	return max(5, m.height-7)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	if m.showHelp {
		return m.viewHelp()
	}

	header := m.renderTabBar()
	footer := m.renderHelp()

	var content string
	if m.reqDetail != nil {
		content = m.viewport.View()
	} else {
		switch m.activeTab {
		case tabOverview:
			content = m.viewOverview()
		case tabChart:
			content = m.viewChart()
		case tabCost:
			content = m.viewCost()
		case tabRequests:
			content = m.viewRequests()
		}
	}

	if lines := strings.Split(content, "\n"); len(lines) > m.contentHeight() {
		content = strings.Join(lines[:m.contentHeight()], "\n")
	}
	return header + "\n\n" + content + "\n" + footer
}

func (m Model) renderTabBar() string {
	statusColor := lipgloss.Color("#10B981")
	if !m.daemonRunning {
		statusColor = lipgloss.Color("#EF4444")
	}
	header := lipgloss.NewStyle().Foreground(statusColor).Render("●") + " " + titleStyle.Render("llm.log")
	gap := strings.Repeat(" ", max(2, m.width/2-24))

	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf("%d·%s", i+1, name)
		if tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render("◆ "+label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}

	return header + gap + mutedStyle.Render("‹") + " " +
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...) + " " + mutedStyle.Render("›")
}

func (m Model) renderHelp() string {
	if m.reqDetail != nil {
		help := "↑↓: scroll · c: copy all · p: copy prompt · r: copy response · esc: back · q: quit"
		if m.copyNotice != "" {
			help = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("✓ "+m.copyNotice) +
				" · esc: back · q: quit"
		}
		return "\n" + helpStyle.Render(help)
	}

	sourceState := "all"
	if label, ok := sourceFilterLabels[m.source]; ok {
		sourceState = label
	}

	nav := "tab: switch · 1-4: jump"
	groupLabel := "model"
	if m.costGroupBy == "model" {
		groupLabel = "provider"
	}
	switch m.activeTab {
	case tabOverview:
		nav += " · hjkl/←→↑↓: navigate"
	case tabCost:
		nav += fmt.Sprintf(" · m: by %s", groupLabel)
	case tabRequests:
		nav += " · ↑↓: navigate · enter/click: detail"
	}
	nav += " · e: export · p: period · s: source · f: provider · ?: help · q: quit"

	if m.copyNotice != "" {
		nav = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("✓ "+m.copyNotice) +
			" · q: quit"
	}

	provState := "all"
	if m.providerFilter != "" {
		provState = m.providerFilter
	}
	filters := mutedStyle.Render("period:") + " " + brightStyle.Render(m.period) +
		"  " + mutedStyle.Render("source:") + " " + brightStyle.Render(sourceState) +
		"  " + mutedStyle.Render("provider:") + " " + brightStyle.Render(provState)

	return "\n" + helpStyle.Render(nav) + "\n" + filters
}

// ── Overview ──

// allStats returns whichever stat list has data (prefer provider, fallback model).
func (m Model) allStats() []storage.StatRow {
	if len(m.providerStats) > 0 {
		return m.providerStats
	}
	return m.modelStats
}

func (m Model) viewOverview() string {
	var totalReqs, totalErrors int
	var totalIn, totalOut, totalCacheRead, totalCacheWrite int64
	var totalCost float64
	var totalDuration int
	// Use allStats for robust totals even if one grouping fails
	for _, s := range m.allStats() {
		totalReqs += s.Requests
		totalErrors += s.Errors
		totalIn += s.InputTokens
		totalOut += s.OutputTokens
		totalCacheRead += s.CacheReadTokens
		totalCacheWrite += s.CacheWriteTokens
		totalCost += s.TotalCost
		totalDuration += s.AvgDurationMs * s.Requests
	}

	// Cost trend
	trendStr := ""
	if m.period != "all" {
		var prevCost float64
		for _, s := range m.prevProviderStats {
			prevCost += s.TotalCost
		}
		if prevCost > 0 {
			diff := (totalCost - prevCost) / prevCost * 100
			if diff > 500 {
				trendStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(
					fmt.Sprintf(" ↑%.1fx", totalCost/prevCost))
			} else if diff > 0 {
				trendStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(
					fmt.Sprintf(" ↑%.0f%%", diff))
			} else if diff < -0.5 {
				trendStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(
					fmt.Sprintf(" ↓%.0f%%", -diff))
			}
		}
	}

	line1 := fmt.Sprintf("  %s%s spent  ·  %s requests  ·  %s in  ·  %s out",
		brightStyle.Render(format.Cost(totalCost)), trendStr,
		brightStyle.Render(fmt.Sprintf("%d", totalReqs)),
		brightStyle.Render(format.Tokens(totalIn)),
		brightStyle.Render(format.Tokens(totalOut)),
	)

	var extras []string
	if totalCacheRead > 0 || totalCacheWrite > 0 {
		parts := []string{}
		if totalCacheRead > 0 {
			hitRate := float64(totalCacheRead) / float64(max(1, totalIn)) * 100
			parts = append(parts, fmt.Sprintf("%s read (%.0f%%)", format.Tokens(totalCacheRead), hitRate))
		}
		if totalCacheWrite > 0 {
			parts = append(parts, format.Tokens(totalCacheWrite)+" write")
		}
		extras = append(extras, "cache: "+strings.Join(parts, ", "))
	}
	if totalReqs > 0 {
		extras = append(extras, fmt.Sprintf("avg %dms", totalDuration/totalReqs))
	}
	if totalErrors > 0 {
		extras = append(extras, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).
			Render(fmt.Sprintf("%d errors", totalErrors)))
	}

	summary := line1
	if len(extras) > 0 {
		summary += "\n  " + strings.Join(extras, "  ·  ")
	}

	// Contribution heatmap with cursor
	if len(m.dailyStats) > 0 {
		hm, _ := heatmap(m.dailyStats, m.width-12, m.hmRow, m.hmCol)
		summary += "\n\n" + hm

		// Selected day breakdown
		if m.hmGrid != nil {
			d := m.hmGrid.CellToDate(m.hmCol, m.hmRow)
			dateStr := d.Format("Mon, Jan 2")
			if len(m.hmDay) > 0 {
				var dayCost float64
				for _, s := range m.hmDay {
					dayCost += s.TotalCost
				}
				summary += "\n\n  " + brightStyle.Render(dateStr) + "  " + format.Cost(dayCost)
				for _, s := range m.hmDay[:min(len(m.hmDay), 5)] {
					vendor, name := splitModel(s.Key)
					dot := lipgloss.NewStyle().Foreground(providerColor(vendor)).Render("●")
					summary += fmt.Sprintf("\n  %s %-20s %4d reqs  %s",
						dot, format.Truncate(name, 20), s.Requests, format.Cost(s.TotalCost))
				}
			} else {
				summary += "\n\n  " + brightStyle.Render(dateStr) + "  " + mutedStyle.Render("no activity")
			}
		}
	}

	summaryBox := boxStyle.Width(m.width - 6).Render(summary)

	// Top models
	if len(m.modelStats) == 0 {
		return summaryBox
	}

	maxCost := m.modelStats[0].TotalCost
	limit := min(len(m.modelStats), 5, max(2, m.contentHeight()/3))
	contentW := m.width - 6 - 4 // box width minus padding

	var rows strings.Builder
	rows.WriteString("    " + padR(mutedStyle.Render("MODEL"), 22) +
		padL(mutedStyle.Render("REQS"), 6) +
		padL(mutedStyle.Render("IN"), 7) +
		padL(mutedStyle.Render("OUT"), 7) +
		padL(mutedStyle.Render("COST"), 10) + "\n")
	for i := range limit {
		s := m.modelStats[i]
		vendor, name := splitModel(s.Key)
		color := providerColor(vendor)
		dot := lipgloss.NewStyle().Foreground(color).Render("●")
		line := "  " + dot + " " + padR(format.Truncate(name, 20), 22) +
			padL(fmt.Sprintf("%d", s.Requests), 6) +
			padL(format.Tokens(s.InputTokens), 7) +
			padL(format.Tokens(s.OutputTokens), 7) +
			padL(format.Cost(s.TotalCost), 10)
		barW := min(15, contentW-lipgloss.Width(line)-1)
		if barW > 2 {
			line += " " + hbar(s.TotalCost, maxCost, barW, color)
		}
		rows.WriteString(line + "\n")
	}

	modelsBox := boxStyle.Width(m.width - 6).Render(
		titleStyle.Render("  Top Models") + "\n" + rows.String())
	return summaryBox + "\n" + modelsBox
}

// ── Chart ──

func (m Model) viewChart() string {
	w := m.width - 6

	if len(m.requests) == 0 {
		return boxStyle.Width(w).Render(mutedStyle.Render("  No data for this period."))
	}

	from, to := storage.PeriodToTimeRange(m.period)
	b := m.buildBuckets(from, to)

	quadH := max(7, (m.contentHeight()-4)/2)
	quadW := max(30, (w-2)/2)

	costChart := m.renderQuadrant("Cumulative Cost", b.times, b.cost, quadW, quadH, formatAxisValue)
	reqChart := m.renderQuadrant("Requests", b.times, b.requests, quadW, quadH, formatAxisInt)
	tokenChart := m.renderQuadrant("Cumulative Tokens", b.times, b.tokens, quadW, quadH, formatAxisTokens)
	cacheChart := m.renderQuadrant("Cache Hit %", b.times, b.cacheRate, quadW, quadH, formatAxisPct)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, costChart, " ", reqChart)
	botRow := lipgloss.JoinHorizontal(lipgloss.Top, tokenChart, " ", cacheChart)
	return topRow + "\n" + botRow
}

func (m Model) renderQuadrant(title string, times []time.Time, values []float64, w, h int, yFmt func(float64) string) string {
	chart := lineChart(
		[]chartSeries{{Values: values}},
		times, w-6, h-4, yFmt,
	)
	return boxStyle.Width(w).Render(titleStyle.Render("  "+title) + "\n" + chart)
}

type chartBuckets struct {
	times     []time.Time
	cost      []float64 // cumulative total cost
	tokens    []float64 // cumulative total tokens (in + out)
	requests  []float64 // requests per bucket
	cacheRate []float64 // cache hit rate % per bucket
}

func (m Model) buildBuckets(from, to time.Time) chartBuckets {
	const numPoints = 60

	n := len(m.requests)
	earliest, latest := from, to
	if earliest.IsZero() {
		earliest = m.requests[n-1].Timestamp
	}
	if latest.IsZero() {
		latest = m.requests[0].Timestamp
	}
	span := latest.Sub(earliest)
	if span <= 0 {
		span = time.Hour
	}
	bucketDur := span / numPoints
	if bucketDur <= 0 {
		bucketDur = time.Second
	}

	var b chartBuckets
	b.cost = make([]float64, numPoints)
	b.tokens = make([]float64, numPoints)
	b.requests = make([]float64, numPoints)
	cacheRead := make([]float64, numPoints)
	totalInput := make([]float64, numPoints)

	for _, r := range m.requests {
		idx := int(r.Timestamp.Sub(earliest) / bucketDur)
		if idx >= numPoints {
			idx = numPoints - 1
		}
		if idx < 0 {
			idx = 0
		}
		if r.TotalCost != nil {
			b.cost[idx] += *r.TotalCost
		}
		b.tokens[idx] += float64(r.InputTokens + r.OutputTokens)
		b.requests[idx]++
		cacheRead[idx] += float64(r.CacheReadTokens)
		totalInput[idx] += float64(r.InputTokens)
	}

	// Cumulative for cost and tokens
	for i := 1; i < numPoints; i++ {
		b.cost[i] += b.cost[i-1]
		b.tokens[i] += b.tokens[i-1]
	}

	// Cache hit rate per bucket
	b.cacheRate = make([]float64, numPoints)
	for i := range numPoints {
		if totalInput[i] > 0 {
			b.cacheRate[i] = cacheRead[i] / totalInput[i] * 100
		}
	}

	b.times = make([]time.Time, numPoints)
	for i := range numPoints {
		b.times[i] = earliest.Add(time.Duration(i)*bucketDur + bucketDur/2)
	}
	return b
}

// ── Cost ──

func (m Model) viewCost() string {
	w := m.width - 6

	stats := m.providerStats
	groupTitle := "By Provider"
	if m.costGroupBy == "model" {
		stats = m.modelStats
		groupTitle = "By Model"
	}

	if len(stats) == 0 {
		return boxStyle.Width(w).Render(mutedStyle.Render("  No data for this period."))
	}

	return m.buildCompactTable(groupTitle, stats, w)
}

func (m Model) buildCompactTable(title string, stats []storage.StatRow, w int) string {
	var totalCost float64
	for _, s := range stats {
		totalCost += s.TotalCost
	}
	maxCost := stats[0].TotalCost

	hasCache := false
	for _, s := range stats {
		if s.CacheReadTokens > 0 || s.CacheWriteTokens > 0 {
			hasCache = true
			break
		}
	}

	// boxStyle.Width(w) includes padding but not border.
	// Content area that fits without wrapping = w - 4 (padding 2+2).
	contentW := w - 4

	// Pre-compute totals for the footer row.
	var totalReqs int
	var totalIn, totalOut int64
	var totalDuration int
	for _, s := range stats {
		totalReqs += s.Requests
		totalIn += s.InputTokens
		totalOut += s.OutputTokens
		totalDuration += s.AvgDurationMs * s.Requests
	}

	// Compute column widths dynamically from data (including totals).
	const gap = 2      // minimum space between columns
	const colName = 20 // name column width (dot+space prefix adds 2 more)
	const nameW = colName + 2
	colReqs := len("REQS")
	colIn := len("IN")
	colOut := len("OUT")
	colCRD := len("C.RD")
	colCWR := len("C.WR")
	colCost := len("COST")
	colPct := len("%")
	colAvg := len("AVG")

	// Measure all data rows + total row.
	for _, s := range stats {
		colReqs = max(colReqs, len(fmt.Sprintf("%d", s.Requests)))
		colIn = max(colIn, len(format.Tokens(s.InputTokens)))
		colOut = max(colOut, len(format.Tokens(s.OutputTokens)))
		if hasCache {
			colCRD = max(colCRD, len(format.Tokens(s.CacheReadTokens)))
			colCWR = max(colCWR, len(format.Tokens(s.CacheWriteTokens)))
		}
		colCost = max(colCost, len(format.Cost(s.TotalCost)))
		colAvg = max(colAvg, len(fmt.Sprintf("%dms", s.AvgDurationMs)))
	}
	colReqs = max(colReqs, len(fmt.Sprintf("%d", totalReqs)))
	colIn = max(colIn, len(format.Tokens(totalIn)))
	colOut = max(colOut, len(format.Tokens(totalOut)))
	colCost = max(colCost, len(format.Cost(totalCost)))
	avgMs := 0
	if totalReqs > 0 {
		avgMs = totalDuration / totalReqs
	}
	colAvg = max(colAvg, len(fmt.Sprintf("%dms", avgMs)))

	// Add gap to each right-aligned column.
	colReqs += gap
	colIn += gap
	colOut += gap
	if hasCache {
		colCRD += gap
		colCWR += gap
	}
	colCost += gap
	colPct += gap
	colAvg += gap

	// Header.
	var rows strings.Builder
	hdr := "    " + padR(mutedStyle.Render("NAME"), nameW) +
		padL(mutedStyle.Render("REQS"), colReqs) +
		padL(mutedStyle.Render("IN"), colIn) +
		padL(mutedStyle.Render("OUT"), colOut)
	if hasCache {
		hdr += padL(mutedStyle.Render("C.RD"), colCRD) + padL(mutedStyle.Render("C.WR"), colCWR)
	}
	hdr += padL(mutedStyle.Render("COST"), colCost) +
		padL(mutedStyle.Render("%"), colPct) +
		padL(mutedStyle.Render("AVG"), colAvg)
	rows.WriteString(hdr + "\n")

	// Data rows.
	for _, s := range stats {
		pct := "  —"
		if totalCost > 0 {
			pct = fmt.Sprintf("%3.0f%%", s.TotalCost/totalCost*100)
		}
		displayKey := s.Key
		colorKey := s.Provider
		if vendor, name := splitModel(s.Key); vendor != "" {
			displayKey = name
			colorKey = vendor
		}
		color := providerColor(colorKey)
		dot := lipgloss.NewStyle().Foreground(color).Render("●")
		latency := padL("—", colAvg)
		if s.AvgDurationMs > 0 {
			latency = padL(fmt.Sprintf("%dms", s.AvgDurationMs), colAvg)
		}
		line := "  " + dot + " " + padR(format.Truncate(displayKey, colName), nameW) +
			padL(fmt.Sprintf("%d", s.Requests), colReqs) +
			padL(format.Tokens(s.InputTokens), colIn) +
			padL(format.Tokens(s.OutputTokens), colOut)
		if hasCache {
			line += padL(format.Tokens(s.CacheReadTokens), colCRD) +
				padL(format.Tokens(s.CacheWriteTokens), colCWR)
		}
		line += padL(format.Cost(s.TotalCost), colCost) +
			mutedStyle.Render(padL(pct, colPct)) +
			mutedStyle.Render(latency)
		barW := min(15, contentW-lipgloss.Width(line)-1)
		if barW > 2 {
			line += " " + hbar(s.TotalCost, maxCost, barW, color)
		}
		rows.WriteString(line + "\n")
	}

	// Total row.
	colsW := lipgloss.Width(hdr)
	rows.WriteString("  " + mutedStyle.Render(strings.Repeat("─", max(1, colsW-2))) + "\n")
	totalLine := "    " + padR(brightStyle.Render("Total"), nameW) +
		padL(brightStyle.Render(fmt.Sprintf("%d", totalReqs)), colReqs) +
		padL(brightStyle.Render(format.Tokens(totalIn)), colIn) +
		padL(brightStyle.Render(format.Tokens(totalOut)), colOut)
	if hasCache {
		totalLine += padL("", colCRD) + padL("", colCWR)
	}
	totalLine += padL(brightStyle.Render(format.Cost(totalCost)), colCost) +
		padL("", colPct) +
		mutedStyle.Render(padL(fmt.Sprintf("%dms", avgMs), colAvg))
	rows.WriteString(totalLine + "\n")

	return boxStyle.Width(w).Render(titleStyle.Render("  "+title) + "\n" + rows.String())
}

// ── Requests ──

func (m Model) viewRequests() string {
	if len(m.requests) == 0 {
		return mutedStyle.Render("  No requests recorded yet.")
	}

	maxVisible := m.reqMaxVisible()
	end := min(len(m.requests), m.reqOffset+maxVisible)

	hasCache := false
	for i := m.reqOffset; i < end; i++ {
		if m.requests[i].CacheReadTokens > 0 || m.requests[i].CacheWriteTokens > 0 {
			hasCache = true
			break
		}
	}

	// Column layout (after 2-char prefix "  " or "▸ "):
	// id(4) " " src(2) " " time(5) " " dot(1) " " model(W) in(6) out(6) [cr(6) cw(6)] cost(9) avg(8)
	fixedW := 4 + 1 + 2 + 1 + 5 + 1 + 1 + 1 + 6 + 6 + 9 + 8 + 2 // +2 for prefix
	if hasCache {
		fixedW += 6 + 6
	}
	modelW := min(28, max(14, m.width-fixedW-10))

	// Header: plain text, styled once — same positions as data
	hdrLine := fmt.Sprintf("%-4s %-2s %-5s   %-*s%6s%6s",
		"#", "", "TIME", modelW, "MODEL", "IN", "OUT")
	if hasCache {
		hdrLine += fmt.Sprintf("%6s%6s", "C.RD", "C.WR")
	}
	hdrLine += fmt.Sprintf("%9s%8s", "COST", "AVG")

	lines := []string{"  " + mutedStyle.Render(hdrLine)}

	for i := m.reqOffset; i < end; i++ {
		r := m.requests[i]
		costStr := padL("—", 9)
		if r.TotalCost != nil {
			costStr = padL(format.Cost(*r.TotalCost), 9)
		}
		latency := padL("—", 8)
		if r.DurationMs > 0 {
			latency = padL(fmt.Sprintf("%dms", r.DurationMs), 8)
		}
		src := sourceTag(r.Source)
		vendor, modelName := splitModel(r.Model)
		dot := lipgloss.NewStyle().Foreground(providerColor(vendor)).Render("●")

		// Truncate plain model name first, then add styled gateway prefix
		gwPrefixLen := 0
		if r.Provider != vendor {
			gwPrefixLen = len(gatewayAbbrev(r.Provider)) + 1 // "or" + "›"
		}
		display := format.Truncate(modelName, modelW-gwPrefixLen)
		if r.Provider != vendor {
			display = lipgloss.NewStyle().Foreground(providerColor(r.Provider)).
				Render(gatewayAbbrev(r.Provider)) + mutedStyle.Render("›") + display
		}

		errMark := ""
		if r.StatusCode >= 400 {
			errMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(" ✗")
		}

		line := padR(fmt.Sprintf("#%d", r.ID), 4) + " " +
			padR(src, 2) + " " +
			r.Timestamp.Local().Format("15:04") + " " +
			dot + " " +
			padR(display, modelW) +
			padL(format.Tokens(int64(r.InputTokens)), 6) +
			padL(format.Tokens(int64(r.OutputTokens)), 6)
		if hasCache {
			line += padL(format.Tokens(int64(r.CacheReadTokens)), 6) +
				padL(format.Tokens(int64(r.CacheWriteTokens)), 6)
		}
		line += costStr + mutedStyle.Render(latency) + errMark

		if i == m.reqCursor {
			lines = append(lines, selectedRowStyle.Render("▸ "+line))
		} else {
			lines = append(lines, "  "+line)
		}
	}

	scrollInfo := ""
	if len(m.requests) > maxVisible {
		scrollInfo = mutedStyle.Render(fmt.Sprintf("  %d/%d", m.reqCursor+1, len(m.requests)))
	}

	title := titleStyle.Render("  Recent Requests") + scrollInfo
	return boxStyle.Width(m.width - 6).Render(title + "\n" + strings.Join(lines, "\n"))
}

// ── Detail ──

func (m Model) renderDetailContent(rec *storage.Record) string {
	costStr := "—"
	if rec.TotalCost != nil {
		costStr = format.Cost(*rec.TotalCost)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("  Request #%d", rec.ID)))
	b.WriteString("\n\n")
	modelDisplay := rec.Model
	if vendor, _ := splitModel(rec.Model); vendor != "" && vendor != rec.Provider {
		modelDisplay = rec.Model + mutedStyle.Render(" via ") +
			lipgloss.NewStyle().Foreground(providerColor(rec.Provider)).Render(rec.Provider)
	}
	b.WriteString(fmt.Sprintf("  %s · %s · %s\n",
		rec.Timestamp.Local().Format("2006-01-02 15:04:05"), modelDisplay,
		mutedStyle.Render(rec.Endpoint)))
	b.WriteString(fmt.Sprintf("  In: %d · Out: %d · Cache read: %d · Cache write: %d · Cost: %s · %dms",
		rec.InputTokens, rec.OutputTokens, rec.CacheReadTokens, rec.CacheWriteTokens, costStr, rec.DurationMs))
	if rec.Streaming {
		b.WriteString(" · streaming")
	}
	if s, ok := sources[rec.Source]; ok {
		b.WriteString(" · " + s.label)
	}
	b.WriteString("\n\n")

	if len(rec.RequestBody) > 0 {
		b.WriteString(mutedStyle.Render("  ── Prompt ──") + "\n\n")
		b.WriteString(prettyJSON(rec.RequestBody))
		b.WriteString("\n\n")
	}
	if len(rec.ResponseBody) > 0 {
		b.WriteString(mutedStyle.Render("  ── Response ──") + "\n\n")
		b.WriteString(prettyJSON(rec.ResponseBody))
	}

	return b.String()
}

// ── Help ──

func (m Model) viewHelp() string {
	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{"Global", []struct{ key, desc string }{
			{"1-4", "Jump to tab"},
			{"tab / shift+tab", "Next / prev tab"},
			{"p", "Cycle period"},
			{"s", "Cycle source filter"},
			{"f", "Cycle provider filter"},
			{"e", "Export filtered data to CSV"},
			{"?", "Toggle help"},
			{"q / ctrl+c", "Quit"},
		}},
		{"Overview", []struct{ key, desc string }{
			{"h j k l / ← → ↑ ↓", "Navigate heatmap"},
			{"click", "Select day"},
		}},
		{"Cost", []struct{ key, desc string }{
			{"m", "Toggle provider / model"},
		}},
		{"Requests", []struct{ key, desc string }{
			{"↑ ↓ / j k / scroll", "Navigate list"},
			{"enter / click×2", "View detail"},
			{"click", "Select request"},
		}},
		{"Detail View", []struct{ key, desc string }{
			{"c / p / r", "Copy all / prompt / response"},
			{"esc / backspace", "Back"},
			{"scroll", "Scroll content"},
		}},
	}

	var b strings.Builder
	for _, sec := range sections {
		b.WriteString(titleStyle.Render("  "+sec.title) + "\n")
		for _, k := range sec.keys {
			b.WriteString(fmt.Sprintf("    %-22s %s\n",
				brightStyle.Render(k.key), mutedStyle.Render(k.desc)))
		}
		b.WriteString("\n")
	}

	w := min(50, m.width-4)
	box := boxStyle.Width(w).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) navigateUp() {
	switch m.activeTab {
	case tabOverview:
		if m.hmGrid != nil {
			m.hmRow = max(m.hmRow-1, 0)
			m.loadHmDay()
		}
	case tabRequests:
		if m.reqCursor > 0 {
			m.reqCursor--
			m.adjustReqScroll()
		}
	}
}

func (m *Model) navigateDown() {
	switch m.activeTab {
	case tabOverview:
		if m.hmGrid != nil {
			m.hmRow = min(m.hmRow+1, 6)
			m.loadHmDay()
		}
	case tabRequests:
		if m.reqCursor < len(m.requests)-1 {
			m.reqCursor++
			m.adjustReqScroll()
		}
	}
}

func (m *Model) handleTabClick(x int) {
	rendered := m.renderTabBar()
	for i := range tabNames {
		label := fmt.Sprintf("%d·%s", i+1, tabNames[i])
		if idx := strings.Index(rendered, label); idx >= 0 {
			screenX := lipgloss.Width(rendered[:idx])
			labelEnd := screenX + lipgloss.Width(label) + 2
			if x >= screenX && x < labelEnd {
				m.switchTab(tab(i))
				return
			}
		}
	}
}

func (m *Model) handleRequestClick(y int) {
	// Find header row via marker text to get robust screen Y offset.
	rendered := m.View()
	lines := strings.Split(rendered, "\n")
	headerY := -1
	for i, line := range lines {
		if strings.Contains(line, "TIME") && strings.Contains(line, "MODEL") {
			headerY = i
			break
		}
	}
	if headerY < 0 {
		return
	}
	idx := m.reqOffset + (y - headerY - 1)
	if idx >= 0 && idx < len(m.requests) {
		if idx == m.reqCursor {
			if rec, err := m.store.Get(m.requests[idx].ID); err == nil {
				m.reqDetail = rec
				m.viewport = viewport.New(m.width-4, m.contentHeight())
				m.viewport.SetContent(m.renderDetailContent(rec))
			}
		} else {
			m.reqCursor = idx
			m.adjustReqScroll()
		}
	}
}

func (m *Model) handleHeatmapClick(x, y int) {
	if m.hmGrid == nil {
		return
	}
	// Find "Mon " marker to get robust screen position.
	// lipgloss.Width strips ANSI codes to convert byte offset → screen column.
	rendered := m.View()
	lines := strings.Split(rendered, "\n")
	gridStartY := -1
	gridStartX := -1
	for i, line := range lines {
		if idx := strings.Index(line, "Mon "); idx >= 0 {
			gridStartY = i
			gridStartX = lipgloss.Width(line[:idx]) + m.hmGrid.LabelW
			break
		}
	}
	if gridStartY < 0 {
		return
	}

	row := y - gridStartY
	col := (x - gridStartX) / m.hmGrid.CellW
	if row >= 0 && row < 7 && col >= 0 && col < m.hmGrid.TotalWeeks {
		m.hmRow = row
		m.hmCol = col
		m.loadHmDay()
	}
}

// ── Data ──

func (m *Model) loadHmDay() {
	if m.hmGrid == nil {
		m.hmDay = nil
		return
	}
	d := m.hmGrid.CellToDate(m.hmCol, m.hmRow)
	dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local).UTC()
	dayEnd := dayStart.AddDate(0, 0, 1)

	// Don't show future days (padding cells from week alignment).
	if dayStart.After(time.Now().UTC()) {
		m.hmDay = nil
		return
	}

	m.hmDay, _ = m.store.Stats(storage.StatsFilter{
		From: dayStart, To: dayEnd, GroupBy: "model",
		Provider: m.providerFilter, Source: m.source,
	})
}

func (m *Model) loadData() {
	_, m.daemonRunning = daemon.IsRunning(m.dataDir)

	from, to := storage.PeriodToTimeRange(m.period)
	m.availSources = m.buildAvailSources(from, to)
	pf := m.providerFilter
	f := func(groupBy string) []storage.StatRow {
		rows, _ := m.store.Stats(storage.StatsFilter{From: from, To: to, GroupBy: groupBy, Provider: pf, Source: m.source})
		return rows
	}

	switch m.activeTab {
	case tabOverview:
		m.providerStats = f("provider")
		m.modelStats = f("model")
		// Heatmap always shows all-time data (independent of period).
		m.dailyStats, _ = m.store.Stats(storage.StatsFilter{GroupBy: "day", Provider: pf, Source: m.source})
		prevFrom, prevTo := previousPeriod(m.period)
		if !prevFrom.IsZero() {
			m.prevProviderStats, _ = m.store.Stats(storage.StatsFilter{
				From: prevFrom, To: prevTo, GroupBy: "provider", Provider: pf, Source: m.source,
			})
		} else {
			m.prevProviderStats = nil
		}
		// Compute heatmap grid for cursor navigation (without rendering).
		wasNil := m.hmGrid == nil
		m.hmGrid = computeHeatmapGrid(m.dailyStats, m.width-12)
		if m.hmGrid != nil {
			if wasNil {
				m.hmCol = m.hmGrid.TotalWeeks - 1
				m.hmRow = (int(time.Now().Weekday()) + 6) % 7
			}
			m.hmCol = min(m.hmCol, m.hmGrid.TotalWeeks-1)
			m.hmRow = min(m.hmRow, 6)
			m.loadHmDay()
		}
	case tabChart:
		m.requests, _ = m.store.Recent(5000, from, to, pf, m.source)
	case tabCost:
		m.providerStats = f("provider")
		m.modelStats = f("model")
	case tabRequests:
		m.requests, _ = m.store.Recent(1000, from, to, pf, m.source)
		if m.reqCursor >= len(m.requests) {
			m.reqCursor = max(0, len(m.requests)-1)
		}
		m.adjustReqScroll()
	}
}

func previousPeriod(period string) (from, to time.Time) {
	now := time.Now()
	switch period {
	case "today":
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		yesterday := todayStart.AddDate(0, 0, -1)
		from = yesterday.UTC()
		to = todayStart.UTC()
	case "week":
		from = now.AddDate(0, 0, -14).UTC()
		to = now.AddDate(0, 0, -7).UTC()
	case "month":
		from = now.AddDate(0, -2, 0).UTC()
		to = now.AddDate(0, -1, 0).UTC()
	default:
		return time.Time{}, time.Time{}
	}
	return
}

// ── Helpers ──

func clipboardCmd() string {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy"
	case "windows":
		return "clip"
	default:
		return "xclip"
	}
}

func (m Model) reqMaxVisible() int {
	return max(3, m.contentHeight()-6)
}

func (m *Model) adjustReqScroll() {
	vis := m.reqMaxVisible()
	if m.reqCursor >= m.reqOffset+vis {
		m.reqOffset = m.reqCursor - vis + 1
	}
	if m.reqCursor < m.reqOffset {
		m.reqOffset = m.reqCursor
	}
}

func (m *Model) switchTab(t tab) {
	m.activeTab = t
	if t == tabRequests {
		m.reqCursor = 0
		m.reqOffset = 0
	}
	m.loadData()
}

func (m *Model) cycleSource() {
	filters := m.availSources
	if len(filters) == 0 {
		filters = []string{""}
	}
	for i, v := range filters {
		if v == m.source {
			m.source = filters[(i+1)%len(filters)]
			return
		}
	}
	m.source = filters[0]
}

// buildAvailSources returns source filter options that have data in the period.
func (m *Model) buildAvailSources(from, to time.Time) []string {
	dbSources, _ := m.store.Sources(from, to)

	has := make(map[string]bool)
	for _, s := range dbSources {
		has[s] = true
	}

	// Always include "all"
	filters := []string{""}

	// Add specific sources only if they exist in the data
	for _, s := range []string{"cc:sub", "cc:key", "copilot:key"} {
		if has[s] {
			filters = append(filters, s)
		}
	}

	// Direct API calls (no recognized client)
	if has[""] {
		filters = append(filters, "direct")
	}

	return filters
}

func (m *Model) cycleProvider() {
	// Get providers for current period (unfiltered)
	from, to := storage.PeriodToTimeRange(m.period)
	allProviders, _ := m.store.Stats(storage.StatsFilter{From: from, To: to, GroupBy: "provider", Source: m.source})
	providers := []string{""}
	for _, s := range allProviders {
		providers = append(providers, s.Key)
	}
	for i, v := range providers {
		if v == m.providerFilter {
			m.providerFilter = providers[(i+1)%len(providers)]
			return
		}
	}
	m.providerFilter = ""
}

func (m *Model) cyclePeriod() {
	for i, p := range periods {
		if p == m.period {
			m.period = periods[(i+1)%len(periods)]
			return
		}
	}
}

// ── Source display ──

type sourceInfo struct {
	tag   string
	label string
	style lipgloss.Style
}

var sources = map[string]sourceInfo{
	"cc:sub":      {"CC", "Claude Code (sub)", sourceTagStyle},
	"cc:key":      {"CC", "Claude Code (key)", sourceTagKeyStyle},
	"copilot:key": {"CP", "GitHub Copilot", sourceTagCopilotStyle},
}

var sourceFilterLabels = map[string]string{
	"cc:sub":      "CC subscription",
	"cc:key":      "CC api-key",
	"copilot:key": "Copilot api-key",
	"direct":      "direct",
}

func sourceTag(source string) string {
	if s, ok := sources[source]; ok {
		return s.style.Render(s.tag)
	}
	return "  "
}

// ── Formatting ──

var (
	jsonKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	jsonStrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	jsonNumStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	jsonBoolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	jsonNullStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	jsonSyntaxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

func prettyJSON(data []byte) string {
	var v any
	if json.Unmarshal(data, &v) != nil {
		s := string(data)
		if len(s) > 2000 {
			s = s[:2000] + "…"
		}
		return "  " + s
	}
	plain, _ := json.MarshalIndent(v, "  ", "  ")
	return "  " + colorizeJSON(string(plain))
}

func colorizeJSON(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		ch := s[i]
		switch {
		case ch == '"':
			end := i + 1
			for end < len(s) && s[end] != '"' {
				if s[end] == '\\' {
					end++
				}
				end++
			}
			if end < len(s) {
				end++
			}
			token := s[i:end]
			rest := strings.TrimLeft(s[end:], " \t")
			if len(rest) > 0 && rest[0] == ':' {
				b.WriteString(jsonKeyStyle.Render(token))
			} else {
				b.WriteString(jsonStrStyle.Render(token))
			}
			i = end
		case ch == 't' && strings.HasPrefix(s[i:], "true"):
			b.WriteString(jsonBoolStyle.Render("true"))
			i += 4
		case ch == 'f' && strings.HasPrefix(s[i:], "false"):
			b.WriteString(jsonBoolStyle.Render("false"))
			i += 5
		case ch == 'n' && strings.HasPrefix(s[i:], "null"):
			b.WriteString(jsonNullStyle.Render("null"))
			i += 4
		case ch == '-' || (ch >= '0' && ch <= '9'):
			end := i + 1
			for end < len(s) && (s[end] == '.' || s[end] == 'e' || s[end] == 'E' || s[end] == '+' || s[end] == '-' || (s[end] >= '0' && s[end] <= '9')) {
				end++
			}
			b.WriteString(jsonNumStyle.Render(s[i:end]))
			i = end
		case ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ':' || ch == ',':
			b.WriteString(jsonSyntaxStyle.Render(string(ch)))
			i++
		default:
			b.WriteByte(ch)
			i++
		}
	}
	return b.String()
}

// gatewayAbbrev returns a short abbreviation for a gateway name.
func gatewayAbbrev(gw string) string {
	switch gw {
	case "openrouter":
		return "or"
	case "anthropic":
		return "an"
	case "openai":
		return "oa"
	default:
		if len(gw) > 2 {
			return gw[:2]
		}
		return gw
	}
}

// splitModel splits "vendor/model" into parts. Returns ("", model) if no prefix.
func splitModel(model string) (vendor, name string) {
	if i := strings.Index(model, "/"); i >= 0 {
		return model[:i], model[i+1:]
	}
	return "", model
}

// padL right-aligns s within width, using visible (ANSI-aware) width.
func padL(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return strings.Repeat(" ", width-vis) + s
}

// padR left-aligns s within width, using visible (ANSI-aware) width.
func padR(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}
