package format

import "fmt"

// Cost formats a dollar amount for display.
func Cost(c float64) string {
	if c == 0 {
		return "—"
	}
	if c < 0.001 {
		return fmt.Sprintf("$%.6f", c)
	}
	if c < 1.0 {
		return fmt.Sprintf("$%.4f", c)
	}
	return fmt.Sprintf("$%.2f", c)
}

// Tokens formats a token count with K/M suffixes.
func Tokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// Truncate trims a string to maxLen runes, adding "…" if truncated.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if runes := []rune(s); len(runes) > maxLen {
		if maxLen == 1 {
			return "…"
		}
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}
