package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kintsugi-storage/internal/storage"
)

// TestNewRouterUnknownRoute tests that the NewRouter function returns a 404 status code for an unknown route.
func TestNewRouterUnknownRoute(t *testing.T) {
	t.Parallel()

	router := NewRouter(RouterDependencies{})

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusNotFound)
	}

	body := readRouterBody(t, res.Body)
	if !strings.Contains(body, "route not found") {
		t.Fatalf("body = %q, want route not found error", body)
	}
}

// TestNewRouterObjectsPutRoute tests that the NewRouter function returns a 201 status code for a successful PUT request.
func TestNewRouterObjectsPutRoute(t *testing.T) {
	t.Parallel()

	store := &mockObjectStore{
		putFn: func(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error {
			if key != "user-1" {
				t.Fatalf("Put() key = %q, want %q", key, "user-1")
			}

			if string(payload) != `{"name":"alice"}` {
				t.Fatalf("Put() payload = %s, want %s", string(payload), `{"name":"alice"}`)
			}

			return nil
		},
	}

	objects := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)
	objects.now = func() time.Time {
		return time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	}

	router := NewRouter(RouterDependencies{
		Objects: objects,
	})

	req := httptest.NewRequest(http.MethodPut, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusCreated)
	}
}

// TestNewRouterObjectsGetRoute tests that the NewRouter function returns a 200 status code for a successful GET request.
func TestNewRouterObjectsGetRoute(t *testing.T) {
	t.Parallel()

	store := &mockObjectStore{
		getFn: func(key string, now time.Time) (storage.Item, error) {
			if key != "user-1" {
				t.Fatalf("Get() key = %q, want %q", key, "user-1")
			}

			return storage.NewItem(json.RawMessage(`{"name":"alice"}`), nil), nil
		},
	}

	objects := NewObjectsHandler(store, slog.Default(), &mockObjectMetrics{}, 1024)
	objects.now = func() time.Time {
		return time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	}

	router := NewRouter(RouterDependencies{
		Objects: objects,
	})

	req := httptest.NewRequest(http.MethodGet, "/objects/user-1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	body := readRouterBody(t, res.Body)
	if body != `{"name":"alice"}` {
		t.Fatalf("body = %q, want %q", body, `{"name":"alice"}`)
	}
}

// TestNewRouterObjectsMethodNotAllowed tests that the NewRouter function returns a 405 status code for a method not allowed request.
func TestNewRouterObjectsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	objects := NewObjectsHandler(&mockObjectStore{}, slog.Default(), &mockObjectMetrics{}, 1024)

	router := NewRouter(RouterDependencies{
		Objects: objects,
	})

	req := httptest.NewRequest(http.MethodPost, "/objects/user-1", strings.NewReader(`{"name":"alice"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}

	body := readRouterBody(t, res.Body)
	if !strings.Contains(body, "method not allowed") {
		t.Fatalf("body = %q, want method not allowed error", body)
	}
}

// TestNewRouterLivenessRoute tests that the NewRouter function returns a 200 status code for a successful Liveness request.
func TestNewRouterLivenessRoute(t *testing.T) {
	t.Parallel()

	probes := NewProbesHandler(&mockReadinessChecker{ready: true}, slog.Default())

	router := NewRouter(RouterDependencies{
		Probes: probes,
	})

	req := httptest.NewRequest(http.MethodGet, "/probes/liveness", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}
}

// TestNewRouterReadinessRoute tests that the NewRouter function returns a 200 status code for a successful Readiness request.
func TestNewRouterReadinessRoute(t *testing.T) {
	t.Parallel()

	probes := NewProbesHandler(&mockReadinessChecker{ready: true}, slog.Default())

	router := NewRouter(RouterDependencies{
		Probes: probes,
	})

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}
}

// TestNewRouterDocsRoute tests that the NewRouter function returns a 200 status code for a successful Docs request.
func TestNewRouterDocsRoute(t *testing.T) {
	t.Parallel()

	router := NewRouter(RouterDependencies{})

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	if got := res.Header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want to contain text/html", got)
	}

	body := readRouterBody(t, res.Body)
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("body = %q, want Swagger UI page", body)
	}
}

// TestNewRouterOpenAPIRoute tests that the NewRouter function returns a 200 status code for a successful OpenAPI request.
func TestNewRouterOpenAPIRoute(t *testing.T) {
	t.Parallel()

	router := NewRouter(RouterDependencies{})

	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	body := readRouterBody(t, res.Body)
	if !strings.Contains(body, "openapi: 3.0.3") {
		t.Fatalf("body = %q, want openapi yaml", body)
	}
}

// TestNewRouterMetricsRoute tests that the NewRouter function returns a 200 status code for a successful Metrics request.
func TestNewRouterMetricsRoute(t *testing.T) {
	t.Parallel()

	metrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("storage_put_requests_total 1\n"))
	})

	router := NewRouter(RouterDependencies{
		Metrics: metrics,
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	body := readRouterBody(t, res.Body)
	if !strings.Contains(body, "storage_put_requests_total 1") {
		t.Fatalf("body = %q, want metrics payload", body)
	}
}

// readRouterBody is a helper function to read the body of a response.
func readRouterBody(t *testing.T, body io.ReadCloser) string {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v, want nil", err)
	}

	return strings.TrimSpace(string(data))
}
