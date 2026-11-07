package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxBodyBytes = 1 << 20

type errorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return errors.New("request body must not be empty")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("request body is not valid JSON")
		case errors.As(err, &typeErr) && typeErr.Field == "":
			return errors.New("request body must be a JSON object")
		case errors.As(err, &typeErr):
			return fmt.Errorf("field %q has invalid type", typeErr.Field)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return fmt.Errorf("unknown field %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
		case errors.As(err, &maxBytesErr):
			return fmt.Errorf("request body must not exceed %d bytes", maxBodyBytes)
		default:
			return errors.New("request body is not valid JSON")
		}
	}
	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
