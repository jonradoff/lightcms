package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer returns a test server that records the last request and serves
// the given status code + JSON body on every request.
func newTestServer(t *testing.T, statusCode int, body interface{}) (*httptest.Server, *http.Request) {
	t.Helper()
	var lastReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, lastReq
}

func TestNew(t *testing.T) {
	c := New("http://localhost:8082", "lc_test")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.baseURL != "http://localhost:8082" {
		t.Errorf("unexpected baseURL: %q", c.baseURL)
	}
	if c.apiKey != "lc_test" {
		t.Errorf("unexpected apiKey: %q", c.apiKey)
	}
	if c.httpClient == nil {
		t.Error("expected non-nil httpClient")
	}
}

// assertAuth checks that the Authorization header is correct.
func assertAuth(t *testing.T, r *http.Request, expectedKey string) {
	t.Helper()
	got := r.Header.Get("Authorization")
	want := "Bearer " + expectedKey
	if got != want {
		t.Errorf("Authorization header: got %q, want %q", got, want)
	}
}

// --- do() branches ---

func TestDo_Success_NilResult(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusOK, nil)
	c := New(srv.URL, "tok")

	err := c.do(context.Background(), "DELETE", "/content/abc", nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDo_ErrorResponse_JSONError(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusNotFound, map[string]string{"error": "not found"})
	c := New(srv.URL, "tok")

	err := c.do(context.Background(), "GET", "/content/missing", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDo_ErrorResponse_RawBody(t *testing.T) {
	// Non-JSON error body falls back to raw body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	err := c.do(context.Background(), "GET", "/content", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestDo_SetsAuthHeader(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "my_api_key")
	c.do(context.Background(), "GET", "/content", nil, nil) //nolint
	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	assertAuth(t, capturedReq, "my_api_key")
}

func TestDo_SetsContentTypeForBody(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	c.do(context.Background(), "POST", "/content", map[string]string{"title": "Test"}, nil) //nolint
	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	ct := capturedReq.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// --- Content methods ---

func TestListContent(t *testing.T) {
	want := []Content{{ID: "abc", Title: "Hello"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/content") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListContent(context.Background(), false, "", "")
	if err != nil {
		t.Fatalf("ListContent: %v", err)
	}
	if len(got) != 1 || got[0].ID != "abc" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestListContent_WithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("include_deleted") != "true" {
			t.Error("expected include_deleted=true")
		}
		if q.Get("category") != "blog" {
			t.Error("expected category=blog")
		}
		json.NewEncoder(w).Encode([]Content{})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	c.ListContent(context.Background(), true, "blog", "") //nolint
}

func TestGetContent(t *testing.T) {
	want := Content{ID: "xyz", Title: "Test Page"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/content/xyz" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetContent(context.Background(), "xyz", false)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if got.ID != "xyz" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestGetContent_IncludeRendered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("include_rendered") != "true" {
			t.Error("expected include_rendered=true")
		}
		json.NewEncoder(w).Encode(Content{ID: "xyz"})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	c.GetContent(context.Background(), "xyz", true) //nolint
}

func TestGetContentByPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/content/by-path" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("path") != "/blog/my-post" {
			t.Errorf("unexpected path param: %q", r.URL.Query().Get("path"))
		}
		json.NewEncoder(w).Encode(Content{ID: "123"})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetContentByPath(context.Background(), "/blog/my-post", false)
	if err != nil {
		t.Fatalf("GetContentByPath: %v", err)
	}
	if got.ID != "123" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestGetBacklinks(t *testing.T) {
	want := []Content{{ID: "a"}, {ID: "b"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetBacklinks(context.Background(), "/some/page")
	if err != nil {
		t.Fatalf("GetBacklinks: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 backlinks, got %d", len(got))
	}
}

func TestCreateContent(t *testing.T) {
	want := Content{ID: "new1", Title: "New Page"}
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateContent(context.Background(), CreateContentRequest{
		Title:      "New Page",
		TemplateID: "tmpl1",
	})
	if err != nil {
		t.Fatalf("CreateContent: %v", err)
	}
	if got.ID != "new1" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
	if capturedBody["title"] != "New Page" {
		t.Errorf("unexpected body title: %v", capturedBody["title"])
	}
}

func TestUpdateContent(t *testing.T) {
	want := Content{ID: "u1", Title: "Updated"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/content/u1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateContent(context.Background(), "u1", UpdateContentRequest{"title": "Updated"})
	if err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("unexpected Title: %q", got.Title)
	}
}

func TestDeleteContent(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteContent(context.Background(), "del1"); err != nil {
		t.Fatalf("DeleteContent: %v", err)
	}
	if method != "DELETE" {
		t.Errorf("expected DELETE, got %s", method)
	}
}

func TestRestoreContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/restore") {
			t.Errorf("expected /restore suffix, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.RestoreContent(context.Background(), "r1"); err != nil {
		t.Fatalf("RestoreContent: %v", err)
	}
}

func TestPublishContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/publish") {
			t.Errorf("expected /publish suffix, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.PublishContent(context.Background(), "p1"); err != nil {
		t.Fatalf("PublishContent: %v", err)
	}
}

func TestUnpublishContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/unpublish") {
			t.Errorf("expected /unpublish suffix, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.UnpublishContent(context.Background(), "up1"); err != nil {
		t.Fatalf("UnpublishContent: %v", err)
	}
}

func TestGetContentVersions(t *testing.T) {
	want := []ContentVersion{{ID: "v1", Version: 1}, {ID: "v2", Version: 2}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetContentVersions(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetContentVersions: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 versions, got %d", len(got))
	}
}

func TestGetContentVersion(t *testing.T) {
	want := ContentVersion{ID: "v3", Version: 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/versions/3") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetContentVersion(context.Background(), "c1", 3)
	if err != nil {
		t.Fatalf("GetContentVersion: %v", err)
	}
	if got.Version != 3 {
		t.Errorf("unexpected version: %d", got.Version)
	}
}

func TestRevertContentVersion(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.RevertContentVersion(context.Background(), "c1", 2, "rolling back"); err != nil {
		t.Fatalf("RevertContentVersion: %v", err)
	}
	if capturedBody["version_comment"] != "rolling back" {
		t.Errorf("unexpected comment: %v", capturedBody["version_comment"])
	}
}

func TestRevertContentVersion_NoComment(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	c.RevertContentVersion(context.Background(), "c1", 1, "") //nolint
	// empty body map is still sent, but no version_comment key
	if _, ok := capturedBody["version_comment"]; ok {
		t.Error("expected no version_comment for empty comment")
	}
}

func TestUpdateContentByPath(t *testing.T) {
	want := Content{ID: "bp1"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/content/by-path" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateContentByPath(context.Background(), "/my/page", map[string]interface{}{"title": "New"})
	if err != nil {
		t.Fatalf("UpdateContentByPath: %v", err)
	}
	if got.ID != "bp1" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestPreviewContent(t *testing.T) {
	want := PreviewContentResult{ContentID: "prev1", RenderedHTML: "<p>hi</p>"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.PreviewContent(context.Background(), "prev1", nil)
	if err != nil {
		t.Fatalf("PreviewContent: %v", err)
	}
	if got.RenderedHTML != "<p>hi</p>" {
		t.Errorf("unexpected rendered HTML: %q", got.RenderedHTML)
	}
}

func TestBatchPublishContent(t *testing.T) {
	want := map[string]interface{}{"published": float64(3)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.BatchPublishContent(context.Background(), []string{"a", "b", "c"}, false)
	if err != nil {
		t.Fatalf("BatchPublishContent: %v", err)
	}
	if got["published"] != float64(3) {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestBulkUpdateContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"updated": float64(2)})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.BulkUpdateContent(context.Background(), map[string]interface{}{"ids": []string{"x"}})
	if err != nil {
		t.Fatalf("BulkUpdateContent: %v", err)
	}
	if got["updated"] != float64(2) {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestBulkFieldOperation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"affected": float64(5)})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.BulkFieldOperation(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("BulkFieldOperation: %v", err)
	}
	_ = got
}

func TestExportContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ExportContent(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExportContent: %v", err)
	}
	_ = got
}

func TestListContentWithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("include_data") != "true" {
			t.Error("expected include_data=true")
		}
		if q.Get("include_fields") != "title,body" {
			t.Errorf("unexpected include_fields: %q", q.Get("include_fields"))
		}
		json.NewEncoder(w).Encode([]Content{})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	_, err := c.ListContentWithOptions(context.Background(), ListContentOptions{
		IncludeData:   true,
		IncludeFields: []string{"title", "body"},
	})
	if err != nil {
		t.Fatalf("ListContentWithOptions: %v", err)
	}
}

// --- Template methods ---

func TestListTemplates(t *testing.T) {
	want := []Template{{ID: "t1", Name: "Blog"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Blog" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestGetTemplate(t *testing.T) {
	want := Template{ID: "t2", Name: "Page"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetTemplate(context.Background(), "t2")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if got.Name != "Page" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestCreateTemplate(t *testing.T) {
	want := Template{ID: "t3", Name: "Custom"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateTemplate(context.Background(), CreateTemplateRequest{Name: "Custom"})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if got.ID != "t3" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestUpdateTemplate(t *testing.T) {
	want := Template{ID: "t4", Name: "Updated"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateTemplate(context.Background(), "t4", UpdateTemplateRequest{"name": "Updated"})
	if err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestDeleteTemplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteTemplate(context.Background(), "t5"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
}

// --- Asset methods ---

func TestListAssets(t *testing.T) {
	want := []AssetSummary{{ID: "a1", Filename: "logo.png"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListAssets(context.Background(), "")
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 asset, got %d", len(got))
	}
}

func TestListAssets_WithFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("folder") != "images" {
			t.Errorf("expected folder=images, got %q", r.URL.Query().Get("folder"))
		}
		json.NewEncoder(w).Encode([]AssetSummary{})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	c.ListAssets(context.Background(), "images") //nolint
}

func TestGetAsset(t *testing.T) {
	want := AssetSummary{ID: "a2"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetAsset(context.Background(), "a2")
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.ID != "a2" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestGetAssetByPath(t *testing.T) {
	want := AssetSummary{ID: "a3", ServePath: "/images/logo.png"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetAssetByPath(context.Background(), "/images/logo.png")
	if err != nil {
		t.Fatalf("GetAssetByPath: %v", err)
	}
	if got.ServePath != "/images/logo.png" {
		t.Errorf("unexpected ServePath: %q", got.ServePath)
	}
}

func TestUploadAsset(t *testing.T) {
	want := AssetSummary{ID: "a4", Filename: "upload.png"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UploadAsset(context.Background(), UploadAssetRequest{Filename: "upload.png"})
	if err != nil {
		t.Fatalf("UploadAsset: %v", err)
	}
	if got.Filename != "upload.png" {
		t.Errorf("unexpected filename: %q", got.Filename)
	}
}

func TestUploadAssetFromURL(t *testing.T) {
	want := AssetSummary{ID: "a5"}
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UploadAssetFromURL(context.Background(), "https://example.com/img.png", "/images/img.png", "test img")
	if err != nil {
		t.Fatalf("UploadAssetFromURL: %v", err)
	}
	if got.ID != "a5" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
	if capturedBody["url"] != "https://example.com/img.png" {
		t.Errorf("unexpected url body: %v", capturedBody["url"])
	}
}

func TestDeleteAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteAsset(context.Background(), "a6"); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}
}

func TestListAssetFolders(t *testing.T) {
	want := []string{"images", "docs"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListAssetFolders(context.Background())
	if err != nil {
		t.Fatalf("ListAssetFolders: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 folders, got %d", len(got))
	}
}

// --- Theme methods ---

func TestGetTheme(t *testing.T) {
	want := ThemeSettings{SiteName: "My Site"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetTheme(context.Background())
	if err != nil {
		t.Fatalf("GetTheme: %v", err)
	}
	if got.SiteName != "My Site" {
		t.Errorf("unexpected SiteName: %q", got.SiteName)
	}
}

func TestUpdateTheme(t *testing.T) {
	want := ThemeSettings{SiteName: "Updated Site"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateTheme(context.Background(), map[string]interface{}{"site_name": "Updated Site"})
	if err != nil {
		t.Fatalf("UpdateTheme: %v", err)
	}
	if got.SiteName != "Updated Site" {
		t.Errorf("unexpected SiteName: %q", got.SiteName)
	}
}

func TestListThemeVersions(t *testing.T) {
	want := []ThemeVersion{{Version: 1}, {Version: 2}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListThemeVersions(context.Background())
	if err != nil {
		t.Fatalf("ListThemeVersions: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 versions, got %d", len(got))
	}
}

func TestGetThemeVersion(t *testing.T) {
	want := ThemeVersion{Version: 5}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetThemeVersion(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetThemeVersion: %v", err)
	}
	if got.Version != 5 {
		t.Errorf("unexpected version: %d", got.Version)
	}
}

func TestRevertThemeVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.RevertThemeVersion(context.Background(), 3, "revert"); err != nil {
		t.Fatalf("RevertThemeVersion: %v", err)
	}
}

func TestPinThemeVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/pin") {
			t.Errorf("expected /pin suffix, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.PinThemeVersion(context.Background(), 2); err != nil {
		t.Fatalf("PinThemeVersion: %v", err)
	}
}

func TestUnpinThemeVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/unpin") {
			t.Errorf("expected /unpin suffix, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.UnpinThemeVersion(context.Background(), 2); err != nil {
		t.Fatalf("UnpinThemeVersion: %v", err)
	}
}

// --- Site Config ---

func TestGetSiteConfig(t *testing.T) {
	want := SiteConfig{TitleTemplate: "{{.Title}} | My Site"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetSiteConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSiteConfig: %v", err)
	}
	if got.TitleTemplate != "{{.Title}} | My Site" {
		t.Errorf("unexpected TitleTemplate: %q", got.TitleTemplate)
	}
}

func TestUpdateSiteConfig(t *testing.T) {
	want := SiteConfig{MarkdownScriptPolicy: "admin_only"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateSiteConfig(context.Background(), map[string]interface{}{"markdown_script_policy": "admin_only"})
	if err != nil {
		t.Fatalf("UpdateSiteConfig: %v", err)
	}
	if got.MarkdownScriptPolicy != "admin_only" {
		t.Errorf("unexpected policy: %q", got.MarkdownScriptPolicy)
	}
}

// --- Redirects ---

func TestListRedirects(t *testing.T) {
	want := []Redirect{{ID: "r1", FromPath: "/old", ToPath: "/new"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListRedirects(context.Background())
	if err != nil {
		t.Fatalf("ListRedirects: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 redirect, got %d", len(got))
	}
}

func TestGetRedirect(t *testing.T) {
	want := Redirect{ID: "r2"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetRedirect(context.Background(), "r2")
	if err != nil {
		t.Fatalf("GetRedirect: %v", err)
	}
	if got.ID != "r2" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestCreateRedirect(t *testing.T) {
	want := Redirect{ID: "r3", FromPath: "/a", ToPath: "/b"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateRedirect(context.Background(), CreateRedirectRequest{FromPath: "/a", ToPath: "/b"})
	if err != nil {
		t.Fatalf("CreateRedirect: %v", err)
	}
	if got.FromPath != "/a" {
		t.Errorf("unexpected FromPath: %q", got.FromPath)
	}
}

func TestUpdateRedirect(t *testing.T) {
	want := Redirect{ID: "r4", ToPath: "/updated"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateRedirect(context.Background(), "r4", map[string]interface{}{"to_path": "/updated"})
	if err != nil {
		t.Fatalf("UpdateRedirect: %v", err)
	}
	if got.ToPath != "/updated" {
		t.Errorf("unexpected ToPath: %q", got.ToPath)
	}
}

func TestDeleteRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteRedirect(context.Background(), "r5"); err != nil {
		t.Fatalf("DeleteRedirect: %v", err)
	}
}

// --- Folders ---

func TestListFolders(t *testing.T) {
	want := []Folder{{ID: "f1", Name: "Blog"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListFolders(context.Background())
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 folder, got %d", len(got))
	}
}

func TestGetFolder(t *testing.T) {
	want := Folder{ID: "f2", Name: "Docs"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetFolder(context.Background(), "f2")
	if err != nil {
		t.Fatalf("GetFolder: %v", err)
	}
	if got.Name != "Docs" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestCreateFolder(t *testing.T) {
	want := Folder{ID: "f3", Name: "News"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateFolder(context.Background(), CreateFolderRequest{Name: "News"})
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if got.Name != "News" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestDeleteFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteFolder(context.Background(), "f4"); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
}

// --- Collections ---

func TestListCollections(t *testing.T) {
	want := []Collection{{ID: "col1", Name: "Featured"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListCollections(context.Background())
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 collection, got %d", len(got))
	}
}

func TestGetCollection(t *testing.T) {
	want := Collection{ID: "col2"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetCollection(context.Background(), "col2")
	if err != nil {
		t.Fatalf("GetCollection: %v", err)
	}
	if got.ID != "col2" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
}

func TestCreateCollection(t *testing.T) {
	want := Collection{ID: "col3", Name: "Latest"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateCollection(context.Background(), map[string]interface{}{"name": "Latest"})
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if got.Name != "Latest" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestUpdateCollection(t *testing.T) {
	want := Collection{ID: "col4", Name: "Updated"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateCollection(context.Background(), "col4", map[string]interface{}{"name": "Updated"})
	if err != nil {
		t.Fatalf("UpdateCollection: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestDeleteCollection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteCollection(context.Background(), "col5"); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
}

// --- Search ---

func TestSearchContent(t *testing.T) {
	want := SearchResult{Query: "hello", Total: 2}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "hello" {
			t.Errorf("expected q=hello, got %q", r.URL.Query().Get("q"))
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.SearchContent(context.Background(), "hello", "", false)
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	if got.Total != 2 {
		t.Errorf("unexpected total: %d", got.Total)
	}
}

func TestSearchReplacePreview(t *testing.T) {
	want := SearchReplaceResult{TotalMatches: 5}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.SearchReplacePreview(context.Background(), "foo", "bar", false)
	if err != nil {
		t.Fatalf("SearchReplacePreview: %v", err)
	}
	if got.TotalMatches != 5 {
		t.Errorf("unexpected total matches: %d", got.TotalMatches)
	}
}

func TestSearchReplaceExecute(t *testing.T) {
	want := SearchReplaceResult{Success: true, TotalReplacements: 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.SearchReplaceExecute(context.Background(), "foo", "bar", "replace foo with bar", false, true)
	if err != nil {
		t.Fatalf("SearchReplaceExecute: %v", err)
	}
	if !got.Success {
		t.Error("expected success=true")
	}
}

func TestScopedSearchReplacePreview(t *testing.T) {
	want := SearchReplaceResult{TotalMatches: 2}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ScopedSearchReplacePreview(context.Background(), "old", "new", false, ScopedSearchReplaceScope{Category: "blog"})
	if err != nil {
		t.Fatalf("ScopedSearchReplacePreview: %v", err)
	}
	if got.TotalMatches != 2 {
		t.Errorf("unexpected total: %d", got.TotalMatches)
	}
}

func TestScopedSearchReplaceExecute(t *testing.T) {
	want := SearchReplaceResult{Success: true}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ScopedSearchReplaceExecute(context.Background(), "old", "new", "scoped replace", false, true, ScopedSearchReplaceScope{})
	if err != nil {
		t.Fatalf("ScopedSearchReplaceExecute: %v", err)
	}
	if !got.Success {
		t.Error("expected success")
	}
}

// --- API Keys ---

func TestListAPIKeys(t *testing.T) {
	want := []APIKey{{ID: "k1", Name: "CI Key"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(got) != 1 || got[0].Name != "CI Key" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestCreateAPIKey(t *testing.T) {
	want := APIKeyCreated{ID: "k2", Key: "lc_newkey", Name: "Deploy"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateAPIKey(context.Background(), "Deploy", "Deployment key")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if got.Key != "lc_newkey" {
		t.Errorf("unexpected key: %q", got.Key)
	}
}

func TestDeleteAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteAPIKey(context.Background(), "k3"); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
}

// --- Misc ---

func TestRegenerateAllContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.RegenerateAllContent(context.Background()); err != nil {
		t.Fatalf("RegenerateAllContent: %v", err)
	}
}

func TestEndUserSearch(t *testing.T) {
	want := EndUserSearchResult{Query: "cats", Total: 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "cats" {
			t.Errorf("expected q=cats, got %q", r.URL.Query().Get("q"))
		}
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.EndUserSearch(context.Background(), "cats", "semantic", 10)
	if err != nil {
		t.Fatalf("EndUserSearch: %v", err)
	}
	if got.Total != 3 {
		t.Errorf("unexpected total: %d", got.Total)
	}
}

func TestEndUserSearch_NoModeOrLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mode") != "" {
			t.Errorf("expected no mode param, got %q", r.URL.Query().Get("mode"))
		}
		if r.URL.Query().Get("limit") != "" {
			t.Errorf("expected no limit param, got %q", r.URL.Query().Get("limit"))
		}
		json.NewEncoder(w).Encode(EndUserSearchResult{})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	c.EndUserSearch(context.Background(), "test", "", 0) //nolint
}

func TestReindexEmbeddings(t *testing.T) {
	want := map[string]interface{}{"indexed": float64(42)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ReindexEmbeddings(context.Background())
	if err != nil {
		t.Fatalf("ReindexEmbeddings: %v", err)
	}
	if got["indexed"] != float64(42) {
		t.Errorf("unexpected result: %v", got)
	}
}

// --- Snippet methods ---

func TestListSnippets(t *testing.T) {
	want := []Snippet{{ID: "s1", Name: "footer-cta"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.ListSnippets(context.Background())
	if err != nil {
		t.Fatalf("ListSnippets: %v", err)
	}
	if len(got) != 1 || got[0].Name != "footer-cta" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestGetSnippet(t *testing.T) {
	want := Snippet{ID: "s2", HTML: "<p>hi</p>"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.GetSnippet(context.Background(), "s2")
	if err != nil {
		t.Fatalf("GetSnippet: %v", err)
	}
	if got.HTML != "<p>hi</p>" {
		t.Errorf("unexpected HTML: %q", got.HTML)
	}
}

func TestCreateSnippet(t *testing.T) {
	want := Snippet{ID: "s3", Name: "callout"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.CreateSnippet(context.Background(), CreateSnippetRequest{Name: "callout", HTML: "<div/>"})
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	if got.Name != "callout" {
		t.Errorf("unexpected name: %q", got.Name)
	}
}

func TestUpdateSnippet(t *testing.T) {
	want := Snippet{ID: "s4", HTML: "<div>updated</div>"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	got, err := c.UpdateSnippet(context.Background(), "s4", UpdateSnippetRequest{HTML: "<div>updated</div>"})
	if err != nil {
		t.Fatalf("UpdateSnippet: %v", err)
	}
	if got.HTML != "<div>updated</div>" {
		t.Errorf("unexpected HTML: %q", got.HTML)
	}
}

func TestDeleteSnippet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "tok")
	if err := c.DeleteSnippet(context.Background(), "s5"); err != nil {
		t.Fatalf("DeleteSnippet: %v", err)
	}
}
