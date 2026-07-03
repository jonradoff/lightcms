package errors

import (
	"log"
	"net/http"
)

// Handler provides environment-aware error handling
type Handler struct {
	isDev bool
}

// NewHandler creates a new error handler
func NewHandler(isDev bool) *Handler {
	return &Handler{isDev: isDev}
}

// HTTPError writes an error response, showing details only in development mode
func (h *Handler) HTTPError(w http.ResponseWriter, err error, statusCode int) {
	// Always log the full error server-side
	log.Printf("HTTP Error %d: %v", statusCode, err)

	if h.isDev {
		// In development, show the actual error
		http.Error(w, err.Error(), statusCode)
	} else {
		// In production, show generic messages
		http.Error(w, genericMessage(statusCode), statusCode)
	}
}

// HTTPErrorMessage writes a specific error message (use when the message is safe to show)
func (h *Handler) HTTPErrorMessage(w http.ResponseWriter, message string, statusCode int) {
	log.Printf("HTTP Error %d: %s", statusCode, message)
	http.Error(w, message, statusCode)
}

// genericMessage returns a generic error message for a status code
func genericMessage(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "Bad request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not found"
	case http.StatusMethodNotAllowed:
		return "Method not allowed"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusTooManyRequests:
		return "Too many requests"
	case http.StatusInternalServerError:
		return "Internal server error"
	case http.StatusBadGateway:
		return "Bad gateway"
	case http.StatusServiceUnavailable:
		return "Service unavailable"
	default:
		return "An error occurred"
	}
}

// IsDev returns whether we're in development mode
func (h *Handler) IsDev() bool {
	return h.isDev
}
