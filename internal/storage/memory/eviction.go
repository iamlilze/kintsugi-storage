package memory

import (
	"context"
	"log/slog"
	"time"
)

type EvictionMetrics interface {
	AddExpiredDeletions(n int)
	SetObjectsTotal(n int)
}

type nopEvictionMetrics struct{}

func (nopEvictionMetrics) AddExpiredDeletions(int) {}
func (nopEvictionMetrics) SetObjectsTotal(int)     {}

type EvictionWorker struct {
	store    *Store
	interval time.Duration
	logger   *slog.Logger
	metrics  EvictionMetrics
}

func NewEvictionWorker(store *Store, interval time.Duration, logger *slog.Logger, metrics EvictionMetrics) *EvictionWorker {
	if logger == nil {
		logger = slog.Default()
	}
	if metrics == nil {
		metrics = nopEvictionMetrics{}
	}
	return &EvictionWorker{
		store:    store,
		interval: interval,
		logger:   logger,
		metrics:  metrics,
	}
}

func (w *EvictionWorker) Run(ctx context.Context) error {
	if w.interval <= 0 {
		w.logger.Warn("eviction worker disabled", "interval", w.interval)
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("eviction worker started", "interval", w.interval)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("eviction worker stopped")
			return nil
		case <-ticker.C:
			deleted := w.store.DeleteExpired()

			if deleted > 0 {
				w.metrics.AddExpiredDeletions(deleted)
				w.logger.Info("expired objects deleted", "count", deleted)
			}

			n, _ := w.store.Len(ctx)
			w.metrics.SetObjectsTotal(n)
		}
	}
}
