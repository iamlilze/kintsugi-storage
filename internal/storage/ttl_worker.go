package storage

import (
	"context"
	"log/slog"
	"time"
)

// ExpiredDeleter - узкий интерфейс потребности worker'а.
type ExpiredDeleter interface {
	DeleteExpired(now time.Time) int
	Len(now time.Time) int
}

// ExpirationMetrics - еще один узкий интерфейс потребности worker'а.
type ExpirationMetrics interface {
	AddExpiredDeletions(n int)
	SetObjectsTotal(n int)
}

// TTLWorker - фоновый процесс, который периодически чистит просроченные объекты.
type TTLWorker struct {
	store    ExpiredDeleter
	interval time.Duration
	logger   *slog.Logger
	metrics  ExpirationMetrics
}

// NewTTLWorker создает worker и сразу нормализует зависимости.
func NewTTLWorker(
	store ExpiredDeleter,
	interval time.Duration,
	logger *slog.Logger,
	metrics ExpirationMetrics,
) *TTLWorker {

	if logger == nil {
		logger = slog.Default()
	}

	return &TTLWorker{
		store:    store,
		interval: interval,
		logger:   logger,
		metrics:  metrics,
	}
}

// Run запускает бесконечный цикл очистки, пока не будет отменен context.
func (w *TTLWorker) Run(ctx context.Context) error {
	if w.interval <= 0 {
		w.logger.Warn("ttl worker disabled because interval is non-positive", "interval", w.interval)
		<-ctx.Done()
		return ctx.Err()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("ttl worker started", "interval", w.interval)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("ttl worker stopped", "reason", ctx.Err())
			return ctx.Err()
		case tickTime := <-ticker.C:
			deleted := w.store.DeleteExpired(tickTime)

			if w.metrics != nil && deleted > 0 {
				w.metrics.AddExpiredDeletions(deleted)
			}

			if w.metrics != nil {
				w.metrics.SetObjectsTotal(w.store.Len(tickTime))
			}

			if deleted > 0 {
				w.logger.Info("expired objects deleted", "count", deleted, "at", tickTime)
			}
		}
	}
}
