package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockEvictionMetrics struct {
	mu           sync.Mutex
	totalDeleted int
	updateCalls  int
	objectsTotal int
}

func (m *mockEvictionMetrics) AddExpiredDeletions(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalDeleted += n
	m.updateCalls++
}

func (m *mockEvictionMetrics) SetObjectsTotal(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objectsTotal = n
}

func (m *mockEvictionMetrics) TotalDeleted() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalDeleted
}

func (m *mockEvictionMetrics) UpdateCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCalls
}

func (m *mockEvictionMetrics) ObjectsTotal() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.objectsTotal
}

func TestEvictionWorkerDeletesExpiredObjects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	store := New(WithClock(func() time.Time { return now }))
	ctx := context.Background()

	shortTTL := 50 * time.Millisecond
	longTTL := time.Hour

	_ = store.Put(ctx, "a", []byte(`{"id":1}`), &shortTTL)
	_ = store.Put(ctx, "b", []byte(`{"id":2}`), &shortTTL)
	_ = store.Put(ctx, "c", []byte(`{"id":3}`), &longTTL)

	now = now.Add(100 * time.Millisecond)

	metrics := &mockEvictionMetrics{}
	worker := NewEvictionWorker(store, 10*time.Millisecond, nil, metrics)

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- worker.Run(runCtx) }()

	waitUntil(t, 200*time.Millisecond, func() bool {
		return metrics.TotalDeleted() >= 2
	})

	cancel()
	<-done

	if got := metrics.TotalDeleted(); got < 2 {
		t.Fatalf("metrics.TotalDeleted() = %d, want >= 2", got)
	}
}

func TestEvictionWorkerStopsOnCancel(t *testing.T) {
	t.Parallel()

	store := New()
	worker := NewEvictionWorker(store, 50*time.Millisecond, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run() did not stop after cancel")
	}
}

func TestEvictionWorkerDisabledWithZeroInterval(t *testing.T) {
	t.Parallel()

	store := New()
	worker := NewEvictionWorker(store, 0, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run() did not stop after cancel")
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
