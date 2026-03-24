package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Run запускает фоновые компоненты приложения и HTTP-сервер.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("application starting", "addr", a.cfg.HTTPAddr)

	serverErrCh := make(chan error, 1)
	workerErrCh := make(chan error, 1)
	snapshotErrCh := make(chan error, 1)

	go func() {
		a.logger.Info("http server started", "addr", a.cfg.HTTPAddr)
		err := a.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- fmt.Errorf("http server: %w", err)
			return
		}

		serverErrCh <- nil
	}()

	go func() {
		err := a.ttlWorker.Run(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			workerErrCh <- fmt.Errorf("ttl worker: %w", err)
			return
		}

		workerErrCh <- nil
	}()

	go func() {
		err := a.runSnapshotWorker(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			snapshotErrCh <- fmt.Errorf("snapshot worker: %w", err)
			return
		}

		snapshotErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		a.logger.Info("shutdown signal received", "reason", ctx.Err())
	case err := <-serverErrCh:
		if err != nil {
			return err
		}
	case err := <-workerErrCh:
		if err != nil {
			return err
		}
	case err := <-snapshotErrCh:
		if err != nil {
			return err
		}
	}

	a.readiness.SetReady(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	if err := a.saveSnapshot(); err != nil {
		return err
	}

	a.logger.Info("application stopped")
	return nil
}

func (a *App) runSnapshotWorker(ctx context.Context) error {
	if a.cfg.SnapshotInterval <= 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	ticker := time.NewTicker(a.cfg.SnapshotInterval)
	defer ticker.Stop()

	a.logger.Info("snapshot worker started", "interval", a.cfg.SnapshotInterval)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("snapshot worker stopped", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := a.saveSnapshot(); err != nil {
				return err
			}
		}
	}
}

// saveSnapshot собирает и сохраняет текущее состояние store на диск.
func (a *App) saveSnapshot() error {
	start := time.Now()

	snapshot := a.store.Snapshot(start)
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot for metrics: %w", err)
	}
	if err := a.snapshotter.Save(snapshot); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	duration := time.Since(start)

	a.metrics.ObserveSnapshotDuration(duration.Seconds())
	a.metrics.SetObjectsTotal(a.store.Len(timeNow()))
	a.metrics.SetSnapshotSizeBytes(len(data))

	a.logger.Info(
		"snapshot saved",
		"path", a.cfg.SnapshotPath,
		"objects_count", len(snapshot.Items),
		"size_bytes", len(data),
		"duration", duration,
	)

	return nil
}
