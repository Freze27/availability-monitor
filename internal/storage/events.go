package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Freze27/availability-monitor/internal/domain"
)

func (s *Storage) ListEvents(ctx context.Context, limit int) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, target_id, kind, occurred_at
		 FROM events
		 ORDER BY occurred_at DESC, id DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select events: %w", err)
	}

	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Event, error) {
		var e domain.Event
		err := row.Scan(&e.ID, &e.TargetID, &e.Kind, &e.OccurredAt)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan events: %w", err)
	}
	return events, nil
}
