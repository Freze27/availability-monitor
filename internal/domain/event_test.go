package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransitionEvent(t *testing.T) {
	tests := []struct {
		name      string
		prev      Outcome
		curr      Outcome
		wantKind  EventKind
		wantEvent bool
	}{
		{"WhenUpToUp_ShouldReturnNoEvent", OutcomeUp, OutcomeUp, "", false},
		{"WhenUpToHTTPError_ShouldReturnDown", OutcomeUp, OutcomeHTTPError, EventDown, true},
		{"WhenUpToTimeout_ShouldReturnDown", OutcomeUp, OutcomeTimeout, EventDown, true},
		{"WhenUpToNetworkError_ShouldReturnDown", OutcomeUp, OutcomeNetworkError, EventDown, true},
		{"WhenHTTPErrorToUp_ShouldReturnUp", OutcomeHTTPError, OutcomeUp, EventUp, true},
		{"WhenTimeoutToUp_ShouldReturnUp", OutcomeTimeout, OutcomeUp, EventUp, true},
		{"WhenNetworkErrorToUp_ShouldReturnUp", OutcomeNetworkError, OutcomeUp, EventUp, true},
		{"WhenHTTPErrorToHTTPError_ShouldReturnNoEvent", OutcomeHTTPError, OutcomeHTTPError, "", false},
		{"WhenHTTPErrorToTimeout_ShouldReturnNoEvent", OutcomeHTTPError, OutcomeTimeout, "", false},
		{"WhenHTTPErrorToNetworkError_ShouldReturnNoEvent", OutcomeHTTPError, OutcomeNetworkError, "", false},
		{"WhenTimeoutToHTTPError_ShouldReturnNoEvent", OutcomeTimeout, OutcomeHTTPError, "", false},
		{"WhenTimeoutToTimeout_ShouldReturnNoEvent", OutcomeTimeout, OutcomeTimeout, "", false},
		{"WhenTimeoutToNetworkError_ShouldReturnNoEvent", OutcomeTimeout, OutcomeNetworkError, "", false},
		{"WhenNetworkErrorToHTTPError_ShouldReturnNoEvent", OutcomeNetworkError, OutcomeHTTPError, "", false},
		{"WhenNetworkErrorToTimeout_ShouldReturnNoEvent", OutcomeNetworkError, OutcomeTimeout, "", false},
		{"WhenNetworkErrorToNetworkError_ShouldReturnNoEvent", OutcomeNetworkError, OutcomeNetworkError, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, ok := TransitionEvent(tt.prev, tt.curr)

			assert.Equal(t, tt.wantEvent, ok)
			assert.Equal(t, tt.wantKind, kind)
		})
	}
}

func TestTransitionEvent_WhenFirstCheckOfNewTargetFails_ShouldReturnDown(t *testing.T) {
	kind, ok := TransitionEvent(InitialOutcome, OutcomeTimeout)

	assert.True(t, ok)
	assert.Equal(t, EventDown, kind)
}

func TestTransitionEvent_WhenFirstCheckOfNewTargetSucceeds_ShouldReturnNoEvent(t *testing.T) {
	_, ok := TransitionEvent(InitialOutcome, OutcomeUp)

	assert.False(t, ok)
}
