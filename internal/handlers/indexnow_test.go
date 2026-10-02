package handlers

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/services"
)

func TestServeIndexNowKey(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	in := services.NewIndexNowService(h.db, "https://metavert.io", false)
	h.SetIndexNowService(in)
	key := in.KeyForFile(context.Background())
	if len(key) != 32 {
		t.Fatalf("key = %q", key)
	}

	rr := httptest.NewRecorder()
	h.ServeIndexNowKey(rr, sessionReq("GET", "/"+key+".txt", nil, map[string]string{"indexnowkey": key}))
	if rr.Code != 200 || rr.Body.String() != key || !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("key file: status=%d body=%q ct=%q", rr.Code, rr.Body.String(), rr.Header().Get("Content-Type"))
	}

	// A different *.txt path falls through to page serving (404 here), never leaks the key.
	rr = httptest.NewRecorder()
	h.ServeIndexNowKey(rr, sessionReq("GET", "/notthekey123.txt", nil, map[string]string{"indexnowkey": "notthekey123"}))
	if rr.Code == 200 && strings.Contains(rr.Body.String(), key) {
		t.Error("non-matching path served the key")
	}
}

func TestIndexNowToolPage(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	// Dev-mode service: page must explain why it's inactive.
	h.SetIndexNowService(services.NewIndexNowService(h.db, "http://localhost:8082", true))

	rr := httptest.NewRecorder()
	h.IndexNowToolPage(rr, sessionReq("GET", "/cm/tools/indexnow", nil, nil))
	body := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(body, "IndexNow") || !strings.Contains(body, "development mode") {
		t.Fatalf("status=%d, missing inactive explanation", rr.Code)
	}

	// Toggle off via form POST → redirect with saved flag.
	form := url.Values{"action": {"save"}}
	req := sessionReq("POST", "/cm/tools/indexnow", strings.NewReader(form.Encode()), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.IndexNowToolAction(rr, req)
	if rr.Code != 303 || !strings.Contains(rr.Header().Get("Location"), "saved=1") {
		t.Errorf("save: status=%d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	st, _ := h.indexNowService.Status(context.Background())
	if st.Enabled {
		t.Error("expected disabled after unchecked save")
	}

	// Submit-all on an inactive install reports why.
	form = url.Values{"action": {"submit_all"}}
	req = sessionReq("POST", "/cm/tools/indexnow", strings.NewReader(form.Encode()), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.IndexNowToolAction(rr, req)
	if !strings.Contains(rr.Header().Get("Location"), "error=") {
		t.Errorf("submit_all on inactive install should error, loc=%s", rr.Header().Get("Location"))
	}

	// Unauthenticated → redirected away.
	rr = httptest.NewRecorder()
	h.IndexNowToolPage(rr, httptest.NewRequest("GET", "/cm/tools/indexnow", nil))
	if rr.Code != 303 {
		t.Errorf("unauth: status = %d", rr.Code)
	}
}
