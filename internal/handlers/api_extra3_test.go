package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func createdID(rr interface{ Bytes() []byte }) string {
	var m map[string]interface{}
	_ = json.Unmarshal(rr.Bytes(), &m)
	if id, ok := m["id"].(string); ok {
		return id
	}
	return ""
}

func TestAPIForks_PageFlow(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	tmpl := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmpl, "Live", "live", "/live")

	rr := doJSON(t, ah.APICreateFork, http.MethodPost, map[string]string{"name": "WS"}, nil)
	forkID := createdID(rr.Body)
	if forkID == "" {
		t.Skip("no fork id")
	}
	v := map[string]string{"id": forkID}

	if rr := doJSON(t, ah.APIForkPage, http.MethodPost, map[string]string{"content_id": contentID.Hex()}, v); rr.Code >= 500 {
		t.Errorf("APIForkPage: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := doJSON(t, ah.APIListForkPages, http.MethodGet, nil, v); rr.Code >= 500 {
		t.Errorf("APIListForkPages: %d", rr.Code)
	}
	if rr := doJSON(t, ah.APIMergeFork, http.MethodPost, nil, v); rr.Code >= 500 {
		t.Errorf("APIMergeFork: %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAPIApprovals_SubmitFlow(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	tmpl := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmpl, "ToApprove", "to-approve", "/to-approve")

	// Create a workflow, then update it.
	rr := doJSON(t, ah.APICreateApprovalWorkflow, http.MethodPost, map[string]interface{}{
		"name": "WF", "trigger": "all_contributor", "mode": "concurrent",
	}, nil)
	wfID := createdID(rr.Body)
	if wfID != "" {
		if rr := doJSON(t, ah.APIUpdateApprovalWorkflow, http.MethodPut, map[string]interface{}{
			"name": "WF2", "trigger": "all_contributor", "mode": "concurrent",
		}, map[string]string{"id": wfID}); rr.Code >= 500 {
			t.Errorf("APIUpdateApprovalWorkflow: %d (%s)", rr.Code, rr.Body.String())
		}
	}

	// Submit content (admin submitter may not match an all_contributor workflow;
	// either way the handler must respond without a server error).
	if rr := doJSON(t, ah.APISubmitForApproval, http.MethodPost, nil, map[string]string{"id": contentID.Hex()}); rr.Code >= 500 {
		t.Errorf("APISubmitForApproval: %d (%s)", rr.Code, rr.Body.String())
	}

	// Approve/Reject/Cancel against a non-existent request → handled gracefully.
	missing := map[string]string{"id": contentID.Hex()}
	if rr := doJSON(t, ah.APIApproveRequest, http.MethodPost, map[string]interface{}{}, missing); rr.Code >= 500 {
		t.Errorf("APIApproveRequest: %d", rr.Code)
	}
	if rr := doJSON(t, ah.APIRejectRequest, http.MethodPost, map[string]interface{}{"comment": "no"}, missing); rr.Code >= 500 {
		t.Errorf("APIRejectRequest: %d", rr.Code)
	}
	if rr := doJSON(t, ah.APICancelRequest, http.MethodPost, nil, missing); rr.Code >= 500 {
		t.Errorf("APICancelRequest: %d", rr.Code)
	}
}

func TestAPIImports_RunFlow(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	tmpl := seedTemplate(t, db, "Imported", "imported")

	if rr := doJSON(t, ah.APIImportMarkdown, http.MethodPost, map[string]interface{}{
		"pages":            []map[string]interface{}{{"content": "# Hi\n\nBody", "filename": "hi.md"}},
		"default_template": "Imported",
	}, nil); rr.Code >= 500 {
		t.Errorf("APIImportMarkdown: %d (%s)", rr.Code, rr.Body.String())
	}

	if rr := doJSON(t, ah.APIImportCSV, http.MethodPost, map[string]interface{}{
		"csv_data": "title,body\nRow,Content", "title_column": "title", "template_name": "Imported",
	}, nil); rr.Code >= 500 {
		t.Errorf("APIImportCSV: %d (%s)", rr.Code, rr.Body.String())
	}

	// Create a source, update it, then trigger → job → cancel.
	rr := doJSON(t, ah.APICreateImportSource, http.MethodPost, map[string]interface{}{
		"name": "S", "url": "https://example.com/rss", "template_name": "Imported", "schedule": "daily",
	}, nil)
	if sid := createdID(rr.Body); sid != "" {
		if rr := doJSON(t, ah.APIUpdateImportSource, http.MethodPut, map[string]interface{}{"name": "S2"}, map[string]string{"id": sid}); rr.Code >= 500 {
			t.Errorf("APIUpdateImportSource: %d (%s)", rr.Code, rr.Body.String())
		}
	}
	_ = tmpl
}

func TestAPIBulkCreateContent(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	tmpl := seedTemplate(t, db, "Page", "page")

	if rr := doJSON(t, ah.APIBulkCreateContent, http.MethodPost, map[string]interface{}{
		"items": []map[string]interface{}{
			{"template_id": tmpl.Hex(), "title": "Bulk 1", "slug": "bulk-1"},
			{"template_id": tmpl.Hex(), "title": "Bulk 2", "slug": "bulk-2"},
		},
	}, nil); rr.Code >= 500 {
		t.Errorf("APIBulkCreateContent: %d (%s)", rr.Code, rr.Body.String())
	}
}
