package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kintsugi-storage/internal/storage"
)

// mockObjectStore is a mock implementation of the ObjectStore interface.
type mockObjectStore struct {
	putFn func(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error
	getFn func(key string, now time.Time) (storage.Item, error)
	lenFn func(now time.Time) int
}

// Put is a mock implementation of the Put method.
func (m *mockObjectStore) Put(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error {
	if m.putFn == nil {
		return nil
	}

	return m.putFn(key, payload, expiresAt, now)
}

// Get is a mock implementation of the Get method.
func (m *mockObjectStore) Get(key string, now time.Time) (storage.Item, error) {
	if m.getFn == nil {
		return storage.Item{}, nil
	}

	return m.getFn(key, now)
}

func (m *mockObjectStore) Len(now time.Time) int {
	if m.lenFn == nil {
		return 0
	}

	return m.lenFn(now)
}

// mockObjectMetrics is a mock implementation of the ObjectMetrics interface.
type mockObjectMetrics struct {
	putRequests  int
	getRequests  int
	hits         int
	misses       int
	objectsTotal int
}

func (m *mockObjectMetrics) IncPutRequests() { m.putRequests++ }
func (m *mockObjectMetrics) IncGetRequests() { m.getRequests++ }
func (m *mockObjectMetrics) IncHits()        { m.hits++ }
func (m *mockObjectMetrics) IncMisses()      { m.misses++ }
func (m *mockObjectMetrics) SetObjectsTotal(n int) {
	m.objectsTotal = n
}

// TestObjectsHandlerPutObjectSuccess tests that the PutObject method works correctly.
func TestObjectsHandlerPutObjectSuccess(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	store := &mockObjectStore{
		putFn: func(key string, payload json.RawMessage, expiresAt *time.Time, gotNow time.Time) error {
			if key != "user-1" {
				t.Fatalf("Put() key = %q, want %q", key, "user-1")
			}

			if string(payload) != `{"name":"alice"}` {
				t.Fatalf("Put() payload = %s, want %s", string(payload), `{"name":"alice"}`)
			}

			if expiresAt != nil {
				t.Fatalf("Put() expiresAt = %v, want nil", expiresAt)
			}

			if !gotNow.Equal(now) {
				t.Fatalf("Put() now = %v, want %v", gotNow, now)
			}

			return nil
		},
	}

	metrics := &mockObjectMetrics{}
	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)
	handler.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	if metrics.putRequests != 1 {
		t.Fatalf("metrics.putRequests = %d, want 1", metrics.putRequests)
	}
}

// TestObjectsHandlerPutObjectInvalidJSON tests that the PutObject method returns an error if the request body is invalid JSON.
func TestObjectsHandlerPutObjectInvalidJSON(t *testing.T) {
	store := &mockObjectStore{}
	metrics := &mockObjectMetrics{}

	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

	body := readBody(t, res.Body)
	if !strings.Contains(body, "request body must be valid JSON") {
		t.Fatalf("body = %q, want error about invalid JSON", body)
	}
}

// TestObjectsHandlerPutObjectInvalidExpires tests that the PutObject method returns an error if the Expires header is invalid.
func TestObjectsHandlerPutObjectInvalidExpires(t *testing.T) {
	store := &mockObjectStore{}
	metrics := &mockObjectMetrics{}

	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Expires", "tomorrow maybe")

	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

	body := readBody(t, res.Body)
	if !strings.Contains(body, "invalid Expires header") {
		t.Fatalf("body = %q, want error about invalid Expires", body)
	}
}

func TestObjectsHandlerPutObjectExpiredExpires(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	store := &mockObjectStore{}
	metrics := &mockObjectMetrics{}

	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)
	handler.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Expires", "Mon, 23 Mar 2026 11:59:59 UTC")

	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

	body := readBody(t, res.Body)
	if !strings.Contains(body, "object expiration time is in the past") {
		t.Fatalf("body = %q, want error about expiration time in the past", body)
	}
}

// TestObjectsHandlerPutObjectStoreValidationErrors tests that the PutObject method returns an error if the store returns a validation error.
func TestObjectsHandlerPutObjectStoreValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		storeErr   error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "invalid key",
			storeErr:   storage.ErrInvalidKey,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid object key",
		},
		{
			name:       "invalid payload",
			storeErr:   storage.ErrInvalidPayload,
			wantStatus: http.StatusBadRequest,
			wantBody:   "request body must be valid JSON",
		},
		{
			name:       "already expired",
			storeErr:   storage.ErrAlreadyExpired,
			wantStatus: http.StatusBadRequest,
			wantBody:   "object expiration time is in the past",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockObjectStore{
				putFn: func(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error {
					return tt.storeErr
				},
			}

			handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

			req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
			rec := httptest.NewRecorder()

			handler.PutObject(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("StatusCode = %d, want %d", res.StatusCode, tt.wantStatus)
			}

			body := readBody(t, res.Body)
			if !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q, want to contain %q", body, tt.wantBody)
			}
		})
	}
}

// TestObjectsHandlerPutObjectStoreUnknownError tests that the PutObject method returns an error if the store returns an unknown error.
func TestObjectsHandlerPutObjectStoreUnknownError(t *testing.T) {
	store := &mockObjectStore{
		putFn: func(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error {
			return errors.New("boom")
		},
	}

	handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusInternalServerError)
	}
}

// TestObjectsHandlerPutObjectBodyTooLarge tests that the PutObject method returns an error if the request body is too large.
func TestObjectsHandlerPutObjectBodyTooLarge(t *testing.T) {
	store := &mockObjectStore{}
	handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 5)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

// TestObjectsHandlerGetObjectSuccess tests that the GetObject method works correctly.
func TestObjectsHandlerGetObjectSuccess(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	store := &mockObjectStore{
		getFn: func(key string, gotNow time.Time) (storage.Item, error) {
			if key != "user-1" {
				t.Fatalf("Get() key = %q, want %q", key, "user-1")
			}

			if !gotNow.Equal(now) {
				t.Fatalf("Get() now = %v, want %v", gotNow, now)
			}

			return storage.NewItem(json.RawMessage(`{"name":"alice"}`), nil), nil
		},
	}

	metrics := &mockObjectMetrics{}
	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)
	handler.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/objects/user-1", nil)
	rec := httptest.NewRecorder()

	handler.GetObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}

	body := readBody(t, res.Body)
	if body != `{"name":"alice"}` {
		t.Fatalf("body = %q, want %q", body, `{"name":"alice"}`)
	}

	if metrics.getRequests != 1 {
		t.Fatalf("metrics.getRequests = %d, want 1", metrics.getRequests)
	}

	if metrics.hits != 1 {
		t.Fatalf("metrics.hits = %d, want 1", metrics.hits)
	}
}

// TestObjectsHandlerGetObjectNotFound tests that the GetObject method returns an error if the object is not found.
func TestObjectsHandlerGetObjectNotFound(t *testing.T) {
	store := &mockObjectStore{
		getFn: func(key string, now time.Time) (storage.Item, error) {
			return storage.Item{}, storage.ErrNotFound
		},
	}

	metrics := &mockObjectMetrics{}
	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)

	req := httptest.NewRequest(http.MethodGet, "/objects/missing", nil)
	rec := httptest.NewRecorder()

	handler.GetObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusNotFound)
	}

	body := readBody(t, res.Body)
	if !strings.Contains(body, "object not found") {
		t.Fatalf("body = %q, want error about object not found", body)
	}

	if metrics.getRequests != 1 {
		t.Fatalf("metrics.getRequests = %d, want 1", metrics.getRequests)
	}

	if metrics.misses != 1 {
		t.Fatalf("metrics.misses = %d, want 1", metrics.misses)
	}
}

// TestObjectsHandlerGetObjectInvalidKey tests that the GetObject method returns an error if the key is invalid.
func TestObjectsHandlerGetObjectInvalidKey(t *testing.T) {
	store := &mockObjectStore{
		getFn: func(key string, now time.Time) (storage.Item, error) {
			return storage.Item{}, storage.ErrInvalidKey
		},
	}

	handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

	req := httptest.NewRequest(http.MethodGet, "/objects/user-1", nil)
	rec := httptest.NewRecorder()

	handler.GetObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

// TestObjectsHandlerGetObjectUnknownError tests that the GetObject method returns an error if the store returns an unknown error.
func TestObjectsHandlerGetObjectUnknownError(t *testing.T) {
	store := &mockObjectStore{
		getFn: func(key string, now time.Time) (storage.Item, error) {
			return storage.Item{}, errors.New("boom")
		},
	}

	handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

	req := httptest.NewRequest(http.MethodGet, "/objects/user-1", nil)
	rec := httptest.NewRecorder()

	handler.GetObject(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusInternalServerError)
	}
}

// TestObjectsHandlerDefaultMaxBodyBytes tests that the DefaultMaxBodyBytes method returns the correct value.
func TestObjectsHandlerDefaultMaxBodyBytes(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 0)

	if handler.maxBodyBytes != 1<<20 {
		t.Fatalf("maxBodyBytes = %d, want %d", handler.maxBodyBytes, 1<<20)
	}
}

// readBody is a helper function to read the body of a response.
func readBody(t *testing.T, body io.ReadCloser) string {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v, want nil", err)
	}

	return strings.TrimSpace(string(data))
}
