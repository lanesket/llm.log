package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/storage"
)

// hbar renders a horizontal bar with a visible track showing the max boundary.
func hbar(value, maxVal float64, width int, color lipgloss.Color) string {
	if maxVal == 0 || width <= 0 || value == 0 {
		return ""
	}
	filled := max(1, int(value/maxVal*float64(width)))
	track := width - filled
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	if track > 0 {
		bar += lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("░", track))
	}
	return bar
}

// heatmapGrid holds computed grid geometry so the caller can map cursor/mouse to dates.
type heatmapGrid struct {
	StartMon   time.Time
	TotalWeeks int
	CellW      int // chars per cell column (including gap)
	LabelW     int // chars for day label column
}

func (g *heatmapGrid) CellToDate(col, row int) time.Time {
	return g.StartMon.AddDate(0, 0, col*7+row)
}

// computeHeatmapGrid returns grid metadata without rendering the full heatmap string.
func computeHeatmapGrid(dailyStats []storage.StatRow, width int) *heatmapGrid {
	if len(dailyStats) == 0 {
		return nil
	}
	var earliest, latest time.Time
	for _, s := range dailyStats {
		t, err := time.Parse("2006-01-02", s.Key)
		if err != nil {
			continue
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
		if t.After(latest) {
			latest = t
		}
	}
	if earliest.IsZero() {
		return nil
	}

	startMon := earliest.AddDate(0, 0, -((int(earliest.Weekday()) + 6) % 7))
	endSun := latest.AddDate(0, 0, (7-int(latest.Weekday()))%7)
	totalWeeks := int(endSun.Sub(startMon).Hours()/24)/7 + 1

	cellW := 3
	labelW := 4
	maxWeeks := max(1, (width-labelW)/cellW)
	if totalWeeks > maxWeeks {
		startMon = endSun.AddDate(0, 0, -(maxWeeks*7)+1)
		startMon = startMon.AddDate(0, 0, -((int(startMon.Weekday()) + 6) % 7))
		totalWeeks = maxWeeks
	}
	return &heatmapGrid{StartMon: startMon, TotalWeeks: totalWeeks, CellW: cellW, LabelW: labelW}
}

// Pre-rendered cells per intensity level.
var (
	heatmapCells      [5]string
	heatmapCursorCell [5]string
)

func init() {
	cursorFrame := lipgloss.Color("#E2E8F0")
	heatmapCells[0] = lipgloss.NewStyle().Foreground(heatmapColors[0]).Render("··")
	heatmapCursorCell[0] = lipgloss.NewStyle().Foreground(cursorFrame).Background(heatmapColors[0]).Render("▌▐")
	for i := 1; i < len(heatmapColors); i++ {
		heatmapCells[i] = lipgloss.NewStyle().Foreground(heatmapColors[i]).Render("██")
		heatmapCursorCell[i] = lipgloss.NewStyle().Foreground(cursorFrame).Background(heatmapColors[i]).Render("▌▐")
	}
}

// heatmap renders a GitHub-style contribution heatmap from daily stats.
// cursorRow/cursorCol < 0 means no cursor highlight.
func heatmap(dailyStats []storage.StatRow, width, cursorRow, cursorCol int) (string, *heatmapGrid) {
	grid := computeHeatmapGrid(dailyStats, width)
	if grid == nil {
		return "", nil
	}

	startMon := grid.StartMon
	totalWeeks := grid.TotalWeeks
	cellW := grid.CellW
	labelW := grid.LabelW

	// Build cost lookup and find max in a single pass.
	costByDate := make(map[string]float64, len(dailyStats))
	var maxCost float64
	for _, s := range dailyStats {
		costByDate[s.Key] = s.TotalCost
		if s.TotalCost > maxCost {
			maxCost = s.TotalCost
		}
	}

	// Intensity level: 0-4 based on cost relative to max.
	level := func(cost float64) int {
		if cost == 0 || maxCost == 0 {
			return 0
		}
		ratio := cost / maxCost
		switch {
		case ratio >= 0.75:
			return 4
		case ratio >= 0.50:
			return 3
		case ratio >= 0.25:
			return 2
		default:
			return 1
		}
	}

	// Build month label positions: which weeks start a new month.
	type monthLabel struct {
		week  int
		label string
	}
	var months []monthLabel
	prevMonth := time.Month(0)
	monthWeekCount := make(map[time.Month]int)
	for w := range totalWeeks {
		weekStart := startMon.AddDate(0, 0, w*7)
		mon := weekStart.Month()
		monthWeekCount[mon]++
		if mon != prevMonth {
			months = append(months, monthLabel{week: w, label: weekStart.Format("Jan")})
			prevMonth = mon
		}
	}

	// Render month labels into a buffer, then convert to string.
	rowLen := totalWeeks * cellW
	buf := make([]byte, rowLen)
	for i := range buf {
		buf[i] = ' '
	}
	prevEnd := 0
	for _, ml := range months {
		pos := ml.week * cellW
		label := ml.label
		// Skip if it would overlap with previous label (need 1+ char gap).
		if pos < prevEnd+1 && prevEnd > 0 {
			continue
		}
		// Skip months that occupy only 1 week (padding from week alignment).
		weekStart := startMon.AddDate(0, 0, ml.week*7)
		if monthWeekCount[weekStart.Month()] < 2 && len(months) > 1 {
			continue
		}
		if pos+len(label) <= rowLen {
			copy(buf[pos:], label)
			prevEnd = pos + len(label)
		}
	}

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", labelW))
	b.WriteString(strings.TrimRight(string(buf), " "))
	b.WriteString("\n")

	// Render 7 day rows (Monday=0 .. Sunday=6).
	dayLabels := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for row := range 7 {
		// Show label for Mon, Wed, Fri only (like GitHub).
		if row%2 == 0 {
			b.WriteString(fmt.Sprintf("%-4s", dayLabels[row]))
		} else {
			b.WriteString(strings.Repeat(" ", labelW))
		}

		for w := range totalWeeks {
			d := startMon.AddDate(0, 0, w*7+row)
			lvl := level(costByDate[d.Format("2006-01-02")])
			if row == cursorRow && w == cursorCol {
				b.WriteString(heatmapCursorCell[lvl])
			} else {
				b.WriteString(heatmapCells[lvl])
			}
			b.WriteString(" ")
		}
		b.WriteString("\n")
	}

	// Legend row — blank line for breathing room, then aligned with grid.
	b.WriteString("\n")
	b.WriteString(strings.Repeat(" ", labelW))
	b.WriteString(mutedStyle.Render("Less "))
	for _, cell := range heatmapCells {
		b.WriteString(cell)
		b.WriteString(" ")
	}
	b.WriteString(mutedStyle.Render("More  │  Click on a day to see breakdown"))

	return b.String(), grid
}

// ── Line chart (powered by asciigraph) ──

type chartSeries struct {
	Values []float64
}

// lineChart renders a multi-series line chart using asciigraph, with a smart time axis below.
func lineChart(series []chartSeries, times []time.Time, width, height int, fmtY ...func(float64) string) string {
	if len(series) == 0 || height < 2 || width < 10 {
		return ""
	}

	yFmt := formatAxisValue
	if len(fmtY) > 0 && fmtY[0] != nil {
		yFmt = fmtY[0]
	}

	chartH := max(5, height-2)

	// Estimate Y-axis label width for the time axis offset.
	globalMax := 0.0
	for _, s := range series {
		for _, v := range s.Values {
			if v > globalMax {
				globalMax = v
			}
		}
	}
	yLabelW := 0
	for _, v := range []float64{0, globalMax / 4, globalMax / 2, globalMax * 3 / 4, globalMax} {
		if w := len(yFmt(v)); w > yLabelW {
			yLabelW = w
		}
	}
	yLabelW += 2

	// Plot width = available space minus Y-axis labels.
	// asciigraph stretches/compresses data points to fit this width.
	plotW := max(10, width-yLabelW-2)

	var data [][]float64
	for _, s := range series {
		data = append(data, s.Values)
	}

	chart := asciigraph.PlotMany(data, []asciigraph.Option{
		asciigraph.Height(chartH),
		asciigraph.Width(plotW),
		asciigraph.SeriesColors(asciigraph.Cyan),
		asciigraph.LowerBound(0),
		asciigraph.YAxisValueFormatter(yFmt),
		asciigraph.AxisColor(asciigraph.Gray),
		asciigraph.LabelColor(asciigraph.Gray),
	}...)

	var b strings.Builder
	b.WriteString(chart)
	b.WriteString("\n")

	if len(times) >= 2 {
		b.WriteString(renderTimeAxis(times[0], times[len(times)-1], plotW, yLabelW))
	}

	return b.String()
}

// renderTimeAxis generates smart axis labels based on the time span.
func renderTimeAxis(first, last time.Time, chartW, offset int) string {
	first = first.Local()
	last = last.Local()
	span := last.Sub(first)
	if span <= 0 {
		return ""
	}

	type tickConfig struct {
		interval time.Duration
		labelFmt string
		snap     func(time.Time) time.Time
	}

	var cfg tickConfig
	switch {
	case span <= 3*time.Hour:
		cfg = tickConfig{30 * time.Minute, "15:04", func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), (t.Minute()/30)*30, 0, 0, t.Location())
		}}
	case span <= 12*time.Hour:
		cfg = tickConfig{time.Hour, "15:04", func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
		}}
	case span <= 36*time.Hour:
		cfg = tickConfig{3 * time.Hour, "15:04", func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), (t.Hour()/3)*3, 0, 0, 0, t.Location())
		}}
	case span <= 7*24*time.Hour:
		cfg = tickConfig{24 * time.Hour, "Jan 02", func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}}
	default:
		days := int(span.Hours() / 24)
		step := max(1, days/8)
		cfg = tickConfig{time.Duration(step) * 24 * time.Hour, "Jan 02", func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}}
	}

	maxLabels := max(2, chartW/10)

	type posLabel struct {
		pos   int
		label string
	}
	var ticks []posLabel

	tick := cfg.snap(first.Add(cfg.interval))
	for !tick.After(last) && len(ticks) < maxLabels {
		frac := float64(tick.Sub(first)) / float64(span)
		pos := int(frac * float64(chartW-1))
		if pos >= 0 && pos < chartW {
			ticks = append(ticks, posLabel{pos, tick.Format(cfg.labelFmt)})
		}
		tick = tick.Add(cfg.interval)
	}

	firstLabel := first.Format(cfg.labelFmt)
	lastLabel := last.Format(cfg.labelFmt)
	lastPos := chartW - len(lastLabel)

	// Filter ticks that would overlap
	var filtered []posLabel
	prevEnd := len(firstLabel)
	for _, t := range ticks {
		if t.pos < prevEnd+2 {
			continue
		}
		if t.pos+len(t.label) > lastPos-2 {
			continue
		}
		filtered = append(filtered, t)
		prevEnd = t.pos + len(t.label)
	}

	buf := make([]byte, offset+chartW+20)
	for i := range buf {
		buf[i] = ' '
	}
	place := func(pos int, label string) {
		p := offset + pos
		for i, ch := range []byte(label) {
			if p+i < len(buf) {
				buf[p+i] = ch
			}
		}
	}

	place(0, firstLabel)
	for _, t := range filtered {
		place(t.pos, t.label)
	}
	if lastPos > prevEnd+2 {
		place(lastPos, lastLabel)
	}

	return mutedStyle.Render(strings.TrimRight(string(buf), " "))
}

func formatAxisValue(v float64) string {
	return format.Cost(v)
}

func formatAxisTokens(v float64) string {
	return format.Tokens(int64(v))
}

func formatAxisInt(v float64) string {
	n := int64(v)
	if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatAxisPct(v float64) string {
	return fmt.Sprintf("%.0f%%", v)
}
