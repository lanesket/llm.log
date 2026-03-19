package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/lanesket/llm.log/internal/storage"
)

// Format specifies the output format.
type Format string

const (
	CSV   Format = "csv"
	JSON  Format = "json"
	JSONL Format = "jsonl"
)

// Options controls what and how to export.
type Options struct {
	Format     Format
	WithBodies bool
}

// Write exports records to w in the specified format.
// bodyFetcher is called per-record when WithBodies is true to retrieve full bodies.
func Write(w io.Writer, records []storage.Record, opts Options, bodyFetcher func(int64) (*storage.Record, error)) error {
	switch opts.Format {
	case CSV:
		return writeCSV(w, records, opts, bodyFetcher)
	case JSON:
		return writeJSON(w, records, opts, bodyFetcher)
	case JSONL:
		return writeJSONL(w, records, opts, bodyFetcher)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}
}

func writeCSV(w io.Writer, records []storage.Record, opts Options, bodyFetcher func(int64) (*storage.Record, error)) error {
	cw := csv.NewWriter(w)

	header := []string{
		"id", "timestamp", "provider", "model", "source",
		"input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens",
		"total_cost", "duration_ms", "streaming", "status_code", "endpoint",
	}
	if opts.WithBodies {
		header = append(header, "request_body", "response_body")
	}
	if err := cw.Write(header); err != nil {
		return err
	}

	for _, r := range records {
		costStr := ""
		if r.TotalCost != nil {
			costStr = strconv.FormatFloat(*r.TotalCost, 'f', -1, 64)
		}
		row := []string{
			strconv.FormatInt(r.ID, 10),
			r.Timestamp.UTC().Format(time.RFC3339),
			r.Provider,
			r.Model,
			r.Source,
			strconv.Itoa(r.InputTokens),
			strconv.Itoa(r.OutputTokens),
			strconv.Itoa(r.CacheReadTokens),
			strconv.Itoa(r.CacheWriteTokens),
			costStr,
			strconv.Itoa(r.DurationMs),
			strconv.FormatBool(r.Streaming),
			strconv.Itoa(r.StatusCode),
			r.Endpoint,
		}
		if opts.WithBodies {
			reqBody, respBody, err := fetchBodies(r.ID, bodyFetcher)
			if err != nil {
				return err
			}
			row = append(row, string(reqBody), string(respBody))
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

type jsonRecord struct {
	ID               int64           `json:"id"`
	Timestamp        string          `json:"timestamp"`
	Provider         string          `json:"provider"`
	Model            string          `json:"model"`
	Source           string          `json:"source"`
	InputTokens      int             `json:"input_tokens"`
	OutputTokens     int             `json:"output_tokens"`
	CacheReadTokens  int             `json:"cache_read_tokens"`
	CacheWriteTokens int             `json:"cache_write_tokens"`
	TotalCost        *float64        `json:"total_cost"`
	DurationMs       int             `json:"duration_ms"`
	Streaming        bool            `json:"streaming"`
	StatusCode       int             `json:"status_code"`
	Endpoint         string          `json:"endpoint"`
	RequestBody      json.RawMessage `json:"request_body,omitempty"`
	ResponseBody     json.RawMessage `json:"response_body,omitempty"`
}

func toJSONRecord(r storage.Record, opts Options, bodyFetcher func(int64) (*storage.Record, error)) (jsonRecord, error) {
	jr := jsonRecord{
		ID:               r.ID,
		Timestamp:        r.Timestamp.UTC().Format(time.RFC3339),
		Provider:         r.Provider,
		Model:            r.Model,
		Source:           r.Source,
		InputTokens:      r.InputTokens,
		OutputTokens:     r.OutputTokens,
		CacheReadTokens:  r.CacheReadTokens,
		CacheWriteTokens: r.CacheWriteTokens,
		TotalCost:        r.TotalCost,
		DurationMs:       r.DurationMs,
		Streaming:        r.Streaming,
		StatusCode:       r.StatusCode,
		Endpoint:         r.Endpoint,
	}
	if opts.WithBodies {
		reqBody, respBody, err := fetchBodies(r.ID, bodyFetcher)
		if err != nil {
			return jr, err
		}
		if len(reqBody) > 0 {
			jr.RequestBody = json.RawMessage(reqBody)
		}
		if len(respBody) > 0 {
			jr.ResponseBody = json.RawMessage(respBody)
		}
	}
	return jr, nil
}

func writeJSON(w io.Writer, records []storage.Record, opts Options, bodyFetcher func(int64) (*storage.Record, error)) error {
	jrs := make([]jsonRecord, 0, len(records))
	for _, r := range records {
		jr, err := toJSONRecord(r, opts, bodyFetcher)
		if err != nil {
			return err
		}
		jrs = append(jrs, jr)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jrs)
}

func writeJSONL(w io.Writer, records []storage.Record, opts Options, bodyFetcher func(int64) (*storage.Record, error)) error {
	enc := json.NewEncoder(w)
	for _, r := range records {
		jr, err := toJSONRecord(r, opts, bodyFetcher)
		if err != nil {
			return err
		}
		if err := enc.Encode(jr); err != nil {
			return err
		}
	}
	return nil
}

func fetchBodies(id int64, bodyFetcher func(int64) (*storage.Record, error)) (reqBody, respBody []byte, err error) {
	if bodyFetcher == nil {
		return nil, nil, nil
	}
	full, err := bodyFetcher(id)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch bodies for record %d: %w", id, err)
	}
	return full.RequestBody, full.ResponseBody, nil
}
