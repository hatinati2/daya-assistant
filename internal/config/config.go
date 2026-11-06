package config

import (
	"os"
	"time"
)

type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

func Load() Config {
	return Config{
		Addr:              envOr("DAYA_ADDR", ":8100"),
		ReadHeaderTimeout: durationOr("DAYA_READ_HEADER_TIMEOUT", 5*time.Second),
		ShutdownTimeout:   durationOr("DAYA_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
