package httpapi

import (
	"context"
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

type mockObjectStore struct {
	putFn func(ctx context.Context, key string, value json.RawMessage, ttl *time.Duration) error
	getFn func(ctx context.Context, key string) (json.RawMessage, error)
}

func (m *mockObjectStore) Put(ctx context.Context, key string, value json.RawMessage, ttl *time.Duration) error {
	if m.putFn == nil {
		return nil
	}
	return m.putFn(ctx, key, value, ttl)
}

func (m *mockObjectStore) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if m.getFn == nil {
		return nil, nil
	}
	return m.getFn(ctx, key)
}

type mockObjectMetrics struct {
	putRequests int
	getRequests int
	hits        int
	misses      int
}

func (m *mockObjectMetrics) IncPutRequests() { m.putRequests++ }
func (m *mockObjectMetrics) IncGetRequests() { m.getRequests++ }
func (m *mockObjectMetrics) IncHits()        { m.hits++ }
func (m *mockObjectMetrics) IncMisses()      { m.misses++ }

func TestPutObjectSuccess(t *testing.T) {
	store := &mockObjectStore{
		putFn: func(_ context.Context, key string, value json.RawMessage, ttl *time.Duration) error {
			if key != "user-1" {
				t.Fatalf("Put() key = %q, want %q", key, "user-1")
			}
			if string(value) != `{"name":"alice"}` {
				t.Fatalf("Put() value = %s, want %s", value, `{"name":"alice"}`)
			}
			if ttl != nil {
				t.Fatalf("Put() ttl = %v, want nil", ttl)
			}
			return nil
		},
	}

	metrics := &mockObjectMetrics{}
	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	rec := httptest.NewRecorder()
	handler.PutObject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusCreated)
	}
	if metrics.putRequests != 1 {
		t.Fatalf("metrics.putRequests = %d, want 1", metrics.putRequests)
	}
}

func TestPutObjectWithExpires(t *testing.T) {
	store := &mockObjectStore{
		putFn: func(_ context.Context, key string, _ json.RawMessage, ttl *time.Duration) error {
			if ttl == nil || *ttl <= 0 {
				t.Fatalf("Put() ttl = %v, want positive duration", ttl)
			}
			return nil
		},
	}

	handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC1123)
	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Expires", expires)
	rec := httptest.NewRecorder()

	handler.PutObject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestPutObjectInvalidJSON(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":`))
	rec := httptest.NewRecorder()
	handler.PutObject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, "request body must be valid JSON") {
		t.Fatalf("body = %q, want JSON error", body)
	}
}

func TestPutObjectInvalidExpires(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Expires", "tomorrow maybe")
	rec := httptest.NewRecorder()
	handler.PutObject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, "invalid Expires header") {
		t.Fatalf("body = %q, want Expires error", body)
	}
}

func TestPutObjectExpiredExpires(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 1024)

	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC1123)
	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Expires", past)
	rec := httptest.NewRecorder()
	handler.PutObject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, "object expiration time is in the past") {
		t.Fatalf("body = %q, want past-expiration error", body)
	}
}

// проверяем что ошибки из store правильно маппятся в HTTP коды
func TestPutObjectStoreErrors(t *testing.T) {
	tests := []struct {
		name       string
		storeErr   error
		wantStatus int
		wantBody   string
	}{
		{"invalid key", storage.ErrInvalidKey, http.StatusBadRequest, "invalid object key"},
		{"invalid payload", storage.ErrInvalidPayload, http.StatusBadRequest, "request body must be valid JSON"},
		{"already expired", storage.ErrAlreadyExpired, http.StatusBadRequest, "object expiration time is in the past"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockObjectStore{
				putFn: func(context.Context, string, json.RawMessage, *time.Duration) error { return tt.storeErr },
			}
			handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

			req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
			rec := httptest.NewRecorder()
			handler.PutObject(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("StatusCode = %d, want %d", rec.Code, tt.wantStatus)
			}
			body := readBody(t, rec.Result().Body)
			if !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestPutObjectBodyTooLarge(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 5)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	rec := httptest.NewRecorder()
	handler.PutObject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetObjectSuccess(t *testing.T) {
	store := &mockObjectStore{
		getFn: func(_ context.Context, key string) (json.RawMessage, error) {
			if key != "user-1" {
				t.Fatalf("Get() key = %q, want %q", key, "user-1")
			}
			return json.RawMessage(`{"name":"alice"}`), nil
		},
	}

	metrics := &mockObjectMetrics{}
	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)

	req := httptest.NewRequest(http.MethodGet, "/objects/user-1", nil)
	rec := httptest.NewRecorder()
	handler.GetObject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	body := readBody(t, rec.Result().Body)
	if body != `{"name":"alice"}` {
		t.Fatalf("body = %q, want %q", body, `{"name":"alice"}`)
	}
	if metrics.getRequests != 1 {
		t.Fatalf("getRequests = %d, want 1", metrics.getRequests)
	}
	if metrics.hits != 1 {
		t.Fatalf("hits = %d, want 1", metrics.hits)
	}
}

func TestGetObjectNotFound(t *testing.T) {
	store := &mockObjectStore{
		getFn: func(context.Context, string) (json.RawMessage, error) {
			return nil, storage.ErrNotFound
		},
	}
	metrics := &mockObjectMetrics{}
	handler := NewObjectsHandler(store, slog.Default(), metrics, 1024)

	req := httptest.NewRequest(http.MethodGet, "/objects/missing", nil)
	rec := httptest.NewRecorder()
	handler.GetObject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if metrics.misses != 1 {
		t.Fatalf("misses = %d, want 1", metrics.misses)
	}
}

func TestGetObjectUnknownError(t *testing.T) {
	store := &mockObjectStore{
		getFn: func(context.Context, string) (json.RawMessage, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)

	req := httptest.NewRequest(http.MethodGet, "/objects/user-1", nil)
	rec := httptest.NewRecorder()
	handler.GetObject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestObjectKeyValidation(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 1024)

	tests := []struct {
		name string
		path string
	}{
		{"empty key", "/objects/"},
		{"invalid chars", "/objects/foo!bar"},
		{"slash in key", "/objects/foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.GetObject(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

// убеждаемся что nil metrics не вызывает панику
func TestNilMetricsSafe(t *testing.T) {
	handler := NewObjectsHandler(&mockObjectStore{}, slog.Default(), nil, 1024)

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"ok":true}`))
	rec := httptest.NewRecorder()
	handler.PutObject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func readBody(t *testing.T, body io.ReadCloser) string {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	return strings.TrimSpace(string(data))
}
