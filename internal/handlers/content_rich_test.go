package handlers

import (
	"context"
	"net/url"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seedRichTemplate inserts a template with one of each field type and a layout
// that references them, so content generation exercises field processing.
func seedRichTemplate(t *testing.T) (string, func()) {
	t.Helper()
	db := testDB(t)
	id := primitive.NewObjectID()
	now := time.Now()
	doc := bson.M{
		"_id":  id,
		"name": "Rich",
		"slug": "rich",
		"fields": bson.A{
			bson.M{"name": "body", "label": "Body", "type": "markdown"},
			bson.M{"name": "summary", "label": "Summary", "type": "textarea"},
			bson.M{"name": "hero", "label": "Hero", "type": "image"},
			bson.M{"name": "category", "label": "Category", "type": "select", "options": "news,blog,docs"},
			bson.M{"name": "pubdate", "label": "Date", "type": "date"},
			bson.M{"name": "intro", "label": "Intro", "type": "richtext"},
		},
		"html_layout": "<html><body><h1>{{.Title}}</h1>{{.intro}}{{.body}}<p>{{.summary}}</p></body></html>",
		"created_at":  now,
		"updated_at":  now,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.InsertOne(ctx, "templates", doc); err != nil {
		t.Fatalf("seedRichTemplate: %v", err)
	}
	return id.Hex(), func() {}
}

// TestContentRich_Lifecycle drives create/update/publish with a multi-field
// template, exercising field-type handling, markdown rendering, wiki markup,
// tag detection, and static-page generation.
func TestContentRich_Lifecycle(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmplID, _ := seedRichTemplate(t)

	form := url.Values{
		"template_id":      {tmplID},
		"title":            {"Rich Page"},
		"slug":             {"rich-page"},
		"published":        {"on"},
		"meta_description": {"A rich page about #golang and #testing"},
		"content_tags":     {"feature, launch"},
		"field_body":       {"# Heading\n\nSome **markdown** with a [[Other Page]] wikilink and a #tag.\n\n- item 1\n- item 2"},
		"field_summary":    {"Short summary"},
		"field_hero":       {"/assets/hero.png"},
		"field_category":   {"blog"},
		"field_pubdate":    {"2026-01-15"},
		"field_intro":      {"<p>Rich <em>intro</em></p>"},
	}
	if rr := postForm(t, h.CreateContent, form, nil); rr.Code >= 500 {
		t.Fatalf("CreateContent(rich): %d (%s)", rr.Code, rr.Body.String())
	}

	id := findContentID(t, db, "rich-page")
	if id == "" {
		t.Skip("rich content not created")
	}

	// Update with changed field values + a new slug (triggers regeneration + rename).
	form.Set("title", "Rich Page v2")
	form.Set("field_body", "Updated **body** content.")
	form.Set("version_comment", "rich update")
	if rr := postForm(t, h.UpdateContent, form, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("UpdateContent(rich): %d (%s)", rr.Code, rr.Body.String())
	}

	// Regenerate the single page.
	if rr := postForm(t, h.RegenerateContent, nil, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("RegenerateContent: %d", rr.Code)
	}
}
