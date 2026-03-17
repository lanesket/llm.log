package dashboard

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
	"github.com/lanesket/llm.log/internal/format"
)

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// sparkline renders a sparkline from values, resampled to fit width.
func sparkline(values []float64, width int) string {
	if len(values) == 0 {
		return strings.Repeat("▁", width)
	}

	data := values
	if len(data) > width {
		data = resample(data, width)
	}

	maxVal := 0.0
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		return strings.Repeat("▁", len(data))
	}

	var b strings.Builder
	for _, v := range data {
		idx := int(v / maxVal * float64(len(sparkBlocks)-1))
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}

func resample(values []float64, n int) []float64 {
	result := make([]float64, n)
	step := float64(len(values)) / float64(n)
	for i := range n {
		idx := int(math.Round(float64(i) * step))
		if idx >= len(values) {
			idx = len(values) - 1
		}
		result[i] = values[idx]
	}
	return result
}

// hbar renders a horizontal bar with a visible track showing the max boundary.
func hbar(value, maxVal float64, width int, color lipgloss.Color) string {
	if maxVal == 0 || width <= 0 || value == 0 {
		return ""
	}
	filled := int(value / maxVal * float64(width))
	if filled < 1 {
		filled = 1
	}
	track := width - filled
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	if track > 0 {
		bar += lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("░", track))
	}
	return bar
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
