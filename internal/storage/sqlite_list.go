package storage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// listCursor is the internal representation of an opaque pagination cursor.
type listCursor struct {
	V  json.RawMessage `json:"v"`
	ID int64           `json:"id"`
}

// sortColumn maps a SortBy value to the DB column used for ordering.
func sortColumn(sortBy string) string {
	switch sortBy {
	case "cost":
		return "total_cost"
	case "input_tokens":
		return "input_tokens"
	case "output_tokens":
		return "output_tokens"
	case "duration_ms":
		return "duration_ms"
	default: // "timestamp" or empty
		return "id"
	}
}

func (s *SQLite) List(f ListFilter) (*ListResult, error) {
	// Apply defaults.
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if f.SortBy == "" {
		f.SortBy = "timestamp"
	}
	if f.SortDir == "" {
		f.SortDir = "desc"
	}

	col := sortColumn(f.SortBy)
	desc := strings.ToLower(f.SortDir) == "desc"

	// Build WHERE clauses (shared between count and data queries).
	var wheres []string
	var args []any

	if !f.From.IsZero() {
		wheres = append(wheres, "timestamp >= ?")
		args = append(args, f.From.UTC().Format(time.RFC3339))
	}
	if !f.To.IsZero() {
		wheres = append(wheres, "timestamp <= ?")
		args = append(args, f.To.UTC().Format(time.RFC3339))
	}
	if f.Provider != "" {
		wheres = append(wheres, "provider = ?")
		args = append(args, f.Provider)
	}
	if f.Model != "" {
		wheres = append(wheres, "model = ?")
		args = append(args, f.Model)
	}
	if f.Source != "" {
		wheres = append(wheres, sourceFilter(f.Source, &args))
	}
	if f.StatusCode != 0 {
		wheres = append(wheres, "status_code = ?")
		args = append(args, f.StatusCode)
	}
	if f.Streaming != nil {
		v := 0
		if *f.Streaming {
			v = 1
		}
		wheres = append(wheres, "streaming = ?")
		args = append(args, v)
	}
	if f.MinCost != nil {
		wheres = append(wheres, "total_cost >= ?")
		args = append(args, *f.MinCost)
	}
	if f.MaxCost != nil {
		wheres = append(wheres, "total_cost <= ?")
		args = append(args, *f.MaxCost)
	}
	if f.MinTokens != nil {
		wheres = append(wheres, "(input_tokens + output_tokens) >= ?")
		args = append(args, *f.MinTokens)
	}
	if f.MaxTokens != nil {
		wheres = append(wheres, "(input_tokens + output_tokens) <= ?")
		args = append(args, *f.MaxTokens)
	}

	hasSearchIDs := len(f.SearchIDs) > 0
	if hasSearchIDs {
		placeholders := make([]string, len(f.SearchIDs))
		for i, id := range f.SearchIDs {
			placeholders[i] = strconv.FormatInt(id, 10)
		}
		wheres = append(wheres, "id IN ("+strings.Join(placeholders, ",")+")")
	}

	whereClause := "1=1"
	if len(wheres) > 0 {
		whereClause = strings.Join(wheres, " AND ")
	}

	result := &ListResult{}

	// Count query (only when SearchIDs is not set).
	if !hasSearchIDs {
		countQuery := "SELECT COUNT(*) FROM requests WHERE " + whereClause
		var total int
		if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
			return nil, fmt.Errorf("count query: %w", err)
		}
		result.Total = &total
	}

	// Decode cursor if present.
	var cursorArgs []any
	var cursorClause string
	if f.Cursor != "" {
		raw, err := base64.StdEncoding.DecodeString(f.Cursor)
		if err != nil {
			return nil, fmt.Errorf("decode cursor: %w", err)
		}
		var cur listCursor
		if err := json.Unmarshal(raw, &cur); err != nil {
			return nil, fmt.Errorf("unmarshal cursor: %w", err)
		}

		cmp := "<"
		if !desc {
			cmp = ">"
		}

		if col == "id" {
			cursorClause = fmt.Sprintf("id %s ?", cmp)
			cursorArgs = append(cursorArgs, cur.ID)
		} else {
			// Keyset: (col < val OR (col = val AND id < id))
			// Use COALESCE for nullable columns (e.g. total_cost) so NULLs
			// sort consistently as -1 (at the end for DESC).
			coalesced := fmt.Sprintf("COALESCE(%s, -1)", col)
			cursorClause = fmt.Sprintf("(%s %s ? OR (%s = ? AND id %s ?))", coalesced, cmp, coalesced, cmp)
			// Parse the sort value from the cursor.
			var sortVal any
			if err := json.Unmarshal(cur.V, &sortVal); err != nil {
				return nil, fmt.Errorf("unmarshal cursor sort value: %w", err)
			}
			// If cursor value is null, use the coalesced sentinel.
			if sortVal == nil {
				sortVal = float64(-1)
			}
			cursorArgs = append(cursorArgs, sortVal, sortVal, cur.ID)
		}
	}

	// Build data query.
	dataWhere := whereClause
	dataArgs := append([]any{}, args...)
	if cursorClause != "" {
		dataWhere += " AND " + cursorClause
		dataArgs = append(dataArgs, cursorArgs...)
	}

	orderDir := "DESC"
	if !desc {
		orderDir = "ASC"
	}

	var orderClause string
	if col == "id" {
		orderClause = fmt.Sprintf("id %s", orderDir)
	} else {
		// Use COALESCE for nullable columns so NULLs sort consistently.
		orderClause = fmt.Sprintf("COALESCE(%s, -1) %s, id %s", col, orderDir, orderDir)
	}

	// Fetch limit+1 to determine if there are more results.
	dataQuery := fmt.Sprintf(
		`SELECT id, timestamp, provider, model, endpoint, source, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, total_cost, duration_ms, streaming, status_code
		FROM requests WHERE %s ORDER BY %s LIMIT ?`,
		dataWhere, orderClause)
	dataArgs = append(dataArgs, f.Limit+1)

	rows, err := s.db.Query(dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	items, err := scanRecords(rows)
	if err != nil {
		return nil, err
	}

	// If we got limit+1 items, there are more results.
	if len(items) > f.Limit {
		items = items[:f.Limit]
		last := items[f.Limit-1]

		// Encode cursor from last item.
		var sortVal any
		switch col {
		case "id":
			sortVal = last.ID
		case "total_cost":
			if last.TotalCost != nil {
				sortVal = *last.TotalCost
			} else {
				sortVal = float64(-1) // matches COALESCE(total_cost, -1)
			}
		case "input_tokens":
			sortVal = last.InputTokens
		case "output_tokens":
			sortVal = last.OutputTokens
		case "duration_ms":
			sortVal = last.DurationMs
		}

		vBytes, _ := json.Marshal(sortVal)
		cur := listCursor{V: vBytes, ID: last.ID}
		curBytes, _ := json.Marshal(cur)
		result.NextCursor = base64.StdEncoding.EncodeToString(curBytes)
	}

	result.Items = items
	return result, nil
}
