package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseTimeRange parses "from" and "to" query parameters.
// Accepts RFC3339 or "2006-01-02T15:04" format.
// Missing params yield zero time (unbounded).
func parseTimeRange(r *http.Request) (from, to time.Time, err error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr != "" {
		from, err = parseTime(fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from': %w", err)
		}
	}
	if toStr != "" {
		to, err = parseTime(toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to': %w", err)
		}
	}

	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("'from' must be before 'to'")
	}

	if !from.IsZero() && !to.IsZero() && to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("time range must not exceed 1 year")
	}

	return from, to, nil
}

func parseTime(s string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try compact datetime format
	if t, err := time.Parse("2006-01-02T15:04", s); err == nil {
		return t, nil
	}
	// Try date-only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q (expected RFC3339, 2006-01-02T15:04, or 2006-01-02)", s)
}
