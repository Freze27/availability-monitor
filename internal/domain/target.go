package domain

import "time"

type Target struct {
	ID              int64
	Name            string
	URL             string
	IntervalSeconds int
	Paused          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
