package errors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler(true)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if !h.IsDev() {
		t.Error("expected IsDev=true")
	}

	h2 := NewHandler(false)
	if h2.IsDev() {
		t.Error("expected IsDev=false")
	}
}

func TestHTTPError_Dev(t *testing.T) {
	h := NewHandler(true)
	rr := httptest.NewRecorder()
	h.HTTPError(rr, fmt.Errorf("database exploded"), http.StatusInternalServerError)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	// In dev mode the actual error message is surfaced
	if rr.Body.String() == "" {
		t.Error("expected non-empty body")
	}
}

func TestHTTPError_Prod(t *testing.T) {
	h := NewHandler(false)

	cases := []struct {
		code    int
		message string
	}{
		{http.StatusBadRequest, "Bad request"},
		{http.StatusUnauthorized, "Unauthorized"},
		{http.StatusForbidden, "Forbidden"},
		{http.StatusNotFound, "Not found"},
		{http.StatusMethodNotAllowed, "Method not allowed"},
		{http.StatusConflict, "Conflict"},
		{http.StatusTooManyRequests, "Too many requests"},
		{http.StatusInternalServerError, "Internal server error"},
		{http.StatusBadGateway, "Bad gateway"},
		{http.StatusServiceUnavailable, "Service unavailable"},
		{http.StatusTeapot, "An error occurred"}, // unknown code fallthrough
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d", tc.code), func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.HTTPError(rr, fmt.Errorf("internal details"), tc.code)
			if rr.Code != tc.code {
				t.Errorf("expected %d, got %d", tc.code, rr.Code)
			}
			body := rr.Body.String()
			if len(body) == 0 {
				t.Error("expected non-empty body")
			}
		})
	}
}

func TestHTTPErrorMessage(t *testing.T) {
	h := NewHandler(false)
	rr := httptest.NewRecorder()
	h.HTTPErrorMessage(rr, "custom safe message", http.StatusBadRequest)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	if rr.Body.String() == "" {
		t.Error("expected non-empty body")
	}
}
