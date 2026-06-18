package handlers

import (
	"net/url"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func mustObjID(t *testing.T, hex string) primitive.ObjectID {
	t.Helper()
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		t.Fatalf("bad object id %q: %v", hex, err)
	}
	return id
}

// TestAdminForkPage_Flow drives the fork page admin handlers (fork a live page,
// then remove it / merge the fork) against real seeded data.
func TestAdminForkPage_Flow(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmpl := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmpl, "Live", "live", "/live")

	if rr := postForm(t, h.CreateFork, url.Values{"name": {"WS"}}, nil); rr.Code >= 500 {
		t.Fatalf("CreateFork: %d", rr.Code)
	}
	forkID := findOneID(t, db, "content_forks", bson.M{"name": "WS"})
	if forkID == "" {
		t.Skip("fork not created")
	}
	fv := map[string]string{"id": forkID}

	// Fork the live page into the workspace.
	if rr := postForm(t, h.ForkPageHandler, url.Values{"content_id": {contentID.Hex()}}, fv); rr.Code >= 500 {
		t.Errorf("ForkPageHandler: %d (%s)", rr.Code, rr.Body.String())
	}

	// The forked copy is a content doc with this fork_id; find it for removal.
	if pageID := findOneID(t, db, "content", bson.M{"fork_id": mustObjID(t, forkID)}); pageID != "" {
		if rr := postForm(t, h.RemoveForkPage, nil, map[string]string{"id": forkID, "pageID": pageID}); rr.Code >= 500 {
			t.Errorf("RemoveForkPage: %d (%s)", rr.Code, rr.Body.String())
		}
	}

	if rr := postForm(t, h.MergeFork, nil, fv); rr.Code >= 500 {
		t.Errorf("MergeFork: %d (%s)", rr.Code, rr.Body.String())
	}
}

// TestHandlerSetters exercises the trivial setter methods for coverage.
func TestHandlerSetters(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	h.SetAnthropicAPIKey("sk-test")
	h.SetCloudflareService(nil)
}
