package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lanesket/llm.log/internal/storage"
)

func ptr(f float64) *float64 { return &f }

func testRecords() []storage.Record {
	return []storage.Record{
		{
			ID: 1, Timestamp: time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC),
			Provider: "anthropic", Model: "claude-sonnet-4-20250514", Endpoint: "/v1/messages",
			Source: "cc:sub", InputTokens: 1500, OutputTokens: 800,
			CacheReadTokens: 500, CacheWriteTokens: 0,
			TotalCost: ptr(0.0234), DurationMs: 2100, Streaming: true, StatusCode: 200,
		},
		{
			ID: 2, Timestamp: time.Date(2026, 3, 19, 11, 0, 0, 0, time.UTC),
			Provider: "openai", Model: "gpt-4o", Endpoint: "/v1/chat/completions",
			Source: "", InputTokens: 1000, OutputTokens: 500,
			CacheReadTokens: 0, CacheWriteTokens: 0,
			TotalCost: nil, DurationMs: 1500, Streaming: false, StatusCode: 200,
		},
	}
}

func mockBodyFetcher(id int64) (*storage.Record, error) {
	return &storage.Record{
		RequestBody:  []byte(`{"prompt":"hello"}`),
		ResponseBody: []byte(`{"text":"world"}`),
	}, nil
}

func TestCSV(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, testRecords(), Options{Format: CSV}, nil)
	if err != nil {
		t.Fatal(err)
	}

	r := csv.NewReader(&buf)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 3 { // header + 2 records
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if len(rows[0]) != 14 {
		t.Fatalf("expected 14 columns, got %d", len(rows[0]))
	}
	if rows[0][0] != "id" {
		t.Fatalf("expected first header 'id', got %q", rows[0][0])
	}
	// Check values of first record
	if rows[1][0] != "1" {
		t.Errorf("expected id '1', got %q", rows[1][0])
	}
	if rows[1][9] != "0.0234" {
		t.Errorf("expected cost '0.0234', got %q", rows[1][9])
	}
	// Empty cost for nil
	if rows[2][9] != "" {
		t.Errorf("expected empty cost, got %q", rows[2][9])
	}
}

func TestCSVEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, nil, Options{Format: CSV}, nil)
	if err != nil {
		t.Fatal(err)
	}

	r := csv.NewReader(&buf)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 { // header only
		t.Fatalf("expected 1 row (header), got %d", len(rows))
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, testRecords(), Options{Format: JSON}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var records []jsonRecord
	if err := json.Unmarshal(buf.Bytes(), &records); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", records[0].Provider)
	}
	if records[0].TotalCost == nil {
		t.Error("expected non-nil cost for record 0")
	}
	if records[1].TotalCost != nil {
		t.Errorf("expected null cost for record 1, got %v", *records[1].TotalCost)
	}
}

func TestJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, nil, Options{Format: JSON}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("expected '[]', got %q", buf.String())
	}
}

func TestJSONL(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, testRecords(), Options{Format: JSONL}, nil)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		var jr jsonRecord
		if err := json.Unmarshal([]byte(line), &jr); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestJSONLEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, nil, Options{Format: JSONL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buf.String())
	}
}

func TestWithBodies(t *testing.T) {
	records := testRecords()[:1]

	// CSV with bodies
	var csvBuf bytes.Buffer
	err := Write(&csvBuf, records, Options{Format: CSV, WithBodies: true}, mockBodyFetcher)
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(&csvBuf)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows[0]) != 16 { // 14 + 2 body columns
		t.Fatalf("expected 16 columns with bodies, got %d", len(rows[0]))
	}
	if rows[1][14] != `{"prompt":"hello"}` {
		t.Errorf("unexpected request body: %q", rows[1][14])
	}

	// JSON with bodies
	var jsonBuf bytes.Buffer
	err = Write(&jsonBuf, records, Options{Format: JSON, WithBodies: true}, mockBodyFetcher)
	if err != nil {
		t.Fatal(err)
	}
	var jrs []jsonRecord
	if err := json.Unmarshal(jsonBuf.Bytes(), &jrs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if jrs[0].RequestBody == nil {
		t.Error("expected request_body to be present")
	}
	if jrs[0].ResponseBody == nil {
		t.Error("expected response_body to be present")
	}
}
