package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type fakeEventRepo struct {
	limit int
	calls int
}

func (r *fakeEventRepo) ListEvents(_ context.Context, limit int) ([]domain.Event, error) {
	r.limit = limit
	r.calls++
	return nil, nil
}

func TestListEvents_WhenLimitNotSet_ShouldUseDefault(t *testing.T) {
	repo := &fakeEventRepo{}

	_, err := NewEventService(repo).ListEvents(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, defaultListLimit, repo.limit)
}

func TestListEvents_WhenLimitOutOfRange_ShouldReturnValidationErrorAndNotQuery(t *testing.T) {
	for _, limit := range []int{0, maxListLimit + 1} {
		repo := &fakeEventRepo{}

		_, err := NewEventService(repo).ListEvents(context.Background(), new(limit))

		assert.Equal(t, []string{"limit"}, invalidFields(t, err), "limit %d", limit)
		assert.Zero(t, repo.calls, "limit %d: repository must not be called", limit)
	}
}
