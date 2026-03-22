package storage

import (
	"fmt"
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
		Timestamp:       time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
		Provider:        "openai",
		Model:           "gpt-4",
		Endpoint:        "/v1/chat/completions",
		InputTokens:     100,
		OutputTokens:    50,
		CacheReadTokens: 20,
		TotalCost:       &cost,
		DurationMs:      150,
		Streaming:       true,
		StatusCode:      200,
		RequestBody:     []byte(`{"prompt":"hello"}`),
		ResponseBody:    []byte(`{"content":"world"}`),
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

func TestSaveBatch_Empty(t *testing.T) {
	store := testDB(t)
	if err := store.SaveBatch(nil); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBatch([]*Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestSaveBatch_SetsIDs(t *testing.T) {
	store := testDB(t)

	recs := []*Record{
		{Timestamp: time.Now(), Provider: "openai", Model: "gpt-4", Endpoint: "/v1/chat/completions", StatusCode: 200},
		{Timestamp: time.Now(), Provider: "anthropic", Model: "claude-3", Endpoint: "/v1/messages", StatusCode: 200},
	}
	if err := store.SaveBatch(recs); err != nil {
		t.Fatal(err)
	}
	for i, r := range recs {
		if r.ID == 0 {
			t.Errorf("recs[%d].ID not set after SaveBatch", i)
		}
	}
	if recs[0].ID == recs[1].ID {
		t.Error("records got the same ID")
	}
}

func TestSaveBatch_DataIntegrity(t *testing.T) {
	store := testDB(t)

	cost := 0.42
	rec := &Record{
		Timestamp:    time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC),
		Provider:     "openai",
		Model:        "gpt-4",
		Endpoint:     "/v1/chat/completions",
		InputTokens:  100,
		OutputTokens: 50,
		TotalCost:    &cost,
		DurationMs:   300,
		Streaming:    true,
		StatusCode:   200,
		RequestBody:  []byte(`{"req":true}`),
		ResponseBody: []byte(`{"resp":true}`),
	}

	if err := store.SaveBatch([]*Record{rec}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "openai" {
		t.Errorf("provider = %q", got.Provider)
	}
	if got.InputTokens != 100 || got.OutputTokens != 50 {
		t.Errorf("tokens = %d/%d", got.InputTokens, got.OutputTokens)
	}
	if got.TotalCost == nil || *got.TotalCost != 0.42 {
		t.Errorf("cost = %v", got.TotalCost)
	}
	if !got.Streaming {
		t.Error("streaming should be true")
	}
	if string(got.RequestBody) != `{"req":true}` {
		t.Errorf("request body = %q", got.RequestBody)
	}
	if string(got.ResponseBody) != `{"resp":true}` {
		t.Errorf("response body = %q", got.ResponseBody)
	}
}

func TestSaveBatch_MultipleRecords(t *testing.T) {
	store := testDB(t)

	const n = 25
	recs := make([]*Record, n)
	for i := range recs {
		recs[i] = &Record{
			Timestamp:  time.Now(),
			Provider:   "openai",
			Model:      "gpt-4",
			Endpoint:   "/v1/chat/completions",
			StatusCode: 200,
		}
	}

	if err := store.SaveBatch(recs); err != nil {
		t.Fatal(err)
	}

	records, err := store.Recent(0, time.Time{}, time.Time{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != n {
		t.Errorf("got %d records, want %d", len(records), n)
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

// insertTestRecords is a helper that inserts n records with configurable fields.
func insertTestRecords(t *testing.T, store *SQLite, n int, modify func(i int, r *Record)) []Record {
	t.Helper()
	var records []Record
	for i := 0; i < n; i++ {
		cost := float64(i) * 0.01
		rec := &Record{
			Timestamp:    time.Date(2025, 6, 1, 12, i, 0, 0, time.UTC),
			Provider:     "openai",
			Model:        "gpt-4",
			Endpoint:     "/v1/chat/completions",
			Source:       "",
			InputTokens:  100 + i*10,
			OutputTokens: 50 + i*5,
			TotalCost:    &cost,
			DurationMs:   100 + i*50,
			Streaming:    false,
			StatusCode:   200,
		}
		if modify != nil {
			modify(i, rec)
		}
		if err := store.Save(rec); err != nil {
			t.Fatal(err)
		}
		records = append(records, *rec)
	}
	return records
}

func TestList_BasicPagination(t *testing.T) {
	store := testDB(t)
	insertTestRecords(t, store, 10, nil)

	// First page: limit 3
	res, err := store.List(ListFilter{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(res.Items))
	}
	if res.NextCursor == "" {
		t.Fatal("expected non-empty cursor")
	}
	if res.Total == nil {
		t.Fatal("expected non-nil total")
	}
	if *res.Total != 10 {
		t.Errorf("total = %d, want 10", *res.Total)
	}

	// Default sort is timestamp desc (newest first = highest ID first)
	if res.Items[0].ID < res.Items[1].ID || res.Items[1].ID < res.Items[2].ID {
		t.Errorf("not in DESC order: IDs %d, %d, %d", res.Items[0].ID, res.Items[1].ID, res.Items[2].ID)
	}

	// Second page
	res2, err := store.List(ListFilter{Limit: 3, Cursor: res.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Items) != 3 {
		t.Fatalf("page 2: got %d items, want 3", len(res2.Items))
	}
	// IDs should not overlap
	if res2.Items[0].ID >= res.Items[2].ID {
		t.Errorf("page 2 first ID %d should be < page 1 last ID %d", res2.Items[0].ID, res.Items[2].ID)
	}

	// Walk all pages to collect all items
	var allIDs []int64
	cursor := ""
	for {
		r, err := store.List(ListFilter{Limit: 4, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range r.Items {
			allIDs = append(allIDs, item.ID)
		}
		if r.NextCursor == "" {
			break
		}
		cursor = r.NextCursor
	}
	if len(allIDs) != 10 {
		t.Errorf("total items across pages = %d, want 10", len(allIDs))
	}
}

func TestList_Filters(t *testing.T) {
	store := testDB(t)

	boolPtr := func(b bool) *bool { return &b }
	floatPtr := func(f float64) *float64 { return &f }
	intPtr := func(i int) *int { return &i }

	insertTestRecords(t, store, 10, func(i int, r *Record) {
		switch {
		case i < 3:
			r.Provider = "anthropic"
			r.Model = "claude-sonnet-4-6"
			r.Source = "cc:sub"
		case i < 6:
			r.Provider = "openai"
			r.Model = "gpt-4"
			r.Source = "cc:key"
		default:
			r.Provider = "openai"
			r.Model = "gpt-4.1-nano"
			r.Source = ""
		}
		if i%2 == 0 {
			r.Streaming = true
		}
		if i == 5 {
			r.StatusCode = 429
		}
	})

	tests := []struct {
		name   string
		filter ListFilter
		want   int
	}{
		{"by provider", ListFilter{Provider: "anthropic"}, 3},
		{"by model", ListFilter{Model: "gpt-4"}, 3},
		{"by source", ListFilter{Source: "cc:sub"}, 3},
		{"by status code", ListFilter{StatusCode: 429}, 1},
		{"by streaming true", ListFilter{Streaming: boolPtr(true)}, 5},
		{"by streaming false", ListFilter{Streaming: boolPtr(false)}, 5},
		{"by min cost", ListFilter{MinCost: floatPtr(0.05)}, 5},
		{"by max cost", ListFilter{MaxCost: floatPtr(0.04)}, 5},
		{"by cost range", ListFilter{MinCost: floatPtr(0.02), MaxCost: floatPtr(0.06)}, 5},
		{"by min tokens (input+output)", ListFilter{MinTokens: intPtr(225)}, 5},
		{"by max tokens (input+output)", ListFilter{MaxTokens: intPtr(224)}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := store.List(tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Items) != tt.want {
				t.Errorf("got %d items, want %d", len(res.Items), tt.want)
			}
		})
	}
}

func TestList_SortByCost(t *testing.T) {
	store := testDB(t)

	// Insert 6 records with distinct costs
	insertTestRecords(t, store, 6, func(i int, r *Record) {
		cost := float64(i) * 0.10
		r.TotalCost = &cost
	})

	// Sort by cost desc, page size 2
	res, err := store.List(ListFilter{SortBy: "cost", SortDir: "desc", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(res.Items))
	}
	// Highest cost first
	if *res.Items[0].TotalCost < *res.Items[1].TotalCost {
		t.Errorf("not sorted desc: %f < %f", *res.Items[0].TotalCost, *res.Items[1].TotalCost)
	}

	// Follow cursor for next page
	res2, err := store.List(ListFilter{SortBy: "cost", SortDir: "desc", Limit: 2, Cursor: res.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Items) != 2 {
		t.Fatalf("page 2: got %d items, want 2", len(res2.Items))
	}
	// Last of page 1 should have cost >= first of page 2
	if *res.Items[1].TotalCost < *res2.Items[0].TotalCost {
		t.Errorf("cursor pagination broken: page1 last cost %f < page2 first cost %f",
			*res.Items[1].TotalCost, *res2.Items[0].TotalCost)
	}
}

func TestList_TimeRange(t *testing.T) {
	store := testDB(t)

	insertTestRecords(t, store, 5, func(i int, r *Record) {
		r.Timestamp = time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC)
	})

	// Filter: Jan 2 to Jan 4
	from := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 4, 23, 59, 59, 0, time.UTC)

	res, err := store.List(ListFilter{From: from, To: to})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 3 {
		t.Errorf("got %d items, want 3", len(res.Items))
		for _, item := range res.Items {
			t.Logf("  id=%d ts=%s", item.ID, item.Timestamp)
		}
	}
}

func TestList_SearchIDs(t *testing.T) {
	store := testDB(t)

	records := insertTestRecords(t, store, 10, nil)

	// Pick IDs 2, 5, 8
	ids := []int64{records[1].ID, records[4].ID, records[7].ID}

	res, err := store.List(ListFilter{SearchIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(res.Items))
	}
	if res.Total != nil {
		t.Errorf("total should be nil when SearchIDs is set, got %v", *res.Total)
	}

	// Verify only requested IDs returned
	gotIDs := map[int64]bool{}
	for _, item := range res.Items {
		gotIDs[item.ID] = true
	}
	for _, id := range ids {
		if !gotIDs[id] {
			t.Errorf("expected ID %d in results", id)
		}
	}
}

func TestList_DefaultSort(t *testing.T) {
	store := testDB(t)
	insertTestRecords(t, store, 5, nil)

	// No sort specified — should default to timestamp desc (id desc)
	res, err := store.List(ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 5 {
		t.Fatalf("got %d items, want 5", len(res.Items))
	}
	for i := 1; i < len(res.Items); i++ {
		if res.Items[i].ID > res.Items[i-1].ID {
			t.Errorf("not in desc order at index %d: %d > %d", i, res.Items[i].ID, res.Items[i-1].ID)
		}
	}
}

func TestList_LimitDefaults(t *testing.T) {
	store := testDB(t)
	insertTestRecords(t, store, 55, func(i int, r *Record) {
		r.Model = fmt.Sprintf("model-%d", i)
	})

	// No limit — should default to 50
	res, err := store.List(ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 50 {
		t.Errorf("default limit: got %d items, want 50", len(res.Items))
	}
}

// --- DashboardStats tests ---

func TestDashboardStats_Totals(t *testing.T) {
	store := testDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	cost1 := 0.10
	cost2 := 0.20
	// Insert 3 records: 2 OK, 1 error
	store.Save(&Record{
		Timestamp: base, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
		CacheReadTokens: 10, CacheWriteTokens: 5,
		TotalCost: &cost1, DurationMs: 200, StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: base.Add(time.Minute), Provider: "anthropic", Model: "claude-sonnet-4-6",
		Endpoint: "/v1/messages", InputTokens: 200, OutputTokens: 80,
		CacheReadTokens: 20, CacheWriteTokens: 10,
		TotalCost: &cost2, DurationMs: 300, StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: base.Add(2 * time.Minute), Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 50, OutputTokens: 25,
		TotalCost: &cost1, DurationMs: 100, StatusCode: 500,
	})

	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)
	data, err := store.DashboardStats(from, to)
	if err != nil {
		t.Fatal(err)
	}

	tot := data.Totals
	if tot.Requests != 3 {
		t.Errorf("requests = %d, want 3", tot.Requests)
	}
	if tot.InputTokens != 350 {
		t.Errorf("input_tokens = %d, want 350", tot.InputTokens)
	}
	if tot.OutputTokens != 155 {
		t.Errorf("output_tokens = %d, want 155", tot.OutputTokens)
	}
	if tot.CacheReadTokens != 30 {
		t.Errorf("cache_read_tokens = %d, want 30", tot.CacheReadTokens)
	}
	if tot.CacheWriteTokens != 15 {
		t.Errorf("cache_write_tokens = %d, want 15", tot.CacheWriteTokens)
	}
	if tot.TotalCost != 0.40 {
		t.Errorf("total_cost = %f, want 0.40", tot.TotalCost)
	}
	if tot.Errors != 1 {
		t.Errorf("errors = %d, want 1", tot.Errors)
	}
	if tot.UnknownCostRequests != 0 {
		t.Errorf("unknown_cost_requests = %d, want 0", tot.UnknownCostRequests)
	}
	if tot.AvgDurationMs != 200 {
		t.Errorf("avg_duration_ms = %d, want 200", tot.AvgDurationMs)
	}
}

func TestDashboardStats_PrevTotals(t *testing.T) {
	store := testDB(t)
	// Current period: hour 12-13, prev period: hour 11-12
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	cost := 0.10
	// Previous period record
	store.Save(&Record{
		Timestamp: base.Add(-30 * time.Minute), Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
		TotalCost: &cost, DurationMs: 150, StatusCode: 200,
	})
	// Current period record
	store.Save(&Record{
		Timestamp: base.Add(30 * time.Minute), Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 200, OutputTokens: 80,
		TotalCost: &cost, DurationMs: 250, StatusCode: 200,
	})

	from := base
	to := base.Add(time.Hour)
	data, err := store.DashboardStats(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if data.Totals.Requests != 1 {
		t.Errorf("current requests = %d, want 1", data.Totals.Requests)
	}
	if data.PrevTotals == nil {
		t.Fatal("expected non-nil PrevTotals")
	}
	if data.PrevTotals.Requests != 1 {
		t.Errorf("prev requests = %d, want 1", data.PrevTotals.Requests)
	}
	if data.PrevTotals.InputTokens != 100 {
		t.Errorf("prev input_tokens = %d, want 100", data.PrevTotals.InputTokens)
	}
}

func TestDashboardStats_PrevTotals_Nil(t *testing.T) {
	store := testDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	cost := 0.10
	// Only current period record, no previous period data
	store.Save(&Record{
		Timestamp: base.Add(30 * time.Minute), Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 200, OutputTokens: 80,
		TotalCost: &cost, DurationMs: 250, StatusCode: 200,
	})

	from := base
	to := base.Add(time.Hour)
	data, err := store.DashboardStats(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if data.PrevTotals != nil {
		t.Errorf("expected nil PrevTotals when no previous data, got %+v", data.PrevTotals)
	}
}

func TestDashboardStats_Breakdowns(t *testing.T) {
	store := testDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	cost1 := 0.30
	cost2 := 0.10
	cost3 := 0.20
	store.Save(&Record{
		Timestamp: base, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
		TotalCost: &cost1, StatusCode: 200, Source: "cc:sub",
	})
	store.Save(&Record{
		Timestamp: base.Add(time.Minute), Provider: "anthropic", Model: "claude-sonnet-4-6",
		Endpoint: "/v1/messages", InputTokens: 200, OutputTokens: 80,
		TotalCost: &cost2, StatusCode: 200, Source: "cc:key",
	})
	store.Save(&Record{
		Timestamp: base.Add(2 * time.Minute), Provider: "openai", Model: "gpt-4.1-nano",
		Endpoint: "/v1/chat/completions", InputTokens: 150, OutputTokens: 60,
		TotalCost: &cost3, StatusCode: 200, Source: "",
	})

	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)
	data, err := store.DashboardStats(from, to)
	if err != nil {
		t.Fatal(err)
	}

	// by_provider: openai ($0.50) first, anthropic ($0.10) second
	if len(data.ByProvider) != 2 {
		t.Fatalf("by_provider: got %d rows, want 2", len(data.ByProvider))
	}
	if data.ByProvider[0].Name != "openai" {
		t.Errorf("by_provider[0].name = %q, want openai", data.ByProvider[0].Name)
	}
	if data.ByProvider[0].Requests != 2 {
		t.Errorf("by_provider[0].requests = %d, want 2", data.ByProvider[0].Requests)
	}

	// by_model: 3 distinct models
	if len(data.ByModel) != 3 {
		t.Fatalf("by_model: got %d rows, want 3", len(data.ByModel))
	}
	// First should be gpt-4 ($0.30)
	if data.ByModel[0].Name != "gpt-4" {
		t.Errorf("by_model[0].name = %q, want gpt-4", data.ByModel[0].Name)
	}
	// Model rows should include provider
	if data.ByModel[0].Provider != "openai" {
		t.Errorf("by_model[0].provider = %q, want openai", data.ByModel[0].Provider)
	}

	// by_source: 3 distinct sources
	if len(data.BySource) != 3 {
		t.Fatalf("by_source: got %d rows, want 3", len(data.BySource))
	}
}

func TestDashboardStats_ChartBuckets(t *testing.T) {
	store := testDB(t)

	// Test hourly buckets: range ~4 hours => 1-hour buckets
	base := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	cost := 0.01
	for i := 0; i < 4; i++ {
		store.Save(&Record{
			Timestamp: base.Add(time.Duration(i) * time.Hour).Add(15 * time.Minute),
			Provider:  "openai", Model: "gpt-4",
			Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
			TotalCost: &cost, StatusCode: 200,
		})
	}

	from := base
	to := base.Add(4 * time.Hour)
	data, err := store.DashboardStats(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if len(data.Chart) != 4 {
		t.Errorf("chart points = %d, want 4 (hourly buckets)", len(data.Chart))
		for _, cp := range data.Chart {
			t.Logf("  ts=%s requests=%d", cp.Timestamp, cp.Requests)
		}
	}
	for _, cp := range data.Chart {
		if cp.Requests != 1 {
			t.Errorf("chart point at %s: requests = %d, want 1", cp.Timestamp, cp.Requests)
		}
	}

	// Test minute buckets: range ~30 min => 1-minute buckets
	store2 := testDB(t)
	for i := 0; i < 3; i++ {
		store2.Save(&Record{
			Timestamp: base.Add(time.Duration(i) * time.Minute).Add(10 * time.Second),
			Provider:  "openai", Model: "gpt-4",
			Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
			TotalCost: &cost, StatusCode: 200,
		})
	}

	from2 := base
	to2 := base.Add(30 * time.Minute)
	data2, err := store2.DashboardStats(from2, to2)
	if err != nil {
		t.Fatal(err)
	}

	if len(data2.Chart) != 3 {
		t.Errorf("minute chart points = %d, want 3", len(data2.Chart))
		for _, cp := range data2.Chart {
			t.Logf("  ts=%s requests=%d", cp.Timestamp, cp.Requests)
		}
	}

	// Test daily buckets: range ~5 days => 1-day buckets
	store3 := testDB(t)
	for i := 0; i < 5; i++ {
		store3.Save(&Record{
			Timestamp: base.Add(time.Duration(i) * 24 * time.Hour).Add(6 * time.Hour),
			Provider:  "openai", Model: "gpt-4",
			Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
			TotalCost: &cost, StatusCode: 200,
		})
	}

	from3 := base
	to3 := base.Add(5 * 24 * time.Hour)
	data3, err := store3.DashboardStats(from3, to3)
	if err != nil {
		t.Fatal(err)
	}

	if len(data3.Chart) != 5 {
		t.Errorf("daily chart points = %d, want 5", len(data3.Chart))
		for _, cp := range data3.Chart {
			t.Logf("  ts=%s requests=%d", cp.Timestamp, cp.Requests)
		}
	}
}

func TestDashboardStats_NullCost(t *testing.T) {
	store := testDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	cost := 0.10
	// 1 record with cost, 2 with nil cost
	store.Save(&Record{
		Timestamp: base, Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 100, OutputTokens: 50,
		TotalCost: &cost, StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: base.Add(time.Minute), Provider: "openai", Model: "gpt-4",
		Endpoint: "/v1/chat/completions", InputTokens: 200, OutputTokens: 80,
		TotalCost: nil, StatusCode: 200,
	})
	store.Save(&Record{
		Timestamp: base.Add(2 * time.Minute), Provider: "anthropic", Model: "claude-sonnet-4-6",
		Endpoint: "/v1/messages", InputTokens: 150, OutputTokens: 60,
		TotalCost: nil, StatusCode: 200,
	})

	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)
	data, err := store.DashboardStats(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if data.Totals.UnknownCostRequests != 2 {
		t.Errorf("unknown_cost_requests = %d, want 2", data.Totals.UnknownCostRequests)
	}
	if data.Totals.TotalCost != 0.10 {
		t.Errorf("total_cost = %f, want 0.10", data.Totals.TotalCost)
	}
	if data.Totals.Requests != 3 {
		t.Errorf("requests = %d, want 3", data.Totals.Requests)
	}
}

// --- Analytics tests ---

func TestAnalytics_OverTime(t *testing.T) {
	store := testDB(t)

	// Insert data across 3 different hours so bucketing is visible.
	for h := 0; h < 3; h++ {
		for i := 0; i < 5; i++ {
			cost := 0.01 * float64(h+1)
			store.Save(&Record{
				Timestamp:    time.Date(2025, 6, 15, h, i, 0, 0, time.UTC),
				Provider:     "openai",
				Model:        "gpt-4",
				Endpoint:     "/v1/chat/completions",
				InputTokens:  100,
				OutputTokens: 50,
				TotalCost:    &cost,
				StatusCode:   200,
			})
		}
	}

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)

	data, err := store.Analytics(from, to, "hour")
	if err != nil {
		t.Fatal(err)
	}

	if len(data.OverTime) != 3 {
		t.Fatalf("over_time buckets = %d, want 3", len(data.OverTime))
	}

	// Each hour has 5 requests.
	for i, pt := range data.OverTime {
		if pt.Requests != 5 {
			t.Errorf("bucket %d: requests = %d, want 5", i, pt.Requests)
		}
	}
}

func TestAnalytics_Heatmap(t *testing.T) {
	store := testDB(t)

	// Wednesday (day_of_week=3) at 14:00
	cost := 0.05
	for i := 0; i < 3; i++ {
		store.Save(&Record{
			Timestamp:  time.Date(2025, 6, 11, 14, i, 0, 0, time.UTC), // Wednesday
			Provider:   "openai",
			Model:      "gpt-4",
			Endpoint:   "/v1/chat/completions",
			TotalCost:  &cost,
			StatusCode: 200,
		})
	}
	// Sunday (day_of_week=0) at 9:00
	store.Save(&Record{
		Timestamp:  time.Date(2025, 6, 8, 9, 0, 0, 0, time.UTC), // Sunday
		Provider:   "openai",
		Model:      "gpt-4",
		Endpoint:   "/v1/chat/completions",
		TotalCost:  &cost,
		StatusCode: 200,
	})

	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 30, 23, 59, 59, 0, time.UTC)

	data, err := store.Analytics(from, to, "day")
	if err != nil {
		t.Fatal(err)
	}

	if len(data.Heatmap) != 2 {
		t.Fatalf("heatmap entries = %d, want 2", len(data.Heatmap))
	}

	// Find the Wednesday entry.
	found := false
	for _, h := range data.Heatmap {
		if h.DayOfWeek == 3 && h.Hour == 14 {
			found = true
			if h.Requests != 3 {
				t.Errorf("wednesday 14:00 requests = %d, want 3", h.Requests)
			}
		}
	}
	if !found {
		t.Error("heatmap missing Wednesday 14:00 entry")
	}
}

func TestAnalytics_CostDistribution(t *testing.T) {
	store := testDB(t)

	// Insert 100 records with costs 0.01, 0.02, ..., 1.00
	for i := 1; i <= 100; i++ {
		cost := float64(i) * 0.01
		store.Save(&Record{
			Timestamp:  time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
			Provider:   "openai",
			Model:      "gpt-4",
			Endpoint:   "/v1/chat/completions",
			TotalCost:  &cost,
			StatusCode: 200,
		})
	}

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 23, 59, 59, 0, time.UTC)

	data, err := store.Analytics(from, to, "hour")
	if err != nil {
		t.Fatal(err)
	}

	cd := data.CostDistribution
	if cd.Max != 1.0 {
		t.Errorf("max = %f, want 1.0", cd.Max)
	}
	// P50 of 1..100 * 0.01 = ~0.505
	if cd.P50 < 0.49 || cd.P50 > 0.52 {
		t.Errorf("p50 = %f, want ~0.505", cd.P50)
	}
	if cd.P90 < 0.89 || cd.P90 > 0.92 {
		t.Errorf("p90 = %f, want ~0.905", cd.P90)
	}
	if cd.P99 < 0.98 || cd.P99 > 1.01 {
		t.Errorf("p99 = %f, want ~0.99", cd.P99)
	}
}

func TestAnalytics_TopExpensive(t *testing.T) {
	store := testDB(t)

	// Insert 15 records with different costs.
	for i := 1; i <= 15; i++ {
		cost := float64(i) * 0.10
		store.Save(&Record{
			Timestamp:    time.Date(2025, 6, 15, 12, i, 0, 0, time.UTC),
			Provider:     "openai",
			Model:        fmt.Sprintf("model-%d", i),
			Endpoint:     "/v1/chat/completions",
			InputTokens:  i * 100,
			OutputTokens: i * 50,
			TotalCost:    &cost,
			StatusCode:   200,
		})
	}

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 23, 59, 59, 0, time.UTC)

	data, err := store.Analytics(from, to, "hour")
	if err != nil {
		t.Fatal(err)
	}

	if len(data.TopExpensive) != 10 {
		t.Fatalf("top_expensive = %d, want 10", len(data.TopExpensive))
	}

	// Should be in descending cost order, highest first.
	if data.TopExpensive[0].Cost < data.TopExpensive[1].Cost {
		t.Errorf("top_expensive not in DESC order: %f < %f", data.TopExpensive[0].Cost, data.TopExpensive[1].Cost)
	}
	// Most expensive is model-15 at $1.50
	if data.TopExpensive[0].Cost != 1.5 {
		t.Errorf("most expensive cost = %f, want 1.5", data.TopExpensive[0].Cost)
	}
}

func TestAnalytics_CumulativeCost(t *testing.T) {
	store := testDB(t)

	// Insert 3 hours of data.
	for h := 0; h < 3; h++ {
		cost := float64(h+1) * 0.10
		store.Save(&Record{
			Timestamp:  time.Date(2025, 6, 15, h, 0, 0, 0, time.UTC),
			Provider:   "openai",
			Model:      "gpt-4",
			Endpoint:   "/v1/chat/completions",
			TotalCost:  &cost,
			StatusCode: 200,
		})
	}

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)

	data, err := store.Analytics(from, to, "hour")
	if err != nil {
		t.Fatal(err)
	}

	if len(data.CumulativeCost) != 3 {
		t.Fatalf("cumulative_cost points = %d, want 3", len(data.CumulativeCost))
	}

	// 0.10, 0.30, 0.60
	expected := []float64{0.10, 0.30, 0.60}
	for i, pt := range data.CumulativeCost {
		diff := pt.Cumulative - expected[i]
		if diff > 0.001 || diff < -0.001 {
			t.Errorf("cumulative[%d] = %f, want %f", i, pt.Cumulative, expected[i])
		}
	}
}

func TestAnalytics_CacheHitRate(t *testing.T) {
	store := testDB(t)

	// Hour 0: 100 input tokens, 50 cache read => 50% hit rate
	store.Save(&Record{
		Timestamp:       time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Provider:        "anthropic",
		Model:           "claude-sonnet-4-6",
		Endpoint:        "/v1/messages",
		InputTokens:     100,
		CacheReadTokens: 50,
		StatusCode:      200,
	})

	// Hour 1: 200 input tokens, 0 cache read => 0% hit rate
	store.Save(&Record{
		Timestamp:   time.Date(2025, 6, 15, 1, 0, 0, 0, time.UTC),
		Provider:    "openai",
		Model:       "gpt-4",
		Endpoint:    "/v1/chat/completions",
		InputTokens: 200,
		StatusCode:  200,
	})

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 2, 0, 0, 0, time.UTC)

	data, err := store.Analytics(from, to, "hour")
	if err != nil {
		t.Fatal(err)
	}

	if len(data.CacheHitRate) != 2 {
		t.Fatalf("cache_hit_rate points = %d, want 2", len(data.CacheHitRate))
	}

	// First bucket: 50/100 = 0.5
	if data.CacheHitRate[0].Rate != 0.5 {
		t.Errorf("cache_hit_rate[0] = %f, want 0.5", data.CacheHitRate[0].Rate)
	}
	// Second bucket: 0/200 = 0
	if data.CacheHitRate[1].Rate != 0 {
		t.Errorf("cache_hit_rate[1] = %f, want 0", data.CacheHitRate[1].Rate)
	}
}

func TestSearchByBody_MatchFound(t *testing.T) {
	store := testDB(t)

	// Insert records with bodies containing searchable text.
	store.Save(&Record{
		Timestamp:    time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		Provider:     "openai",
		Model:        "gpt-4",
		Endpoint:     "/v1/chat/completions",
		StatusCode:   200,
		RequestBody:  []byte(`{"messages":[{"content":"Hello World"}]}`),
		ResponseBody: []byte(`{"choices":[{"message":{"content":"Hi there"}}]}`),
	})
	store.Save(&Record{
		Timestamp:    time.Date(2025, 6, 15, 12, 1, 0, 0, time.UTC),
		Provider:     "openai",
		Model:        "gpt-4",
		Endpoint:     "/v1/chat/completions",
		StatusCode:   200,
		RequestBody:  []byte(`{"messages":[{"content":"Goodbye"}]}`),
		ResponseBody: []byte(`{"choices":[{"message":{"content":"See you"}}]}`),
	})
	store.Save(&Record{
		Timestamp:    time.Date(2025, 6, 15, 12, 2, 0, 0, time.UTC),
		Provider:     "openai",
		Model:        "gpt-4",
		Endpoint:     "/v1/chat/completions",
		StatusCode:   200,
		RequestBody:  []byte(`{"messages":[{"content":"Another message"}]}`),
		ResponseBody: []byte(`{"choices":[{"message":{"content":"hello again"}}]}`),
	})

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 23, 59, 59, 0, time.UTC)

	// Search for "hello" (case insensitive) — should match record 1 (request) and 3 (response).
	ids, err := store.SearchByBody("hello", from, to, 100, 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(ids) != 2 {
		t.Fatalf("matches = %d, want 2", len(ids))
	}
}

func TestSearchByBody_MaxScanLimit(t *testing.T) {
	store := testDB(t)

	// Insert 10 records, all containing "needle".
	for i := 0; i < 10; i++ {
		store.Save(&Record{
			Timestamp:    time.Date(2025, 6, 15, 12, i, 0, 0, time.UTC),
			Provider:     "openai",
			Model:        "gpt-4",
			Endpoint:     "/v1/chat/completions",
			StatusCode:   200,
			RequestBody:  []byte(fmt.Sprintf(`{"content":"needle-%d"}`, i)),
			ResponseBody: []byte(`{"resp":"ok"}`),
		})
	}

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 23, 59, 59, 0, time.UTC)

	// maxScan=3: only scan 3 bodies, all match.
	ids, err := store.SearchByBody("needle", from, to, 3, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Errorf("matches with maxScan=3: got %d, want 3", len(ids))
	}

	// limit=2: scan all but return max 2.
	ids, err = store.SearchByBody("needle", from, to, 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("matches with limit=2: got %d, want 2", len(ids))
	}
}

func TestFilters_DistinctValues(t *testing.T) {
	store := testDB(t)

	records := []Record{
		{Timestamp: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC), Provider: "openai", Model: "gpt-4", Endpoint: "/v1/chat/completions", Source: "cc:sub", StatusCode: 200},
		{Timestamp: time.Date(2025, 6, 15, 12, 1, 0, 0, time.UTC), Provider: "anthropic", Model: "claude-sonnet-4-6", Endpoint: "/v1/messages", Source: "cc:key", StatusCode: 200},
		{Timestamp: time.Date(2025, 6, 15, 12, 2, 0, 0, time.UTC), Provider: "openai", Model: "gpt-4.1-nano", Endpoint: "/v1/chat/completions", Source: "", StatusCode: 200},
		{Timestamp: time.Date(2025, 6, 15, 12, 3, 0, 0, time.UTC), Provider: "anthropic", Model: "claude-sonnet-4-6", Endpoint: "/v1/messages", Source: "cc:sub", StatusCode: 200},
	}

	for i := range records {
		if err := store.Save(&records[i]); err != nil {
			t.Fatal(err)
		}
	}

	from := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 6, 15, 23, 59, 59, 0, time.UTC)

	opts, err := store.Filters(from, to)
	if err != nil {
		t.Fatal(err)
	}

	// Providers: anthropic, openai (sorted)
	if len(opts.Providers) != 2 {
		t.Fatalf("providers = %d, want 2", len(opts.Providers))
	}
	if opts.Providers[0] != "anthropic" || opts.Providers[1] != "openai" {
		t.Errorf("providers = %v, want [anthropic openai]", opts.Providers)
	}

	// Models: claude-sonnet-4-6, gpt-4, gpt-4.1-nano (sorted)
	if len(opts.Models) != 3 {
		t.Fatalf("models = %d, want 3", len(opts.Models))
	}

	// Sources: "", cc:key, cc:sub (sorted)
	if len(opts.Sources) != 3 {
		t.Fatalf("sources = %d, want 3", len(opts.Sources))
	}
}
