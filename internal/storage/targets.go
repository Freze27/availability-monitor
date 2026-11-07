package storage

import (
	"context"
	"errors"
	"fmt"

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

func (s *Storage) ListTargets(ctx context.Context) ([]domain.Target, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+targetColumns+` FROM targets ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("select targets: %w", err)
	}
	defer rows.Close()

	targets := []domain.Target{}
	for rows.Next() {
		target, err := scanTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate targets: %w", err)
	}
	return targets, nil
}

func (s *Storage) UpdateTarget(ctx context.Context, id int64, u domain.TargetUpdate) (domain.Target, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE targets
		 SET name             = COALESCE($2, name),
		     url              = COALESCE($3, url),
		     interval_seconds = COALESCE($4, interval_seconds),
		     updated_at       = now()
		 WHERE id = $1::bigint
		 RETURNING `+targetColumns,
		id, u.Name, u.URL, u.IntervalSeconds,
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
