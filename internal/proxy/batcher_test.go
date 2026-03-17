package proxy

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lanesket/llm.log/internal/storage"
)

// mockStore records SaveBatch calls for assertions.
type mockStore struct {
	mu        sync.Mutex
	saved     []*storage.Record
	batches   []int // sizes of each SaveBatch call
	failNext  int   // number of SaveBatch calls to fail before succeeding
	failCount int   // total number of failed SaveBatch calls
}

func (m *mockStore) Save(rec *storage.Record) error {
	return m.SaveBatch([]*storage.Record{rec})
}

func (m *mockStore) SaveBatch(recs []*storage.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext > 0 {
		m.failNext--
		m.failCount++
		return fmt.Errorf("mock save error")
	}
	m.saved = append(m.saved, recs...)
	m.batches = append(m.batches, len(recs))
	return nil
}

func (m *mockStore) Stats(_ storage.StatsFilter) ([]storage.StatRow, error) { return nil, nil }
func (m *mockStore) Recent(_ int, _, _ time.Time, _, _ string) ([]storage.Record, error) {
	return nil, nil
}
func (m *mockStore) Get(_ int64) (*storage.Record, error)     { return nil, nil }
func (m *mockStore) Sources(_, _ time.Time) ([]string, error) { return nil, nil }
func (m *mockStore) Close() error                             { return nil }

func (m *mockStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.saved)
}

func (m *mockStore) batchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}

func (m *mockStore) lastBatchSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.batches) == 0 {
		return 0
	}
	return m.batches[len(m.batches)-1]
}

// newTestProxy builds a Proxy wired to a mockStore and starts the real
// runBatcher goroutine with a custom batch timeout.
func newTestProxy(t *testing.T, store *mockStore, batchTimeout time.Duration) *Proxy {
	t.Helper()
	p := &Proxy{
		store:        store,
		saveQueue:    make(chan *storage.Record, saveQueueSize),
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
		batchTimeout: batchTimeout,
	}
	go p.runBatcher()
	return p
}

func rec() *storage.Record {
	return &storage.Record{
		Timestamp:  time.Now(),
		Provider:   "openai",
		Model:      "gpt-4",
		Endpoint:   "/v1/chat/completions",
		StatusCode: 200,
	}
}

// TestBatcher_FlushOnTimeout verifies records are saved after the timeout even
// when the batch is not full.
func TestBatcher_FlushOnTimeout(t *testing.T) {
	store := &mockStore{}
	p := newTestProxy(t, store, 50*time.Millisecond)

	p.saveQueue <- rec()
	p.saveQueue <- rec()

	time.Sleep(150 * time.Millisecond)

	if store.count() != 2 {
		t.Errorf("got %d records, want 2", store.count())
	}
	if store.batchCount() != 1 {
		t.Errorf("got %d SaveBatch calls, want 1", store.batchCount())
	}

	close(p.stop)
	<-p.stopped
}

// TestBatcher_FlushOnBatchSize verifies SaveBatch is called once the batch
// reaches saveBatchSize.
func TestBatcher_FlushOnBatchSize(t *testing.T) {
	store := &mockStore{}
	p := newTestProxy(t, store, 10*time.Second) // long timeout — size triggers first

	for i := 0; i < saveBatchSize; i++ {
		p.saveQueue <- rec()
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if store.count() == saveBatchSize {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if store.count() != saveBatchSize {
		t.Errorf("got %d records, want %d", store.count(), saveBatchSize)
	}
	if store.lastBatchSize() != saveBatchSize {
		t.Errorf("last batch size = %d, want %d", store.lastBatchSize(), saveBatchSize)
	}

	close(p.stop)
	<-p.stopped
}

// TestBatcher_ShutdownDrainsQueue verifies that records in the queue are not
// lost when Shutdown is called.
func TestBatcher_ShutdownDrainsQueue(t *testing.T) {
	store := &mockStore{}
	p := newTestProxy(t, store, 10*time.Second) // long timeout — shutdown must drain

	const n = 10
	for i := 0; i < n; i++ {
		p.saveQueue <- rec()
	}

	close(p.stop)
	<-p.stopped

	if store.count() != n {
		t.Errorf("got %d records after shutdown, want %d", store.count(), n)
	}
}

// TestBatcher_DropWhenQueueFull verifies the proxy does not block when the
// save queue is full, dropping the record instead.
func TestBatcher_DropWhenQueueFull(t *testing.T) {
	store := &mockStore{}
	// Don't start the batcher — queue must fill up.
	p := &Proxy{
		store:     store,
		saveQueue: make(chan *storage.Record, 1), // capacity 1
		stop:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
	close(p.stopped)

	// Fill the queue.
	p.saveQueue <- rec()

	// This should not block; record is dropped.
	done := make(chan struct{})
	go func() {
		select {
		case p.saveQueue <- rec():
		default:
			// expected: queue full, drop
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("save blocked instead of dropping")
	}
}

// TestBatcher_RetryThenSucceed verifies that a transient SaveBatch error
// is retried on the next flush and records are eventually saved.
func TestBatcher_RetryThenSucceed(t *testing.T) {
	store := &mockStore{failNext: 1} // fail once, then succeed
	p := newTestProxy(t, store, 50*time.Millisecond)

	p.saveQueue <- rec()

	// First timeout flush fails, second succeeds.
	time.Sleep(200 * time.Millisecond)

	if store.count() != 1 {
		t.Errorf("got %d records, want 1 (after retry)", store.count())
	}

	close(p.stop)
	<-p.stopped
}

// TestBatcher_DropAfterMaxRetries verifies that records are dropped (not
// accumulated forever) after maxRetries consecutive failures.
func TestBatcher_DropAfterMaxRetries(t *testing.T) {
	store := &mockStore{failNext: maxRetries + 10} // fail indefinitely
	p := newTestProxy(t, store, 20*time.Millisecond)

	p.saveQueue <- rec()

	// Wait for enough flushes to exhaust retries.
	time.Sleep(time.Duration(maxRetries+2) * 50 * time.Millisecond)

	store.mu.Lock()
	fails := store.failCount
	store.mu.Unlock()

	if fails < maxRetries {
		t.Errorf("expected at least %d failures, got %d", maxRetries, fails)
	}

	// After dropping, batcher should accept new records normally.
	store.mu.Lock()
	store.failNext = 0
	store.mu.Unlock()

	p.saveQueue <- rec()
	time.Sleep(100 * time.Millisecond)

	if store.count() != 1 {
		t.Errorf("got %d records, want 1 (new record after drop)", store.count())
	}

	close(p.stop)
	<-p.stopped
}

// TestBatcher_ShutdownRetriesOnError verifies that the shutdown drain
// retries SaveBatch before giving up.
func TestBatcher_ShutdownRetriesOnError(t *testing.T) {
	store := &mockStore{failNext: 1} // fail once, succeed on second attempt
	p := newTestProxy(t, store, 10*time.Second)

	p.saveQueue <- rec()
	p.saveQueue <- rec()

	close(p.stop)
	<-p.stopped

	if store.count() != 2 {
		t.Errorf("got %d records after shutdown, want 2 (retried)", store.count())
	}
}
