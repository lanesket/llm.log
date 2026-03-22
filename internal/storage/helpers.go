package storage

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"
)

// chartBucket returns the SQL bucket expression and Go parse layout for a time
// range, optionally overridden by an explicit granularity string.
func chartBucket(from, to time.Time, granularity string) (bucketExpr, parseLayout string) {
	g := granularity
	if g == "" {
		duration := to.Sub(from)
		switch {
		case duration <= 2*time.Hour:
			g = "minute"
		case duration <= 48*time.Hour:
			g = "hour"
		case duration <= 90*24*time.Hour:
			g = "day"
		default:
			g = "week"
		}
	}

	switch g {
	case "minute":
		return "strftime('%Y-%m-%dT%H:%M:00', timestamp)", "2006-01-02T15:04:05"
	case "hour":
		return "strftime('%Y-%m-%dT%H:00:00', timestamp)", "2006-01-02T15:04:05"
	case "day":
		return "date(timestamp)", "2006-01-02"
	default: // week
		return "strftime('%Y-W%W', timestamp)", ""
	}
}

// parseWeekBucket parses a "YYYY-WNN" string into the Monday of that week.
func parseWeekBucket(bucket string) (time.Time, error) {
	var year, week int
	if _, err := fmt.Sscanf(bucket, "%d-W%d", &year, &week); err != nil {
		return time.Time{}, fmt.Errorf("parse weekly bucket %q: %w", bucket, err)
	}
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	offset := int(time.Monday - jan1.Weekday())
	if offset > 0 {
		offset -= 7
	}
	return jan1.AddDate(0, 0, offset+week*7), nil
}

// parseBucket converts a bucket string to a time.Time using the given layout.
func parseBucket(bucket, layout string) (time.Time, error) {
	if layout != "" {
		return time.Parse(layout, bucket)
	}
	return parseWeekBucket(bucket)
}

func scanRecords(rows *sql.Rows) ([]Record, error) {
	var records []Record
	for rows.Next() {
		var rec Record
		var ts string
		var cost sql.NullFloat64
		var streaming int
		if err := rows.Scan(&rec.ID, &ts, &rec.Provider, &rec.Model, &rec.Endpoint, &rec.Source,
			&rec.InputTokens, &rec.OutputTokens, &rec.CacheReadTokens, &rec.CacheWriteTokens,
			&cost, &rec.DurationMs, &streaming, &rec.StatusCode); err != nil {
			return nil, err
		}
		var err error
		if rec.Timestamp, err = time.Parse(time.RFC3339, ts); err != nil {
			return nil, fmt.Errorf("parse timestamp for record %d: %w", rec.ID, err)
		}
		rec.Streaming = streaming == 1
		if cost.Valid {
			rec.TotalCost = &cost.Float64
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// sourceFilter returns a SQL clause for filtering by source.
// "cc:sub" -> exact match; "cc:" -> prefix match (all Claude Code).
func sourceFilter(source string, args *[]any) string {
	if source == "direct" {
		return "source = ''"
	}
	if strings.HasSuffix(source, ":") {
		*args = append(*args, source+"%")
		return "source LIKE ?"
	}
	*args = append(*args, source)
	return "source = ?"
}

// timeFilter appends time-range WHERE clauses when fromStr/toStr are non-empty.
func timeFilter(query string, args []any, fromStr, toStr string) (string, []any) {
	if fromStr != "" {
		query += " AND timestamp >= ?"
		args = append(args, fromStr)
	}
	if toStr != "" {
		query += " AND timestamp <= ?"
		args = append(args, toStr)
	}
	return query, args
}

func compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
