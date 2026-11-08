package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type CheckService interface {
	RunCheck(ctx context.Context, targetID int64) (domain.Check, error)
	ListChecks(ctx context.Context, targetID int64, limit *int) ([]domain.Check, error)
	Summary(ctx context.Context, targetID int64, hours *int) (domain.Summary, error)
}

type checkHandler struct {
	svc    CheckService
	logger *slog.Logger
}

type checkResponse struct {
	ID         int64          `json:"id"`
	TargetID   int64          `json:"target_id"`
	CheckedAt  time.Time      `json:"checked_at"`
	Outcome    domain.Outcome `json:"outcome"`
	StatusCode *int           `json:"status_code"`
	DurationMS int64          `json:"duration_ms"`
	Error      *string        `json:"error"`
}

func toCheckResponse(c domain.Check) checkResponse {
	return checkResponse{
		ID:         c.ID,
		TargetID:   c.TargetID,
		CheckedAt:  c.CheckedAt,
		Outcome:    c.Outcome,
		StatusCode: c.StatusCode,
		DurationMS: c.Duration.Milliseconds(),
		Error:      c.Error,
	}
}

func (h *checkHandler) run(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	check, err := h.svc.RunCheck(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCheckResponse(check))
}

func (h *checkHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	limit, err := parseIntQuery(r, "limit")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	checks, err := h.svc.ListChecks(r.Context(), id, limit)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	resp := make([]checkResponse, 0, len(checks))
	for _, c := range checks {
		resp = append(resp, toCheckResponse(c))
	}
	writeJSON(w, http.StatusOK, resp)
}

type summaryPointResponse struct {
	CheckedAt time.Time      `json:"checked_at"`
	Outcome   domain.Outcome `json:"outcome"`
}

type summaryResponse struct {
	TargetID       int64                  `json:"target_id"`
	Hours          int                    `json:"hours"`
	From           time.Time              `json:"from"`
	To             time.Time              `json:"to"`
	Up             int                    `json:"up"`
	Other          int                    `json:"other"`
	OtherByOutcome map[domain.Outcome]int `json:"other_by_outcome"`
	Points         []summaryPointResponse `json:"points"`
}

func (h *checkHandler) summary(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	hours, err := parseIntQuery(r, "hours")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	summary, err := h.svc.Summary(r.Context(), id, hours)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	resp := summaryResponse{
		TargetID:       summary.TargetID,
		Hours:          summary.Hours,
		From:           summary.From.Truncate(time.Second),
		To:             summary.To.Truncate(time.Second),
		Up:             summary.Up,
		Other:          summary.Other,
		OtherByOutcome: summary.OtherByOutcome,
		Points:         make([]summaryPointResponse, 0, len(summary.Points)),
	}
	for _, p := range summary.Points {
		resp.Points = append(resp.Points, summaryPointResponse{
			CheckedAt: p.CheckedAt.Truncate(time.Second),
			Outcome:   p.Outcome,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
