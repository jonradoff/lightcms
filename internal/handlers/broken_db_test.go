package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

const validHex = "0123456789abcdef01234567"

// callAPI invokes an API handler with an admin auth context, optional JSON body,
// and optional path vars; it only requires the handler not to panic.
func callAPI(t *testing.T, name string, h http.HandlerFunc, method, body string, vars map[string]string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		var r *http.Request
		if body != "" {
			r = authReq(method, "/api/v1/x", strings.NewReader(body))
		} else {
			r = authReq(method, "/api/v1/x", nil)
		}
		if vars != nil {
			r = mux.SetURLVars(r, vars)
		}
		rr := httptest.NewRecorder()
		h(rr, r)
		if rr.Code == 0 {
			t.Errorf("%s: handler wrote no response", name)
		}
	})
}

// TestBrokenDB_APIHandlers drives the API handlers against a dead database so
// their database-error branches execute.
func TestBrokenDB_APIHandlers(t *testing.T) {
	ah := newBrokenAPIHandler(t)
	id := map[string]string{"id": validHex}

	// List/get handlers that hit the DB immediately.
	callAPI(t, "APIListContent", ah.APIListContent, "GET", "", nil)
	callAPI(t, "APIListTemplates", ah.APIListTemplates, "GET", "", nil)
	callAPI(t, "APIListAssets", ah.APIListAssets, "GET", "", nil)
	callAPI(t, "APIListAssetFolders", ah.APIListAssetFolders, "GET", "", nil)
	callAPI(t, "APIListSnippets", ah.APIListSnippets, "GET", "", nil)
	callAPI(t, "APIListForks", ah.APIListForks, "GET", "", nil)
	callAPI(t, "APIListWebhooks", ah.APIListWebhooks, "GET", "", nil)
	callAPI(t, "APIListImportSources", ah.APIListImportSources, "GET", "", nil)
	callAPI(t, "APIListImportJobs", ah.APIListImportJobs, "GET", "", nil)
	callAPI(t, "APIListApprovalWorkflows", ah.APIListApprovalWorkflows, "GET", "", nil)
	callAPI(t, "APIListApprovalRequests", ah.APIListApprovalRequests, "GET", "", nil)
	callAPI(t, "APIListUsers", ah.APIListUsers, "GET", "", nil)
	callAPI(t, "APIListAuditLogs", ah.APIListAuditLogs, "GET", "", nil)
	callAPI(t, "APIListScheduledContent", ah.APIListScheduledContent, "GET", "", nil)
	callAPI(t, "APIListAPIKeys", ah.APIListAPIKeys, "GET", "", nil)
	callAPI(t, "APIListCollections", ah.APIListCollections, "GET", "", nil)
	callAPI(t, "APIListFolders", ah.APIListFolders, "GET", "", nil)
	callAPI(t, "APIListRedirects", ah.APIListRedirects, "GET", "", nil)
	callAPI(t, "APIGetTheme", ah.APIGetTheme, "GET", "", nil)
	callAPI(t, "APIGetSiteConfig", ah.APIGetSiteConfig, "GET", "", nil)
	callAPI(t, "APIListThemeVersions", ah.APIListThemeVersions, "GET", "", nil)
	callAPI(t, "APIRegenerateAllContent", ah.APIRegenerateAllContent, "POST", "", nil)
	callAPI(t, "APIReindexEmbeddings", ah.APIReindexEmbeddings, "POST", "", nil)

	// id-based gets/deletes (valid hex → past parse → DB error).
	callAPI(t, "APIGetContent", ah.APIGetContent, "GET", "", id)
	callAPI(t, "APIDeleteContent", ah.APIDeleteContent, "DELETE", "", id)
	callAPI(t, "APIPublishContent", ah.APIPublishContent, "POST", "", id)
	callAPI(t, "APIGetTemplate", ah.APIGetTemplate, "GET", "", id)
	callAPI(t, "APIDeleteTemplate", ah.APIDeleteTemplate, "DELETE", "", id)
	callAPI(t, "APIGetAsset", ah.APIGetAsset, "GET", "", id)
	callAPI(t, "APIDeleteAsset", ah.APIDeleteAsset, "DELETE", "", id)
	callAPI(t, "APIGetCollection", ah.APIGetCollection, "GET", "", id)
	callAPI(t, "APIGetFolder", ah.APIGetFolder, "GET", "", id)
	callAPI(t, "APIGetSnippet", ah.APIGetSnippet, "GET", "", id)
	callAPI(t, "APIGetRedirect", ah.APIGetRedirect, "GET", "", id)
	callAPI(t, "APIGetFork", ah.APIGetFork, "GET", "", id)
	callAPI(t, "APIListForkPages", ah.APIListForkPages, "GET", "", id)
	callAPI(t, "APIGetImportJob", ah.APIGetImportJob, "GET", "", id)
	callAPI(t, "APIGetApprovalWorkflow", ah.APIGetApprovalWorkflow, "GET", "", id)
	callAPI(t, "APIGetApprovalRequest", ah.APIGetApprovalRequest, "GET", "", id)
	callAPI(t, "APIGetContentLock", ah.APIGetContentLock, "GET", "", id)
	callAPI(t, "APIAcquireContentLock", ah.APIAcquireContentLock, "POST", "", id)
	callAPI(t, "APIListComments", ah.APIListComments, "GET", "", id)
	callAPI(t, "APIListWebhookDeliveries", ah.APIListWebhookDeliveries, "GET", "", id)
	callAPI(t, "APISubmitForApproval", ah.APISubmitForApproval, "POST", "", id)

	// Creates with valid bodies (DB write fails).
	callAPI(t, "APICreateContent", ah.APICreateContent, "POST", `{"template_id":"`+validHex+`","title":"T"}`, nil)
	callAPI(t, "APICreateTemplate", ah.APICreateTemplate, "POST", `{"name":"T","slug":"t"}`, nil)
	callAPI(t, "APICreateSnippet", ah.APICreateSnippet, "POST", `{"name":"s","html":"<b>x</b>"}`, nil)
	callAPI(t, "APICreateCollection", ah.APICreateCollection, "POST", `{"name":"c","category":"x"}`, nil)
	callAPI(t, "APICreateFolder", ah.APICreateFolder, "POST", `{"name":"f","slug":"f"}`, nil)
	callAPI(t, "APICreateRedirect", ah.APICreateRedirect, "POST", `{"from_path":"/a","to_path":"/b"}`, nil)
	callAPI(t, "APICreateWebhook", ah.APICreateWebhook, "POST", `{"name":"w","url":"https://x.com","events":["content.published"]}`, nil)
	callAPI(t, "APICreateImportSource", ah.APICreateImportSource, "POST", `{"name":"s","url":"https://x.com/rss"}`, nil)
	callAPI(t, "APICreateApprovalWorkflow", ah.APICreateApprovalWorkflow, "POST", `{"name":"w","trigger":"all_contributor","mode":"concurrent"}`, nil)
	callAPI(t, "APICreateFork", ah.APICreateFork, "POST", `{"name":"F"}`, nil)
	callAPI(t, "APICreateAPIKey", ah.APICreateAPIKey, "POST", `{"name":"k"}`, nil)
	callAPI(t, "APISearchContent", ah.APISearchContent, "GET", "", nil)
	callAPI(t, "APIBatchPublishContent", ah.APIBatchPublishContent, "POST", `{"ids":["`+validHex+`"]}`, nil)
}

// callPage invokes a session-authed admin handler against the dead DB.
func callPage(t *testing.T, name string, h http.HandlerFunc, method string, vars map[string]string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		req := sessionReq(method, "/cm/x", nil, vars)
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code == 0 {
			t.Errorf("%s: handler wrote no response", name)
		}
	})
}

// TestBrokenDB_AdminHandlers drives the session admin handlers against a dead DB.
func TestBrokenDB_AdminHandlers(t *testing.T) {
	h := newBrokenHandler(t)
	id := map[string]string{"id": validHex}

	callPage(t, "AdminDashboard", h.AdminDashboard, "GET", nil)
	callPage(t, "ListContent", h.ListContent, "GET", nil)
	callPage(t, "ListTemplates", h.ListTemplates, "GET", nil)
	callPage(t, "ListCollections", h.ListCollections, "GET", nil)
	callPage(t, "ListFolders", h.ListFolders, "GET", nil)
	callPage(t, "ListRedirects", h.ListRedirects, "GET", nil)
	callPage(t, "UsersPage", h.UsersPage, "GET", nil)
	callPage(t, "WebhooksPage", h.WebhooksPage, "GET", nil)
	callPage(t, "ImportsPage", h.ImportsPage, "GET", nil)
	callPage(t, "AnalyticsPage", h.AnalyticsPage, "GET", nil)
	callPage(t, "AuditLogPage", h.AuditLogPage, "GET", nil)
	callPage(t, "ThemeSettings", h.ThemeSettings, "GET", nil)
	callPage(t, "ThemeVersions", h.ThemeVersions, "GET", nil)
	callPage(t, "SiteConfiguration", h.SiteConfiguration, "GET", nil)
	callPage(t, "APIKeysPage", h.APIKeysPage, "GET", nil)
	callPage(t, "ListForks", h.ListForks, "GET", nil)
	callPage(t, "ApprovalsPage", h.ApprovalsPage, "GET", nil)
	callPage(t, "AssetLibrary", h.AssetLibrary, "GET", nil)
	callPage(t, "ListContactMessages", h.ListContactMessages, "GET", nil)
	callPage(t, "EditContent", h.EditContent, "GET", id)
	callPage(t, "EditTemplate", h.EditTemplate, "GET", id)
	callPage(t, "EditCollection", h.EditCollection, "GET", id)
	callPage(t, "EditFolder", h.EditFolder, "GET", id)
	callPage(t, "EditUserPage", h.EditUserPage, "GET", id)
	callPage(t, "ViewFork", h.ViewFork, "GET", id)
	callPage(t, "ListContentVersions", h.ListContentVersions, "GET", id)
}
