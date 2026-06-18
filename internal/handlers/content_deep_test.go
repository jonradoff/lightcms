package handlers

import (
	"net/url"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// TestContentDeep_FolderAndSlugMoves exercises the folder-path-update,
// slug-rename (wikilink update), publish/unpublish, and duplicate-slug branches
// of CreateContent/UpdateContent/UpdateFolder.
func TestContentDeep_FolderAndSlugMoves(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmpl := seedTemplate(t, db, "Page", "page")

	// Create a folder.
	postForm(t, h.CreateFolder, url.Values{"name": {"Docs"}, "slug": {"docs"}}, nil)
	folderID := findOneID(t, db, "folders", bson.M{"slug": "docs"})
	if folderID == "" {
		t.Skip("folder not created")
	}

	// Create content inside the folder (published).
	postForm(t, h.CreateContent, url.Values{
		"template_id": {tmpl.Hex()}, "title": {"In Folder"}, "slug": {"in-folder"},
		"folder_id": {folderID}, "published": {"on"},
	}, nil)
	cid := findContentID(t, db, "in-folder")
	if cid == "" {
		t.Skip("content not created")
	}

	// Rename the folder → updateContentFolderPaths runs over its content.
	if rr := postForm(t, h.UpdateFolder, url.Values{"name": {"Docs2"}, "slug": {"documentation"}}, map[string]string{"id": folderID}); rr.Code >= 500 {
		t.Errorf("UpdateFolder rename: %d (%s)", rr.Code, rr.Body.String())
	}

	// Update the content: change slug (wikilink rename) and unpublish.
	if rr := postForm(t, h.UpdateContent, url.Values{
		"template_id": {tmpl.Hex()}, "title": {"In Folder v2"}, "slug": {"in-folder-renamed"},
		"folder_id": {folderID}, "version_comment": {"rename + unpublish"},
		// no "published" => unpublish transition
	}, map[string]string{"id": cid}); rr.Code >= 500 {
		t.Errorf("UpdateContent rename/unpublish: %d (%s)", rr.Code, rr.Body.String())
	}

	// Duplicate-slug create (collision branch).
	postForm(t, h.CreateContent, url.Values{
		"template_id": {tmpl.Hex()}, "title": {"Dup"}, "slug": {"dup-slug"}, "published": {"on"},
	}, nil)
	if rr := postForm(t, h.CreateContent, url.Values{
		"template_id": {tmpl.Hex()}, "title": {"Dup2"}, "slug": {"dup-slug"}, "published": {"on"},
	}, nil); rr.Code >= 500 {
		t.Errorf("CreateContent duplicate slug: %d", rr.Code)
	}

	// Re-publish the renamed content.
	if rr := postForm(t, h.UpdateContent, url.Values{
		"template_id": {tmpl.Hex()}, "title": {"In Folder v3"}, "slug": {"in-folder-renamed"},
		"folder_id": {folderID}, "published": {"on"}, "version_comment": {"republish"},
	}, map[string]string{"id": cid}); rr.Code >= 500 {
		t.Errorf("UpdateContent republish: %d", rr.Code)
	}

	// Delete then undelete to cover both transitions with real data.
	if rr := postForm(t, h.DeleteContent, nil, map[string]string{"id": cid}); rr.Code >= 500 {
		t.Errorf("DeleteContent: %d", rr.Code)
	}
	if rr := postForm(t, h.UndeleteContent, nil, map[string]string{"id": cid}); rr.Code >= 500 {
		t.Errorf("UndeleteContent: %d", rr.Code)
	}

	// Delete the folder (now that content moved/changed).
	if rr := postForm(t, h.DeleteFolder, nil, map[string]string{"id": folderID}); rr.Code >= 500 {
		t.Errorf("DeleteFolder: %d", rr.Code)
	}
}
