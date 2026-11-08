package service

import (
	"context"
	"fmt"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const (
	defaultChecksLimit = 100
	maxChecksLimit     = 1000
)

type CheckRepository interface {
	GetTarget(ctx context.Context, id int64) (domain.Target, error)
	CreateCheck(ctx context.Context, c domain.Check) (domain.Check, error)
	ListChecks(ctx context.Context, targetID int64, limit int) ([]domain.Check, error)
}

type Checker interface {
	Check(ctx context.Context, url string) (domain.Check, error)
}

type CheckService struct {
	repo    CheckRepository
	checker Checker
}

func NewCheckService(repo CheckRepository, checker Checker) *CheckService {
	return &CheckService{repo: repo, checker: checker}
}

func (s *CheckService) RunCheck(ctx context.Context, targetID int64) (domain.Check, error) {
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
	n := defaultChecksLimit
	if limit != nil {
		if *limit < 1 || *limit > maxChecksLimit {
			return nil, &domain.ValidationError{Fields: map[string]string{
				"limit": fmt.Sprintf("must be between 1 and %d", maxChecksLimit),
			}}
		}
		n = *limit
	}

	if _, err := s.repo.GetTarget(ctx, targetID); err != nil {
		return nil, err
	}
	return s.repo.ListChecks(ctx, targetID, n)
}
