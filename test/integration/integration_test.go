package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/Freze27/availability-monitor/internal/checker"
	"github.com/Freze27/availability-monitor/internal/httpapi"
	"github.com/Freze27/availability-monitor/internal/scheduler"
	"github.com/Freze27/availability-monitor/internal/service"
	"github.com/Freze27/availability-monitor/internal/storage"
)

var databaseURL, skipReason string

func TestMain(m *testing.M) {
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("monitor_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		skipReason = fmt.Sprintf("postgres container is not available (is Docker running?): %v", err)
		os.Exit(m.Run())
	}

	databaseURL, err = pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		skipReason = fmt.Sprintf("postgres connection string: %v", err)
	}
	code := m.Run()
	if err := pg.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminate postgres container: %v\n", err)
	}
	os.Exit(code)
}

type env struct {
	db     *pgxpool.Pool
	store  *storage.Storage
	checks *service.CheckService
	api    http.Handler
	site   *site
}

type site struct {
	srv  *httptest.Server
	mode atomic.Value
}

func newSite(t *testing.T) *site {
	t.Helper()
	s := &site{}
	s.mode.Store("up")
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch s.mode.Load().(string) {
		case "up":
			w.WriteHeader(http.StatusOK)
		case "slow":
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
		case "busy":
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *site) set(mode string) {
	s.mode.Store(mode)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if databaseURL == "" {
		t.Skip(skipReason)
	}
	ctx := context.Background()

	store, err := storage.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect storage: %v", err)
	}
	t.Cleanup(store.Close)
	if _, err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(ctx, `TRUNCATE targets, checks, events RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	checks := service.NewCheckService(store, checker.New(500*time.Millisecond))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &env{
		db:     db,
		store:  store,
		checks: checks,
		api:    httpapi.NewRouter(service.NewTargetService(store), checks, service.NewEventService(store), logger),
		site:   newSite(t),
	}
}

func (e *env) call(t *testing.T, method, path, body string) (int, []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	e.api.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec.Code, rec.Body.Bytes()
}

func (e *env) createTarget(t *testing.T, name string, interval int) int64 {
	t.Helper()
	status, body := e.call(t, http.MethodPost, "/targets",
		fmt.Sprintf(`{"name":%q,"url":%q,"interval_seconds":%d}`, name, e.site.srv.URL, interval))
	if status != http.StatusCreated {
		t.Fatalf("create target: status %d, body %s", status, body)
	}
	var target struct {
		ID int64 `json:"id"`
	}
	mustDecode(t, body, &target)
	return target.ID
}

func (e *env) runCheck(t *testing.T, id int64) map[string]any {
	t.Helper()
	status, body := e.call(t, http.MethodPost, fmt.Sprintf("/targets/%d/checks", id), "")
	if status != http.StatusCreated {
		t.Fatalf("run check: status %d, body %s", status, body)
	}
	var check map[string]any
	mustDecode(t, body, &check)
	return check
}

func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

func (e *env) startScheduler(tick time.Duration) (stop func()) {
	s := scheduler.New(e.store, e.checks, tick, 5, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	return func() {
		cancel()
		<-done
	}
}

func mustDecode(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
}

func TestTargetLifecycle_WhenCreatedCheckedAndDeleted_ShouldValidateAndCascade(t *testing.T) {
	e := newEnv(t)

	status, _ := e.call(t, http.MethodPost, "/targets", `{"name":"","url":"example.com","interval_seconds":60}`)
	if status != http.StatusBadRequest || e.count(t, `SELECT count(*) FROM targets`) != 0 {
		t.Fatalf("invalid target: status %d, want 400 and nothing saved", status)
	}

	id := e.createTarget(t, "Lifecycle", 60)
	_, body := e.call(t, http.MethodGet, "/targets", "")
	if !strings.Contains(string(body), `"last_check":null`) {
		t.Errorf("list before checks = %s, want last_check null", body)
	}

	e.site.set("503")
	e.runCheck(t, id)
	_, body = e.call(t, http.MethodGet, "/targets", "")
	if !strings.Contains(string(body), `"outcome":"http_error"`) {
		t.Errorf("list after check = %s, want last_check http_error", body)
	}

	status, _ = e.call(t, http.MethodDelete, fmt.Sprintf("/targets/%d", id), "")
	if status != http.StatusNoContent {
		t.Fatalf("delete: status %d, want 204", status)
	}
	if n := e.count(t, `SELECT (SELECT count(*) FROM targets) + (SELECT count(*) FROM checks) + (SELECT count(*) FROM events)`); n != 0 {
		t.Errorf("rows left after delete = %d, want target, checks and events removed", n)
	}
}

func TestManualCheck_WhenSiteReturnsDifferentResults_ShouldPersistOutcomesNewestFirst(t *testing.T) {
	e := newEnv(t)
	id := e.createTarget(t, "Outcomes", 60)

	for _, mode := range []string{"up", "503", "slow"} {
		e.site.set(mode)
		e.runCheck(t, id)
	}

	_, body := e.call(t, http.MethodGet, fmt.Sprintf("/targets/%d/checks", id), "")
	var history []map[string]any
	mustDecode(t, body, &history)
	var outcomes []string
	for _, c := range history {
		outcomes = append(outcomes, c["outcome"].(string))
	}
	if strings.Join(outcomes, ",") != "timeout,http_error,up" {
		t.Errorf("history = %v, want newest first: timeout, http_error, up", outcomes)
	}
	if n := e.count(t, `SELECT count(*) FROM checks WHERE outcome = 'timeout' AND status_code IS NULL`); n != 1 {
		t.Errorf("timeout rows without status code = %d, want 1", n)
	}
}

func TestEvents_WhenSiteGoesDownAndRecovers_ShouldRecordOnlyStateChanges(t *testing.T) {
	e := newEnv(t)
	id := e.createTarget(t, "Flapping", 60)

	for _, mode := range []string{"up", "up", "503", "slow", "up", "up"} {
		e.site.set(mode)
		e.runCheck(t, id)
	}

	_, body := e.call(t, http.MethodGet, "/events", "")
	var events []map[string]any
	mustDecode(t, body, &events)
	if len(events) != 2 || events[0]["kind"] != "up" || events[1]["kind"] != "down" {
		t.Errorf("events = %s, want exactly up then down (newest first)", body)
	}
}

func TestSummary_WhenChecksInsideAndOutsideWindow_ShouldCountOnlyWindowAndKeepOldChecks(t *testing.T) {
	e := newEnv(t)
	id := e.createTarget(t, "Summary", 3600)
	_, err := e.db.Exec(context.Background(), `
		INSERT INTO checks (target_id, checked_at, outcome, status_code, duration_ms) VALUES
		($1, now() - interval '30 minutes', 'up', 200, 10),
		($1, now() - interval '2 hours', 'timeout', NULL, 500),
		($1, now() - interval '30 hours', 'up', 200, 10)`, id)
	if err != nil {
		t.Fatalf("insert checks: %v", err)
	}

	_, body := e.call(t, http.MethodGet, fmt.Sprintf("/targets/%d/summary?hours=24", id), "")

	var summary struct {
		Up             int            `json:"up"`
		Other          int            `json:"other"`
		OtherByOutcome map[string]int `json:"other_by_outcome"`
		Points         []any          `json:"points"`
	}
	mustDecode(t, body, &summary)
	if summary.Up != 1 || summary.Other != 1 || summary.OtherByOutcome["timeout"] != 1 || len(summary.Points) != 2 {
		t.Errorf("summary = %s, want 1 up, 1 timeout, 2 points inside 24h", body)
	}
	if n := e.count(t, `SELECT count(*) FROM checks WHERE target_id = $1`, id); n != 3 {
		t.Errorf("checks in database = %d, want old check kept", n)
	}
}

func TestScheduler_WhenRunning_ShouldCheckDueTargetsSkipPausedAndContinueAfterRestart(t *testing.T) {
	e := newEnv(t)
	active := e.createTarget(t, "Active", 2)
	paused := e.createTarget(t, "Paused", 1)
	e.call(t, http.MethodPatch, fmt.Sprintf("/targets/%d", paused), `{"paused":true}`)

	stop := e.startScheduler(100 * time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for e.count(t, `SELECT count(*) FROM checks WHERE target_id = $1`, active) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	first := time.Now()
	time.Sleep(300 * time.Millisecond)
	stop()

	time.Sleep(200 * time.Millisecond)
	stop = e.startScheduler(100 * time.Millisecond)
	defer stop()

	time.Sleep(time.Until(first.Add(1500 * time.Millisecond)))
	if n := e.count(t, `SELECT count(*) FROM checks WHERE target_id = $1`, active); n != 1 {
		t.Errorf("checks right after restart = %d, want 1 (schedule continues, no immediate recheck)", n)
	}
	time.Sleep(time.Until(first.Add(2600 * time.Millisecond)))
	if n := e.count(t, `SELECT count(*) FROM checks WHERE target_id = $1`, active); n != 2 {
		t.Errorf("checks after interval = %d, want 2", n)
	}
	if n := e.count(t, `SELECT count(*) FROM checks WHERE target_id = $1`, paused); n != 0 {
		t.Errorf("checks of paused target = %d, want 0", n)
	}
}

func TestManualCheck_WhenRequestedConcurrently_ShouldRunOnceAndReturn409ForOthers(t *testing.T) {
	e := newEnv(t)
	id := e.createTarget(t, "Concurrent", 3600)
	e.site.set("busy")

	var wg sync.WaitGroup
	codes := make([]int, 20)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _ = e.call(t, http.MethodPost, fmt.Sprintf("/targets/%d/checks", id), "")
		}()
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	if created != 1 || conflicts != 19 {
		t.Errorf("201 = %d, 409 = %d, want 1 and 19", created, conflicts)
	}
	if n := e.count(t, `SELECT count(*) FROM checks WHERE target_id = $1`, id); n != 1 {
		t.Errorf("checks saved = %d, want 1", n)
	}
}
