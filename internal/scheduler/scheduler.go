package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type DueTargets interface {
	ListDueTargets(ctx context.Context, dueBy time.Time, limit int) ([]int64, error)
}

type Checks interface {
	TryReserve(targetID int64) bool
	Release(targetID int64)
	RunReserved(ctx context.Context, targetID int64) (domain.Check, error)
}

type Scheduler struct {
	targets DueTargets
	checks  Checks
	tick    time.Duration
	workers int
	logger  *slog.Logger
}

func New(targets DueTargets, checks Checks, tick time.Duration, workers int, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		targets: targets,
		checks:  checks,
		tick:    tick,
		workers: workers,
		logger:  logger,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	queue := make(chan int64, s.workers)

	var wg sync.WaitGroup
	for range s.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.work(ctx, queue)
		}()
	}
	s.logger.Info("scheduler started", "workers", s.workers, "tick", s.tick.String())

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	s.dispatch(ctx, queue)
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			s.releaseQueued(queue)
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.dispatch(ctx, queue)
		}
	}
}

func (s *Scheduler) dispatch(ctx context.Context, queue chan<- int64) {
	free := cap(queue) - len(queue)
	if free == 0 {
		return
	}

	ids, err := s.targets.ListDueTargets(ctx, time.Now().Add(s.tick), free+2*s.workers)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("list due targets", "error", err)
		}
		return
	}

	for _, id := range ids {
		if free == 0 {
			return
		}
		if !s.checks.TryReserve(id) {
			continue
		}
		select {
		case queue <- id:
			free--
		default:
			s.checks.Release(id)
			return
		}
	}
}

func (s *Scheduler) work(ctx context.Context, queue <-chan int64) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-queue:
			if ctx.Err() != nil {
				s.checks.Release(id)
				return
			}
			s.check(ctx, id)
		}
	}
}

func (s *Scheduler) check(ctx context.Context, id int64) {
	defer func() {
		if rec := recover(); rec != nil {
			s.logger.Error("background check panicked",
				"target_id", id,
				"panic", rec,
				"stack", string(debug.Stack()),
			)
		}
	}()

	result, err := s.checks.RunReserved(context.WithoutCancel(ctx), id)
	switch {
	case errors.Is(err, domain.ErrTargetNotFound):
		return
	case err != nil:
		s.logger.Error("background check failed", "target_id", id, "error", err)
		return
	}

	attrs := []any{
		"target_id", id,
		"outcome", result.Outcome,
		"duration_ms", result.Duration.Milliseconds(),
	}
	if result.StatusCode != nil {
		attrs = append(attrs, "status_code", *result.StatusCode)
	}
	if result.Error != nil {
		attrs = append(attrs, "error", *result.Error)
	}
	s.logger.Info("background check", attrs...)
}

func (s *Scheduler) releaseQueued(queue chan int64) {
	for {
		select {
		case id := <-queue:
			s.checks.Release(id)
		default:
			return
		}
	}
}
