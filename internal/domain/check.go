package domain

import "time"

type Outcome string

const (
	OutcomeUp           Outcome = "up"
	OutcomeHTTPError    Outcome = "http_error"
	OutcomeTimeout      Outcome = "timeout"
	OutcomeNetworkError Outcome = "network_error"
)

type Check struct {
	ID         int64
	TargetID   int64
	CheckedAt  time.Time
	Outcome    Outcome
	StatusCode *int
	Duration   time.Duration
	Error      *string
}
