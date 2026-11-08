package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Freze27/availability-monitor/internal/domain"
)

type TargetService interface {
	Create(ctx context.Context, t domain.NewTarget) (domain.Target, error)
	Get(ctx context.Context, id int64) (domain.Target, error)
	List(ctx context.Context) ([]domain.TargetWithLastCheck, error)
	Update(ctx context.Context, id int64, u domain.TargetUpdate) (domain.Target, error)
	Delete(ctx context.Context, id int64) error
}

type targetHandler struct {
	svc    TargetService
	logger *slog.Logger
}

type createTargetRequest struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	IntervalSeconds int    `json:"interval_seconds"`
}

type updateTargetRequest struct {
	Name            *string `json:"name"`
	URL             *string `json:"url"`
	IntervalSeconds *int    `json:"interval_seconds"`
	Paused          *bool   `json:"paused"`
}

type targetResponse struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	Paused          bool      `json:"paused"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type lastCheckResponse struct {
	Outcome    domain.Outcome `json:"outcome"`
	CheckedAt  time.Time      `json:"checked_at"`
	DurationMS int64          `json:"duration_ms"`
}

type targetListItemResponse struct {
	targetResponse
	LastCheck *lastCheckResponse `json:"last_check"`
}

func toTargetListItemResponse(t domain.TargetWithLastCheck) targetListItemResponse {
	item := targetListItemResponse{targetResponse: toTargetResponse(t.Target)}
	if t.LastCheck != nil {
		item.LastCheck = &lastCheckResponse{
			Outcome:    t.LastCheck.Outcome,
			CheckedAt:  t.LastCheck.CheckedAt,
			DurationMS: t.LastCheck.Duration.Milliseconds(),
		}
	}
	return item
}

func toTargetResponse(t domain.Target) targetResponse {
	return targetResponse{
		ID:              t.ID,
		Name:            t.Name,
		URL:             t.URL,
		IntervalSeconds: t.IntervalSeconds,
		Paused:          t.Paused,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func (h *targetHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createTargetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	target, err := h.svc.Create(r.Context(), domain.NewTarget{
		Name:            req.Name,
		URL:             req.URL,
		IntervalSeconds: req.IntervalSeconds,
	})
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	w.Header().Set("Location", "/targets/"+strconv.FormatInt(target.ID, 10))
	writeJSON(w, http.StatusCreated, toTargetResponse(target))
}

func (h *targetHandler) list(w http.ResponseWriter, r *http.Request) {
	targets, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	resp := make([]targetListItemResponse, 0, len(targets))
	for _, t := range targets {
		resp = append(resp, toTargetListItemResponse(t))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *targetHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	target, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toTargetResponse(target))
}

func (h *targetHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req updateTargetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	target, err := h.svc.Update(r.Context(), id, domain.TargetUpdate{
		Name:            req.Name,
		URL:             req.URL,
		IntervalSeconds: req.IntervalSeconds,
		Paused:          req.Paused,
	})
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toTargetResponse(target))
}

func (h *targetHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
