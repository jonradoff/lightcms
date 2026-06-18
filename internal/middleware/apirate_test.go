package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func doReq(h http.Handler, token string, bodySize int) int {
	var body *strings.Reader
	if bodySize > 0 {
		body = strings.NewReader(strings.Repeat("x", bodySize))
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Code
}

func TestAPIRateLimit(t *testing.T) {
	h := APIRateLimit(okHandler())

	// No token → passthrough.
	if code := doReq(h, "", 0); code != http.StatusOK {
		t.Errorf("no-token passthrough: got %d", code)
	}

	tok := "ratekey-unique-1"
	// Up to the limit should pass.
	for i := 0; i < apiRateLimit; i++ {
		if code := doReq(h, tok, 0); code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, code)
		}
	}
	// The next request exceeds the window limit.
	if code := doReq(h, tok, 0); code != http.StatusTooManyRequests {
		t.Errorf("over-limit: expected 429, got %d", code)
	}
}

func TestAPIBurstRateLimit(t *testing.T) {
	h := APIBurstRateLimit(okHandler())
	if code := doReq(h, "", 0); code != http.StatusOK {
		t.Errorf("no-token passthrough: got %d", code)
	}
	tok := "burstkey-unique-1"
	for i := 0; i < burstRateLimit; i++ {
		if code := doReq(h, tok, 0); code != http.StatusOK {
			t.Fatalf("burst request %d: expected 200, got %d", i, code)
		}
	}
	if code := doReq(h, tok, 0); code != http.StatusTooManyRequests {
		t.Errorf("over-burst: expected 429, got %d", code)
	}
}

func TestAPIBodySizeLimit(t *testing.T) {
	// The handler reads the (capped) body; small bodies pass through fine.
	h := APIBodySizeLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 32)
		_, _ = r.Body.Read(buf)
		w.WriteHeader(http.StatusOK)
	}))
	if code := doReq(h, "tok", 16); code != http.StatusOK {
		t.Errorf("small body: got %d", code)
	}
}

func TestEndpointRateLimiters(t *testing.T) {
	// Each constructor returns working middleware. reindex=1/min makes the
	// second call rate-limited deterministically.
	mw := ReindexLimiter()
	h := mw(okHandler())
	tok := "endpointkey-unique-1"
	if code := doReq(h, tok, 0); code != http.StatusOK {
		t.Fatalf("first reindex: expected 200, got %d", code)
	}
	if code := doReq(h, tok, 0); code != http.StatusTooManyRequests {
		t.Errorf("second reindex: expected 429, got %d", code)
	}

	// No token → passthrough (not limited).
	if code := doReq(h, "", 0); code != http.StatusOK {
		t.Errorf("no-token endpoint: got %d", code)
	}

	// Smoke: the other constructors build valid middleware.
	for _, build := range []func() func(http.Handler) http.Handler{
		RegenerateLimiter, SearchReplaceExecuteLimiter, AssetFromURLLimiter,
		BulkUpdateLimiter, ExportLimiter, CommentCreateLimiter,
	} {
		if build()(okHandler()) == nil {
			t.Error("nil middleware from constructor")
		}
	}
}

func TestEndpointRateLimiter_CheckAndPrune(t *testing.T) {
	e := newEndpointRateLimiter(2)
	tok := "checkkey-1"
	if e.check(tok) {
		t.Error("1st check should not be limited")
	}
	if e.check(tok) {
		t.Error("2nd check should not be limited")
	}
	if !e.check(tok) {
		t.Error("3rd check should be limited")
	}
	// prune is safe to call and clears nothing recent.
	e.prune()
	if _, ok := e.tokens[tok]; !ok {
		t.Error("recent token should survive prune")
	}
}

func TestPruneAPIKeyRateLimiter(t *testing.T) {
	// Seed an entry and prune; just verify it runs without panicking.
	apiKeyRateLimiter.Lock()
	apiKeyRateLimiter.tokens["prunekey"] = nil
	apiKeyRateLimiter.Unlock()
	pruneAPIKeyRateLimiter()
}
