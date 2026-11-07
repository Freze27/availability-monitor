package service

import (
	"context"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type CheckRepository interface {
	GetTarget(ctx context.Context, id int64) (domain.Target, error)
	CreateCheck(ctx context.Context, c domain.Check) (domain.Check, error)
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
