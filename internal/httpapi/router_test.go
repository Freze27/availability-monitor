package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type fakeTargets struct {
	createErr error
	getErr    error
	listErr   error
	updateErr error
	list      []domain.TargetWithLastCheck
	updated   *domain.TargetUpdate
	panics    bool
}

func (f *fakeTargets) Create(_ context.Context, t domain.NewTarget) (domain.Target, error) {
	if f.createErr != nil {
		return domain.Target{}, f.createErr
	}
	return domain.Target{ID: 42, Name: t.Name, URL: t.URL, IntervalSeconds: t.IntervalSeconds}, nil
}

func (f *fakeTargets) Get(_ context.Context, id int64) (domain.Target, error) {
	if f.panics {
		panic("boom")
	}
	return domain.Target{ID: id}, f.getErr
}

func (f *fakeTargets) List(context.Context) ([]domain.TargetWithLastCheck, error) {
	return f.list, f.listErr
}

func (f *fakeTargets) Update(_ context.Context, id int64, u domain.TargetUpdate) (domain.Target, error) {
	f.updated = &u
	if f.updateErr != nil {
		return domain.Target{}, f.updateErr
	}
	return domain.Target{ID: id, Name: "Renamed", Paused: u.Paused != nil && *u.Paused}, nil
}

func (f *fakeTargets) Delete(context.Context, int64) error {
	return nil
}

type fakeChecks struct {
	runErr  error
	checks  []domain.Check
	summary domain.Summary
	limit   *int
	hours   *int
}

func (f *fakeChecks) RunCheck(_ context.Context, id int64) (domain.Check, error) {
	if f.runErr != nil {
		return domain.Check{}, f.runErr
	}
	return domain.Check{ID: 1, TargetID: id, Outcome: domain.OutcomeTimeout, Duration: 5 * time.Second}, nil
}

func (f *fakeChecks) ListChecks(_ context.Context, _ int64, limit *int) ([]domain.Check, error) {
	f.limit = limit
	return f.checks, nil
}

func (f *fakeChecks) Summary(_ context.Context, _ int64, hours *int) (domain.Summary, error) {
	f.hours = hours
	return f.summary, nil
}

type fakeEvents struct {
	events []domain.Event
	limit  *int
}

func (f *fakeEvents) ListEvents(_ context.Context, limit *int) ([]domain.Event, error) {
	f.limit = limit
	return f.events, nil
}

func newTestRouter(targets *fakeTargets, checks *fakeChecks, events *fakeEvents) http.Handler {
	if targets == nil {
		targets = &fakeTargets{}
	}
	if checks == nil {
		checks = &fakeChecks{}
	}
	if events == nil {
		events = &fakeEvents{}
	}
	return NewRouter(targets, checks, events, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestHealth_WhenCalled_ShouldReturnOK(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/health", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestCreateTarget_WhenCreated_ShouldReturn201WithLocation(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodPost, "/targets",
		`{"name":"Example","url":"https://example.com","interval_seconds":60}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "/targets/42", rec.Header().Get("Location"))
	assert.Contains(t, rec.Body.String(), `"id":42`)
}

func TestCreateTarget_WhenValidationFails_ShouldReturn400WithFields(t *testing.T) {
	targets := &fakeTargets{createErr: &domain.ValidationError{Fields: map[string]string{"name": "must not be empty"}}}

	rec := do(newTestRouter(targets, nil, nil), http.MethodPost, "/targets", `{"name":"","url":"x","interval_seconds":1}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"validation failed","fields":{"name":"must not be empty"}}`, rec.Body.String())
}

func TestCreateTarget_WhenBodyInvalid_ShouldReturn400WithClearMessage(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{"WhenBodyEmpty_ShouldSayEmpty", ``, "request body must not be empty"},
		{"WhenJSONBroken_ShouldSayInvalidJSON", `{"name":`, "request body is not valid JSON"},
		{"WhenBodyIsArray_ShouldSayObject", `[]`, "request body must be a JSON object"},
		{"WhenFieldTypeWrong_ShouldNameField", `{"interval_seconds":"60"}`, `field "interval_seconds" has invalid type`},
		{"WhenUnknownField_ShouldNameField", `{"paused":true}`, `unknown field "paused"`},
		{"WhenTwoObjects_ShouldSaySingleObject", `{}{}`, "request body must contain a single JSON object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(newTestRouter(nil, nil, nil), http.MethodPost, "/targets", tt.body)

			var resp struct{ Error string }
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, tt.message, resp.Error)
		})
	}
}

func TestGetTarget_WhenNotFound_ShouldReturn404(t *testing.T) {
	targets := &fakeTargets{getErr: domain.ErrTargetNotFound}

	rec := do(newTestRouter(targets, nil, nil), http.MethodGet, "/targets/5", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"target not found"}`, rec.Body.String())
}

func TestGetTarget_WhenIDInvalid_ShouldReturn400(t *testing.T) {
	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/targets/"+id, "")

		assert.Equal(t, http.StatusBadRequest, rec.Code, "id %q", id)
		assert.JSONEq(t, `{"error":"id must be a positive integer"}`, rec.Body.String(), "id %q", id)
	}
}

func TestListTargets_WhenServiceFails_ShouldReturn500WithoutDetails(t *testing.T) {
	targets := &fakeTargets{listErr: errors.New("connection refused to 10.0.0.5")}

	rec := do(newTestRouter(targets, nil, nil), http.MethodGet, "/targets", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "10.0.0.5")
}

func TestListTargets_WhenNoChecksYet_ShouldReturnNullLastCheck(t *testing.T) {
	targets := &fakeTargets{list: []domain.TargetWithLastCheck{{Target: domain.Target{ID: 1}}}}

	rec := do(newTestRouter(targets, nil, nil), http.MethodGet, "/targets", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"last_check":null`)
}

func TestListTargets_WhenEmpty_ShouldReturnEmptyArray(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/targets", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestUpdateTarget_WhenFieldsGiven_ShouldPassOnlyGivenFieldsAndReturn200(t *testing.T) {
	targets := &fakeTargets{}

	rec := do(newTestRouter(targets, nil, nil), http.MethodPatch, "/targets/5", `{"name":"Renamed","paused":true}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, &domain.TargetUpdate{Name: new("Renamed"), Paused: new(true)}, targets.updated)
	assert.Contains(t, rec.Body.String(), `"paused":true`)
}

func TestUpdateTarget_WhenBodyInvalid_ShouldReturn400AndNotCallService(t *testing.T) {
	targets := &fakeTargets{}

	rec := do(newTestRouter(targets, nil, nil), http.MethodPatch, "/targets/5", `{"paused":"yes"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"field \"paused\" has invalid type"}`, rec.Body.String())
	assert.Nil(t, targets.updated)
}

func TestUpdateTarget_WhenValidationFails_ShouldReturn400WithFields(t *testing.T) {
	targets := &fakeTargets{updateErr: &domain.ValidationError{Fields: map[string]string{"url": "must start with http:// or https://"}}}

	rec := do(newTestRouter(targets, nil, nil), http.MethodPatch, "/targets/5", `{"url":"bad"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"validation failed","fields":{"url":"must start with http:// or https://"}}`, rec.Body.String())
}

func TestDeleteTarget_WhenDeleted_ShouldReturn204WithoutBody(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodDelete, "/targets/5", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestRunCheck_WhenCheckAlreadyInProgress_ShouldReturn409(t *testing.T) {
	checks := &fakeChecks{runErr: domain.ErrCheckInProgress}

	rec := do(newTestRouter(nil, checks, nil), http.MethodPost, "/targets/5/checks", "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"check already in progress for this target"}`, rec.Body.String())
}

func TestRunCheck_WhenTimeout_ShouldReturn201WithNullStatusCode(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodPost, "/targets/5/checks", "")

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"id":1,"target_id":5,"checked_at":"0001-01-01T00:00:00Z","outcome":"timeout",
		"status_code":null,"duration_ms":5000,"error":null}`, rec.Body.String())
}

func TestListChecks_WhenLimitGiven_ShouldPassItAndReturnChecks(t *testing.T) {
	checks := &fakeChecks{checks: []domain.Check{
		{ID: 2, TargetID: 5, Outcome: domain.OutcomeUp, StatusCode: new(200), Duration: 120 * time.Millisecond},
	}}

	rec := do(newTestRouter(nil, checks, nil), http.MethodGet, "/targets/5/checks?limit=10", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, new(10), checks.limit)
	assert.JSONEq(t, `[{"id":2,"target_id":5,"checked_at":"0001-01-01T00:00:00Z","outcome":"up",
		"status_code":200,"duration_ms":120,"error":null}]`, rec.Body.String())
}

func TestListChecks_WhenLimitNotInteger_ShouldReturn400(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/targets/5/checks?limit=abc", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"validation failed","fields":{"limit":"must be an integer"}}`, rec.Body.String())
}

func TestSummary_WhenCalled_ShouldReturnCountsBreakdownAndPointsInSeconds(t *testing.T) {
	at := time.Date(2026, 11, 8, 14, 0, 0, 123456789, time.UTC)
	checks := &fakeChecks{summary: domain.Summary{
		TargetID:       5,
		Hours:          24,
		From:           at.Add(-24 * time.Hour),
		To:             at,
		Up:             1,
		Other:          1,
		OtherByOutcome: map[domain.Outcome]int{domain.OutcomeHTTPError: 0, domain.OutcomeTimeout: 1, domain.OutcomeNetworkError: 0},
		Points:         []domain.SummaryPoint{{CheckedAt: at, Outcome: domain.OutcomeUp}},
	}}

	rec := do(newTestRouter(nil, checks, nil), http.MethodGet, "/targets/5/summary?hours=24", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, new(24), checks.hours)
	assert.JSONEq(t, `{"target_id":5,"hours":24,"from":"2026-11-07T14:00:00Z","to":"2026-11-08T14:00:00Z",
		"up":1,"other":1,"other_by_outcome":{"http_error":0,"network_error":0,"timeout":1},
		"points":[{"checked_at":"2026-11-08T14:00:00Z","outcome":"up"}]}`, rec.Body.String())
}

func TestSummary_WhenHoursNotInteger_ShouldReturn400(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/targets/5/summary?hours=1.5", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"validation failed","fields":{"hours":"must be an integer"}}`, rec.Body.String())
}

func TestListEvents_WhenCalled_ShouldPassLimitAndReturnEvents(t *testing.T) {
	at := time.Date(2026, 11, 8, 14, 0, 0, 0, time.UTC)
	events := &fakeEvents{events: []domain.Event{{ID: 1, TargetID: 5, Kind: domain.EventDown, OccurredAt: at}}}

	rec := do(newTestRouter(nil, nil, events), http.MethodGet, "/events?limit=3", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, new(3), events.limit)
	assert.JSONEq(t, `[{"id":1,"target_id":5,"kind":"down","occurred_at":"2026-11-08T14:00:00Z"}]`, rec.Body.String())
}

func TestListEvents_WhenLimitNotInteger_ShouldReturn400(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/events?limit=abc", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"validation failed","fields":{"limit":"must be an integer"}}`, rec.Body.String())
}

func TestRouter_WhenRouteUnknown_ShouldReturn404JSON(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodGet, "/unknown", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error":"route not found"}`, rec.Body.String())
}

func TestRouter_WhenMethodNotAllowed_ShouldReturn405JSONWithAllow(t *testing.T) {
	rec := do(newTestRouter(nil, nil, nil), http.MethodPut, "/targets/5", "")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "DELETE, GET, HEAD, PATCH", rec.Header().Get("Allow"))
	assert.JSONEq(t, `{"error":"method not allowed"}`, rec.Body.String())
}

func TestRouter_WhenHandlerPanics_ShouldReturn500JSON(t *testing.T) {
	rec := do(newTestRouter(&fakeTargets{panics: true}, nil, nil), http.MethodGet, "/targets/5", "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, rec.Body.String())
}
