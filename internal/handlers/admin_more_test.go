package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// postMultipart invokes a session-authed multipart POST with one file field
// plus extra form values.
func postMultipart(t *testing.T, h http.HandlerFunc, fileField, filename, fileBody string, fields url.Values, vars map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile(fileField, filename)
	_, _ = fw.Write([]byte(fileBody))
	for k, vs := range fields {
		for _, v := range vs {
			_ = mw.WriteField(k, v)
		}
	}
	mw.Close()

	req := sessionReq(http.MethodPost, "/cm/x", &buf, vars)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func TestAdminAudit_Actions(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	if rr := postForm(t, h.ClearRateLimit, nil, map[string]string{"ip": "203.0.113.9"}); rr.Code >= 500 {
		t.Errorf("ClearRateLimit: %d (%s)", rr.Code, rr.Body.String())
	}
	missing := "000000000000000000000abc"
	if rr := postForm(t, h.ForceUnlock, nil, map[string]string{"id": missing}); rr.Code >= 500 {
		t.Errorf("ForceUnlock: %d", rr.Code)
	}
	if rr := postForm(t, h.RefreshLock, nil, map[string]string{"id": missing}); rr.Code >= 500 {
		t.Errorf("RefreshLock: %d", rr.Code)
	}
}

func TestAdminMisc_Pages(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	for name, handler := range map[string]http.HandlerFunc{
		"AssetLibrary":  h.AssetLibrary,
		"ApprovalsPage": h.ApprovalsPage,
	} {
		if rr := getPage(t, handler, nil); rr.Code >= 500 {
			t.Errorf("%s: %d (%s)", name, rr.Code, rr.Body.String())
		}
	}
	if rr := postForm(t, h.MarkAllMessagesRead, nil, nil); rr.Code >= 500 {
		t.Errorf("MarkAllMessagesRead: %d", rr.Code)
	}
}

func TestAdminAssetUpload(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// A tiny PNG-ish payload; the handler validates and stores. Any non-5xx is fine.
	if rr := postMultipart(t, h.AssetUpload, "file", "pixel.txt", "hello world", url.Values{}, nil); rr.Code >= 500 {
		t.Errorf("AssetUpload: %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAdminImports_Forms(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmplID := seedTemplate(t, db, "Imported", "imported")

	// Markdown import (multipart .md).
	if rr := postMultipart(t, h.DoImportMarkdown, "file", "post.md", "---\ntitle: Hi\n---\nBody",
		url.Values{"default_template": {"Imported"}, "auto_publish": {"false"}}, nil); rr.Code >= 500 {
		t.Errorf("DoImportMarkdown: %d (%s)", rr.Code, rr.Body.String())
	}

	// CSV import (multipart .csv).
	if rr := postMultipart(t, h.DoImportCSV, "file", "data.csv", "title,body\nRow,Content",
		url.Values{"template_id": {tmplID.Hex()}, "title_column": {"title"}, "auto_publish": {"false"}}, nil); rr.Code >= 500 {
		t.Errorf("DoImportCSV: %d (%s)", rr.Code, rr.Body.String())
	}

	// RSS source update/trigger/delete against a seeded source.
	srcID := seedImportSource(t, db, "Feed")
	v := map[string]string{"id": srcID}
	if rr := postForm(t, h.UpdateRSSSource, url.Values{
		"name": {"Feed2"}, "url": {"https://example.com/rss"}, "template_id": {tmplID.Hex()}, "schedule": {"daily"},
	}, v); rr.Code >= 500 {
		t.Errorf("UpdateRSSSource: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.TriggerRSSSource, nil, v); rr.Code >= 500 {
		t.Errorf("TriggerRSSSource: %d", rr.Code)
	}
	if rr := postForm(t, h.DeleteRSSSource, nil, v); rr.Code >= 500 {
		t.Errorf("DeleteRSSSource: %d", rr.Code)
	}
}

// seedImportSource inserts an import source and returns its hex ID.
func seedImportSource(t *testing.T, db *database.DB, name string) string {
	t.Helper()
	id := primitive.NewObjectID()
	now := time.Now()
	if _, err := db.InsertOne(context.Background(), "import_sources", bson.M{
		"_id": id, "name": name, "url": "https://example.com/rss",
		"schedule": "daily", "active": true, "created_at": now, "updated_at": now,
	}); err != nil {
		t.Fatalf("seedImportSource: %v", err)
	}
	return id.Hex()
}
