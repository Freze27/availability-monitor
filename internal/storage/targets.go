package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const targetColumns = "id, name, url, interval_seconds, paused, created_at, updated_at"

func (s *Storage) CreateTarget(ctx context.Context, t domain.NewTarget) (domain.Target, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO targets (name, url, interval_seconds)
		 VALUES ($1, $2, $3)
		 RETURNING `+targetColumns,
		t.Name, t.URL, t.IntervalSeconds,
	)
	target, err := scanTarget(row)
	if err != nil {
		return domain.Target{}, fmt.Errorf("insert target: %w", err)
	}
	return target, nil
}

func (s *Storage) GetTarget(ctx context.Context, id int64) (domain.Target, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+targetColumns+` FROM targets WHERE id = $1::bigint`,
		id,
	)
	target, err := scanTarget(row)
	if err != nil {
		return domain.Target{}, fmt.Errorf("select target %d: %w", id, err)
	}
	return target, nil
}

func (s *Storage) ListTargets(ctx context.Context) ([]domain.TargetWithLastCheck, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+targetColumns+`, lc.outcome, lc.checked_at, lc.duration_ms
		 FROM targets t
		 LEFT JOIN LATERAL (
		     SELECT outcome, checked_at, duration_ms
		     FROM checks
		     WHERE target_id = t.id
		     ORDER BY checked_at DESC, id DESC
		     LIMIT 1
		 ) lc ON true
		 ORDER BY t.id`,
	)
	if err != nil {
		return nil, fmt.Errorf("select targets: %w", err)
	}
	defer rows.Close()

	targets := []domain.TargetWithLastCheck{}
	for rows.Next() {
		var t domain.TargetWithLastCheck
		var outcome *domain.Outcome
		var checkedAt *time.Time
		var durationMS *int64
		err := rows.Scan(&t.ID, &t.Name, &t.URL, &t.IntervalSeconds, &t.Paused, &t.CreatedAt, &t.UpdatedAt,
			&outcome, &checkedAt, &durationMS)
		if err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		if outcome != nil {
			t.LastCheck = &domain.LastCheck{
				Outcome:   *outcome,
				CheckedAt: *checkedAt,
				Duration:  time.Duration(*durationMS) * time.Millisecond,
			}
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate targets: %w", err)
	}
	return targets, nil
}

func (s *Storage) ListDueTargets(ctx context.Context, dueBy time.Time, limit int) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id
		 FROM targets t
		 LEFT JOIN LATERAL (
		     SELECT checked_at
		     FROM checks
		     WHERE target_id = t.id
		     ORDER BY checked_at DESC
		     LIMIT 1
		 ) last ON true
		 WHERE NOT t.paused
		   AND (last.checked_at IS NULL
		        OR last.checked_at + t.interval_seconds * interval '1 second' <= $1)
		 ORDER BY last.checked_at + t.interval_seconds * interval '1 second' NULLS FIRST, t.id
		 LIMIT $2`,
		dueBy, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select due targets: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("scan due targets: %w", err)
	}
	return ids, nil
}

func (s *Storage) UpdateTarget(ctx context.Context, id int64, u domain.TargetUpdate) (domain.Target, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE targets
		 SET name             = COALESCE($2, name),
		     url              = COALESCE($3, url),
		     interval_seconds = COALESCE($4, interval_seconds),
		     paused           = COALESCE($5, paused),
		     updated_at       = now()
		 WHERE id = $1::bigint
		 RETURNING `+targetColumns,
		id, u.Name, u.URL, u.IntervalSeconds, u.Paused,
	)
	target, err := scanTarget(row)
	if err != nil {
		return domain.Target{}, fmt.Errorf("update target %d: %w", id, err)
	}
	return target, nil
}

func (s *Storage) DeleteTarget(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM targets WHERE id = $1::bigint`, id)
	if err != nil {
		return fmt.Errorf("delete target %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete target %d: %w", id, domain.ErrTargetNotFound)
	}
	return nil
}

func scanTarget(row pgx.Row) (domain.Target, error) {
	var t domain.Target
	err := row.Scan(&t.ID, &t.Name, &t.URL, &t.IntervalSeconds, &t.Paused, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Target{}, domain.ErrTargetNotFound
	}
	return t, err
}
