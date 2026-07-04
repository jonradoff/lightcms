package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// createFork inserts an active fork directly and returns its ID.
func createTestFork(t *testing.T, db *database.DB, name string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	_, err := db.InsertOne(context.Background(), "content_forks", &models.ContentFork{
		ID: id, Name: name, Status: "active", PreviewToken: "tok-" + name,
		CreatedBy: primitive.NewObjectID(), CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("createTestFork: %v", err)
	}
	return id
}

func TestAPICreateContent_InFork(t *testing.T) {
	a, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, db, "Page", "page")
	forkID := createTestFork(t, db, "sandbox")

	body := `{"template_id":"` + tmplID.Hex() + `","title":"Fork Draft","slug":"fork-draft",` +
		`"data":{},"published":true,"fork_id":"` + forkID.Hex() + `"}`
	req := authReq("POST", "/api/v1/content", strings.NewReader(body))
	rr := httptest.NewRecorder()
	a.APICreateContent(rr, req)

	if rr.Code != 201 {
		t.Fatalf("status = %d, body: %s", rr.Code, rr.Body.String())
	}
	var created models.Content
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ForkID == nil || *created.ForkID != forkID {
		t.Errorf("fork_id not set on created content: %+v", created.ForkID)
	}

	// Even though published=true was requested, no static file may appear at
	// the live path for fork content.
	staticPath := filepath.Join("content", "generated", "fork-draft.html")
	if _, err := os.Stat(staticPath); err == nil {
		os.Remove(staticPath)
		t.Errorf("static file was generated for fork content at %s", staticPath)
	}

	// Invalid fork id is rejected.
	req = authReq("POST", "/api/v1/content", strings.NewReader(
		`{"template_id":"`+tmplID.Hex()+`","title":"X","slug":"x","data":{},"fork_id":"nothex"}`))
	rr = httptest.NewRecorder()
	a.APICreateContent(rr, req)
	if rr.Code != 400 {
		t.Errorf("invalid fork_id: status = %d, want 400", rr.Code)
	}

	// Nonexistent fork is rejected.
	req = authReq("POST", "/api/v1/content", strings.NewReader(
		`{"template_id":"`+tmplID.Hex()+`","title":"X","slug":"x","data":{},"fork_id":"`+primitive.NewObjectID().Hex()+`"}`))
	rr = httptest.NewRecorder()
	a.APICreateContent(rr, req)
	if rr.Code != 400 {
		t.Errorf("unknown fork_id: status = %d, want 400", rr.Code)
	}

	// upsert + fork_id combination is rejected.
	req = authReq("POST", "/api/v1/content", strings.NewReader(
		`{"template_id":"`+tmplID.Hex()+`","title":"X","slug":"x","data":{},"upsert":true,"fork_id":"`+forkID.Hex()+`"}`))
	rr = httptest.NewRecorder()
	a.APICreateContent(rr, req)
	if rr.Code != 400 {
		t.Errorf("upsert+fork_id: status = %d, want 400", rr.Code)
	}
}

func TestAPIForkDiff(t *testing.T) {
	a, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	forkID := createTestFork(t, db, "diff-api")

	// Live page + modified fork copy at the same path.
	now := time.Now()
	_, _ = db.InsertOne(context.Background(), "content", &models.Content{
		ID: primitive.NewObjectID(), Title: "Live", FullPath: "/dp", Slug: "dp",
		Published: true, CreatedAt: now, UpdatedAt: now,
	})
	_, _ = db.InsertOne(context.Background(), "content", &models.Content{
		ID: primitive.NewObjectID(), Title: "Fork Edit", FullPath: "/dp", Slug: "dp",
		ForkID: &forkID, CreatedAt: now, UpdatedAt: now,
	})

	req := authReq("GET", "/api/v1/forks/"+forkID.Hex()+"/diff", nil)
	req = mux.SetURLVars(req, map[string]string{"id": forkID.Hex()})
	rr := httptest.NewRecorder()
	a.APIForkDiff(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ForkID string `json:"fork_id"`
		Pages  []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
			Fields []struct {
				Name string `json:"name"`
				Live string `json:"live"`
				Fork string `json:"fork"`
			} `json:"fields"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Pages) != 1 || resp.Pages[0].Status != "modified" {
		t.Fatalf("unexpected diff: %s", rr.Body.String())
	}
	if len(resp.Pages[0].Fields) == 0 || resp.Pages[0].Fields[0].Name != "title" {
		t.Errorf("expected title field diff, got: %s", rr.Body.String())
	}

	// Invalid ID.
	req = authReq("GET", "/api/v1/forks/zzz/diff", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "zzz"})
	rr = httptest.NewRecorder()
	a.APIForkDiff(rr, req)
	if rr.Code != 400 {
		t.Errorf("invalid id: status = %d, want 400", rr.Code)
	}
}

// silence unused-import guards if helpers change
var _ = bson.M{}
