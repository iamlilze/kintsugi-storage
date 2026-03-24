package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"kintsugi-storage/internal/storage"
)

// ObjectStore - узкий интерфейс потребности объектового хендлера.
type ObjectStore interface {
	Put(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error
	Get(key string, now time.Time) (storage.Item, error)
	Len(now time.Time) int
}

// ObjectMetrics - узкий контракт для обновления прикладных метрик.
type ObjectMetrics interface {
	IncPutRequests()
	IncGetRequests()
	IncHits()
	IncMisses()
	SetObjectsTotal(n int)
}

// ObjectsHandler обслуживает PUT/GET операции над JSON-объектами.
type ObjectsHandler struct {
	store        ObjectStore
	logger       *slog.Logger
	metrics      ObjectMetrics
	maxBodyBytes int64
	now          func() time.Time
}

// NewObjectsHandler создает handler для работы с объектами.
func NewObjectsHandler(
	store ObjectStore,
	logger *slog.Logger,
	metrics ObjectMetrics,
	maxBodyBytes int64,
) *ObjectsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	// Default limit is 1 MiB to protect the API from unbounded request bodies.
	if maxBodyBytes <= 0 {
		maxBodyBytes = 1 << 20 // 1 MiB
	}

	return &ObjectsHandler{
		store:        store,
		logger:       logger,
		metrics:      metrics,
		maxBodyBytes: maxBodyBytes,
		now:          time.Now,
	}
}

// PutObject обрабатывает PUT /objects/{key}.
func (h *ObjectsHandler) PutObject(w http.ResponseWriter, r *http.Request) {
	if h.metrics != nil {
		h.metrics.IncPutRequests()
	}

	key, err := objectKeyFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid object key")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	expiresAt, err := parseExpiresHeader(r.Header.Get("Expires"), h.now())
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExpired) {
			writeError(w, http.StatusBadRequest, "object expiration time is in the past")
			return
		}

		writeError(w, http.StatusBadRequest, "invalid Expires header")
		return
	}

	err = h.store.Put(key, json.RawMessage(body), expiresAt, h.now())
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidKey):
			writeError(w, http.StatusBadRequest, "invalid object key")
			return
		case errors.Is(err, storage.ErrInvalidPayload):
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
			return
		case errors.Is(err, storage.ErrAlreadyExpired):
			writeError(w, http.StatusBadRequest, "object expiration time is in the past")
			return
		default:
			h.logger.Error("failed to store object", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if h.metrics != nil {
		h.metrics.SetObjectsTotal(h.store.Len(h.now()))
	}

	w.WriteHeader(http.StatusCreated)
}

// GetObject обрабатывает GET /objects/{key}.
func (h *ObjectsHandler) GetObject(w http.ResponseWriter, r *http.Request) {
	if h.metrics != nil {
		h.metrics.IncGetRequests()
	}

	key, err := objectKeyFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid object key")
		return
	}

	item, err := h.store.Get(key, h.now())
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidKey):
			writeError(w, http.StatusBadRequest, "invalid object key")
			return
		case errors.Is(err, storage.ErrNotFound):
			if h.metrics != nil {
				h.metrics.IncMisses()
				h.metrics.SetObjectsTotal(h.store.Len(h.now()))
			}
			writeError(w, http.StatusNotFound, "object not found")
			return
		default:
			h.logger.Error("failed to get object", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if h.metrics != nil {
		h.metrics.IncHits()
		h.metrics.SetObjectsTotal(h.store.Len(h.now()))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(item.Payload)
}

// parseExpiresHeader парсит Expires и возвращает абсолютное время истечения.
func parseExpiresHeader(raw string, now time.Time) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC1123, raw)
	if err != nil {
		return nil, err
	}

	if !parsed.After(now) {
		return nil, storage.ErrAlreadyExpired
	}

	return &parsed, nil
}

// objectKeyFromRequest извлекает key из URL.
// Это минимальный вариант для стандартного ServeMux; если будешь использовать chi, заменишь на chi.URLParam.
func objectKeyFromRequest(r *http.Request) (string, error) {
	const prefix = "/objects/"

	if !strings.HasPrefix(r.URL.Path, prefix) {
		return "", storage.ErrInvalidKey
	}

	key := strings.TrimPrefix(r.URL.Path, prefix)
	key = strings.TrimSpace(key)

	if key == "" {
		return "", storage.ErrInvalidKey
	}

	return key, nil
}
