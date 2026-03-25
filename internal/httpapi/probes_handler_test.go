package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockReadinessChecker struct {
	err error
}

func (m *mockReadinessChecker) CheckReady(_ context.Context) error {
	return m.err
}

func TestLivenessSuccess(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/liveness", nil)
	rec := httptest.NewRecorder()
	handler.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %q, want liveness ok", body)
	}
}

func TestLivenessMethodNotAllowed(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{}, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/probes/liveness", nil)
	rec := httptest.NewRecorder()
	handler.Liveness(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestReadinessSuccess(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, `"status":"ready"`) {
		t.Fatalf("body = %q, want ready", body)
	}
}

func TestReadinessNotReady(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{err: fmt.Errorf("store down")}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := readBody(t, rec.Result().Body)
	if !strings.Contains(body, "service is not ready") {
		t.Fatalf("body = %q, want not ready", body)
	}
}

func TestReadinessNilChecker(t *testing.T) {
	handler := NewProbesHandler(nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/probes/readiness", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestReadinessMethodNotAllowed(t *testing.T) {
	handler := NewProbesHandler(&mockReadinessChecker{}, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/probes/readiness", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
