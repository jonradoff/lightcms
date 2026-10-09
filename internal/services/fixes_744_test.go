package services

import (
	"context"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func stored744(t *testing.T, cs *ContentService, id primitive.ObjectID) bson.M {
	t.Helper()
	ctx := context.Background()
	var row bson.M
	if err := cs.db.Collection("content").FindOne(ctx, bson.M{"_id": id}).Decode(&row); err != nil {
		t.Fatalf("read row: %v", err)
	}
	return row
}

// A page deleted through the service is no longer flagged published, and
// restoring it publishes it again.
func TestDeleteContent_ClearsPublished_RestorePublishesAgain(t *testing.T) {
	cs, _, _, ctx := hooks742(t)
	id := seed742(t, cs, models.Content{Title: "G1", Slug: "g1-pub", FullPath: "/g1-pub", Published: true})

	if err := cs.DeleteContent(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	row := stored744(t, cs, id)
	if row["published"] != false {
		t.Errorf("deleted row still has published=%v", row["published"])
	}
	if row["published_before_delete"] != true {
		t.Errorf("deleted row has published_before_delete=%v, want true", row["published_before_delete"])
	}

	if err := cs.RestoreContent(ctx, id); err != nil {
		t.Fatalf("restore: %v", err)
	}
	row = stored744(t, cs, id)
	if row["published"] != true || row["deleted"] != false {
		t.Errorf("restored row: published=%v deleted=%v, want true/false", row["published"], row["deleted"])
	}
	if _, ok := row["published_before_delete"]; ok {
		t.Error("restored row keeps published_before_delete")
	}
}

func TestRestoreContent_DraftStaysDraft_HeldStaysUnpublished(t *testing.T) {
	cs, _, _, ctx := hooks742(t)
	draft := seed742(t, cs, models.Content{Title: "G2", Slug: "g2-draft", FullPath: "/g2-draft"})
	held := seed742(t, cs, models.Content{Title: "G3", Slug: "g3-held", FullPath: "/g3-held", Published: true})

	for _, id := range []primitive.ObjectID{draft, held} {
		if err := cs.DeleteContent(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
	}
	// Put on hold while deleted
	if err := cs.db.UpdateOne(ctx, "content", bson.M{"_id": held}, bson.M{"$set": bson.M{"hold": true}}); err != nil {
		t.Fatalf("hold: %v", err)
	}
	for _, id := range []primitive.ObjectID{draft, held} {
		if err := cs.RestoreContent(ctx, id); err != nil {
			t.Fatalf("restore: %v", err)
		}
		if row := stored744(t, cs, id); row["published"] == true {
			t.Errorf("%v restored as published", row["full_path"])
		}
	}
}

// Rows deleted before 7.4.4 kept published=true; restoring one still brings
// it back published.
func TestRestoreContent_RowDeletedBefore744(t *testing.T) {
	cs, _, _, ctx := hooks742(t)
	id := seed742(t, cs, models.Content{Title: "G4", Slug: "g4-old", FullPath: "/g4-old", Published: true, Deleted: true})
	if err := cs.RestoreContent(ctx, id); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if row := stored744(t, cs, id); row["published"] != true || row["deleted"] != false {
		t.Errorf("published=%v deleted=%v, want true/false", row["published"], row["deleted"])
	}
}
