package httpapi

import (
	"log/slog"
	"net/http"
)

// ReadinessChecker - узкий интерфейс для readiness handler.
type ReadinessChecker interface {
	IsReady() bool
}

// ProbesHandler обслуживает liveness и readiness probes.
type ProbesHandler struct {
	readiness ReadinessChecker
	logger    *slog.Logger
}

// NewProbesHandler создает handler для health probes.
func NewProbesHandler(readiness ReadinessChecker, logger *slog.Logger) *ProbesHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &ProbesHandler{
		readiness: readiness,
		logger:    logger,
	}
}

// Liveness обрабатывает GET /probes/liveness.
func (h *ProbesHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// Readiness обрабатывает GET /probes/readiness.
func (h *ProbesHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.readiness == nil {
		writeError(w, http.StatusServiceUnavailable, "service is not ready")
		return
	}

	if !h.readiness.IsReady() {
		writeError(w, http.StatusServiceUnavailable, "service is not ready")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
