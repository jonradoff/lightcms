package handlers

import (
	"context"
	"net/url"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAdminAPIKeys_Flow(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)

	if rr := postForm(t, h.CreateAPIKey, url.Values{"name": {"CI Key"}, "description": {"d"}}, nil); rr.Code >= 500 {
		t.Fatalf("CreateAPIKey: %d (%s)", rr.Code, rr.Body.String())
	}
	if id := findOneID(t, db, "api_keys", bson.M{"name": "CI Key"}); id != "" {
		if rr := postForm(t, h.DeleteAPIKey, nil, map[string]string{"id": id}); rr.Code >= 500 {
			t.Errorf("DeleteAPIKey: %d", rr.Code)
		}
	}
}

func TestAdminTemplateFields_And_ConfirmChange(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmplID := seedTemplate(t, db, "Page", "page")
	tmpl2 := seedTemplate(t, db, "Other", "other")
	contentID := seedContent(t, db, tmplID, "Doc", "doc", "/doc")

	if rr := getPage(t, h.GetTemplateFields, map[string]string{"id": tmplID.Hex()}); rr.Code >= 500 {
		t.Errorf("GetTemplateFields: %d (%s)", rr.Code, rr.Body.String())
	}

	if rr := postForm(t, h.ConfirmChangeTemplate, url.Values{"template_id": {tmpl2.Hex()}}, map[string]string{"id": contentID.Hex()}); rr.Code >= 500 {
		t.Errorf("ConfirmChangeTemplate: %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAdminThemeVersions_Flow(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// Two theme saves to generate version history.
	for i := 0; i < 2; i++ {
		postForm(t, h.UpdateTheme, url.Values{
			"site_name": {"S"}, "primary_color": {"#111111"}, "secondary_color": {"#222222"},
			"accent_color": {"#333333"}, "background_color": {"#ffffff"}, "text_color": {"#000000"},
			"font_family": {"Inter"}, "heading_font": {"Inter"}, "border_radius": {"6px"},
		}, nil)
	}

	if rr := getPage(t, h.ThemeVersions, nil); rr.Code >= 500 {
		t.Errorf("ThemeVersions: %d", rr.Code)
	}
	if rr := getPage(t, h.ThemeVersionDiff, map[string]string{"version": "1"}); rr.Code >= 500 {
		t.Errorf("ThemeVersionDiff: %d", rr.Code)
	}
	if rr := postForm(t, h.RevertThemeVersion, nil, map[string]string{"version": "1"}); rr.Code >= 500 {
		t.Errorf("RevertThemeVersion: %d", rr.Code)
	}
}

func TestAdminContactMessages_Flow(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)

	// Seed a contact message.
	id := primitive.NewObjectID()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = db.InsertOne(ctx, "contact_messages", bson.M{
		"_id": id, "name": "Visitor", "email": "v@x.com", "message": "Hello",
		"read": false, "created_at": time.Now(),
	})
	v := map[string]string{"id": id.Hex()}

	if rr := getPage(t, h.ViewContactMessage, v); rr.Code >= 500 {
		t.Errorf("ViewContactMessage: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.DeleteContactMessage, nil, v); rr.Code >= 500 {
		t.Errorf("DeleteContactMessage: %d", rr.Code)
	}
}
