package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAPIHandlers_InvalidID drives the invalid-ObjectID error branch of the
// many id-based API handlers (each rejects a non-hex id with 400), plus a few
// invalid-JSON-body branches.
func TestAPIHandlers_InvalidID(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	bad := map[string]string{"id": "not-a-valid-objectid"}

	idHandlers := map[string]http.HandlerFunc{
		"APIGetContent":              ah.APIGetContent,
		"APIDeleteContent":           ah.APIDeleteContent,
		"APIRestoreContent":          ah.APIRestoreContent,
		"APIPublishContent":          ah.APIPublishContent,
		"APIUnpublishContent":        ah.APIUnpublishContent,
		"APIListContentVersions":     ah.APIListContentVersions,
		"APIGetTemplate":             ah.APIGetTemplate,
		"APIDeleteTemplate":          ah.APIDeleteTemplate,
		"APIUpdateTemplate":          ah.APIUpdateTemplate,
		"APIGetAsset":                ah.APIGetAsset,
		"APIDeleteAsset":             ah.APIDeleteAsset,
		"APIGetCollection":           ah.APIGetCollection,
		"APIDeleteCollection":        ah.APIDeleteCollection,
		"APIUpdateCollection":        ah.APIUpdateCollection,
		"APIGetFolder":               ah.APIGetFolder,
		"APIDeleteFolder":            ah.APIDeleteFolder,
		"APIGetSnippet":              ah.APIGetSnippet,
		"APIDeleteSnippet":           ah.APIDeleteSnippet,
		"APIUpdateSnippet":           ah.APIUpdateSnippet,
		"APIGetRedirect":             ah.APIGetRedirect,
		"APIDeleteRedirect":          ah.APIDeleteRedirect,
		"APIUpdateRedirect":          ah.APIUpdateRedirect,
		"APIDeleteAPIKey":            ah.APIDeleteAPIKey,
		"APIGetFork":                 ah.APIGetFork,
		"APIArchiveFork":             ah.APIArchiveFork,
		"APIDeleteFork":              ah.APIDeleteFork,
		"APIGetImportJob":            ah.APIGetImportJob,
		"APICancelImportJob":         ah.APICancelImportJob,
		"APIDeleteImportSource":      ah.APIDeleteImportSource,
		"APITriggerImportSource":     ah.APITriggerImportSource,
		"APIDeleteWebhook":           ah.APIDeleteWebhook,
		"APIRegenerateWebhookSecret": ah.APIRegenerateWebhookSecret,
		"APIListWebhookDeliveries":   ah.APIListWebhookDeliveries,
		"APIGetContentLock":          ah.APIGetContentLock,
		"APIAcquireContentLock":      ah.APIAcquireContentLock,
		"APIGetApprovalWorkflow":     ah.APIGetApprovalWorkflow,
		"APIDeleteApprovalWorkflow":  ah.APIDeleteApprovalWorkflow,
		"APIGetApprovalRequest":      ah.APIGetApprovalRequest,
		"APICancelRequest":           ah.APICancelRequest,
		"APIGetLinkCheckJob":         ah.APIGetLinkCheckJob,
	}
	for name, h := range idHandlers {
		t.Run(name, func(t *testing.T) {
			rr := doJSON(t, h, http.MethodGet, nil, bad)
			if rr.Code >= 500 {
				t.Errorf("%s with bad id: server error %d", name, rr.Code)
			}
		})
	}
}

// TestAPIHandlers_InvalidBody drives the invalid-JSON-body branch of POST/PUT
// handlers (malformed body → 400).
func TestAPIHandlers_InvalidBody(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	bodyHandlers := map[string]http.HandlerFunc{
		"APICreateContent":          ah.APICreateContent,
		"APICreateTemplate":         ah.APICreateTemplate,
		"APICreateCollection":       ah.APICreateCollection,
		"APICreateFolder":           ah.APICreateFolder,
		"APICreateRedirect":         ah.APICreateRedirect,
		"APICreateSnippet":          ah.APICreateSnippet,
		"APICreateFork":             ah.APICreateFork,
		"APICreateWebhook":          ah.APICreateWebhook,
		"APICreateImportSource":     ah.APICreateImportSource,
		"APICreateApprovalWorkflow": ah.APICreateApprovalWorkflow,
		"APIBatchPublishContent":    ah.APIBatchPublishContent,
		"APIBulkCreateContent":      ah.APIBulkCreateContent,
		"APIExportContent":          ah.APIExportContent,
		"APIUpdateTheme":            ah.APIUpdateTheme,
		"APIUpdateSiteConfig":       ah.APIUpdateSiteConfig,
	}
	for name, h := range bodyHandlers {
		t.Run(name, func(t *testing.T) {
			req := authReq(http.MethodPost, "/", strings.NewReader("{ this is not valid json "))
			rr := httptest.NewRecorder()
			h(rr, req)
			if rr.Code >= 500 {
				t.Errorf("%s with bad body: server error %d", name, rr.Code)
			}
		})
	}
}
