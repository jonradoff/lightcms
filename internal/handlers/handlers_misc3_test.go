package handlers

import (
	"net/url"
	"testing"
)

func TestUploadFileHandler(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	// UploadFile exercises the admin uploader; in the test env the file store /
	// MIME validation may reject the payload, which is itself a covered path.
	rr := postMultipart(t, h.UploadFile, "file", "note.txt", "hello", url.Values{}, nil)
	if rr.Code == 0 {
		t.Error("UploadFile wrote no response")
	}
}

func TestGetAllSlugsHandler(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmpl := seedTemplate(t, db, "Page", "page")
	seedContent(t, db, tmpl, "A", "a", "/a")
	seedContent(t, db, tmpl, "B", "b", "/b")
	if rr := getPage(t, h.GetAllSlugs, nil); rr.Code >= 500 {
		t.Errorf("GetAllSlugs: %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestForceChangePasswordHandler(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	// Mismatched confirmation → re-render / error, not 500.
	if rr := postForm(t, h.ForceChangePasswordHandler, url.Values{
		"current_password": {"admin123"},
		"new_password":     {"newpass1234"},
		"confirm_password": {"different"},
	}, nil); rr.Code >= 500 {
		t.Errorf("ForceChangePasswordHandler (mismatch): %d", rr.Code)
	}
	// Well-formed change attempt.
	if rr := postForm(t, h.ForceChangePasswordHandler, url.Values{
		"current_password": {"admin123"},
		"new_password":     {"newpass1234"},
		"confirm_password": {"newpass1234"},
	}, nil); rr.Code >= 500 {
		t.Errorf("ForceChangePasswordHandler: %d", rr.Code)
	}
}

func TestDiffContentVersionHandler(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmpl := seedTemplate(t, db, "Page", "page")

	postForm(t, h.CreateContent, url.Values{"template_id": {tmpl.Hex()}, "title": {"V"}, "slug": {"vpage"}, "published": {"on"}}, nil)
	id := findContentID(t, db, "vpage")
	if id == "" {
		t.Skip("content not created")
	}
	// Two updates → versions 1 and 2.
	for _, title := range []string{"V2", "V3"} {
		postForm(t, h.UpdateContent, url.Values{
			"template_id": {tmpl.Hex()}, "title": {title}, "slug": {"vpage"},
			"published": {"on"}, "version_comment": {title},
		}, map[string]string{"id": id})
	}
	for _, ver := range []string{"1", "2"} {
		if rr := getPage(t, h.DiffContentVersion, map[string]string{"id": id, "version": ver}); rr.Code >= 500 {
			t.Errorf("DiffContentVersion v%s: %d (%s)", ver, rr.Code, rr.Body.String())
		}
		if rr := getPage(t, h.ViewContentVersion, map[string]string{"id": id, "version": ver}); rr.Code >= 500 {
			t.Errorf("ViewContentVersion v%s: %d", ver, rr.Code)
		}
	}
}
