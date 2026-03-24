package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockReadinessChecker is a mock implementation of the ReadinessChecker interface.
type mockReadinessChecker struct {
	ready bool
}

// IsReady is a mock implementation of the IsReady method.
func (m *mockReadinessChecker) IsReady() bool {
	return m.ready
}

// TestProbesHandlerLivenessSuccess tests that the Liveness method works correctly.
func TestProbesHandlerLivenessSuccess(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{ready: true}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/liveness", nil)
	rec := httptest.NewRecorder()

	handler.Liveness(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	body := readProbeBody(t, res.Body)
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %q, want liveness status ok", body)
	}
}

// TestProbesHandlerLivenessMethodNotAllowed tests that the Liveness method returns an error if the method is not allowed.
func TestProbesHandlerLivenessMethodNotAllowed(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{ready: true}, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/probes/liveness", nil)
	rec := httptest.NewRecorder()

	handler.Liveness(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}

	body := readProbeBody(t, res.Body)
	if !strings.Contains(body, "method not allowed") {
		t.Fatalf("body = %q, want method not allowed error", body)
	}
}

// TestProbesHandlerReadinessSuccess tests that the Readiness method works correctly.
func TestProbesHandlerReadinessSuccess(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{ready: true}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	body := readProbeBody(t, res.Body)
	if !strings.Contains(body, `"status":"ready"`) {
		t.Fatalf("body = %q, want readiness status ready", body)
	}
}

// TestProbesHandlerReadinessNotReady tests that the Readiness method returns an error if the service is not ready.
func TestProbesHandlerReadinessNotReady(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{ready: false}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusServiceUnavailable)
	}

	body := readProbeBody(t, res.Body)
	if !strings.Contains(body, "service is not ready") {
		t.Fatalf("body = %q, want readiness error", body)
	}
}

// TestProbesHandlerReadinessNilChecker tests that the Readiness method returns an error if the checker is nil.
func TestProbesHandlerReadinessNilChecker(t *testing.T) {
	handler := NewProbesHandler(nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusServiceUnavailable)
	}

	body := readProbeBody(t, res.Body)
	if !strings.Contains(body, "service is not ready") {
		t.Fatalf("body = %q, want readiness error", body)
	}
}

// TestProbesHandlerReadinessMethodNotAllowed tests that the Readiness method returns an error if the method is not allowed.
func TestProbesHandlerReadinessMethodNotAllowed(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{ready: true}, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/probes/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}

	body := readProbeBody(t, res.Body)
	if !strings.Contains(body, "method not allowed") {
		t.Fatalf("body = %q, want method not allowed error", body)
	}
}

// readProbeBody is a helper function to read the body of a response.
func readProbeBody(t *testing.T, body io.ReadCloser) string {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v, want nil", err)
	}

	return strings.TrimSpace(string(data))
}
