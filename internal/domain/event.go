package domain

import "time"

type EventKind string

const (
	EventUp   EventKind = "up"
	EventDown EventKind = "down"
)

type Event struct {
	ID         int64
	TargetID   int64
	Kind       EventKind
	OccurredAt time.Time
}

func TransitionEvent(prev, curr Outcome) (EventKind, bool) {
	switch {
	case prev == OutcomeUp && curr != OutcomeUp:
		return EventDown, true
	case prev != OutcomeUp && curr == OutcomeUp:
		return EventUp, true
	}
	return "", false
}
