package service

import (
	"context"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type EventRepository interface {
	ListEvents(ctx context.Context, limit int) ([]domain.Event, error)
}

type EventService struct {
	repo EventRepository
}

func NewEventService(repo EventRepository) *EventService {
	return &EventService{repo: repo}
}

func (s *EventService) ListEvents(ctx context.Context, limit *int) ([]domain.Event, error) {
	n, err := resolveLimit(limit)
	if err != nil {
		return nil, err
	}
	return s.repo.ListEvents(ctx, n)
}
