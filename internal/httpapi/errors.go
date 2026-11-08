package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Freze27/availability-monitor/internal/domain"
)

func writeServiceError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var validationErr *domain.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "validation failed", Fields: validationErr.Fields})
	case errors.Is(err, domain.ErrTargetNotFound):
		writeError(w, http.StatusNotFound, domain.ErrTargetNotFound.Error())
	case errors.Is(err, domain.ErrCheckInProgress):
		writeError(w, http.StatusConflict, domain.ErrCheckInProgress.Error())
	default:
		logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func handleRouteErrors(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern == "" {
			w = &routeErrorWriter{ResponseWriter: w}
		}
		mux.ServeHTTP(w, r)
	})
}

type routeErrorWriter struct {
	http.ResponseWriter
	replaced bool
}

func (w *routeErrorWriter) WriteHeader(status int) {
	switch status {
	case http.StatusNotFound:
		w.replaced = true
		writeError(w.ResponseWriter, status, "route not found")
	case http.StatusMethodNotAllowed:
		w.replaced = true
		writeError(w.ResponseWriter, status, "method not allowed")
	default:
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *routeErrorWriter) Write(b []byte) (int, error) {
	if w.replaced {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

func (w *routeErrorWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
