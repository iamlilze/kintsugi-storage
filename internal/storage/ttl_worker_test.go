package storage

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// mockExpiredDeleter is a mock implementation of the ExpiredDeleter interface.
type mockExpiredDeleter struct {
	mu        sync.Mutex
	calls     int
	returnSeq []int
	lenValue  int
}

// DeleteExpired is a mock implementation of the DeleteExpired method.
func (m *mockExpiredDeleter) DeleteExpired(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++

	if len(m.returnSeq) == 0 {
		return 0
	}

	n := m.returnSeq[0]
	m.returnSeq = m.returnSeq[1:]
	return n
}

// CallCount is a mock implementation of the CallCount method.
func (m *mockExpiredDeleter) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.calls
}

// Len is a mock implementation of the Len method.
func (m *mockExpiredDeleter) Len(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.lenValue
}

// mockExpirationMetrics is a mock implementation of the ExpirationMetrics interface.
type mockExpirationMetrics struct {
	mu           sync.Mutex
	totalDeleted int
	updateCalls  int
	objectsTotal int
}

// AddExpiredDeletions is a mock implementation of the AddExpiredDeletions method.
func (m *mockExpirationMetrics) AddExpiredDeletions(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalDeleted += n
	m.updateCalls++
}

// TotalDeleted is a mock implementation of the TotalDeleted method.
func (m *mockExpirationMetrics) TotalDeleted() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.totalDeleted
}

// UpdateCalls is a mock implementation of the UpdateCalls method.
func (m *mockExpirationMetrics) UpdateCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.updateCalls
}

// SetObjectsTotal is a mock implementation of the SetObjectsTotal method.
func (m *mockExpirationMetrics) SetObjectsTotal(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.objectsTotal = n
}

// ObjectsTotal is a mock implementation of the ObjectsTotal method.
func (m *mockExpirationMetrics) ObjectsTotal() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.objectsTotal
}

// TestTTLWorkerRunDeletesExpiredObjects tests that the Run method deletes expired objects.
func TestTTLWorkerRunDeletesExpiredObjects(t *testing.T) {
	t.Parallel()

	store := &mockExpiredDeleter{
		returnSeq: []int{2, 1},
		lenValue:  7,
	}
	metrics := &mockExpirationMetrics{}

	worker := NewTTLWorker(store, 10*time.Millisecond, slog.Default(), metrics)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- worker.Run(ctx)
	}()

	waitUntil(t, 200*time.Millisecond, func() bool {
		return store.CallCount() >= 2
	})

	cancel()

	err := <-done
	if err == nil {
		t.Fatal("Run() error = nil, want context cancellation error")
	}

	if got := store.CallCount(); got < 2 {
		t.Fatalf("DeleteExpired() call count = %d, want at least 2", got)
	}

	if got := metrics.TotalDeleted(); got != 3 {
		t.Fatalf("metrics.TotalDeleted() = %d, want 3", got)
	}

	if got := metrics.UpdateCalls(); got != 2 {
		t.Fatalf("metrics.UpdateCalls() = %d, want 2", got)
	}

	if got := metrics.ObjectsTotal(); got != 7 {
		t.Fatalf("metrics.ObjectsTotal() = %d, want 7", got)
	}
}

// TestTTLWorkerRunDoesNotUpdateMetricsWhenNothingDeleted tests that the Run method does not update metrics when nothing is deleted.
func TestTTLWorkerRunDoesNotUpdateMetricsWhenNothingDeleted(t *testing.T) {
	t.Parallel()

	store := &mockExpiredDeleter{
		returnSeq: []int{0, 0},
		lenValue:  5,
	}
	metrics := &mockExpirationMetrics{}

	worker := NewTTLWorker(store, 10*time.Millisecond, slog.Default(), metrics)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- worker.Run(ctx)
	}()

	waitUntil(t, 200*time.Millisecond, func() bool {
		return store.CallCount() >= 2
	})

	cancel()
	<-done

	if got := metrics.TotalDeleted(); got != 0 {
		t.Fatalf("metrics.TotalDeleted() = %d, want 0", got)
	}

	if got := metrics.UpdateCalls(); got != 0 {
		t.Fatalf("metrics.UpdateCalls() = %d, want 0", got)
	}

	if got := metrics.ObjectsTotal(); got != 5 {
		t.Fatalf("metrics.ObjectsTotal() = %d, want 5", got)
	}
}

// TestTTLWorkerRunWorksWithNilMetrics tests that the Run method works with nil metrics.
func TestTTLWorkerRunWorksWithNilMetrics(t *testing.T) {
	t.Parallel()

	store := &mockExpiredDeleter{
		returnSeq: []int{1},
	}

	worker := NewTTLWorker(store, 10*time.Millisecond, slog.Default(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- worker.Run(ctx)
	}()

	waitUntil(t, 200*time.Millisecond, func() bool {
		return store.CallCount() >= 1
	})

	cancel()
	err := <-done
	if err == nil {
		t.Fatal("Run() error = nil, want context cancellation error")
	}

	if got := store.CallCount(); got < 1 {
		t.Fatalf("DeleteExpired() call count = %d, want at least 1", got)
	}
}

// TestTTLWorkerRunStopsOnContextCancel tests that the Run method stops on context cancel.
func TestTTLWorkerRunStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	store := &mockExpiredDeleter{}
	worker := NewTTLWorker(store, 50*time.Millisecond, slog.Default(), nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- worker.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() error = nil, want context cancellation error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

// TestTTLWorkerRunWithNonPositiveIntervalWaitsForContextCancel tests that the Run method waits for context cancel when the interval is non-positive.
func TestTTLWorkerRunWithNonPositiveIntervalWaitsForContextCancel(t *testing.T) {
	t.Parallel()

	store := &mockExpiredDeleter{}
	worker := NewTTLWorker(store, 0, slog.Default(), nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- worker.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)

	if got := store.CallCount(); got != 0 {
		t.Fatalf("DeleteExpired() call count = %d, want 0", got)
	}

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() error = nil, want context cancellation error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

// waitUntil is a helper function to wait until a condition is met.
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
