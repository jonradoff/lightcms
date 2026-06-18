package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// redirectTransport rewrites every request to point at a test server's host,
// letting us exercise CloudflareService's hardcoded api.cloudflare.com calls.
type redirectTransport struct{ target *url.URL }

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func cfService(t *testing.T, handler http.HandlerFunc, zone, token string, enabled bool) *CloudflareService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	svc := NewCloudflareService(func() (string, string, bool) { return zone, token, enabled }, "https://example.com")
	svc.client.Transport = redirectTransport{target: u}
	return svc
}

func TestCloudflare_PurgeByURLs_Disabled(t *testing.T) {
	called := false
	svc := cfService(t, func(w http.ResponseWriter, r *http.Request) { called = true }, "z", "tok", false)
	if err := svc.PurgeByURLs(context.Background(), []string{"/a"}); err != nil {
		t.Fatalf("disabled purge should be a no-op, got %v", err)
	}
	if called {
		t.Error("expected no HTTP call when disabled")
	}
}

func TestCloudflare_PurgeByURLs_MissingCreds(t *testing.T) {
	svc := cfService(t, func(w http.ResponseWriter, r *http.Request) {}, "", "", true)
	if err := svc.PurgeByURLs(context.Background(), []string{"/a"}); err != nil {
		t.Fatalf("missing creds should be a no-op, got %v", err)
	}
}

func TestCloudflare_PurgeByURLs_Success(t *testing.T) {
	svc := cfService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad auth header: %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}, "z", "tok", true)
	if err := svc.PurgeByURLs(context.Background(), []string{"/a", "/b"}); err != nil {
		t.Fatalf("PurgeByURLs: %v", err)
	}
}

func TestCloudflare_PurgeByURLs_HTTPError(t *testing.T) {
	svc := cfService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}, "z", "tok", true)
	if err := svc.PurgeByURLs(context.Background(), []string{"/a"}); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestCloudflare_PurgeByURLs_SuccessFalse(t *testing.T) {
	svc := cfService(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}, "z", "tok", true)
	if err := svc.PurgeByURLs(context.Background(), []string{"/a"}); err == nil {
		t.Fatal("expected error when success=false")
	}
}

func TestCloudflare_PurgeEverything(t *testing.T) {
	svc := cfService(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}, "z", "tok", true)
	if err := svc.PurgeEverything(context.Background()); err != nil {
		t.Fatalf("PurgeEverything: %v", err)
	}

	// Disabled is a no-op.
	svc2 := cfService(t, func(w http.ResponseWriter, r *http.Request) {}, "z", "tok", false)
	if err := svc2.PurgeEverything(context.Background()); err != nil {
		t.Fatalf("disabled PurgeEverything: %v", err)
	}
}

func TestCloudflare_TestConnection(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		success bool
		zone    string
		token   string
		wantErr bool
	}{
		{"missing creds", 0, false, "", "", true},
		{"ok", http.StatusOK, true, "z", "tok", false},
		{"ok but success false", http.StatusOK, false, "z", "tok", true},
		{"unauthorized", http.StatusUnauthorized, false, "z", "tok", true},
		{"forbidden", http.StatusForbidden, false, "z", "tok", true},
		{"not found", http.StatusNotFound, false, "z", "tok", true},
		{"server error", http.StatusInternalServerError, false, "z", "tok", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := cfService(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				json.NewEncoder(w).Encode(map[string]bool{"success": tc.success})
			}, tc.zone, tc.token, true)
			err := svc.TestConnection(context.Background())
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}
