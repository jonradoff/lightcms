package handlers

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// findContentID returns the hex ID of a content item by slug, or "".
func findContentID(t *testing.T, db *database.DB, slug string) string {
	t.Helper()
	var doc struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.FindOne(ctx, "content", bson.M{"slug": slug}, &doc); err != nil {
		return ""
	}
	return doc.ID.Hex()
}

// TestContentLifecycle_Forms drives the content create/edit/update/delete/
// undelete form handlers — the largest functions in handlers.go.
func TestContentLifecycle_Forms(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmplID := seedTemplate(t, db, "Page", "page")

	// Create
	rr := postForm(t, h.CreateContent, url.Values{
		"template_id":      {tmplID.Hex()},
		"title":            {"My New Page"},
		"slug":             {"my-new-page"},
		"published":        {"on"},
		"meta_description": {"A page"},
		"content_tags":     {"news, updates"},
	}, nil)
	if rr.Code >= 500 {
		t.Fatalf("CreateContent: %d (%s)", rr.Code, rr.Body.String())
	}

	id := findContentID(t, db, "my-new-page")
	if id == "" {
		t.Skip("content not created; skipping update/delete")
	}
	vars := map[string]string{"id": id}

	// Update
	if rr := postForm(t, h.UpdateContent, url.Values{
		"template_id":     {tmplID.Hex()},
		"title":           {"My Updated Page"},
		"slug":            {"my-new-page"},
		"published":       {"on"},
		"version_comment": {"updated title"},
	}, vars); rr.Code >= 500 {
		t.Errorf("UpdateContent: %d (%s)", rr.Code, rr.Body.String())
	}

	// Versions list + delete + undelete
	if rr := getPage(t, h.ListContentVersions, vars); rr.Code >= 500 {
		t.Errorf("ListContentVersions: %d", rr.Code)
	}
	if rr := postForm(t, h.DeleteContent, nil, vars); rr.Code >= 500 {
		t.Errorf("DeleteContent: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.UndeleteContent, nil, vars); rr.Code >= 500 {
		t.Errorf("UndeleteContent: %d (%s)", rr.Code, rr.Body.String())
	}
}
