package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// doJSON invokes an APIHandler method with an admin-authed JSON request and
// optional mux path vars, returning the recorder.
func doJSON(t *testing.T, h http.HandlerFunc, method string, body interface{}, vars map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := authReq(method, "/", rdr)
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// --- Forks ---

func TestAPIForks_Lifecycle(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// List (empty)
	if rr := doJSON(t, ah.APIListForks, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
		t.Fatalf("APIListForks: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Create
	rr := doJSON(t, ah.APICreateFork, http.MethodPost, map[string]string{"name": "Campaign", "description": "d"}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("APICreateFork: got %d (%s)", rr.Code, rr.Body.String())
	}
	var created map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &created)
	forkID, _ := created["id"].(string)
	if forkID == "" {
		t.Fatal("APICreateFork: no id in response")
	}

	// Create with missing name → 400
	if rr := doJSON(t, ah.APICreateFork, http.MethodPost, map[string]string{}, nil); rr.Code != http.StatusBadRequest {
		t.Errorf("APICreateFork missing name: expected 400, got %d", rr.Code)
	}

	// Get
	if rr := doJSON(t, ah.APIGetFork, http.MethodGet, nil, map[string]string{"id": forkID}); rr.Code != http.StatusOK {
		t.Errorf("APIGetFork: got %d (%s)", rr.Code, rr.Body.String())
	}
	// Get invalid id → 400
	if rr := doJSON(t, ah.APIGetFork, http.MethodGet, nil, map[string]string{"id": "notanid"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIGetFork invalid id: expected 400, got %d", rr.Code)
	}

	// List pages
	if rr := doJSON(t, ah.APIListForkPages, http.MethodGet, nil, map[string]string{"id": forkID}); rr.Code != http.StatusOK {
		t.Errorf("APIListForkPages: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Archive
	if rr := doJSON(t, ah.APIArchiveFork, http.MethodPost, nil, map[string]string{"id": forkID}); rr.Code != http.StatusOK {
		t.Errorf("APIArchiveFork: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Delete
	if rr := doJSON(t, ah.APIDeleteFork, http.MethodDelete, nil, map[string]string{"id": forkID}); rr.Code != http.StatusOK {
		t.Errorf("APIDeleteFork: got %d (%s)", rr.Code, rr.Body.String())
	}
}

// --- Webhooks ---

func TestAPIWebhooks_Lifecycle(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	if rr := doJSON(t, ah.APIListWebhooks, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
		t.Fatalf("APIListWebhooks: got %d (%s)", rr.Code, rr.Body.String())
	}

	rr := doJSON(t, ah.APICreateWebhook, http.MethodPost, map[string]interface{}{
		"name": "hook", "url": "https://example.com/h", "events": []string{"content.published"}, "active": true,
	}, nil)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("APICreateWebhook: got %d (%s)", rr.Code, rr.Body.String())
	}
	var created map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("APICreateWebhook: no id")
	}

	if rr := doJSON(t, ah.APIUpdateWebhook, http.MethodPut, map[string]interface{}{
		"name": "hook2", "url": "https://example.com/h2", "events": []string{"content.deleted"}, "active": false,
	}, map[string]string{"id": id}); rr.Code != http.StatusOK {
		t.Errorf("APIUpdateWebhook: got %d (%s)", rr.Code, rr.Body.String())
	}

	if rr := doJSON(t, ah.APIRegenerateWebhookSecret, http.MethodPost, nil, map[string]string{"id": id}); rr.Code != http.StatusOK {
		t.Errorf("APIRegenerateWebhookSecret: got %d (%s)", rr.Code, rr.Body.String())
	}

	if rr := doJSON(t, ah.APIListWebhookDeliveries, http.MethodGet, nil, map[string]string{"id": id}); rr.Code != http.StatusOK {
		t.Errorf("APIListWebhookDeliveries: got %d (%s)", rr.Code, rr.Body.String())
	}

	if rr := doJSON(t, ah.APIDeleteWebhook, http.MethodDelete, nil, map[string]string{"id": id}); rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("APIDeleteWebhook: got %d (%s)", rr.Code, rr.Body.String())
	}

	// invalid id on update → 400
	if rr := doJSON(t, ah.APIUpdateWebhook, http.MethodPut, map[string]interface{}{"name": "x"}, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIUpdateWebhook bad id: expected 400, got %d", rr.Code)
	}
}

// --- Comments ---

func TestAPIComments_Lifecycle(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	tmpl := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmpl, "Doc", "doc", "/doc")

	if rr := doJSON(t, ah.APIListComments, http.MethodGet, nil, map[string]string{"id": contentID.Hex()}); rr.Code != http.StatusOK {
		t.Fatalf("APIListComments: got %d (%s)", rr.Code, rr.Body.String())
	}

	rr := doJSON(t, ah.APICreateComment, http.MethodPost, map[string]interface{}{"text": "nice page"}, map[string]string{"id": contentID.Hex()})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("APICreateComment: got %d (%s)", rr.Code, rr.Body.String())
	}
	var created map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &created)
	cid, _ := created["id"].(string)

	// Create empty text → 400
	if rr := doJSON(t, ah.APICreateComment, http.MethodPost, map[string]interface{}{"text": ""}, map[string]string{"id": contentID.Hex()}); rr.Code != http.StatusBadRequest {
		t.Errorf("APICreateComment empty: expected 400, got %d", rr.Code)
	}

	if cid != "" {
		if rr := doJSON(t, ah.APIDeleteComment, http.MethodDelete, nil, map[string]string{"id": contentID.Hex(), "cid": cid}); rr.Code != http.StatusOK {
			t.Errorf("APIDeleteComment: got %d (%s)", rr.Code, rr.Body.String())
		}
	}
}

// --- Locks ---

func TestAPILocks_Lifecycle(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	tmpl := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmpl, "Doc", "doc", "/doc").Hex()

	// Acquire
	if rr := doJSON(t, ah.APIAcquireContentLock, http.MethodPost, nil, map[string]string{"id": contentID}); rr.Code != http.StatusOK {
		t.Fatalf("APIAcquireContentLock: got %d (%s)", rr.Code, rr.Body.String())
	}
	// Get
	if rr := doJSON(t, ah.APIGetContentLock, http.MethodGet, nil, map[string]string{"id": contentID}); rr.Code != http.StatusOK {
		t.Errorf("APIGetContentLock: got %d (%s)", rr.Code, rr.Body.String())
	}
	// Release
	if rr := doJSON(t, ah.APIReleaseContentLock, http.MethodDelete, nil, map[string]string{"id": contentID}); rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("APIReleaseContentLock: got %d (%s)", rr.Code, rr.Body.String())
	}
	// Force unlock
	if rr := doJSON(t, ah.APIForceUnlockContent, http.MethodDelete, nil, map[string]string{"id": contentID}); rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("APIForceUnlockContent: got %d (%s)", rr.Code, rr.Body.String())
	}
	// Invalid id
	if rr := doJSON(t, ah.APIGetContentLock, http.MethodGet, nil, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIGetContentLock bad id: expected 400, got %d", rr.Code)
	}
}

// --- Link check ---

func TestAPILinkCheck(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := doJSON(t, ah.APIStartLinkCheck, http.MethodPost, nil, nil)
	if rr.Code != http.StatusOK && rr.Code != http.StatusAccepted {
		t.Fatalf("APIStartLinkCheck: got %d (%s)", rr.Code, rr.Body.String())
	}
	var started map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &started)
	jobID, _ := started["job_id"].(string)
	if jobID == "" {
		jobID, _ = started["id"].(string)
	}
	if jobID != "" {
		if rr := doJSON(t, ah.APIGetLinkCheckJob, http.MethodGet, nil, map[string]string{"id": jobID}); rr.Code != http.StatusOK {
			t.Errorf("APIGetLinkCheckJob: got %d (%s)", rr.Code, rr.Body.String())
		}
	}
	// invalid job id → 400
	if rr := doJSON(t, ah.APIGetLinkCheckJob, http.MethodGet, nil, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIGetLinkCheckJob bad id: expected 400, got %d", rr.Code)
	}
}

// --- Audit ---

func TestAPIListAuditLogs(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()
	if rr := doJSON(t, ah.APIListAuditLogs, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
		t.Fatalf("APIListAuditLogs: got %d (%s)", rr.Code, rr.Body.String())
	}
}
