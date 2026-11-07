package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	CheckTimeout  time.Duration
	SchedulerTick time.Duration
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	checkTimeout, err := getDuration("CHECK_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	schedulerTick, err := getDuration("SCHEDULER_TICK", time.Second)
	if err != nil {
		return Config{}, err
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return Config{
		HTTPAddr:      getEnv("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:   databaseURL,
		CheckTimeout:  checkTimeout,
		SchedulerTick: schedulerTick,
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %s", key, v)
	}
	return d, nil
}
