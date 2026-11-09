package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Freze27/availability-monitor/internal/domain"
)

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func respondWith(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}
}

func TestCheck_WhenStatus200_ShouldReturnUp(t *testing.T) {
	srv := newServer(t, respondWith(http.StatusOK))

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeUp, got.Outcome)
	assert.Equal(t, new(http.StatusOK), got.StatusCode)
	assert.Nil(t, got.Error)
}

func TestCheck_WhenStatus204_ShouldReturnUp(t *testing.T) {
	srv := newServer(t, respondWith(http.StatusNoContent))

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeUp, got.Outcome)
	assert.Equal(t, new(http.StatusNoContent), got.StatusCode)
}

func TestCheck_WhenStatus404_ShouldReturnHTTPError(t *testing.T) {
	srv := newServer(t, respondWith(http.StatusNotFound))

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeHTTPError, got.Outcome)
	assert.Equal(t, new(http.StatusNotFound), got.StatusCode)
}

func TestCheck_WhenStatus503_ShouldReturnHTTPErrorWithText(t *testing.T) {
	srv := newServer(t, respondWith(http.StatusServiceUnavailable))

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeHTTPError, got.Outcome)
	assert.Equal(t, new(http.StatusServiceUnavailable), got.StatusCode)
	assert.Equal(t, new("unexpected status: 503 Service Unavailable"), got.Error)
}

func TestCheck_WhenNonStandardStatus_ShouldReturnHTTPError(t *testing.T) {
	srv := newServer(t, respondWith(999))

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeHTTPError, got.Outcome)
	assert.Equal(t, new(999), got.StatusCode)
}

func TestCheck_WhenRedirectToOK_ShouldReturnUp(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	got, err := New(time.Second).Check(context.Background(), srv.URL+"/old")

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeUp, got.Outcome)
	assert.Equal(t, new(http.StatusOK), got.StatusCode)
}

func TestCheck_WhenRedirectToNotFound_ShouldReturnHTTPError(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/missing", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	got, err := New(time.Second).Check(context.Background(), srv.URL+"/old")

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeHTTPError, got.Outcome)
	assert.Equal(t, new(http.StatusNotFound), got.StatusCode)
}

func TestCheck_WhenResponseSlowerThanTimeout_ShouldReturnTimeoutWithoutCode(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})

	got, err := New(100*time.Millisecond).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeTimeout, got.Outcome)
	assert.Nil(t, got.StatusCode)
	assert.Equal(t, new("no response within 100ms"), got.Error)
	assert.GreaterOrEqual(t, got.Duration, 100*time.Millisecond)
	assert.Less(t, got.Duration, time.Second)
}

func TestCheck_WhenServerUnavailable_ShouldReturnNetworkError(t *testing.T) {
	srv := httptest.NewServer(respondWith(http.StatusOK))
	srv.Close()

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeNetworkError, got.Outcome)
	assert.Nil(t, got.StatusCode)
	require.NotNil(t, got.Error)
	assert.NotEmpty(t, *got.Error)
}

func TestCheck_WhenCertificateUntrusted_ShouldReturnNetworkError(t *testing.T) {
	srv := httptest.NewTLSServer(respondWith(http.StatusOK))
	t.Cleanup(srv.Close)

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeNetworkError, got.Outcome)
	assert.Nil(t, got.StatusCode)
}

func TestCheck_WhenCalled_ShouldSendGetWithUserAgent(t *testing.T) {
	var method, agent string
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, agent = r.Method, r.UserAgent()
		w.WriteHeader(http.StatusOK)
	})

	_, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, method)
	assert.Equal(t, userAgent, agent)
}

func TestCheck_WhenCalled_ShouldMeasureDurationAndStartTime(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	before := time.Now()

	got, err := New(time.Second).Check(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, got.Duration, 50*time.Millisecond)
	assert.False(t, got.CheckedAt.Before(before), "checked_at is before the call")
	assert.WithinDuration(t, before, got.CheckedAt, got.Duration)
}

func TestCheck_WhenContextCancelled_ShouldReturnErrorWithoutResult(t *testing.T) {
	srv := newServer(t, respondWith(http.StatusOK))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New(time.Second).Check(ctx, srv.URL)

	assert.ErrorIs(t, err, context.Canceled)
}
