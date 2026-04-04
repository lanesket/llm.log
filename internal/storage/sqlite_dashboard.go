package storage

import (
	"fmt"
	"time"
)

// DashboardStats returns aggregated dashboard data for the given time range.
func (s *SQLite) DashboardStats(from, to time.Time) (*DashboardData, error) {
	data := &DashboardData{}

	var fromStr, toStr string
	if !from.IsZero() {
		fromStr = from.UTC().Format(time.RFC3339)
	}
	if !to.IsZero() {
		toStr = to.UTC().Format(time.RFC3339)
	}

	// 1. Totals for current period.
	totals, err := s.queryTotals(fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("totals: %w", err)
	}
	data.Totals = totals

	// 2. PrevTotals: previous period of same duration.
	// Only meaningful when both from and to are specified.
	if !from.IsZero() && !to.IsZero() {
		duration := to.Sub(from)
		prevFrom := from.Add(-duration).UTC().Format(time.RFC3339)
		prevTo := fromStr
		prev, err := s.queryTotals(prevFrom, prevTo)
		if err != nil {
			return nil, fmt.Errorf("prev totals: %w", err)
		}
		if prev.Requests > 0 {
			data.PrevTotals = &prev
		}
	}

	// 3. Breakdowns.
	data.ByProvider, err = s.queryBreakdown("provider", "", fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("by_provider: %w", err)
	}
	data.ByModel, err = s.queryBreakdown("model", "MIN(provider)", fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("by_model: %w", err)
	}
	data.BySource, err = s.queryBreakdown("source", "", fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("by_source: %w", err)
	}

	// 4. Chart with auto-granularity.
	data.Chart, err = s.queryChart(from, to, fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("chart: %w", err)
	}

	// 5. Daily activity for contribution heatmap (always all-time).
	data.Activity, err = s.queryDailyActivity("", "")
	if err != nil {
		return nil, fmt.Errorf("activity: %w", err)
	}

	return data, nil
}

func (s *SQLite) queryTotals(fromStr, toStr string) (Totals, error) {
	var t Totals
	var avgDur float64

	query := `SELECT
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(cache_write_tokens), 0),
			COALESCE(SUM(total_cost), 0),
			COALESCE(SUM(CASE WHEN total_cost IS NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(duration_ms), 0)
		FROM requests WHERE 1=1`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)

	err := s.db.QueryRow(query, args...).Scan(
		&t.Requests, &t.InputTokens, &t.OutputTokens,
		&t.CacheReadTokens, &t.CacheWriteTokens,
		&t.TotalCost, &t.UnknownCostRequests, &t.Errors, &avgDur,
	)
	t.AvgDurationMs = int(avgDur)
	return t, err
}

func (s *SQLite) queryBreakdown(groupCol, providerExpr, fromStr, toStr string) ([]BreakdownRow, error) {
	providerSelect := "''"
	if providerExpr != "" {
		providerSelect = providerExpr
	}

	query := fmt.Sprintf(`
		SELECT %s, %s, COUNT(*), COALESCE(SUM(total_cost), 0), COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM requests
		WHERE 1=1`,
		groupCol, providerSelect)
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += fmt.Sprintf(`
		GROUP BY %s
		ORDER BY COALESCE(SUM(total_cost), 0) DESC`, groupCol)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]BreakdownRow, 0)
	for rows.Next() {
		var row BreakdownRow
		if err := rows.Scan(&row.Name, &row.Provider, &row.Requests, &row.TotalCost, &row.Tokens); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLite) queryChart(from, to time.Time, fromStr, toStr string) ([]ChartPoint, error) {
	bucketExpr, parseLayout := chartBucket(from, to, "")

	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*), COALESCE(SUM(total_cost), 0), COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM requests
		WHERE 1=1`,
		bucketExpr)
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += `
		GROUP BY bucket
		ORDER BY bucket ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ChartPoint, 0)
	for rows.Next() {
		var bucket string
		var cp ChartPoint
		if err := rows.Scan(&bucket, &cp.Requests, &cp.Cost, &cp.Tokens); err != nil {
			return nil, err
		}
		cp.Timestamp, err = parseBucket(bucket, parseLayout)
		if err != nil {
			return nil, fmt.Errorf("parse chart bucket %q: %w", bucket, err)
		}
		result = append(result, cp)
	}
	return result, rows.Err()
}

func (s *SQLite) queryDailyActivity(fromStr, toStr string) ([]DailyActivity, error) {
	// Aggregate totals per day.
	query := `
		SELECT DATE(timestamp, 'localtime') AS day,
			COUNT(*),
			COALESCE(SUM(total_cost), 0)
		FROM requests
		WHERE 1=1`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += `
		GROUP BY day
		ORDER BY day ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("daily activity: %w", err)
	}
	defer rows.Close()

	result := make([]DailyActivity, 0)
	for rows.Next() {
		var d DailyActivity
		if err := rows.Scan(&d.Date, &d.Requests, &d.Cost); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Top models per day (up to 5 per day, ordered by cost).
	modelQuery := `
		SELECT DATE(timestamp, 'localtime') AS day,
			model, MIN(provider),
			COUNT(*),
			COALESCE(SUM(total_cost), 0)
		FROM requests
		WHERE 1=1`
	var modelArgs []any
	modelQuery, modelArgs = timeFilter(modelQuery, modelArgs, fromStr, toStr)
	modelQuery += `
		GROUP BY day, model
		ORDER BY day ASC, COALESCE(SUM(total_cost), 0) DESC`

	modelRows, err := s.db.Query(modelQuery, modelArgs...)
	if err != nil {
		return result, nil // non-fatal: return totals without model breakdown
	}
	defer modelRows.Close()

	dayIndex := make(map[string]int, len(result))
	for i, d := range result {
		dayIndex[d.Date] = i
	}
	dayCounts := make(map[string]int, len(result))

	for modelRows.Next() {
		var day, model, provider string
		var reqs int
		var cost float64
		if err := modelRows.Scan(&day, &model, &provider, &reqs, &cost); err != nil {
			break
		}
		idx, ok := dayIndex[day]
		if !ok {
			continue
		}
		result[idx].ModelCount++
		if dayCounts[day] >= 5 {
			continue
		}
		dayCounts[day]++
		result[idx].Models = append(result[idx].Models, DailyModelStat{
			Model: model, Provider: provider, Requests: reqs, Cost: cost,
		})
	}

	return result, nil
}
