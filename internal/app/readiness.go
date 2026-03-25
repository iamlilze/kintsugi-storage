package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

type namedCheck struct {
	name  string
	check func(ctx context.Context) error
}

// Пока started == false, CheckReady всегда возвращает ошибку.
type CompositeReadiness struct {
	started atomic.Bool
	mu      sync.RWMutex
	checks  []namedCheck
}

// AddCheck регистрирует проверку (вызывать до Start).
func (c *CompositeReadiness) AddCheck(name string, fn func(ctx context.Context) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, namedCheck{name: name, check: fn})
}

func (c *CompositeReadiness) Start() { c.started.Store(true) }

func (c *CompositeReadiness) Stop() { c.started.Store(false) }

// CheckReady реализует httpapi.ReadinessChecker.
func (c *CompositeReadiness) CheckReady(ctx context.Context) error {
	if !c.started.Load() {
		return fmt.Errorf("service is starting")
	}

	c.mu.RLock()
	checks := c.checks
	c.mu.RUnlock()

	var errs []error
	for _, nc := range checks {
		if err := nc.check(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", nc.name, err))
		}
	}
	return errors.Join(errs...)
}
