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

type TargetWithLastCheck struct {
	Target
	LastCheck *LastCheck
}

type NewTarget struct {
	Name            string
	URL             string
	IntervalSeconds int
}

type TargetUpdate struct {
	Name            *string
	URL             *string
	IntervalSeconds *int
	Paused          *bool
}
