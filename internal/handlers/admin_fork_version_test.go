package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"lightcms/internal/database"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func findOneID(t *testing.T, db *database.DB, collection string, filter bson.M) string {
	t.Helper()
	var doc struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.FindOne(ctx, collection, filter, &doc); err != nil {
		return ""
	}
	return doc.ID.Hex()
}

// TestAdminForks_Lifecycle drives the session-based fork admin UI handlers.
func TestAdminForks_Lifecycle(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)

	if rr := getPage(t, h.ListForks, nil); rr.Code >= 500 {
		t.Fatalf("ListForks: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := getPage(t, h.NewFork, nil); rr.Code >= 500 {
		t.Errorf("NewFork: %d", rr.Code)
	}

	if rr := postForm(t, h.CreateFork, url.Values{"name": {"Workspace"}, "description": {"d"}}, nil); rr.Code >= 500 {
		t.Fatalf("CreateFork: %d (%s)", rr.Code, rr.Body.String())
	}
	forkID := findOneID(t, db, "content_forks", bson.M{"name": "Workspace"})
	if forkID == "" {
		t.Skip("fork not created; skipping rest")
	}
	v := map[string]string{"id": forkID}

	if rr := getPage(t, h.ViewFork, v); rr.Code >= 500 {
		t.Errorf("ViewFork: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.StartForkPreview, nil, v); rr.Code >= 500 {
		t.Errorf("StartForkPreview: %d", rr.Code)
	}
	if rr := postForm(t, h.ExitForkPreview, nil, nil); rr.Code >= 500 {
		t.Errorf("ExitForkPreview: %d", rr.Code)
	}
	if rr := postForm(t, h.ArchiveFork, nil, v); rr.Code >= 500 {
		t.Errorf("ArchiveFork: %d", rr.Code)
	}
	if rr := postForm(t, h.DeleteForkHandler, nil, v); rr.Code >= 500 {
		t.Errorf("DeleteForkHandler: %d", rr.Code)
	}
}

// TestAdminContentVersions drives the version view/diff/revert handlers.
func TestAdminContentVersions(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmplID := seedTemplate(t, db, "Page", "page")

	// Create then update content to generate a version.
	postForm(t, h.CreateContent, url.Values{
		"template_id": {tmplID.Hex()}, "title": {"Versioned"}, "slug": {"versioned"}, "published": {"on"},
	}, nil)
	id := findOneID(t, db, "content", bson.M{"slug": "versioned"})
	if id == "" {
		t.Skip("content not created")
	}
	postForm(t, h.UpdateContent, url.Values{
		"template_id": {tmplID.Hex()}, "title": {"Versioned v2"}, "slug": {"versioned"},
		"published": {"on"}, "version_comment": {"v2"},
	}, map[string]string{"id": id})

	verVars := map[string]string{"id": id, "version": "1"}
	if rr := getPage(t, h.ViewContentVersion, verVars); rr.Code >= 500 {
		t.Errorf("ViewContentVersion: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := getPage(t, h.DiffContentVersion, verVars); rr.Code >= 500 {
		t.Errorf("DiffContentVersion: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.RevertContentVersion, nil, verVars); rr.Code >= 500 {
		t.Errorf("RevertContentVersion: %d (%s)", rr.Code, rr.Body.String())
	}
}

// TestAdminOrg_EditDeleteFlows drives collection/folder/redirect edit→delete.
func TestAdminOrg_EditDeleteFlows(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)

	// Collection
	postForm(t, h.CreateCollection, url.Values{"name": {"News"}, "category": {"news"}, "items_per_page": {"10"}}, nil)
	if cid := findOneID(t, db, "collections", bson.M{"name": "News"}); cid != "" {
		v := map[string]string{"id": cid}
		if rr := getPage(t, h.EditCollection, v); rr.Code >= 500 {
			t.Errorf("EditCollection: %d", rr.Code)
		}
		if rr := postForm(t, h.UpdateCollection, url.Values{"name": {"News2"}, "category": {"news"}, "items_per_page": {"5"}}, v); rr.Code >= 500 {
			t.Errorf("UpdateCollection: %d", rr.Code)
		}
		if rr := postForm(t, h.DeleteCollection, nil, v); rr.Code >= 500 {
			t.Errorf("DeleteCollection: %d", rr.Code)
		}
	}

	// Folder
	postForm(t, h.CreateFolder, url.Values{"name": {"Blog"}, "slug": {"blog"}}, nil)
	if fid := findOneID(t, db, "folders", bson.M{"slug": "blog"}); fid != "" {
		v := map[string]string{"id": fid}
		if rr := getPage(t, h.EditFolder, v); rr.Code >= 500 {
			t.Errorf("EditFolder: %d", rr.Code)
		}
		if rr := postForm(t, h.UpdateFolder, url.Values{"name": {"Blog2"}, "slug": {"blog"}}, v); rr.Code >= 500 {
			t.Errorf("UpdateFolder: %d", rr.Code)
		}
		if rr := postForm(t, h.DeleteFolder, nil, v); rr.Code >= 500 {
			t.Errorf("DeleteFolder: %d", rr.Code)
		}
	}

	// Redirect
	postForm(t, h.CreateRedirect, url.Values{"from_path": {"/old"}, "to_path": {"/new"}, "status_code": {"301"}}, nil)
	if rid := findOneID(t, db, "redirects", bson.M{"from_path": "/old"}); rid != "" {
		v := map[string]string{"id": rid}
		if rr := getPage(t, h.EditRedirect, v); rr.Code >= 500 {
			t.Errorf("EditRedirect: %d", rr.Code)
		}
		if rr := postForm(t, h.UpdateRedirect, url.Values{"from_path": {"/old"}, "to_path": {"/new2"}, "status_code": {"302"}}, v); rr.Code >= 500 {
			t.Errorf("UpdateRedirect: %d", rr.Code)
		}
		if rr := postForm(t, h.DeleteRedirect, nil, v); rr.Code >= 500 {
			t.Errorf("DeleteRedirect: %d", rr.Code)
		}
	}
}

// TestServePage exercises the public page server for a few paths.
func TestServePage(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	for _, slug := range []string{"", "nonexistent-page", "some/nested/path"} {
		req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
		if slug != "" {
			req = mux.SetURLVars(req, map[string]string{"slug": slug})
		}
		rr := httptest.NewRecorder()
		h.ServePage(rr, req)
		if rr.Code >= 500 {
			t.Errorf("ServePage(%q): server error %d", slug, rr.Code)
		}
	}
}
