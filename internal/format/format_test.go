package format

import "testing"

func TestCost(t *testing.T) {
	tests := []struct {
		c    float64
		want string
	}{
		{0, "—"},
		{0.0001, "$0.000100"},
		{0.001, "$0.0010"},
		{0.123, "$0.1230"},
		{5.67, "$5.67"},
	}
	for _, tt := range tests {
		got := Cost(tt.c)
		if got != tt.want {
			t.Errorf("Cost(%v) = %q, want %q", tt.c, got, tt.want)
		}
	}
}

func TestTokens(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{500, "500"},
		{1500, "1.5K"},
		{1_500_000, "1.5M"},
	}
	for _, tt := range tests {
		got := Tokens(tt.n)
		if got != tt.want {
			t.Errorf("Tokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hell…"},
		{"hi", 1, "…"},
		{"", 5, ""},
		{"hello", 0, ""},
	}
	for _, tt := range tests {
		got := Truncate(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}
