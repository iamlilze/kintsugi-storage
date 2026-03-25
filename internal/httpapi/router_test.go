package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterUnknownRoute(t *testing.T) {
	t.Parallel()

	router := NewRouter(RouterDependencies{})
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusNotFound)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, "route not found") {
		t.Fatalf("body = %q, want route not found", body)
	}
}

func TestRouterObjectsPut(t *testing.T) {
	t.Parallel()

	objects := NewObjectsHandler(&mockObjectStore{}, slog.Default(), nil, 1024)
	router := NewRouter(RouterDependencies{Objects: objects})

	req := httptest.NewRequest(http.MethodPut, "/objects/test-key", strings.NewReader(`{"ok":true}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestRouterObjectsGet(t *testing.T) {
	t.Parallel()

	store := &mockObjectStore{
		getFn: func(_ context.Context, _ string) (json.RawMessage, error) {
			return json.RawMessage(`{"ok":true}`), nil
		},
	}
	objects := NewObjectsHandler(store, slog.Default(), nil, 1024)
	router := NewRouter(RouterDependencies{Objects: objects})

	req := httptest.NewRequest(http.MethodGet, "/objects/test-key", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouterObjectsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	objects := NewObjectsHandler(&mockObjectStore{}, slog.Default(), nil, 1024)
	router := NewRouter(RouterDependencies{Objects: objects})

	req := httptest.NewRequest(http.MethodPost, "/objects/user-1", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterLiveness(t *testing.T) {
	t.Parallel()

	probes := NewProbesHandler(&mockReadinessChecker{}, slog.Default())
	router := NewRouter(RouterDependencies{Probes: probes})

	req := httptest.NewRequest(http.MethodGet, "/probes/liveness", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouterReadiness(t *testing.T) {
	t.Parallel()

	probes := NewProbesHandler(&mockReadinessChecker{}, slog.Default())
	router := NewRouter(RouterDependencies{Probes: probes})

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouterMetrics(t *testing.T) {
	t.Parallel()

	metrics := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("storage_put_requests_total 1\n"))
	})

	router := NewRouter(RouterDependencies{Metrics: metrics})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, "storage_put_requests_total 1") {
		t.Fatalf("body = %q, want metrics", body)
	}
}
