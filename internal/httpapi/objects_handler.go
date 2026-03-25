package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"kintsugi-storage/internal/storage"
)

type ObjectStore interface {
	Put(ctx context.Context, key string, value json.RawMessage, ttl *time.Duration) error
	Get(ctx context.Context, key string) (json.RawMessage, error)
}

type ObjectMetrics interface {
	IncPutRequests()
	IncGetRequests()
	IncHits()
	IncMisses()
}

type nopObjectMetrics struct{}

func (nopObjectMetrics) IncPutRequests() {}
func (nopObjectMetrics) IncGetRequests() {}
func (nopObjectMetrics) IncHits()        {}
func (nopObjectMetrics) IncMisses()      {}

const maxKeyLength = 256

var validKeyRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:@\-]*$`)

type ObjectsHandler struct {
	store        ObjectStore
	logger       *slog.Logger
	metrics      ObjectMetrics
	maxBodyBytes int64
}

func NewObjectsHandler(
	store ObjectStore,
	logger *slog.Logger,
	metrics ObjectMetrics,
	maxBodyBytes int64,
) *ObjectsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if metrics == nil {
		metrics = nopObjectMetrics{}
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = 1 << 20
	}
	return &ObjectsHandler{
		store:        store,
		logger:       logger,
		metrics:      metrics,
		maxBodyBytes: maxBodyBytes,
	}
}

func (h *ObjectsHandler) PutObject(w http.ResponseWriter, r *http.Request) {
	h.metrics.IncPutRequests()

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

	ttl, err := parseTTL(r.Header.Get("Expires"), time.Now())
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExpired) {
			writeError(w, http.StatusBadRequest, "object expiration time is in the past")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid Expires header")
		return
	}

	if err := h.store.Put(r.Context(), key, json.RawMessage(body), ttl); err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidKey):
			writeError(w, http.StatusBadRequest, "invalid object key")
		case errors.Is(err, storage.ErrInvalidPayload):
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		case errors.Is(err, storage.ErrAlreadyExpired):
			writeError(w, http.StatusBadRequest, "object expiration time is in the past")
		default:
			h.logger.Error("failed to store object", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *ObjectsHandler) GetObject(w http.ResponseWriter, r *http.Request) {
	h.metrics.IncGetRequests()

	key, err := objectKeyFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid object key")
		return
	}

	payload, err := h.store.Get(r.Context(), key)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidKey):
			writeError(w, http.StatusBadRequest, "invalid object key")
		case errors.Is(err, storage.ErrNotFound):
			h.metrics.IncMisses()
			writeError(w, http.StatusNotFound, "object not found")
		default:
			h.logger.Error("failed to get object", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.metrics.IncHits()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

// parseTTL преобразует Expires header (RFC 1123) в duration относительно now.
func parseTTL(raw string, now time.Time) (*time.Duration, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC1123, raw)
	if err != nil {
		return nil, err
	}
	ttl := parsed.Sub(now)
	if ttl <= 0 {
		return nil, storage.ErrAlreadyExpired
	}
	return &ttl, nil
}

func objectKeyFromRequest(r *http.Request) (string, error) {
	const prefix = "/objects/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return "", storage.ErrInvalidKey
	}

	key := strings.TrimPrefix(r.URL.Path, prefix)
	key = strings.TrimSpace(key)

	if key == "" || len(key) > maxKeyLength {
		return "", storage.ErrInvalidKey
	}

	if !validKeyRe.MatchString(key) {
		return "", storage.ErrInvalidKey
	}

	return key, nil
}
