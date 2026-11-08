package domain

import "time"

type Outcome string

const (
	OutcomeUp           Outcome = "up"
	OutcomeHTTPError    Outcome = "http_error"
	OutcomeTimeout      Outcome = "timeout"
	OutcomeNetworkError Outcome = "network_error"

	InitialOutcome = OutcomeUp
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

type LastCheck struct {
	Outcome   Outcome
	CheckedAt time.Time
	Duration  time.Duration
}

type SummaryPoint struct {
	CheckedAt time.Time
	Outcome   Outcome
}

type Summary struct {
	TargetID       int64
	Hours          int
	From           time.Time
	To             time.Time
	Up             int
	Other          int
	OtherByOutcome map[Outcome]int
	Points         []SummaryPoint
}
