package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPIAuth_SessionFallback covers SetSessionAuth and the session-cookie
// fallback branch (no Authorization header, valid session).
func TestAPIAuth_SessionFallback(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		t.Error("API key validator should not be called for session auth")
		return nil, nil
	})
	m.SetSessionAuth(func(r *http.Request) interface{} {
		if _, err := r.Cookie("session"); err == nil {
			return "session-user"
		}
		return nil
	})

	var gotUser interface{}
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, _ = APIUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Valid session cookie → passes through with user in context.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with session fallback, got %d", rr.Code)
	}
	if gotUser != "session-user" {
		t.Errorf("expected session user in context, got %v", gotUser)
	}

	// No cookie → session validator returns nil → 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session, got %d", rr.Code)
	}
}

// TestInjectAPIUser covers the test-only context injection helper.
func TestInjectAPIUser(t *testing.T) {
	ctx := InjectAPIUser(context.Background(), "someone")
	user, ok := APIUserFromContext(ctx)
	if !ok || user != "someone" {
		t.Errorf("InjectAPIUser: got %v ok=%v", user, ok)
	}
}
