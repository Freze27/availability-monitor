package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const (
	defaultSummaryHours = 24
	maxSummaryHours     = 7 * 24
)

type CheckRepository interface {
	GetTarget(ctx context.Context, id int64) (domain.Target, error)
	CreateCheck(ctx context.Context, c domain.Check) (domain.Check, error)
	ListChecks(ctx context.Context, targetID int64, limit int) ([]domain.Check, error)
	ListSummaryPoints(ctx context.Context, targetID int64, from, to time.Time) ([]domain.SummaryPoint, error)
}

type Checker interface {
	Check(ctx context.Context, url string) (domain.Check, error)
}

type CheckService struct {
	repo    CheckRepository
	checker Checker
	busy    *busySet
}

func NewCheckService(repo CheckRepository, checker Checker) *CheckService {
	return &CheckService{repo: repo, checker: checker, busy: newBusySet()}
}

func (s *CheckService) RunCheck(ctx context.Context, targetID int64) (domain.Check, error) {
	if !s.busy.tryAcquire(targetID) {
		return domain.Check{}, domain.ErrCheckInProgress
	}
	defer s.busy.release(targetID)

	return s.run(ctx, targetID)
}

func (s *CheckService) TryReserve(targetID int64) bool {
	return s.busy.tryAcquire(targetID)
}

func (s *CheckService) Release(targetID int64) {
	s.busy.release(targetID)
}

func (s *CheckService) RunReserved(ctx context.Context, targetID int64) (domain.Check, error) {
	defer s.busy.release(targetID)

	return s.run(ctx, targetID)
}

func (s *CheckService) run(ctx context.Context, targetID int64) (domain.Check, error) {
	target, err := s.repo.GetTarget(ctx, targetID)
	if err != nil {
		return domain.Check{}, err
	}

	result, err := s.checker.Check(ctx, target.URL)
	if err != nil {
		return domain.Check{}, err
	}
	result.TargetID = target.ID

	return s.repo.CreateCheck(context.WithoutCancel(ctx), result)
}

func (s *CheckService) ListChecks(ctx context.Context, targetID int64, limit *int) ([]domain.Check, error) {
	n, err := resolveLimit(limit)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.GetTarget(ctx, targetID); err != nil {
		return nil, err
	}
	return s.repo.ListChecks(ctx, targetID, n)
}

func (s *CheckService) Summary(ctx context.Context, targetID int64, hours *int) (domain.Summary, error) {
	h := defaultSummaryHours
	if hours != nil {
		if *hours < 1 || *hours > maxSummaryHours {
			return domain.Summary{}, &domain.ValidationError{Fields: map[string]string{
				"hours": fmt.Sprintf("must be between 1 and %d", maxSummaryHours),
			}}
		}
		h = *hours
	}

	if _, err := s.repo.GetTarget(ctx, targetID); err != nil {
		return domain.Summary{}, err
	}

	to := time.Now().UTC()
	from := to.Add(-time.Duration(h) * time.Hour)
	points, err := s.repo.ListSummaryPoints(ctx, targetID, from, to)
	if err != nil {
		return domain.Summary{}, err
	}

	summary := domain.Summary{
		TargetID: targetID,
		Hours:    h,
		From:     from,
		To:       to,
		OtherByOutcome: map[domain.Outcome]int{
			domain.OutcomeHTTPError:    0,
			domain.OutcomeTimeout:      0,
			domain.OutcomeNetworkError: 0,
		},
		Points: points,
	}
	for _, p := range points {
		if p.Outcome == domain.OutcomeUp {
			summary.Up++
		} else {
			summary.Other++
			summary.OtherByOutcome[p.Outcome]++
		}
	}
	return summary, nil
}
