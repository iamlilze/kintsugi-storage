package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"kintsugi-storage/internal/config"
	"kintsugi-storage/internal/httpapi"
	"kintsugi-storage/internal/httpapi/middleware"
	"kintsugi-storage/internal/observability"
	"kintsugi-storage/internal/storage"
	"kintsugi-storage/internal/storage/memory"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type App struct {
	cfg       config.Config
	logger    *slog.Logger
	accessLog *slog.Logger
	metrics   *observability.Metrics
	store     storage.Store
	server    *http.Server
	readiness *CompositeReadiness

	// memory-specific (nil для других бэкендов)
	memStore    *memory.Store
	eviction    *memory.EvictionWorker
	snapshotter *memory.FileSnapshotter
}

func New(cfg config.Config) (*App, error) {
	logCfg := observability.LoggerConfig{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	}
	logger := observability.NewLogger(logCfg)
	accessLog := observability.NewAccessLogger(logCfg)

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	metrics := observability.NewMetrics(registry)

	memStore := memory.New()
	var store storage.Store = memStore

	snapshotter := memory.NewFileSnapshotter(cfg.SnapshotPath)
	snapshot, err := snapshotter.Load()
	if err != nil {
		return nil, fmt.Errorf("app: load snapshot: %w", err)
	}
	if err := memStore.Restore(snapshot); err != nil {
		return nil, fmt.Errorf("app: restore snapshot: %w", err)
	}

	n, _ := store.Len(context.Background())
	metrics.SetObjectsTotal(n)

	eviction := memory.NewEvictionWorker(memStore, cfg.CleanupInterval, logger, metrics)

	readiness := &CompositeReadiness{}
	readiness.AddCheck("store", store.Ping)

	objectsHandler := httpapi.NewObjectsHandler(store, logger, metrics, cfg.MaxBodyBytes)
	probesHandler := httpapi.NewProbesHandler(readiness, logger)

	router := httpapi.NewRouter(httpapi.RouterDependencies{
		Objects: objectsHandler,
		Probes:  probesHandler,
		Metrics: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	})

	handler := middleware.Chain(
		router,
		middleware.Recovery(logger),
		middleware.Logging(accessLog),
		middleware.RequestID,
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	return &App{
		cfg:         cfg,
		logger:      logger,
		accessLog:   accessLog,
		metrics:     metrics,
		store:       store,
		server:      server,
		readiness:   readiness,
		memStore:    memStore,
		eviction:    eviction,
		snapshotter: snapshotter,
	}, nil
}
