package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	sharederrors "github.com/focusnest/shared-libs/errors"
)

type errorResponse = sharederrors.ErrorResponse

func writeError(w http.ResponseWriter, status int, message string) {
	code := strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_"))
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondServiceError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, "internal server error")
}
