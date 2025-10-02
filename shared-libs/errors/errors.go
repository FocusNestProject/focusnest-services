package errors

import "net/http"

// ErrorResponse represents the canonical error envelope returned by FocusNest APIs.
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

// ToStatusCode maps a domain specific error code to an HTTP status for default responses.
func ToStatusCode(code string) int {
	switch code {
	case "not_found":
		return http.StatusNotFound
	case "unauthorized":
		return http.StatusUnauthorized
	case "forbidden":
		return http.StatusForbidden
	case "conflict":
		return http.StatusConflict
	case "bad_request":
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
