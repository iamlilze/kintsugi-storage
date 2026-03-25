package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

func (a *App) Run(ctx context.Context) error {
	a.logger.Info("application starting", "addr", a.cfg.HTTPAddr)

	ln, err := net.Listen("tcp", a.cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	a.readiness.Start()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		a.logger.Info("http server started", "addr", a.cfg.HTTPAddr)
		err := a.server.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	g.Go(func() error {
		<-gCtx.Done()
		a.readiness.Stop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer shutdownCancel()
		return a.server.Shutdown(shutdownCtx)
	})

	if a.eviction != nil {
		g.Go(func() error {
			return a.eviction.Run(gCtx)
		})
	}

	if a.snapshotter != nil && a.memStore != nil {
		g.Go(func() error {
			return a.runSnapshotWorker(gCtx)
		})
	}

	err = g.Wait()

	if a.memStore != nil && a.snapshotter != nil {
		if saveErr := a.saveSnapshot(); saveErr != nil {
			a.logger.Error("final snapshot failed", "error", saveErr)
			if err == nil {
				err = saveErr
			}
		}
	}

	if closeErr := a.store.Close(); closeErr != nil {
		a.logger.Error("store close failed", "error", closeErr)
	}

	a.logger.Info("application stopped")
	return err
}

func (a *App) runSnapshotWorker(ctx context.Context) error {
	if a.cfg.SnapshotInterval <= 0 {
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(a.cfg.SnapshotInterval)
	defer ticker.Stop()

	a.logger.Info("snapshot worker started", "interval", a.cfg.SnapshotInterval)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("snapshot worker stopped")
			return nil
		case <-ticker.C:
			if err := a.saveSnapshot(); err != nil {
				return err
			}
		}
	}
}

func (a *App) saveSnapshot() error {
	start := time.Now()

	snapshot := a.memStore.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := a.snapshotter.Save(snapshot); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	duration := time.Since(start)

	a.metrics.ObserveSnapshotDuration(duration.Seconds())
	n, _ := a.store.Len(context.Background())
	a.metrics.SetObjectsTotal(n)
	a.metrics.SetSnapshotSizeBytes(len(data))

	a.logger.Info("snapshot saved",
		"path", a.cfg.SnapshotPath,
		"objects_count", len(snapshot.Items),
		"size_bytes", len(data),
		"duration", duration,
	)
	return nil
}
