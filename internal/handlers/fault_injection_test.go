package handlers

import (
	"net/http"
	"testing"

	"lightcms/internal/testutil"
)

// TestFaultInjection_APIHandlers exercises handler write-error branches: the
// handler reads an entity successfully, then the write fails. Seed with the
// hook cleared, then inject a failure for the relevant operation.
func TestFaultInjection_APIHandlers(t *testing.T) {
	ah, db := newFaultAPIHandler(t)
	tmpl := seedTemplate(t, db, "Page", "page")
	cid := seedContent(t, db, tmpl, "Doc", "doc", "/doc").Hex()
	idv := map[string]string{"id": cid}

	expectErr := func(name string, h http.HandlerFunc, method string, body interface{}, vars map[string]string) {
		rr := doJSON(t, h, method, body, vars)
		if rr.Code < 400 {
			t.Errorf("%s: expected error status when write fails, got %d", name, rr.Code)
		}
	}

	// Read ok, UpdateOne fails.
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	expectErr("APIUpdateContent", ah.APIUpdateContent, http.MethodPut, map[string]interface{}{"title": "New"}, idv)
	expectErr("APIPublishContent", ah.APIPublishContent, http.MethodPost, nil, idv)
	expectErr("APIDeleteContent", ah.APIDeleteContent, http.MethodDelete, nil, idv)

	// Template read ok, content InsertOne fails.
	db.SetFaultHook(testutil.FailOp("InsertOne"))
	expectErr("APICreateContent", ah.APICreateContent, http.MethodPost,
		map[string]interface{}{"template_id": tmpl.Hex(), "title": "T"}, nil)

	db.SetFaultHook(nil)
}
