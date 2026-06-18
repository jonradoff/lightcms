package handlers

import (
	"net/http"
	"testing"
)

// TestAPIContentDeep drives the data-heavy API content handlers (search/replace,
// bulk update, bulk field op, export, scoped replace) against seeded content so
// their match/processing branches execute.
func TestAPIContentDeep(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	tmpl := seedTemplate(t, db, "Page", "page")
	c1 := seedContent(t, db, tmpl, "About Golang", "golang", "/golang")
	seedContent(t, db, tmpl, "More Golang", "more", "/more")

	// Search/replace with a matching term.
	if rr := doJSON(t, ah.APISearchReplacePreview, http.MethodPost, map[string]interface{}{
		"search": "Golang", "replace": "Go",
	}, nil); rr.Code >= 500 {
		t.Errorf("APISearchReplacePreview: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := doJSON(t, ah.APISearchReplaceExecute, http.MethodPost, map[string]interface{}{
		"search": "Golang", "replace": "Go", "version_comment": "rename", "auto_republish": false,
	}, nil); rr.Code >= 500 {
		t.Errorf("APISearchReplaceExecute: %d (%s)", rr.Code, rr.Body.String())
	}

	// Multi-pair search/replace.
	if rr := doJSON(t, ah.APISearchReplacePreview, http.MethodPost, map[string]interface{}{
		"pairs": []map[string]string{{"search": "About", "replace": "Re:"}, {"search": "More", "replace": "Extra"}},
	}, nil); rr.Code >= 500 {
		t.Errorf("APISearchReplacePreview(pairs): %d", rr.Code)
	}

	// Scoped search/replace.
	if rr := doJSON(t, ah.APIScopedSearchReplacePreview, http.MethodPost, map[string]interface{}{
		"search": "Go", "replace": "Golang", "scope": map[string]interface{}{"path_prefix": "/"},
	}, nil); rr.Code >= 500 {
		t.Errorf("APIScopedSearchReplacePreview: %d (%s)", rr.Code, rr.Body.String())
	}

	// Bulk update (dry run) referencing a real id.
	if rr := doJSON(t, ah.APIBulkUpdateContent, http.MethodPost, map[string]interface{}{
		"dry_run":         true,
		"version_comment": "bulk",
		"updates":         []map[string]interface{}{{"id": c1.Hex(), "title": "Renamed"}},
	}, nil); rr.Code >= 500 {
		t.Errorf("APIBulkUpdateContent: %d (%s)", rr.Code, rr.Body.String())
	}

	// Bulk field operation (dry run).
	if rr := doJSON(t, ah.APIBulkFieldOperation, http.MethodPost, map[string]interface{}{
		"dry_run": true, "field": "category", "operation": "set", "scope": map[string]interface{}{},
	}, nil); rr.Code >= 500 {
		t.Errorf("APIBulkFieldOperation: %d (%s)", rr.Code, rr.Body.String())
	}

	// Export.
	if rr := doJSON(t, ah.APIExportContent, http.MethodPost, map[string]interface{}{}, nil); rr.Code >= 500 {
		t.Errorf("APIExportContent: %d (%s)", rr.Code, rr.Body.String())
	}

	// Backlinks for a path.
	if rr := doJSON(t, ah.APIGetBacklinks, http.MethodGet, nil, nil); rr.Code >= 500 {
		t.Errorf("APIGetBacklinks: %d", rr.Code)
	}
}
