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
