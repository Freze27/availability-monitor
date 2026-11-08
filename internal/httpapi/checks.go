package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type CheckService interface {
	RunCheck(ctx context.Context, targetID int64) (domain.Check, error)
	ListChecks(ctx context.Context, targetID int64, limit *int) ([]domain.Check, error)
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

	var limit *int
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeServiceError(w, r, h.logger, &domain.ValidationError{Fields: map[string]string{
				"limit": "must be an integer",
			}})
			return
		}
		limit = &n
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
