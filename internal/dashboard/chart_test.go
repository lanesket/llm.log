package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lanesket/llm.log/internal/storage"
)

func testTimes(n int, interval time.Duration) []time.Time {
	start := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	times := make([]time.Time, n)
	for i := range n {
		times[i] = start.Add(time.Duration(i) * interval)
	}
	return times
}

func TestHbar(t *testing.T) {
	result := hbar(50, 100, 20, "#FFFFFF")
	if !strings.Contains(result, "██████████") {
		t.Errorf("bar should have ~10 blocks, got %q", result)
	}
}

func TestHbar_Zero(t *testing.T) {
	if result := hbar(0, 100, 20, "#FFFFFF"); result != "" {
		t.Errorf("zero value bar should be empty, got %q", result)
	}
}

func TestHbar_ZeroMax(t *testing.T) {
	if result := hbar(50, 0, 20, "#FFFFFF"); result != "" {
		t.Errorf("zero max bar should be empty, got %q", result)
	}
}

func TestLineChart_Basic(t *testing.T) {
	series := []chartSeries{{Values: []float64{1, 3, 2, 5, 4}}}
	result := lineChart(series, testTimes(5, time.Hour), 40, 8)
	if result == "" {
		t.Error("lineChart returned empty string")
	}
	if !strings.Contains(result, "$") {
		t.Error("lineChart should have Y-axis dollar labels")
	}
}

func TestLineChart_Empty(t *testing.T) {
	result := lineChart(nil, nil, 40, 8)
	if result != "" {
		t.Errorf("empty lineChart should return empty, got %q", result)
	}
}

func TestLineChart_SinglePoint(t *testing.T) {
	series := []chartSeries{{Values: []float64{5}}}
	result := lineChart(series, testTimes(1, 0), 30, 6)
	if result == "" {
		t.Error("single-point lineChart returned empty")
	}
}

func TestLineChart_TokenFormatter(t *testing.T) {
	series := []chartSeries{{Values: []float64{1000, 5000, 3000}}}
	result := lineChart(series, testTimes(3, 2*time.Hour), 40, 8, formatAxisTokens)
	if result == "" {
		t.Error("lineChart with token formatter returned empty")
	}
	if strings.Contains(result, "$") {
		t.Error("token chart should not have dollar signs")
	}
}

func TestHeatmap_BasicRendering(t *testing.T) {
	stats := []storage.StatRow{
		{Key: "2026-03-25", TotalCost: 5.0},  // Wednesday
		{Key: "2026-03-26", TotalCost: 10.0}, // Thursday
		{Key: "2026-03-27", TotalCost: 2.0},  // Friday
	}
	result, _ := heatmap(stats, 80, -1, -1)

	// Must contain weekday labels
	if !strings.Contains(result, "Mon") {
		t.Error("heatmap missing Mon label")
	}
	if !strings.Contains(result, "Fri") {
		t.Error("heatmap missing Fri label")
	}
	// Must contain month label
	if !strings.Contains(result, "Mar") {
		t.Error("heatmap missing month label")
	}
	// Must contain legend
	if !strings.Contains(result, "Less") {
		t.Error("heatmap missing legend")
	}
	// Must not be empty
	if len(result) < 50 {
		t.Errorf("heatmap too short: %d chars", len(result))
	}
}

func TestHeatmap_Empty(t *testing.T) {
	result, _ := heatmap(nil, 80, -1, -1)
	if result != "" {
		t.Errorf("empty stats should produce empty string, got %q", result)
	}
}

func TestHeatmap_SingleDay(t *testing.T) {
	stats := []storage.StatRow{
		{Key: "2026-03-28", TotalCost: 1.0},
	}
	result, _ := heatmap(stats, 80, -1, -1)
	if !strings.Contains(result, "Mar") {
		t.Error("single-day heatmap missing month label")
	}
}

func TestHeatmap_MonthLabelsNoOverlap(t *testing.T) {
	// Feb-Mar boundary — labels must not merge into "FebMar".
	stats := make([]storage.StatRow, 0)
	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for i := range 55 {
		d := base.AddDate(0, 0, i)
		stats = append(stats, storage.StatRow{Key: d.Format("2006-01-02"), TotalCost: float64(i + 1)})
	}
	result, _ := heatmap(stats, 80, -1, -1)
	firstLine := strings.Split(result, "\n")[0]
	if strings.Contains(firstLine, "FebMar") || strings.Contains(firstLine, "MarApr") {
		t.Errorf("month labels overlap: %q", firstLine)
	}
	// Both months should still appear.
	if !strings.Contains(firstLine, "Feb") {
		t.Error("missing Feb label")
	}
	if !strings.Contains(firstLine, "Mar") {
		t.Error("missing Mar label")
	}
}

func TestHeatmap_WidthAdaptation(t *testing.T) {
	// Generate 90 days of data
	stats := make([]storage.StatRow, 90)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range stats {
		d := base.AddDate(0, 0, i)
		stats[i] = storage.StatRow{Key: d.Format("2006-01-02"), TotalCost: float64(i)}
	}

	narrow, _ := heatmap(stats, 40, -1, -1)
	wide, _ := heatmap(stats, 120, -1, -1)

	// Narrow rendering should have fewer columns (weeks) than wide
	narrowLines := strings.Split(narrow, "\n")
	wideLines := strings.Split(wide, "\n")
	if len(narrowLines) == 0 || len(wideLines) == 0 {
		t.Fatal("heatmap produced no lines")
	}
	// The Mon row (second data row) should be shorter in narrow mode
	if lipgloss.Width(narrowLines[1]) >= lipgloss.Width(wideLines[1]) {
		t.Error("narrow heatmap should have fewer columns than wide")
	}
}
