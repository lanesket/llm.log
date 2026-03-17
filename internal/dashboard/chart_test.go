package dashboard

import (
	"strings"
	"testing"
	"time"
)

func testTimes(n int, interval time.Duration) []time.Time {
	start := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	times := make([]time.Time, n)
	for i := range n {
		times[i] = start.Add(time.Duration(i) * interval)
	}
	return times
}

func TestSparkline_Basic(t *testing.T) {
	result := sparkline([]float64{0, 0.5, 1.0, 0.5, 0}, 5)
	if len([]rune(result)) != 5 {
		t.Errorf("sparkline length = %d, want 5", len([]rune(result)))
	}
	runes := []rune(result)
	if runes[2] != '█' {
		t.Errorf("peak char = %c, want █", runes[2])
	}
	if runes[0] != '▁' {
		t.Errorf("zero char = %c, want ▁", runes[0])
	}
}

func TestSparkline_Empty(t *testing.T) {
	result := sparkline(nil, 10)
	if len([]rune(result)) != 10 {
		t.Errorf("empty sparkline length = %d, want 10", len([]rune(result)))
	}
}

func TestSparkline_AllZero(t *testing.T) {
	result := sparkline([]float64{0, 0, 0}, 3)
	for _, r := range result {
		if r != '▁' {
			t.Errorf("zero values should produce ▁, got %c", r)
		}
	}
}

func TestSparkline_Resample(t *testing.T) {
	vals := make([]float64, 10)
	for i := range vals {
		vals[i] = float64(i)
	}
	result := sparkline(vals, 5)
	if len([]rune(result)) != 5 {
		t.Errorf("resampled length = %d, want 5", len([]rune(result)))
	}
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
