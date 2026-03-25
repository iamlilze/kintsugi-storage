package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr         string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
	MaxBodyBytes     int64
	SnapshotPath     string
	CleanupInterval  time.Duration
	SnapshotInterval time.Duration
	LogLevel         string
	LogFormat        string
}

func Load() (Config, error) {
	var errs []error

	httpAddr := getEnv("HTTP_ADDR", ":8080")

	readTimeout, err := getEnvDuration("READ_TIMEOUT", 10*time.Second)
	if err != nil {
		errs = append(errs, err)
	}

	writeTimeout, err := getEnvDuration("WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		errs = append(errs, err)
	}

	idleTimeout, err := getEnvDuration("IDLE_TIMEOUT", 120*time.Second)
	if err != nil {
		errs = append(errs, err)
	}

	shutdownTimeout, err := getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		errs = append(errs, err)
	}

	maxBodyBytes, err := getEnvInt64("MAX_BODY_BYTES", 1<<20)
	if err != nil {
		errs = append(errs, err)
	}

	snapshotPath := getEnv("SNAPSHOT_PATH", "./data/snapshot.json")

	cleanupInterval, err := getEnvDuration("CLEANUP_INTERVAL", 5*time.Second)
	if err != nil {
		errs = append(errs, err)
	}

	snapshotInterval, err := getEnvDuration("SNAPSHOT_INTERVAL", 30*time.Second)
	if err != nil {
		errs = append(errs, err)
	}

	logLevel := getEnv("LOG_LEVEL", "info")
	logFormat := getEnv("LOG_FORMAT", "text")

	cfg := Config{
		HTTPAddr:         httpAddr,
		ReadTimeout:      readTimeout,
		WriteTimeout:     writeTimeout,
		IdleTimeout:      idleTimeout,
		ShutdownTimeout:  shutdownTimeout,
		MaxBodyBytes:     maxBodyBytes,
		SnapshotPath:     snapshotPath,
		CleanupInterval:  cleanupInterval,
		SnapshotInterval: snapshotInterval,
		LogLevel:         logLevel,
		LogFormat:        logFormat,
	}

	if vErr := cfg.validate(); vErr != nil {
		errs = append(errs, vErr)
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error

	if strings.TrimSpace(c.HTTPAddr) == "" {
		errs = append(errs, fmt.Errorf("config: HTTP_ADDR must not be empty"))
	}
	if strings.TrimSpace(c.SnapshotPath) == "" {
		errs = append(errs, fmt.Errorf("config: SNAPSHOT_PATH must not be empty"))
	}
	if c.CleanupInterval <= 0 {
		errs = append(errs, fmt.Errorf("config: CLEANUP_INTERVAL must be positive"))
	}
	if c.SnapshotInterval < 0 {
		errs = append(errs, fmt.Errorf("config: SNAPSHOT_INTERVAL must be >= 0"))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("config: SHUTDOWN_TIMEOUT must be positive"))
	}
	if c.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("config: READ_TIMEOUT must be positive"))
	}
	if c.WriteTimeout <= 0 {
		errs = append(errs, fmt.Errorf("config: WRITE_TIMEOUT must be positive"))
	}
	if c.IdleTimeout <= 0 {
		errs = append(errs, fmt.Errorf("config: IDLE_TIMEOUT must be positive"))
	}
	if c.MaxBodyBytes <= 0 {
		errs = append(errs, fmt.Errorf("config: MAX_BODY_BYTES must be positive"))
	}

	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("config: LOG_LEVEL must be one of debug, info, warn, error"))
	}

	switch strings.ToLower(strings.TrimSpace(c.LogFormat)) {
	case "text", "json":
	default:
		errs = append(errs, fmt.Errorf("config: LOG_FORMAT must be one of text, json"))
	}

	return errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s: invalid duration %q: %w", key, raw, err)
	}
	return value, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s: invalid integer %q: %w", key, raw, err)
	}
	return value, nil
}
