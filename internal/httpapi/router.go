package httpapi

import (
	"log/slog"
	"net/http"
)

func NewRouter(targets TargetService, logger *slog.Logger) http.Handler {
	th := &targetHandler{svc: targets, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)

	mux.HandleFunc("POST /targets", th.create)
	mux.HandleFunc("GET /targets", th.list)
	mux.HandleFunc("GET /targets/{id}", th.get)
	mux.HandleFunc("PATCH /targets/{id}", th.update)
	mux.HandleFunc("DELETE /targets/{id}", th.delete)

	return logRequests(logger, recoverPanics(logger, handleRouteErrors(mux)))
}
