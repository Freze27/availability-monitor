package service

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type fakeTargetRepo struct {
	created *domain.NewTarget
	updated *domain.TargetUpdate
}

func (r *fakeTargetRepo) CreateTarget(_ context.Context, t domain.NewTarget) (domain.Target, error) {
	r.created = &t
	return domain.Target{ID: 1, Name: t.Name, URL: t.URL, IntervalSeconds: t.IntervalSeconds}, nil
}

func (r *fakeTargetRepo) GetTarget(context.Context, int64) (domain.Target, error) {
	return domain.Target{}, nil
}

func (r *fakeTargetRepo) ListTargets(context.Context) ([]domain.TargetWithLastCheck, error) {
	return nil, nil
}

func (r *fakeTargetRepo) UpdateTarget(_ context.Context, id int64, u domain.TargetUpdate) (domain.Target, error) {
	r.updated = &u
	return domain.Target{ID: id}, nil
}

func (r *fakeTargetRepo) DeleteTarget(context.Context, int64) error {
	return nil
}

func validTarget() domain.NewTarget {
	return domain.NewTarget{Name: "Example", URL: "https://example.com", IntervalSeconds: 60}
}

func invalidFields(t *testing.T, err error) []string {
	var vErr *domain.ValidationError
	require.ErrorAs(t, err, &vErr)
	return slices.Sorted(maps.Keys(vErr.Fields))
}

func TestCreateTarget_WhenValid_ShouldTrimAndSave(t *testing.T) {
	repo := &fakeTargetRepo{}
	input := domain.NewTarget{Name: "  Example  ", URL: "  https://example.com  ", IntervalSeconds: 60}

	_, err := NewTargetService(repo).Create(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, &domain.NewTarget{Name: "Example", URL: "https://example.com", IntervalSeconds: 60}, repo.created)
}

func TestCreateTarget_WhenNameEmpty_ShouldReturnValidationErrorAndNotSave(t *testing.T) {
	for _, name := range []string{"", "   ", "\t\n"} {
		repo := &fakeTargetRepo{}
		input := validTarget()
		input.Name = name

		_, err := NewTargetService(repo).Create(context.Background(), input)

		assert.Equal(t, []string{"name"}, invalidFields(t, err), "name %q", name)
		assert.Nil(t, repo.created, "name %q: target must not be saved", name)
	}
}

func TestCreateTarget_WhenNameLongerThanLimit_ShouldReturnValidationError(t *testing.T) {
	input := validTarget()
	input.Name = strings.Repeat("a", maxNameLength+1)

	_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

	assert.Equal(t, []string{"name"}, invalidFields(t, err))
}

func TestCreateTarget_WhenNameAtLimitInCyrillic_ShouldSave(t *testing.T) {
	input := validTarget()
	input.Name = strings.Repeat("я", maxNameLength)

	_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

	assert.NoError(t, err)
}

func TestCreateTarget_WhenURLInvalid_ShouldReturnValidationError(t *testing.T) {
	for _, url := range []string{"", "example.com", "ftp://example.com", "https://", "http:example.com", "https://exa mple.com", "//example.com"} {
		input := validTarget()
		input.URL = url

		_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

		assert.Equal(t, []string{"url"}, invalidFields(t, err), "url %q", url)
	}
}

func TestCreateTarget_WhenURLSchemeUppercase_ShouldSave(t *testing.T) {
	input := validTarget()
	input.URL = "HTTPS://EXAMPLE.COM/path"

	_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

	assert.NoError(t, err)
}

func TestCreateTarget_WhenIntervalOutOfRange_ShouldReturnValidationError(t *testing.T) {
	for _, interval := range []int{0, -1, maxIntervalSeconds + 1} {
		input := validTarget()
		input.IntervalSeconds = interval

		_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

		assert.Equal(t, []string{"interval_seconds"}, invalidFields(t, err), "interval %d", interval)
	}
}

func TestCreateTarget_WhenIntervalOnBoundary_ShouldSave(t *testing.T) {
	for _, interval := range []int{1, maxIntervalSeconds} {
		input := validTarget()
		input.IntervalSeconds = interval

		_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

		assert.NoError(t, err, "interval %d", interval)
	}
}

func TestCreateTarget_WhenSeveralFieldsInvalid_ShouldReturnAllErrors(t *testing.T) {
	input := domain.NewTarget{Name: "", URL: "example.com", IntervalSeconds: 0}

	_, err := NewTargetService(&fakeTargetRepo{}).Create(context.Background(), input)

	assert.Equal(t, []string{"interval_seconds", "name", "url"}, invalidFields(t, err))
}

func TestUpdateTarget_WhenNoFields_ShouldReturnValidationError(t *testing.T) {
	repo := &fakeTargetRepo{}

	_, err := NewTargetService(repo).Update(context.Background(), 1, domain.TargetUpdate{})

	assert.Equal(t, []string{"body"}, invalidFields(t, err))
	assert.Nil(t, repo.updated)
}

func TestUpdateTarget_WhenOnlyPaused_ShouldPassPausedToRepository(t *testing.T) {
	repo := &fakeTargetRepo{}

	_, err := NewTargetService(repo).Update(context.Background(), 1, domain.TargetUpdate{Paused: new(true)})

	require.NoError(t, err)
	assert.Equal(t, &domain.TargetUpdate{Paused: new(true)}, repo.updated)
}

func TestUpdateTarget_WhenOneFieldInvalid_ShouldNotCallRepository(t *testing.T) {
	repo := &fakeTargetRepo{}

	_, err := NewTargetService(repo).Update(context.Background(), 1, domain.TargetUpdate{Name: new("ok"), URL: new("bad")})

	assert.Equal(t, []string{"url"}, invalidFields(t, err))
	assert.Nil(t, repo.updated)
}

func TestUpdateTarget_WhenNameHasSpaces_ShouldTrim(t *testing.T) {
	repo := &fakeTargetRepo{}

	_, err := NewTargetService(repo).Update(context.Background(), 1, domain.TargetUpdate{Name: new("  Renamed  ")})

	require.NoError(t, err)
	assert.Equal(t, &domain.TargetUpdate{Name: new("Renamed")}, repo.updated)
}
