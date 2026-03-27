package storage

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Analytics returns aggregated analytics data for the given time range.
func (s *SQLite) Analytics(from, to time.Time, granularity string) (*AnalyticsData, error) {
	data := &AnalyticsData{}

	var fromStr, toStr string
	if !from.IsZero() {
		fromStr = from.UTC().Format(time.RFC3339)
	}
	if !to.IsZero() {
		toStr = to.UTC().Format(time.RFC3339)
	}
	bucketExpr, parseLayout := chartBucket(from, to, granularity)

	var err error

	// 1. over_time
	data.OverTime, err = s.analyticsOverTime(fromStr, toStr, bucketExpr, parseLayout)
	if err != nil {
		return nil, err
	}

	// 2. by_provider_over_time
	data.ByProviderOverTime, err = s.analyticsByProvider(fromStr, toStr, bucketExpr, parseLayout)
	if err != nil {
		return nil, err
	}

	// 3. top_models
	data.TopModels, err = s.analyticsTopModels(fromStr, toStr)
	if err != nil {
		return nil, err
	}

	// 4. top_expensive
	data.TopExpensive, err = s.analyticsTopExpensive(fromStr, toStr)
	if err != nil {
		return nil, err
	}

	// 5. cost_distribution
	data.CostDistribution, err = s.analyticsCostDistribution(fromStr, toStr)
	if err != nil {
		return nil, err
	}

	// 6. cumulative_cost (derived from over_time data)
	{
		var cumulative float64
		for _, pt := range data.OverTime {
			cumulative += pt.Cost
			data.CumulativeCost = append(data.CumulativeCost, CumulativePoint{
				Timestamp:  pt.Timestamp,
				Cumulative: cumulative,
			})
		}
	}

	// 7. cache_hit_rate (derived from over_time data)
	{
		for _, pt := range data.OverTime {
			var rate float64
			if pt.InputTokens > 0 {
				rate = float64(pt.CacheReadTokens) / float64(pt.InputTokens)
			}
			data.CacheHitRate = append(data.CacheHitRate, CacheHitPoint{
				Timestamp: pt.Timestamp,
				Rate:      rate,
			})
		}
	}

	// 8. avg_tokens_per_request
	data.AvgTokensPerReq, err = s.analyticsAvgTokens(fromStr, toStr)
	if err != nil {
		return nil, err
	}

	// 9. heatmap
	data.Heatmap, err = s.analyticsHeatmap(fromStr, toStr)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *SQLite) analyticsOverTime(fromStr, toStr, bucketExpr, parseLayout string) ([]AnalyticsPoint, error) {
	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*), COALESCE(SUM(total_cost), 0),
			COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0), COALESCE(SUM(cache_write_tokens), 0)
		FROM requests
		WHERE 1=1`, bucketExpr)
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += " GROUP BY bucket ORDER BY bucket ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("over_time: %w", err)
	}
	defer rows.Close()

	result := make([]AnalyticsPoint, 0)
	for rows.Next() {
		var bucket string
		var pt AnalyticsPoint
		if err := rows.Scan(&bucket, &pt.Requests, &pt.Cost,
			&pt.InputTokens, &pt.OutputTokens,
			&pt.CacheReadTokens, &pt.CacheWriteTokens); err != nil {
			return nil, fmt.Errorf("over_time scan: %w", err)
		}
		pt.Timestamp, err = parseBucket(bucket, parseLayout)
		if err != nil {
			return nil, err
		}
		result = append(result, pt)
	}
	return result, rows.Err()
}

func (s *SQLite) analyticsByProvider(fromStr, toStr, bucketExpr, parseLayout string) ([]ProviderTimePoint, error) {
	query := fmt.Sprintf(`
		SELECT %s AS bucket, provider, COUNT(*), COALESCE(SUM(total_cost), 0)
		FROM requests
		WHERE 1=1`, bucketExpr)
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += " GROUP BY bucket, provider ORDER BY bucket ASC, provider ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("by_provider_over_time: %w", err)
	}
	defer rows.Close()

	result := make([]ProviderTimePoint, 0)
	for rows.Next() {
		var bucket string
		var pt ProviderTimePoint
		if err := rows.Scan(&bucket, &pt.Provider, &pt.Requests, &pt.Cost); err != nil {
			return nil, fmt.Errorf("by_provider scan: %w", err)
		}
		pt.Timestamp, err = parseBucket(bucket, parseLayout)
		if err != nil {
			return nil, err
		}
		result = append(result, pt)
	}
	return result, rows.Err()
}

func (s *SQLite) analyticsTopModels(fromStr, toStr string) ([]ModelStat, error) {
	query := `
		SELECT model, MIN(provider), COUNT(*), COALESCE(SUM(total_cost), 0), COALESCE(AVG(duration_ms), 0)
		FROM requests
		WHERE 1=1`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += " GROUP BY model ORDER BY COALESCE(SUM(total_cost), 0) DESC LIMIT 10"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("top_models: %w", err)
	}
	defer rows.Close()

	result := make([]ModelStat, 0)
	for rows.Next() {
		var m ModelStat
		var avgDur float64
		if err := rows.Scan(&m.Model, &m.Provider, &m.Requests, &m.Cost, &avgDur); err != nil {
			return nil, fmt.Errorf("top_models scan: %w", err)
		}
		m.AvgDurationMs = int(avgDur)
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *SQLite) analyticsTopExpensive(fromStr, toStr string) ([]ExpensiveRequest, error) {
	query := `
		SELECT id, timestamp, model, COALESCE(total_cost, 0), (input_tokens + output_tokens)
		FROM requests
		WHERE 1=1`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += " ORDER BY COALESCE(total_cost, 0) DESC LIMIT 10"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("top_expensive: %w", err)
	}
	defer rows.Close()

	result := make([]ExpensiveRequest, 0)
	for rows.Next() {
		var e ExpensiveRequest
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Model, &e.Cost, &e.TotalTokens); err != nil {
			return nil, fmt.Errorf("top_expensive scan: %w", err)
		}
		var parseErr error
		e.Timestamp, parseErr = time.Parse(time.RFC3339, ts)
		if parseErr != nil {
			return nil, fmt.Errorf("top_expensive parse ts: %w", parseErr)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *SQLite) analyticsCostDistribution(fromStr, toStr string) (CostDistribution, error) {
	query := `
		SELECT total_cost FROM requests
		WHERE total_cost IS NOT NULL`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += " ORDER BY total_cost ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return CostDistribution{}, fmt.Errorf("cost_distribution: %w", err)
	}
	defer rows.Close()

	var costs []float64
	for rows.Next() {
		var c float64
		if err := rows.Scan(&c); err != nil {
			return CostDistribution{}, err
		}
		costs = append(costs, c)
	}
	if err := rows.Err(); err != nil {
		return CostDistribution{}, err
	}

	if len(costs) > 0 {
		return CostDistribution{
			P50: percentile(costs, 0.50),
			P90: percentile(costs, 0.90),
			P95: percentile(costs, 0.95),
			P99: percentile(costs, 0.99),
			Max: costs[len(costs)-1],
		}, nil
	}
	return CostDistribution{}, nil
}

func (s *SQLite) analyticsHeatmap(fromStr, toStr string) ([]HeatmapEntry, error) {
	query := `
		SELECT CAST(strftime('%w', timestamp) AS INTEGER),
			CAST(strftime('%H', timestamp) AS INTEGER),
			COUNT(*), COALESCE(SUM(total_cost), 0)
		FROM requests
		WHERE 1=1`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += `
		GROUP BY strftime('%w', timestamp), strftime('%H', timestamp)
		ORDER BY 1, 2`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("heatmap: %w", err)
	}
	defer rows.Close()

	result := make([]HeatmapEntry, 0)
	for rows.Next() {
		var h HeatmapEntry
		if err := rows.Scan(&h.DayOfWeek, &h.Hour, &h.Requests, &h.Cost); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

func (s *SQLite) analyticsAvgTokens(fromStr, toStr string) ([]AvgTokensRow, error) {
	query := `
		SELECT model, AVG(input_tokens), AVG(output_tokens)
		FROM requests
		WHERE 1=1`
	var args []any
	query, args = timeFilter(query, args, fromStr, toStr)
	query += " GROUP BY model ORDER BY model ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("avg_tokens: %w", err)
	}
	defer rows.Close()

	result := make([]AvgTokensRow, 0)
	for rows.Next() {
		var r AvgTokensRow
		if err := rows.Scan(&r.Model, &r.AvgInput, &r.AvgOutput); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// percentile returns the p-th percentile from a sorted slice of float64.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// SearchByBody scans request/response bodies for a substring match.
func (s *SQLite) SearchByBody(query string, from, to time.Time, maxScan, limit int) ([]int64, error) {
	q := `SELECT r.id, b.request_body, b.response_body
		FROM requests r JOIN bodies b ON r.id = b.request_id
		WHERE 1=1`
	var args []any

	if !from.IsZero() {
		q += " AND r.timestamp >= ?"
		args = append(args, from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		q += " AND r.timestamp <= ?"
		args = append(args, to.UTC().Format(time.RFC3339))
	}
	q += " ORDER BY r.id DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	lowerQuery := strings.ToLower(query)
	var matches []int64
	scanned := 0

	for rows.Next() {
		if maxScan > 0 && scanned >= maxScan {
			break
		}
		if limit > 0 && len(matches) >= limit {
			break
		}

		var id int64
		var reqBody, respBody []byte
		if err := rows.Scan(&id, &reqBody, &respBody); err != nil {
			return nil, err
		}
		scanned++

		reqData, err := decompress(reqBody)
		if err != nil {
			continue // skip corrupt bodies
		}
		respData, err := decompress(respBody)
		if err != nil {
			continue
		}

		if strings.Contains(strings.ToLower(string(reqData)), lowerQuery) ||
			strings.Contains(strings.ToLower(string(respData)), lowerQuery) {
			matches = append(matches, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

// Filters returns distinct providers, models, and sources in the given time range.
func (s *SQLite) Filters(from, to time.Time) (*FilterOptions, error) {
	opts := &FilterOptions{}

	queryDistinct := func(col string) ([]string, error) {
		q := fmt.Sprintf("SELECT DISTINCT %s FROM requests WHERE 1=1", col)
		var args []any
		if !from.IsZero() {
			q += " AND timestamp >= ?"
			args = append(args, from.UTC().Format(time.RFC3339))
		}
		if !to.IsZero() {
			q += " AND timestamp <= ?"
			args = append(args, to.UTC().Format(time.RFC3339))
		}
		q += fmt.Sprintf(" ORDER BY %s ASC", col)

		rows, err := s.db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := make([]string, 0)
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			result = append(result, v)
		}
		return result, rows.Err()
	}

	var err error
	if opts.Providers, err = queryDistinct("provider"); err != nil {
		return nil, fmt.Errorf("providers: %w", err)
	}
	if opts.Models, err = queryDistinct("model"); err != nil {
		return nil, fmt.Errorf("models: %w", err)
	}
	if opts.Sources, err = queryDistinct("source"); err != nil {
		return nil, fmt.Errorf("sources: %w", err)
	}

	return opts, nil
}
