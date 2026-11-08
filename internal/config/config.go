package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	CheckTimeout  time.Duration
	SchedulerTick time.Duration
	CheckWorkers  int
}

const (
	minSchedulerTick = time.Second
	maxCheckWorkers  = 1000
)

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
	if schedulerTick < minSchedulerTick {
		return Config{}, fmt.Errorf("SCHEDULER_TICK must be at least %s, got %s", minSchedulerTick, schedulerTick)
	}
	checkWorkers, err := getPositiveInt("CHECK_WORKERS", 50)
	if err != nil {
		return Config{}, err
	}
	if checkWorkers > maxCheckWorkers {
		return Config{}, fmt.Errorf("CHECK_WORKERS must be at most %d, got %d", maxCheckWorkers, checkWorkers)
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
		CheckWorkers:  checkWorkers,
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getPositiveInt(key string, fallback int) (int, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %s", key, v)
	}
	return n, nil
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
