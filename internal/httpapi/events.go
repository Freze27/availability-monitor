package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type EventService interface {
	ListEvents(ctx context.Context, limit *int) ([]domain.Event, error)
}

type eventHandler struct {
	svc    EventService
	logger *slog.Logger
}

type eventResponse struct {
	ID         int64            `json:"id"`
	TargetID   int64            `json:"target_id"`
	Kind       domain.EventKind `json:"kind"`
	OccurredAt time.Time        `json:"occurred_at"`
}

func (h *eventHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	events, err := h.svc.ListEvents(r.Context(), limit)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	resp := make([]eventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, eventResponse{
			ID:         e.ID,
			TargetID:   e.TargetID,
			Kind:       e.Kind,
			OccurredAt: e.OccurredAt,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
