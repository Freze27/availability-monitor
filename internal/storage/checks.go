package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const checkColumns = "id, target_id, checked_at, outcome, status_code, duration_ms, error"

func (s *Storage) CreateCheck(ctx context.Context, c domain.Check) (domain.Check, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO checks (target_id, checked_at, outcome, status_code, duration_ms, error)
		 VALUES ($1::bigint, $2, $3, $4, $5, $6)
		 RETURNING `+checkColumns,
		c.TargetID, c.CheckedAt, c.Outcome, c.StatusCode, c.Duration.Milliseconds(), c.Error,
	)
	check, err := scanCheck(row)
	if err != nil {
		return domain.Check{}, fmt.Errorf("insert check: %w", err)
	}
	return check, nil
}

func (s *Storage) ListChecks(ctx context.Context, targetID int64, limit int) ([]domain.Check, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+checkColumns+`
		 FROM checks
		 WHERE target_id = $1::bigint
		 ORDER BY checked_at DESC, id DESC
		 LIMIT $2`,
		targetID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select checks of target %d: %w", targetID, err)
	}
	defer rows.Close()

	checks := []domain.Check{}
	for rows.Next() {
		check, err := scanCheck(rows)
		if err != nil {
			return nil, fmt.Errorf("scan check: %w", err)
		}
		checks = append(checks, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checks: %w", err)
	}
	return checks, nil
}

func scanCheck(row pgx.Row) (domain.Check, error) {
	var c domain.Check
	var durationMS int64
	err := row.Scan(&c.ID, &c.TargetID, &c.CheckedAt, &c.Outcome, &c.StatusCode, &durationMS, &c.Error)
	if err != nil {
		return domain.Check{}, err
	}
	c.Duration = time.Duration(durationMS) * time.Millisecond
	return c, nil
}
