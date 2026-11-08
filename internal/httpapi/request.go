package httpapi

import (
	"net/http"
	"strconv"

	"github.com/Freze27/availability-monitor/internal/domain"
)

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func parseLimit(r *http.Request) (*int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"limit": "must be an integer"}}
	}
	return &n, nil
}
