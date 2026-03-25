package httpapi

import (
	"context"
	"log/slog"
	"net/http"
)

// Возвращает nil, если сервис готов, или error с описанием причины.
type ReadinessChecker interface {
	CheckReady(ctx context.Context) error
}

type ProbesHandler struct {
	readiness ReadinessChecker
	logger    *slog.Logger
}

func NewProbesHandler(readiness ReadinessChecker, logger *slog.Logger) *ProbesHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProbesHandler{
		readiness: readiness,
		logger:    logger,
	}
}

func (h *ProbesHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ProbesHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.readiness == nil {
		writeError(w, http.StatusServiceUnavailable, "service is not ready")
		return
	}

	if err := h.readiness.CheckReady(r.Context()); err != nil {
		h.logger.Warn("readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "service is not ready")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
