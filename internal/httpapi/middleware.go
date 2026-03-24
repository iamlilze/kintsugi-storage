package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware - стандартный тип HTTP middleware.
type Middleware func(http.Handler) http.Handler

// Chain применяет middleware в обратном порядке, чтобы первый в списке был внешним.
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	if handler == nil {
		return http.NotFoundHandler()
	}

	wrapped := handler

	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}

		wrapped = middlewares[i](wrapped)
	}

	return wrapped
}

// statusRecorder перехватывает WriteHeader, чтобы мы могли залогировать итоговый HTTP status.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader перехватывает код ответа.
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write перехватывает запись тела ответа.
func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

// RecoveryMiddleware защищает сервер от panic внутри handler'ов.
func RecoveryMiddleware(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { // Panic перехватываем через deferred recovery.
				if rec := recover(); rec != nil {
					logger.Error(
						"http handler panic",
						"method", r.Method,
						"path", r.URL.Path,
						"panic", rec,
						"stack", string(debug.Stack()),
					)

					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RequestLoggingMiddleware логирует каждый HTTP-запрос после его завершения.
func RequestLoggingMiddleware(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			recorder := &statusRecorder{
				ResponseWriter: w,
			}

			next.ServeHTTP(recorder, r)

			if recorder.status == 0 {
				recorder.status = http.StatusOK
			}

			logger.Info(
				"http request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"bytes", recorder.bytes,
				"duration", time.Since(start),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
