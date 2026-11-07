package httpapi

import (
	"log/slog"
	"net/http"
)

func NewRouter(targets TargetService, checks CheckService, logger *slog.Logger) http.Handler {
	th := &targetHandler{svc: targets, logger: logger}
	ch := &checkHandler{svc: checks, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)

	mux.HandleFunc("POST /targets", th.create)
	mux.HandleFunc("GET /targets", th.list)
	mux.HandleFunc("GET /targets/{id}", th.get)
	mux.HandleFunc("PATCH /targets/{id}", th.update)
	mux.HandleFunc("DELETE /targets/{id}", th.delete)

	mux.HandleFunc("POST /targets/{id}/checks", ch.run)

	return logRequests(logger, recoverPanics(logger, handleRouteErrors(mux)))
}
