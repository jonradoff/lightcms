package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIAuth_MissingAuthorizationHeader(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, nil
	})
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] == nil {
		t.Error("expected error in response body")
	}
}

func TestAPIAuth_MalformedAuthorizationHeader(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, nil
	})
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		header string
	}{
		{"no space", "BearerToken123"},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"empty token", "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			req.Header.Set("Authorization", tt.header)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", rr.Code)
			}
		})
	}
}

func TestAPIAuth_ValidAPIKey(t *testing.T) {
	called := false
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		if rawKey != "lc_testkey123" {
			return nil, fmt.Errorf("invalid key")
		}
		called = true
		return nil, nil
	})
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer lc_testkey123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Error("expected validate function to be called")
	}
}

func TestAPIAuth_InvalidAPIKey(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, fmt.Errorf("invalid key")
	})
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer lc_badkey")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAPIAuth_ValidOAuthToken(t *testing.T) {
	systemKey := "lc_system_key_12345678901234567890"
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, fmt.Errorf("not an API key")
	})
	m.SetOAuth(func(ctx context.Context, rawToken string) (interface{}, error) {
		if rawToken != "oauth_access_token_123" {
			return nil, fmt.Errorf("invalid token")
		}
		return nil, nil
	}, systemKey, "https://example.com/.well-known/oauth-protected-resource")

	var capturedAuth string
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer oauth_access_token_123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	// Verify system API key was substituted
	expected := "Bearer " + systemKey
	if capturedAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, capturedAuth)
	}
}

func TestAPIAuth_InvalidOAuthToken(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, fmt.Errorf("not an API key")
	})
	m.SetOAuth(func(ctx context.Context, rawToken string) (interface{}, error) {
		return nil, fmt.Errorf("invalid token")
	}, "lc_system", "https://example.com/.well-known/oauth-protected-resource")

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer bad_oauth_token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAPIAuth_OAuthNotConfigured_NonLCToken(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, fmt.Errorf("invalid key")
	})
	// OAuth NOT configured

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer some_random_token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAPIAuth_WWWAuthenticateHeader(t *testing.T) {
	resourceURL := "https://example.com/.well-known/oauth-protected-resource"
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, fmt.Errorf("invalid")
	})
	m.SetOAuth(func(ctx context.Context, rawToken string) (interface{}, error) {
		return nil, fmt.Errorf("invalid")
	}, "lc_system", resourceURL)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	wwwAuth := rr.Header().Get("WWW-Authenticate")
	if wwwAuth == "" {
		t.Error("expected WWW-Authenticate header to be set")
	}
	expected := `Bearer resource_metadata="` + resourceURL + `"`
	if wwwAuth != expected {
		t.Errorf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

func TestAPIAuth_NoWWWAuthenticateWithoutResourceMetadata(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, fmt.Errorf("invalid")
	})
	// No OAuth configured, no resource metadata URL

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	wwwAuth := rr.Header().Get("WWW-Authenticate")
	if wwwAuth != "" {
		t.Errorf("expected no WWW-Authenticate header, got %q", wwwAuth)
	}
}

func TestAPIAuth_CaseInsensitiveBearer(t *testing.T) {
	m := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		return nil, nil
	})
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "bearer lc_testkey123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for lowercase bearer, got %d", rr.Code)
	}
}
