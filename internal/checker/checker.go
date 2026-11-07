package checker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

const userAgent = "availability-monitor/1.0"

type Checker struct {
	client  *http.Client
	timeout time.Duration
}

func New(timeout time.Duration) *Checker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true

	return &Checker{
		client:  &http.Client{Transport: transport},
		timeout: timeout,
	}
}

func (c *Checker) Check(ctx context.Context, url string) (domain.Check, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return domain.Check{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := c.client.Do(req)
	result := domain.Check{
		CheckedAt: start,
		Duration:  time.Since(start),
	}

	if err != nil {
		if ctx.Err() != nil {
			return domain.Check{}, ctx.Err()
		}
		if isTimeout(err) {
			result.Outcome = domain.OutcomeTimeout
			result.Error = ptr(fmt.Sprintf("no response within %s", c.timeout))
		} else {
			result.Outcome = domain.OutcomeNetworkError
			result.Error = ptr(err.Error())
		}
		return result, nil
	}
	resp.Body.Close()

	result.StatusCode = &resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		result.Outcome = domain.OutcomeUp
	} else {
		result.Outcome = domain.OutcomeHTTPError
		result.Error = ptr("unexpected status: " + resp.Status)
	}
	return result, nil
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func ptr[T any](v T) *T {
	return &v
}
