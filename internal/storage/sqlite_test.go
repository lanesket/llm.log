package storage

import (
	"os"
	"testing"
	"time"
)

func testDB(t *testing.T) *SQLite {
	t.Helper()
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpen_CreatesDB(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()

	if _, err := os.Stat(dir + "/llm.log.db"); err != nil {
		t.Fatal("database file not created")
	}
}

func TestSave_And_Get(t *testing.T) {
	store := testDB(t)

	cost := 0.05
	rec := &Record{
		Timestamp:    time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
		Provider:     "openai",
		Model:        "gpt-4",
		Endpoint:     "/v1/chat/completions",
		InputTokens:  100,
		OutputTokens: 50,
		CacheReadTokens: 20,
		TotalCost:    &cost,
		DurationMs:   150,
		Streaming:    true,
		StatusCode:   200,
		RequestBody:  []byte(`{"prompt":"hello"}`),
		ResponseBody: []byte(`{"content":"world"}`),
	}

	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	if rec.ID == 0 {
		t.Error("ID not set after save")
	}

	got, err := store.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Provider != "openai" {
		t.Errorf("provider = %q", got.Provider)
	}
	if got.Model != "gpt-4" {
		t.Errorf("model = %q", got.Model)
	}
	if got.InputTokens != 100 {
		t.Errorf("input = %d", got.InputTokens)
	}
	if got.OutputTokens != 50 {
		t.Errorf("output = %d", got.OutputTokens)
	}
	if got.CacheReadTokens != 20 {
		t.Errorf("cached = %d", got.CacheReadTokens)
	}
	if got.TotalCost == nil || *got.TotalCost != 0.05 {
		t.Errorf("cost = %v", got.TotalCost)
	}
	if !got.Streaming {
		t.Error("streaming should be true")
	}
	if got.StatusCode != 200 {
		t.Errorf("status = %d", got.StatusCode)
	}
	if string(got.RequestBody) != `{"prompt":"hello"}` {
		t.Errorf("request body = %q", got.RequestBody)
	}
	if string(got.ResponseBody) != `{"content":"world"}` {
		t.Errorf("response body = %q", got.ResponseBody)
	}
}

func TestSave_NilCost(t *testing.T) {
	store := testDB(t)

	rec := &Record{
		Timestamp:  time.Now(),
		Provider:   "openai",
		Model:      "gpt-4",
		Endpoint:   "/v1/chat/completions",
		StatusCode: 200,
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCost != nil {
		t.Errorf("expected nil cost, got %v", *got.TotalCost)
	}
}

func TestRecent(t *testing.T) {
	store := testDB(t)

	for i := 0; i < 5; i++ {
		store.Save(&Record{
			Timestamp:  time.Now(),
			Provider:   "openai",
			Model:      "gpt-4",
			Endpoint:   "/v1/chat/completions",
			StatusCode: 200,
		})
	}

	records, err := store.Recent(3, time.Time{}, time.Time{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	// Should be ordered newest first (highest ID first)
	if records[0].ID < records[1].ID || records[1].ID < records[2].ID {
		t.Errorf("not in DESC order: IDs %d, %d, %d", records[0].ID, records[1].ID, records[2].ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	store := testDB(t)

	_, err := store.Get(999)
	if err == nil {
		t.Error("expected error for missing record")
	}
}

func TestStats_ByProvider(t *testing.T) {
	store := testDB(t)
	now := time.Now()

	cost1 := 0.10
	cost2 := 0.05
	store.Save(&Record{
		Timestamp: now, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
		TotalCost: &cost1, StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: now, Provider: "anthropic", Model: "claude-sonnet-4-6",
		Endpoint: "/v1/messages", InputTokens: 200, OutputTokens: 80,
		TotalCost: &cost2, StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: now, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 50, OutputTokens: 30,
		TotalCost: &cost1, StatusCode: 200,
	})

	stats, err := store.Stats(StatsFilter{GroupBy: "provider"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d rows, want 2", len(stats))
	}
	// Sorted by cost DESC — openai ($0.20) first
	if stats[0].Key != "openai" {
		t.Errorf("first row = %q, want openai", stats[0].Key)
	}
	if stats[0].Requests != 2 {
		t.Errorf("openai requests = %d, want 2", stats[0].Requests)
	}
	if stats[0].InputTokens != 150 {
		t.Errorf("openai input = %d, want 150", stats[0].InputTokens)
	}
	if stats[1].Key != "anthropic" {
		t.Errorf("second row = %q, want anthropic", stats[1].Key)
	}
}

func TestStats_ByModel(t *testing.T) {
	store := testDB(t)
	now := time.Now()

	store.Save(&Record{
		Timestamp: now, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
		StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: now, Provider: "openai", Model: "gpt-4.1-nano",
		Endpoint: "/v1/chat/completions", InputTokens: 200, OutputTokens: 80,
		StatusCode: 200,
	})

	stats, err := store.Stats(StatsFilter{GroupBy: "model"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d rows, want 2", len(stats))
	}
}

func TestStats_TimeFilter(t *testing.T) {
	store := testDB(t)

	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	store.Save(&Record{
		Timestamp: old, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: recent, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", StatusCode: 200,
	})

	stats, err := store.Stats(StatsFilter{
		GroupBy: "provider",
		From:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("got %d rows, want 1", len(stats))
	}
	if stats[0].Requests != 1 {
		t.Errorf("requests = %d, want 1 (only recent)", stats[0].Requests)
	}
}

func TestCompressDecompress(t *testing.T) {
	original := []byte(`{"large":"json body with content that should compress well"}`)
	compressed, err := compress(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) == 0 {
		t.Fatal("compressed is empty")
	}

	decompressed, err := decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if string(decompressed) != string(original) {
		t.Errorf("roundtrip failed: got %q", decompressed)
	}
}

func TestCompressDecompress_Empty(t *testing.T) {
	compressed, err := compress(nil)
	if err != nil {
		t.Fatal(err)
	}
	if compressed != nil {
		t.Error("expected nil for nil input")
	}

	decompressed, err := decompress(nil)
	if err != nil {
		t.Fatal(err)
	}
	if decompressed != nil {
		t.Error("expected nil for nil input")
	}
}
