package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config хранит весь runtime-конфиг приложения.
type Config struct {
	HTTPAddr         string
	SnapshotPath     string
	CleanupInterval  time.Duration
	SnapshotInterval time.Duration
	ShutdownTimeout  time.Duration
	MaxBodyBytes     int64
	LogLevel         string
	LogFormat        string
}

// Config хранит весь runtime-конфиг приложения.
func MustLoad() (Config, error) {
	cfg := Config{
		HTTPAddr:         getEnv("HTTP_ADDR", ":8080"),
		SnapshotPath:     getEnv("SNAPSHOT_PATH", "./data/snapshot.json"),
		CleanupInterval:  getEnvDuration("CLEANUP_INTERVAL", 5*time.Second),
		SnapshotInterval: getEnvDuration("SNAPSHOT_INTERVAL", 30*time.Second),
		ShutdownTimeout:  getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		MaxBodyBytes:     getEnvInt64("MAX_BODY_BYTES", 1<<20),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		LogFormat:        getEnv("LOG_FORMAT", "text"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate проверяет, что конфиг содержит осмысленные значения.
func (c Config) Validate() error {
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("config: HTTP_ADDR must not be empty")
	}

	if strings.TrimSpace(c.SnapshotPath) == "" {
		return fmt.Errorf("config: SNAPSHOT_PATH must not be empty")
	}

	if c.CleanupInterval <= 0 {
		return fmt.Errorf("config: CLEANUP_INTERVAL must be greater than zero")
	}

	if c.SnapshotInterval < 0 {
		return fmt.Errorf("config: SNAPSHOT_INTERVAL must be greater than or equal to zero")
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("config: SHUTDOWN_TIMEOUT must be greater than zero")
	}

	if c.MaxBodyBytes <= 0 {
		return fmt.Errorf("config: MAX_BODY_BYTES must be greater than zero")
	}

	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: LOG_LEVEL must be one of debug, info, warn, error")
	}

	switch strings.ToLower(strings.TrimSpace(c.LogFormat)) {
	case "text", "json":
	default:
		return fmt.Errorf("config: LOG_FORMAT must be one of text, json")
	}

	return nil
}

// getEnv читает строковую переменную окружения с дефолтным значением.
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

// getEnvDuration читает duration из env или возвращает fallback.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw) // Парсим duration вида 5s, 1m, 250ms.
	if err != nil {
		return fallback
	}

	return value
}

// getEnvInt64 читает int64 из env или возвращает fallback.
func getEnvInt64(key string, fallback int64) int64 {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}

	return value
}
