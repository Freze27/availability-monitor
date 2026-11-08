package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Freze27/availability-monitor/internal/checker"
	"github.com/Freze27/availability-monitor/internal/config"
	"github.com/Freze27/availability-monitor/internal/httpapi"
	"github.com/Freze27/availability-monitor/internal/scheduler"
	"github.com/Freze27/availability-monitor/internal/service"
	"github.com/Freze27/availability-monitor/internal/storage"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
	dbInitTimeout     = 30 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.CheckTimeout >= writeTimeout {
		return fmt.Errorf("CHECK_TIMEOUT must be less than %s, got %s", writeTimeout, cfg.CheckTimeout)
	}

	dbCtx, cancelDB := context.WithTimeout(ctx, dbInitTimeout)
	defer cancelDB()

	store, err := storage.New(dbCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer store.Close()
	logger.Info("database connected")

	applied, err := store.Migrate(dbCtx)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	logger.Info("migrations applied", "count", applied)

	checks := service.NewCheckService(store, checker.New(cfg.CheckTimeout))
	router := httpapi.NewRouter(service.NewTargetService(store), checks, service.NewEventService(store), logger)

	srv := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
	}
	logger.Info("server started", "addr", ln.Addr().String())

	sched := scheduler.New(store, checks, cfg.SchedulerTick, cfg.CheckWorkers, logger)
	schedCtx, stopScheduler := context.WithCancel(ctx)
	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		sched.Run(schedCtx)
	}()
	defer func() {
		stopScheduler()
		<-schedDone
	}()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve http: %w", err)
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	stop()

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	logger.Info("server stopped")
	return nil
}
