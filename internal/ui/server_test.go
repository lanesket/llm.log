package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lanesket/llm.log/internal/storage"
)

func testServer(t *testing.T) (*Server, *storage.SQLite) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	srv := New(store, dir, nil, false)
	return srv, store
}

func seedRecords(t *testing.T, store *storage.SQLite, n int) {
	t.Helper()
	cost := 0.05
	base := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	for i := range n {
		rec := &storage.Record{
			Timestamp:    base.Add(time.Duration(i) * time.Minute),
			Provider:     "openai",
			Model:        "gpt-4",
			Endpoint:     "/v1/chat/completions",
			Source:       "test",
			InputTokens:  100 + i,
			OutputTokens: 50 + i,
			TotalCost:    &cost,
			DurationMs:   150,
			Streaming:    i%2 == 0,
			StatusCode:   200,
			RequestBody:  []byte(`{"prompt":"hello"}`),
			ResponseBody: []byte(`{"text":"world"}`),
		}
		if err := store.Save(rec); err != nil {
			t.Fatal(err)
		}
	}
}

func doRequest(srv *Server, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, w.Body.String())
	}
}

func TestHandleStatus(t *testing.T) {
	srv, _ := testServer(t)
	w := doRequest(srv, "GET", "/api/status")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statusResponse
	decodeJSON(t, w, &resp)

	if resp.ProxyRunning {
		t.Error("expected proxy_running=false")
	}
	if resp.PID != nil {
		t.Error("expected pid=null")
	}
	if resp.UptimeSeconds != nil {
		t.Error("expected uptime_seconds=null")
	}
	if resp.Port != nil {
		t.Error("expected port=null")
	}
}

func TestHandleDashboard(t *testing.T) {
	srv, store := testServer(t)
	seedRecords(t, store, 5)

	w := doRequest(srv, "GET", "/api/dashboard?from=2025-01-01&to=2025-12-31")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp storage.DashboardData
	decodeJSON(t, w, &resp)

	if resp.Totals.Requests != 5 {
		t.Errorf("expected 5 requests, got %d", resp.Totals.Requests)
	}

	// Second request should hit cache
	w2 := doRequest(srv, "GET", "/api/dashboard?from=2025-01-01&to=2025-12-31")
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on cached request, got %d", w2.Code)
	}

	var resp2 storage.DashboardData
	decodeJSON(t, w2, &resp2)
	if resp2.Totals.Requests != 5 {
		t.Errorf("cached: expected 5 requests, got %d", resp2.Totals.Requests)
	}
}

func TestHandleRequests_Pagination(t *testing.T) {
	srv, store := testServer(t)
	seedRecords(t, store, 10)

	// First page
	w := doRequest(srv, "GET", "/api/requests?limit=3&from=2025-01-01&to=2025-12-31")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp listResponse
	decodeJSON(t, w, &resp)

	if len(resp.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(resp.Items))
	}
	if resp.NextCursor == "" {
		t.Error("expected non-empty next_cursor")
	}
	if resp.Total == nil || *resp.Total != 10 {
		t.Errorf("expected total=10, got %v", resp.Total)
	}

	// Second page using cursor
	w2 := doRequest(srv, "GET", "/api/requests?limit=3&cursor="+resp.NextCursor+"&from=2025-01-01&to=2025-12-31")
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 listResponse
	decodeJSON(t, w2, &resp2)

	if len(resp2.Items) != 3 {
		t.Errorf("page 2: expected 3 items, got %d", len(resp2.Items))
	}
}

func TestHandleRequests_Filters(t *testing.T) {
	srv, store := testServer(t)
	seedRecords(t, store, 10)

	// Filter by streaming=true (even indices: 0,2,4,6,8 = 5 records)
	w := doRequest(srv, "GET", "/api/requests?streaming=true&from=2025-01-01&to=2025-12-31")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp listResponse
	decodeJSON(t, w, &resp)

	if resp.Total == nil || *resp.Total != 5 {
		t.Errorf("expected 5 streaming requests, got %v", resp.Total)
	}
	for _, item := range resp.Items {
		if !item.Streaming {
			t.Errorf("expected all items to be streaming, got non-streaming item %d", item.ID)
		}
	}
}

func TestHandleRequestDetail(t *testing.T) {
	srv, store := testServer(t)
	seedRecords(t, store, 1)

	w := doRequest(srv, "GET", "/api/requests/1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp requestDetail
	decodeJSON(t, w, &resp)

	if resp.ID != 1 {
		t.Errorf("expected id=1, got %d", resp.ID)
	}
	if resp.Provider != "openai" {
		t.Errorf("expected provider=openai, got %s", resp.Provider)
	}
	if resp.RequestBody == nil {
		t.Error("expected non-nil request_body")
	}
	if resp.ResponseBody == nil {
		t.Error("expected non-nil response_body")
	}
}

func TestHandleRequestDetail_NotFound(t *testing.T) {
	srv, _ := testServer(t)

	w := doRequest(srv, "GET", "/api/requests/99999")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAnalytics(t *testing.T) {
	srv, store := testServer(t)
	seedRecords(t, store, 5)

	w := doRequest(srv, "GET", "/api/analytics?from=2025-01-01&to=2025-12-31&granularity=day")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp storage.AnalyticsData
	decodeJSON(t, w, &resp)

	if len(resp.OverTime) == 0 {
		t.Error("expected non-empty over_time data")
	}
}

func TestHandleFilters(t *testing.T) {
	srv, store := testServer(t)
	seedRecords(t, store, 3)

	w := doRequest(srv, "GET", "/api/filters?from=2025-01-01&to=2025-12-31")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp storage.FilterOptions
	decodeJSON(t, w, &resp)

	if len(resp.Providers) == 0 {
		t.Error("expected at least one provider")
	}
	if resp.Providers[0] != "openai" {
		t.Errorf("expected provider=openai, got %s", resp.Providers[0])
	}
}

func TestHandleError_InvalidTimeRange(t *testing.T) {
	srv, _ := testServer(t)

	// from > to
	w := doRequest(srv, "GET", "/api/dashboard?from=2025-12-31&to=2025-01-01")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	decodeJSON(t, w, &resp)
	if resp["error"] == "" {
		t.Error("expected error message")
	}

	// Invalid format
	w2 := doRequest(srv, "GET", "/api/dashboard?from=not-a-date")
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}

	// Invalid granularity
	w3 := doRequest(srv, "GET", "/api/analytics?granularity=invalid")
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w3.Code, w3.Body.String())
	}
}
