package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"kintsugi-storage/internal/config"
	"kintsugi-storage/internal/httpapi"
	"kintsugi-storage/internal/observability"
	"kintsugi-storage/internal/persistence"
	"kintsugi-storage/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// readinessState хранит флаг готовности приложения к обслуживанию трафика.
type readinessState struct {
	ready atomic.Bool
}

// IsReady возвращает текущее состояние readiness.
func (r *readinessState) IsReady() bool {
	return r.ready.Load()
}

// SetReady обновляет readiness flag.
func (r *readinessState) SetReady(value bool) {
	r.ready.Store(value)
}

// App хранит все long-lived зависимости приложения.
type App struct {
	cfg         config.Config
	logger      *slog.Logger
	metrics     *observability.Metrics
	store       *storage.MemoryStore
	snapshotter *persistence.FileSnapshotter
	ttlWorker   *storage.TTLWorker
	server      *http.Server
	readiness   *readinessState
}

// New собирает приложение и все его зависимости.
func New(cfg config.Config) (*App, error) {
	// logger
	logger := observability.NewLogger(observability.LoggerConfig{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})

	// metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	metrics := observability.NewMetrics(registry)

	// store
	store := storage.NewMemoryStore()
	// snapshotter
	snapshotter := persistence.NewFileSnapshotter(cfg.SnapshotPath)
	// snapshot
	snapshot, err := snapshotter.Load()
	if err != nil {
		return nil, fmt.Errorf("app: load snapshot: %w", err)
	}

	// restore snapshot
	if err := store.Restore(snapshot, timeNow()); err != nil {
		return nil, fmt.Errorf("app: restore snapshot: %w", err)
	}

	metrics.SetObjectsTotal(store.Len(timeNow()))

	readiness := &readinessState{}
	readiness.SetReady(false)

	// objects handler
	objectsHandler := httpapi.NewObjectsHandler(
		store,
		logger,
		metrics,
		cfg.MaxBodyBytes,
	)

	// probes handler
	probesHandler := httpapi.NewProbesHandler(
		readiness,
		logger,
	)

	// router
	router := httpapi.NewRouter(httpapi.RouterDependencies{
		Objects: objectsHandler,
		Probes:  probesHandler,
		Metrics: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	})

	// chain middleware
	handler := httpapi.Chain(
		router,
		httpapi.RecoveryMiddleware(logger),
		httpapi.RequestLoggingMiddleware(logger),
	)

	// server
	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	// ttl worker
	ttlWorker := storage.NewTTLWorker(
		store,
		cfg.CleanupInterval,
		logger,
		metrics,
	)

	readiness.SetReady(true)

	return &App{
		cfg:         cfg,
		logger:      logger,
		metrics:     metrics,
		store:       store,
		snapshotter: snapshotter,
		ttlWorker:   ttlWorker,
		server:      server,
		readiness:   readiness,
	}, nil
}

// timeNow вынесен в отдельную функцию, чтобы позже при необходимости упростить тестирование.
func timeNow() time.Time {
	return time.Now()
}
