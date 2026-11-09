package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type fakeCheckRepo struct {
	missing   bool
	created   []domain.Check
	limit     int
	listCalls int
	points    []domain.SummaryPoint
	from, to  time.Time
}

func (r *fakeCheckRepo) GetTarget(_ context.Context, id int64) (domain.Target, error) {
	if r.missing {
		return domain.Target{}, domain.ErrTargetNotFound
	}
	return domain.Target{ID: id, URL: "https://example.com"}, nil
}

func (r *fakeCheckRepo) CreateCheck(_ context.Context, c domain.Check) (domain.Check, error) {
	r.created = append(r.created, c)
	return c, nil
}

func (r *fakeCheckRepo) ListChecks(_ context.Context, _ int64, limit int) ([]domain.Check, error) {
	r.limit = limit
	r.listCalls++
	return nil, nil
}

func (r *fakeCheckRepo) ListSummaryPoints(_ context.Context, _ int64, from, to time.Time) ([]domain.SummaryPoint, error) {
	r.from, r.to = from, to
	return r.points, nil
}

type fakeChecker struct {
	started chan struct{}
	release chan struct{}
}

func (c *fakeChecker) Check(context.Context, string) (domain.Check, error) {
	if c.started != nil {
		close(c.started)
		<-c.release
	}
	return domain.Check{Outcome: domain.OutcomeUp}, nil
}

func TestRunCheck_WhenTargetExists_ShouldSaveResultForTarget(t *testing.T) {
	repo := &fakeCheckRepo{}

	_, err := NewCheckService(repo, &fakeChecker{}).RunCheck(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	assert.Equal(t, int64(7), repo.created[0].TargetID)
}

func TestRunCheck_WhenTargetMissing_ShouldReturnErrTargetNotFound(t *testing.T) {
	repo := &fakeCheckRepo{missing: true}

	_, err := NewCheckService(repo, &fakeChecker{}).RunCheck(context.Background(), 7)

	assert.ErrorIs(t, err, domain.ErrTargetNotFound)
	assert.Empty(t, repo.created)
}

func TestRunCheck_WhenCheckAlreadyRunning_ShouldReturnErrCheckInProgress(t *testing.T) {
	checker := &fakeChecker{started: make(chan struct{}), release: make(chan struct{})}
	svc := NewCheckService(&fakeCheckRepo{}, checker)
	firstDone := make(chan error)
	go func() {
		_, err := svc.RunCheck(context.Background(), 7)
		firstDone <- err
	}()
	<-checker.started

	_, err := svc.RunCheck(context.Background(), 7)

	assert.ErrorIs(t, err, domain.ErrCheckInProgress)
	close(checker.release)
	assert.NoError(t, <-firstDone)
}

func TestRunCheck_WhenTargetReservedByScheduler_ShouldReturnErrCheckInProgress(t *testing.T) {
	svc := NewCheckService(&fakeCheckRepo{}, &fakeChecker{})
	svc.TryReserve(7)

	_, err := svc.RunCheck(context.Background(), 7)

	assert.ErrorIs(t, err, domain.ErrCheckInProgress)
}

func TestRunCheck_WhenFinished_ShouldReleaseTarget(t *testing.T) {
	svc := NewCheckService(&fakeCheckRepo{}, &fakeChecker{})
	_, err := svc.RunCheck(context.Background(), 7)
	require.NoError(t, err)

	_, err = svc.RunCheck(context.Background(), 7)

	assert.NoError(t, err)
}

func TestRunReserved_WhenFinished_ShouldReleaseTarget(t *testing.T) {
	svc := NewCheckService(&fakeCheckRepo{}, &fakeChecker{})
	svc.TryReserve(7)

	_, err := svc.RunReserved(context.Background(), 7)

	require.NoError(t, err)
	assert.True(t, svc.TryReserve(7), "target is still reserved")
}

func TestListChecks_WhenLimitNotSet_ShouldUseDefault(t *testing.T) {
	repo := &fakeCheckRepo{}

	_, err := NewCheckService(repo, &fakeChecker{}).ListChecks(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.Equal(t, defaultListLimit, repo.limit)
}

func TestListChecks_WhenLimitOutOfRange_ShouldReturnValidationError(t *testing.T) {
	for _, limit := range []int{0, -1, maxListLimit + 1} {
		repo := &fakeCheckRepo{}

		_, err := NewCheckService(repo, &fakeChecker{}).ListChecks(context.Background(), 1, new(limit))

		assert.Equal(t, []string{"limit"}, invalidFields(t, err), "limit %d", limit)
		assert.Zero(t, repo.listCalls, "limit %d: repository must not be called", limit)
	}
}

func TestListChecks_WhenTargetMissing_ShouldReturnErrTargetNotFound(t *testing.T) {
	_, err := NewCheckService(&fakeCheckRepo{missing: true}, &fakeChecker{}).ListChecks(context.Background(), 1, nil)

	assert.ErrorIs(t, err, domain.ErrTargetNotFound)
}

func TestSummary_WhenHoursNotSet_ShouldUse24Hours(t *testing.T) {
	repo := &fakeCheckRepo{}

	got, err := NewCheckService(repo, &fakeChecker{}).Summary(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.Equal(t, 24, got.Hours)
	assert.Equal(t, 24*time.Hour, repo.to.Sub(repo.from))
}

func TestSummary_WhenHoursOutOfRange_ShouldReturnValidationError(t *testing.T) {
	for _, hours := range []int{0, -1, maxSummaryHours + 1} {
		_, err := NewCheckService(&fakeCheckRepo{}, &fakeChecker{}).Summary(context.Background(), 1, new(hours))

		assert.Equal(t, []string{"hours"}, invalidFields(t, err), "hours %d", hours)
	}
}

func TestSummary_WhenPointsGiven_ShouldCountUpOtherAndBreakdown(t *testing.T) {
	repo := &fakeCheckRepo{points: []domain.SummaryPoint{
		{Outcome: domain.OutcomeUp},
		{Outcome: domain.OutcomeUp},
		{Outcome: domain.OutcomeHTTPError},
		{Outcome: domain.OutcomeTimeout},
		{Outcome: domain.OutcomeTimeout},
	}}

	got, err := NewCheckService(repo, &fakeChecker{}).Summary(context.Background(), 1, new(1))

	require.NoError(t, err)
	assert.Equal(t, 2, got.Up)
	assert.Equal(t, 3, got.Other)
	assert.Equal(t, map[domain.Outcome]int{
		domain.OutcomeHTTPError:    1,
		domain.OutcomeTimeout:      2,
		domain.OutcomeNetworkError: 0,
	}, got.OtherByOutcome)
}

func TestSummary_WhenNoChecks_ShouldReturnZerosForAllOutcomes(t *testing.T) {
	got, err := NewCheckService(&fakeCheckRepo{}, &fakeChecker{}).Summary(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.Zero(t, got.Up)
	assert.Zero(t, got.Other)
	assert.Equal(t, map[domain.Outcome]int{
		domain.OutcomeHTTPError:    0,
		domain.OutcomeTimeout:      0,
		domain.OutcomeNetworkError: 0,
	}, got.OtherByOutcome)
}

func TestSummary_WhenTargetMissing_ShouldReturnErrTargetNotFound(t *testing.T) {
	_, err := NewCheckService(&fakeCheckRepo{missing: true}, &fakeChecker{}).Summary(context.Background(), 1, nil)

	assert.ErrorIs(t, err, domain.ErrTargetNotFound)
}
