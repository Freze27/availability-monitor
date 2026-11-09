package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDatabaseURL = "postgres://user:pass@localhost:5432/db"

func setEnv(t *testing.T, env map[string]string) {
	vars := map[string]string{
		"HTTP_ADDR":      "",
		"DATABASE_URL":   testDatabaseURL,
		"CHECK_TIMEOUT":  "",
		"SCHEDULER_TICK": "",
		"CHECK_WORKERS":  "",
	}
	for k, v := range env {
		vars[k] = v
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad_WhenOnlyDatabaseURLSet_ShouldUseDefaults(t *testing.T) {
	setEnv(t, nil)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, Config{
		HTTPAddr:      "127.0.0.1:8080",
		DatabaseURL:   testDatabaseURL,
		CheckTimeout:  5 * time.Second,
		SchedulerTick: time.Second,
		CheckWorkers:  50,
	}, cfg)
}

func TestLoad_WhenAllValuesSet_ShouldUseThem(t *testing.T) {
	setEnv(t, map[string]string{
		"HTTP_ADDR":      "0.0.0.0:9000",
		"CHECK_TIMEOUT":  "2s",
		"SCHEDULER_TICK": "5s",
		"CHECK_WORKERS":  "10",
	})

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, Config{
		HTTPAddr:      "0.0.0.0:9000",
		DatabaseURL:   testDatabaseURL,
		CheckTimeout:  2 * time.Second,
		SchedulerTick: 5 * time.Second,
		CheckWorkers:  10,
	}, cfg)
}

func TestLoad_WhenBoundaryValuesSet_ShouldAccept(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"WhenTickIsMinimum_ShouldAccept", map[string]string{"SCHEDULER_TICK": "1s"}},
		{"WhenWorkersIsOne_ShouldAccept", map[string]string{"CHECK_WORKERS": "1"}},
		{"WhenWorkersIsMaximum_ShouldAccept", map[string]string{"CHECK_WORKERS": "1000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			_, err := Load()

			assert.NoError(t, err)
		})
	}
}

func TestLoad_WhenValueInvalid_ShouldReturnClearError(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		message string
	}{
		{"WhenDatabaseURLMissing_ShouldRequireIt", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL is required"},
		{"WhenTimeoutNotDuration_ShouldFailParse", map[string]string{"CHECK_TIMEOUT": "abc"}, "parse CHECK_TIMEOUT"},
		{"WhenTimeoutNegative_ShouldRequirePositive", map[string]string{"CHECK_TIMEOUT": "-1s"}, "CHECK_TIMEOUT must be positive"},
		{"WhenTickBelowMinimum_ShouldRequireOneSecond", map[string]string{"SCHEDULER_TICK": "500ms"}, "SCHEDULER_TICK must be at least 1s"},
		{"WhenTickZero_ShouldRequirePositive", map[string]string{"SCHEDULER_TICK": "0s"}, "SCHEDULER_TICK must be positive"},
		{"WhenWorkersNotInteger_ShouldFailParse", map[string]string{"CHECK_WORKERS": "1.5"}, "parse CHECK_WORKERS"},
		{"WhenWorkersZero_ShouldRequirePositive", map[string]string{"CHECK_WORKERS": "0"}, "CHECK_WORKERS must be positive"},
		{"WhenWorkersAboveMaximum_ShouldLimitTo1000", map[string]string{"CHECK_WORKERS": "1001"}, "CHECK_WORKERS must be at most 1000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			_, err := Load()

			assert.ErrorContains(t, err, tt.message)
		})
	}
}
