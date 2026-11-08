package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const checkColumns = "id, target_id, checked_at, outcome, status_code, duration_ms, error"

func (s *Storage) CreateCheck(ctx context.Context, c domain.Check) (domain.Check, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Check{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var lockedID int64
	err = tx.QueryRow(ctx, `SELECT id FROM targets WHERE id = $1::bigint FOR UPDATE`, c.TargetID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Check{}, fmt.Errorf("lock target %d: %w", c.TargetID, domain.ErrTargetNotFound)
	}
	if err != nil {
		return domain.Check{}, fmt.Errorf("lock target %d: %w", c.TargetID, err)
	}

	prev := domain.InitialOutcome
	err = tx.QueryRow(ctx,
		`SELECT outcome FROM checks
		 WHERE target_id = $1::bigint
		 ORDER BY id DESC
		 LIMIT 1`,
		c.TargetID,
	).Scan(&prev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Check{}, fmt.Errorf("select previous outcome: %w", err)
	}

	check, err := scanCheck(tx.QueryRow(ctx,
		`INSERT INTO checks (target_id, checked_at, outcome, status_code, duration_ms, error)
		 VALUES ($1::bigint, $2, $3, $4, $5, $6)
		 RETURNING `+checkColumns,
		c.TargetID, c.CheckedAt, c.Outcome, c.StatusCode, c.Duration.Milliseconds(), c.Error,
	))
	if err != nil {
		return domain.Check{}, fmt.Errorf("insert check: %w", err)
	}

	if kind, ok := domain.TransitionEvent(prev, check.Outcome); ok {
		_, err = tx.Exec(ctx,
			`INSERT INTO events (target_id, kind, occurred_at) VALUES ($1::bigint, $2, $3)`,
			check.TargetID, kind, check.CheckedAt,
		)
		if err != nil {
			return domain.Check{}, fmt.Errorf("insert event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Check{}, fmt.Errorf("commit check: %w", err)
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
