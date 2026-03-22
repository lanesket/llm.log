package ui

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/lanesket/llm.log/internal/storage"
)

// requestItem is the JSON representation of a request in list responses.
type requestItem struct {
	ID               int64    `json:"id"`
	Timestamp        string   `json:"timestamp"`
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	Endpoint         string   `json:"endpoint"`
	Source           string   `json:"source"`
	InputTokens      int      `json:"input_tokens"`
	OutputTokens     int      `json:"output_tokens"`
	CacheReadTokens  int      `json:"cache_read_tokens"`
	CacheWriteTokens int      `json:"cache_write_tokens"`
	TotalCost        *float64 `json:"total_cost"`
	DurationMs       int      `json:"duration_ms"`
	Streaming        bool     `json:"streaming"`
	StatusCode       int      `json:"status_code"`
}

type listResponse struct {
	Items      []requestItem `json:"items"`
	Total      *int          `json:"total"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

// requestDetail is the JSON representation for a single request with bodies.
type requestDetail struct {
	requestItem
	RequestBody  *string `json:"request_body"`
	ResponseBody *string `json:"response_body"`
}

func recordToItem(rec storage.Record) requestItem {
	return requestItem{
		ID:               rec.ID,
		Timestamp:        rec.Timestamp.Format(time.RFC3339),
		Provider:         rec.Provider,
		Model:            rec.Model,
		Endpoint:         rec.Endpoint,
		Source:           rec.Source,
		InputTokens:      rec.InputTokens,
		OutputTokens:     rec.OutputTokens,
		CacheReadTokens:  rec.CacheReadTokens,
		CacheWriteTokens: rec.CacheWriteTokens,
		TotalCost:        rec.TotalCost,
		DurationMs:       rec.DurationMs,
		Streaming:        rec.Streaming,
		StatusCode:       rec.StatusCode,
	}
}

func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	from, to, err := parseTimeRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	filter := storage.ListFilter{
		From:     from,
		To:       to,
		Cursor:   q.Get("cursor"),
		SortBy:   q.Get("sort"),
		SortDir:  q.Get("dir"),
		Provider: q.Get("provider"),
		Model:    q.Get("model"),
		Source:   q.Get("source"),
	}

	if v := q.Get("limit"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			filter.Limit = limit
		}
	}

	if v := q.Get("status_code"); v != "" {
		if sc, err := strconv.Atoi(v); err == nil {
			filter.StatusCode = sc
		}
	}

	if v := q.Get("streaming"); v != "" {
		b := v == "true" || v == "1"
		filter.Streaming = &b
	}

	if v := q.Get("min_cost"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			filter.MinCost = &f
		}
	}
	if v := q.Get("max_cost"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			filter.MaxCost = &f
		}
	}
	if v := q.Get("min_tokens"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinTokens = &n
		}
	}
	if v := q.Get("max_tokens"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MaxTokens = &n
		}
	}

	// Body search: get matching IDs first, then pass as filter
	if search := q.Get("search"); search != "" {
		ids, err := s.store.SearchByBody(search, from, to, 5000, 200)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		filter.SearchIDs = ids
	}

	result, err := s.store.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	items := make([]requestItem, len(result.Items))
	for i, rec := range result.Items {
		items[i] = recordToItem(rec)
	}

	writeJSON(w, http.StatusOK, listResponse{
		Items:      items,
		Total:      result.Total,
		NextCursor: result.NextCursor,
	})
}

func (s *Server) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	rec, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "request not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get request")
		return
	}

	detail := requestDetail{
		requestItem: recordToItem(*rec),
	}
	if rec.RequestBody != nil {
		body := string(rec.RequestBody)
		detail.RequestBody = &body
	}
	if rec.ResponseBody != nil {
		body := string(rec.ResponseBody)
		detail.ResponseBody = &body
	}

	writeJSON(w, http.StatusOK, detail)
}
