package handlers

import (
	"net/http"
	"testing"
)

// --- Imports ---

func TestAPIImports_Endpoints(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	if rr := doJSON(t, ah.APIListImportSources, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
		t.Fatalf("APIListImportSources: got %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := doJSON(t, ah.APIListImportJobs, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
		t.Fatalf("APIListImportJobs: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Create source — missing url => 400
	if rr := doJSON(t, ah.APICreateImportSource, http.MethodPost, map[string]interface{}{"name": "Feed"}, nil); rr.Code != http.StatusBadRequest {
		t.Errorf("APICreateImportSource missing url: expected 400, got %d", rr.Code)
	}
	// Create source — full body
	if rr := doJSON(t, ah.APICreateImportSource, http.MethodPost, map[string]interface{}{
		"name": "Feed", "url": "https://example.com/rss", "template_name": "Blog Post", "schedule": "daily",
	}, nil); rr.Code >= 500 {
		t.Errorf("APICreateImportSource: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Invalid IDs => 400
	if rr := doJSON(t, ah.APIDeleteImportSource, http.MethodDelete, nil, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIDeleteImportSource bad id: expected 400, got %d", rr.Code)
	}
	if rr := doJSON(t, ah.APIGetImportJob, http.MethodGet, nil, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIGetImportJob bad id: expected 400, got %d", rr.Code)
	}
	if rr := doJSON(t, ah.APITriggerImportSource, http.MethodPost, nil, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APITriggerImportSource bad id: expected 400, got %d", rr.Code)
	}
}

// --- Approvals ---

func TestAPIApprovals_Endpoints(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	for name, h := range map[string]http.HandlerFunc{
		"APIListUsers":             ah.APIListUsers,
		"APIListApprovalWorkflows": ah.APIListApprovalWorkflows,
		"APIListApprovalRequests":  ah.APIListApprovalRequests,
	} {
		if rr := doJSON(t, h, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
			t.Errorf("%s: got %d (%s)", name, rr.Code, rr.Body.String())
		}
	}

	// Create workflow — missing name => 400
	if rr := doJSON(t, ah.APICreateApprovalWorkflow, http.MethodPost, map[string]interface{}{}, nil); rr.Code != http.StatusBadRequest {
		t.Errorf("APICreateApprovalWorkflow missing name: expected 400, got %d", rr.Code)
	}
	// Create workflow — valid
	rr := doJSON(t, ah.APICreateApprovalWorkflow, http.MethodPost, map[string]interface{}{
		"name": "Default", "trigger": "all", "mode": "any",
	}, nil)
	if rr.Code >= 500 {
		t.Fatalf("APICreateApprovalWorkflow: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Invalid IDs
	if rr := doJSON(t, ah.APIGetApprovalWorkflow, http.MethodGet, nil, map[string]string{"id": "bad"}); rr.Code >= 500 {
		t.Errorf("APIGetApprovalWorkflow bad id: got %d", rr.Code)
	}
	if rr := doJSON(t, ah.APIDeleteApprovalWorkflow, http.MethodDelete, nil, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIDeleteApprovalWorkflow bad id: expected 400, got %d", rr.Code)
	}
	if rr := doJSON(t, ah.APIGetApprovalRequest, http.MethodGet, nil, map[string]string{"id": "bad"}); rr.Code >= 500 {
		t.Errorf("APIGetApprovalRequest bad id: got %d", rr.Code)
	}
}

// --- Schedule ---

func TestAPISchedule_Endpoints(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	if rr := doJSON(t, ah.APIListScheduledContent, http.MethodGet, nil, nil); rr.Code != http.StatusOK {
		t.Fatalf("APIListScheduledContent: got %d (%s)", rr.Code, rr.Body.String())
	}

	tmpl := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmpl, "Doc", "doc", "/doc").Hex()

	// Schedule a publish
	if rr := doJSON(t, ah.APIScheduleContentPublish, http.MethodPost, map[string]interface{}{
		"publish_at": "2099-01-01T00:00:00Z",
	}, map[string]string{"id": contentID}); rr.Code >= 500 {
		t.Errorf("APIScheduleContentPublish: got %d (%s)", rr.Code, rr.Body.String())
	}

	// Invalid content id
	if rr := doJSON(t, ah.APIScheduleContentPublish, http.MethodPost, map[string]interface{}{"publish_at": "2099-01-01T00:00:00Z"}, map[string]string{"id": "bad"}); rr.Code != http.StatusBadRequest {
		t.Errorf("APIScheduleContentPublish bad id: expected 400, got %d", rr.Code)
	}
}
