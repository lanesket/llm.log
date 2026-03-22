package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite implements Store using modernc.org/sqlite.
type SQLite struct {
	db *sql.DB
}

// Open creates or opens the SQLite database.
func Open(dataDir string) (*SQLite, error) {
	dbPath := filepath.Join(dataDir, "llm.log.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLite{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS requests (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp     TEXT    NOT NULL,
			provider      TEXT    NOT NULL,
			model         TEXT    NOT NULL,
			endpoint      TEXT    NOT NULL,
			source        TEXT    NOT NULL DEFAULT '',
			input_tokens  INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens  INTEGER NOT NULL DEFAULT 0,
			cache_write_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost    REAL,
			duration_ms   INTEGER,
			streaming     INTEGER NOT NULL DEFAULT 0,
			status_code   INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS bodies (
			request_id    INTEGER PRIMARY KEY REFERENCES requests(id) ON DELETE CASCADE,
			request_body  BLOB,
			response_body BLOB
		);
		CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
		CREATE INDEX IF NOT EXISTS idx_requests_provider  ON requests(provider);
		CREATE INDEX IF NOT EXISTS idx_requests_model     ON requests(model);
		CREATE INDEX IF NOT EXISTS idx_requests_source      ON requests(source);
		CREATE INDEX IF NOT EXISTS idx_requests_status_code ON requests(status_code);
		CREATE INDEX IF NOT EXISTS idx_requests_cost        ON requests(total_cost);
	`)
	return err
}

func (s *SQLite) Save(rec *Record) error {
	return s.SaveBatch([]*Record{rec})
}

// SaveBatch writes multiple records in a single transaction.
func (s *SQLite) SaveBatch(recs []*Record) error {
	if len(recs) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertReq, err := tx.Prepare(`
		INSERT INTO requests (timestamp, provider, model, endpoint, source, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, total_cost, duration_ms, streaming, status_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare requests insert: %w", err)
	}
	defer insertReq.Close()

	insertBody, err := tx.Prepare(`INSERT INTO bodies (request_id, request_body, response_body) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare bodies insert: %w", err)
	}
	defer insertBody.Close()

	for _, rec := range recs {
		ts := rec.Timestamp.UTC().Format(time.RFC3339)
		streaming := 0
		if rec.Streaming {
			streaming = 1
		}

		res, err := insertReq.Exec(
			ts, rec.Provider, rec.Model, rec.Endpoint, rec.Source,
			rec.InputTokens, rec.OutputTokens, rec.CacheReadTokens, rec.CacheWriteTokens,
			rec.TotalCost, rec.DurationMs, streaming, rec.StatusCode,
		)
		if err != nil {
			return err
		}

		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("last insert id: %w", err)
		}
		rec.ID = id

		reqBody, err := compress(rec.RequestBody)
		if err != nil {
			return fmt.Errorf("compress request: %w", err)
		}
		respBody, err := compress(rec.ResponseBody)
		if err != nil {
			return fmt.Errorf("compress response: %w", err)
		}

		if _, err = insertBody.Exec(id, reqBody, respBody); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLite) Stats(f StatsFilter) ([]StatRow, error) {
	groupCol := "provider"
	switch f.GroupBy {
	case "model":
		groupCol = "model"
	case "day":
		groupCol = "date(timestamp)"
	}

	query := fmt.Sprintf(`
		SELECT %s, MIN(provider), COUNT(*), SUM(input_tokens), SUM(output_tokens), SUM(cache_read_tokens), SUM(cache_write_tokens), COALESCE(SUM(total_cost), 0),
			COALESCE(AVG(duration_ms), 0), SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END)
		FROM requests
		WHERE 1=1`, groupCol)

	var args []any

	if !f.From.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, f.From.UTC().Format(time.RFC3339))
	}
	if !f.To.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, f.To.UTC().Format(time.RFC3339))
	}
	if f.Provider != "" {
		query += " AND provider = ?"
		args = append(args, f.Provider)
	}
	if f.Model != "" {
		query += " AND model = ?"
		args = append(args, f.Model)
	}
	if f.Source != "" {
		query += " AND " + sourceFilter(f.Source, &args)
	}

	query += fmt.Sprintf(" GROUP BY %s ORDER BY COALESCE(SUM(total_cost), 0) DESC", groupCol)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []StatRow
	for rows.Next() {
		var row StatRow
		var avgDur float64
		if err := rows.Scan(&row.Key, &row.Provider, &row.Requests, &row.InputTokens, &row.OutputTokens, &row.CacheReadTokens, &row.CacheWriteTokens, &row.TotalCost, &avgDur, &row.Errors); err != nil {
			return nil, err
		}
		row.AvgDurationMs = int(avgDur)
		stats = append(stats, row)
	}
	return stats, rows.Err()
}

func (s *SQLite) Recent(n int, from, to time.Time, provider, source string) ([]Record, error) {
	query := `SELECT id, timestamp, provider, model, endpoint, source, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, total_cost, duration_ms, streaming, status_code
		FROM requests WHERE 1=1`
	var args []any
	if !from.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, to.UTC().Format(time.RFC3339))
	}
	if provider != "" {
		query += " AND provider = ?"
		args = append(args, provider)
	}
	if source != "" {
		query += " AND " + sourceFilter(source, &args)
	}
	query += " ORDER BY id DESC"
	if n > 0 {
		query += " LIMIT ?"
		args = append(args, n)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (s *SQLite) Get(id int64) (*Record, error) {
	row := s.db.QueryRow(`
		SELECT r.id, r.timestamp, r.provider, r.model, r.endpoint, r.source, r.input_tokens, r.output_tokens, r.cache_read_tokens, r.cache_write_tokens, r.total_cost, r.duration_ms, r.streaming, r.status_code, b.request_body, b.response_body
		FROM requests r LEFT JOIN bodies b ON r.id = b.request_id
		WHERE r.id = ?`, id)

	var rec Record
	var ts string
	var cost sql.NullFloat64
	var streaming int
	var reqBody, respBody []byte

	err := row.Scan(&rec.ID, &ts, &rec.Provider, &rec.Model, &rec.Endpoint, &rec.Source,
		&rec.InputTokens, &rec.OutputTokens, &rec.CacheReadTokens, &rec.CacheWriteTokens,
		&cost, &rec.DurationMs, &streaming, &rec.StatusCode,
		&reqBody, &respBody)
	if err != nil {
		return nil, err
	}

	rec.Timestamp, err = time.Parse(time.RFC3339, ts)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp for record %d: %w", rec.ID, err)
	}
	rec.Streaming = streaming == 1
	if cost.Valid {
		rec.TotalCost = &cost.Float64
	}
	if rec.RequestBody, err = decompress(reqBody); err != nil {
		return nil, fmt.Errorf("decompress request body for record %d: %w", rec.ID, err)
	}
	if rec.ResponseBody, err = decompress(respBody); err != nil {
		return nil, fmt.Errorf("decompress response body for record %d: %w", rec.ID, err)
	}

	return &rec, nil
}

func (s *SQLite) Sources(from, to time.Time) ([]string, error) {
	query := `SELECT DISTINCT source FROM requests WHERE 1=1`
	var args []any
	if !from.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, to.UTC().Format(time.RFC3339))
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (s *SQLite) PrunePreview(before time.Time) (PruneStats, error) {
	var ps PruneStats
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(COALESCE(LENGTH(b.request_body), 0) + COALESCE(LENGTH(b.response_body), 0)), 0)
		FROM bodies b JOIN requests r ON b.request_id = r.id
		WHERE r.timestamp < ?`, before.UTC().Format(time.RFC3339)).Scan(&ps.Count, &ps.Bytes)
	return ps, err
}

func (s *SQLite) PruneBodies(before time.Time) (int64, error) {
	res, err := s.db.Exec(`
		DELETE FROM bodies WHERE request_id IN (
			SELECT id FROM requests WHERE timestamp < ?
		)`, before.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLite) MaxID() (int64, error) {
	var id int64
	err := s.db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM requests").Scan(&id)
	return id, err
}

func (s *SQLite) Vacuum() error {
	_, err := s.db.Exec("VACUUM")
	return err
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// PeriodToTimeRange converts a period string to a time range.
// "today" uses local time for day boundaries so it matches the user's day.
func PeriodToTimeRange(period string) (from, to time.Time) {
	now := time.Now()
	to = now.UTC()
	switch strings.ToLower(period) {
	case "today":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	case "week":
		from = now.AddDate(0, 0, -7).UTC()
	case "month":
		from = now.AddDate(0, -1, 0).UTC()
	default: // "all"
		from = time.Time{}
		to = time.Time{}
	}
	return
}
