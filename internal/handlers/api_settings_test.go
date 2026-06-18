package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ---------------------------------------------------------------------------
// Theme
// ---------------------------------------------------------------------------

func TestAPIGetTheme(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/theme", nil)
	ah.APIGetTheme(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Theme should exist (defaults created on first access)
	if body == nil {
		t.Fatal("expected non-nil theme response")
	}
}

func TestAPIUpdateTheme(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"site_name":"My Updated Site"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/theme", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUpdateTheme(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["site_name"] != "My Updated Site" {
		t.Fatalf("expected site_name=My Updated Site, got %v", body["site_name"])
	}
}

func TestAPIUpdateTheme_InvalidBody(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/theme", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUpdateTheme(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Theme Versions
// ---------------------------------------------------------------------------

func TestAPIListThemeVersions_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/theme/versions", nil)
	ah.APIListThemeVersions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIGetThemeVersion_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/theme/versions/{version}", ah.APIGetThemeVersion).Methods(http.MethodGet)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/theme/versions/99999", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] == nil {
		t.Fatal("expected error field in response")
	}
}

func TestAPIRevertThemeVersion_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/theme/versions/{version}/revert", ah.APIRevertThemeVersion).Methods(http.MethodPost)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/theme/versions/99999/revert", nil)
	router.ServeHTTP(rr, req)

	// Should return 500 (service error) since version doesn't exist
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIPinThemeVersion_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/theme/versions/{version}/pin", ah.APIPinThemeVersion).Methods(http.MethodPost)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/theme/versions/99999/pin", nil)
	router.ServeHTTP(rr, req)

	// Pin is idempotent — MongoDB UpdateOne succeeds even for non-existent versions
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Site Config
// ---------------------------------------------------------------------------

func TestAPIGetSiteConfig(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/config", nil)
	ah.APIGetSiteConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body == nil {
		t.Fatal("expected non-nil config response")
	}
}

func TestAPIUpdateSiteConfig(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"title_template":"{{.Title}} - My Site"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/config", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUpdateSiteConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	// SiteConfig has no json tags on TitleTemplate, so Go serializes as "TitleTemplate"
	if body["TitleTemplate"] != "{{.Title}} - My Site" {
		t.Fatalf("expected updated TitleTemplate, got %v", body["TitleTemplate"])
	}
}

func TestAPIUpdateSiteConfig_InvalidBody(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/config", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUpdateSiteConfig(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Redirects
// ---------------------------------------------------------------------------

func TestAPIListRedirects_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/redirects", nil)
	ah.APIListRedirects(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestAPICreateRedirect(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"from_path":"/old-page","to_path":"/new-page","status_code":301}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/redirects", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateRedirect(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["from_path"] != "/old-page" {
		t.Fatalf("expected from_path=/old-page, got %v", body["from_path"])
	}
	if body["to_path"] != "/new-page" {
		t.Fatalf("expected to_path=/new-page, got %v", body["to_path"])
	}
}

func TestAPICreateRedirect_MissingFields(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"from_path":"/only-from"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/redirects", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateRedirect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPIGetRedirect_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/redirects/{id}", ah.APIGetRedirect).Methods(http.MethodGet)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/redirects/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIUpdateRedirect(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Create a redirect first
	createPayload := `{"from_path":"/a","to_path":"/b","status_code":301}`
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/redirects", strings.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	ah.APICreateRedirect(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("create failed: %d; body: %s", createRR.Code, createRR.Body.String())
	}
	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		id, _ = created["_id"].(string)
	}
	if id == "" {
		t.Fatalf("could not extract redirect ID: %v", created)
	}

	// Update it
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/redirects/{id}", ah.APIUpdateRedirect).Methods(http.MethodPut)

	updatePayload := `{"to_path":"/c"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/redirects/"+id, strings.NewReader(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["to_path"] != "/c" {
		t.Fatalf("expected to_path=/c, got %v", body["to_path"])
	}
}

func TestAPIDeleteRedirect(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Create a redirect first
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/redirects", strings.NewReader(`{"from_path":"/del","to_path":"/gone","status_code":301}`))
	createReq.Header.Set("Content-Type", "application/json")
	ah.APICreateRedirect(createRR, createReq)

	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		id, _ = created["_id"].(string)
	}
	if id == "" {
		t.Fatalf("could not extract redirect ID: %v", created)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/redirects/{id}", ah.APIDeleteRedirect).Methods(http.MethodDelete)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/redirects/"+id, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

// ---------------------------------------------------------------------------
// Folders
// ---------------------------------------------------------------------------

func TestAPIListFolders_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/folders", nil)
	ah.APIListFolders(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestAPICreateFolder(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"Blog","slug":"blog"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/folders", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateFolder(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "Blog" {
		t.Fatalf("expected name=Blog, got %v", body["name"])
	}
}

func TestAPICreateFolder_MissingFields(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"OnlyName"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/folders", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateFolder(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPIGetFolder_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/folders/{id}", ah.APIGetFolder).Methods(http.MethodGet)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/folders/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIDeleteFolder_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/folders/{id}", ah.APIDeleteFolder).Methods(http.MethodDelete)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/folders/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	// DeleteFolder succeeds for non-existent IDs (MongoDB DeleteOne is idempotent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Collections
// ---------------------------------------------------------------------------

func TestAPIListCollections_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/collections", nil)
	ah.APIListCollections(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestAPICreateCollection(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"Articles","slug":"articles","description":"All articles"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/collections", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateCollection(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "Articles" {
		t.Fatalf("expected name=Articles, got %v", body["name"])
	}
	if body["slug"] != "articles" {
		t.Fatalf("expected slug=articles, got %v", body["slug"])
	}
}

func TestAPICreateCollection_MissingFields(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"NoSlug"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/collections", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateCollection(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPIGetCollection_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/collections/{id}", ah.APIGetCollection).Methods(http.MethodGet)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/collections/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIUpdateCollection(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Create first
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/collections", strings.NewReader(`{"name":"Docs","slug":"docs"}`))
	createReq.Header.Set("Content-Type", "application/json")
	ah.APICreateCollection(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("create failed: %d; body: %s", createRR.Code, createRR.Body.String())
	}
	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		id, _ = created["_id"].(string)
	}
	if id == "" {
		t.Fatalf("could not extract collection ID: %v", created)
	}

	// Update
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/collections/{id}", ah.APIUpdateCollection).Methods(http.MethodPut)

	updatePayload := `{"name":"Documentation","description":"Updated docs"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/collections/"+id, strings.NewReader(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "Documentation" {
		t.Fatalf("expected name=Documentation, got %v", body["name"])
	}
}

func TestAPIDeleteCollection_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/collections/{id}", ah.APIDeleteCollection).Methods(http.MethodDelete)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/collections/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	// DeleteCollection succeeds even for non-existent IDs (MongoDB DeleteOne is idempotent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Regenerate All Content
// ---------------------------------------------------------------------------

func TestAPIRegenerateAllContent(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/regenerate", nil)
	ah.APIRegenerateAllContent(rr, req)

	// Should return 200 (even with no content to regenerate)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

// ---------------------------------------------------------------------------
// Assets: List
// ---------------------------------------------------------------------------

func TestAPIListAssets_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/assets", nil)
	ah.APIListAssets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

// ---------------------------------------------------------------------------
// Assets: Get
// ---------------------------------------------------------------------------

func TestAPIGetAsset_InvalidID(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/assets/{id}", ah.APIGetAsset).Methods(http.MethodGet)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/assets/not-a-valid-id", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIGetAsset_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/assets/{id}", ah.APIGetAsset).Methods(http.MethodGet)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/assets/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Assets: Get By Path
// ---------------------------------------------------------------------------

func TestAPIGetAssetByPath_MissingPath(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/assets/by-path", nil)
	ah.APIGetAssetByPath(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIGetAssetByPath_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/assets/by-path?path=/assets/nonexistent.png", nil)
	ah.APIGetAssetByPath(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Assets: Delete
// ---------------------------------------------------------------------------

func TestAPIDeleteAsset_InvalidID(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/assets/{id}", ah.APIDeleteAsset).Methods(http.MethodDelete)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/assets/bad-id", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIDeleteAsset_NotFound(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/assets/{id}", ah.APIDeleteAsset).Methods(http.MethodDelete)

	fakeID := primitive.NewObjectID().Hex()
	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/assets/"+fakeID, nil)
	router.ServeHTTP(rr, req)

	// DeleteAsset returns 500 on service error for non-existent asset
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Assets: List Folders
// ---------------------------------------------------------------------------

func TestAPIListAssetFolders_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/assets/folders", nil)
	ah.APIListAssetFolders(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Assets: Upload (missing required fields)
// ---------------------------------------------------------------------------

func TestAPIUploadAsset_MissingFile(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Send empty JSON body — missing filename, serve_path, data_base64
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/assets", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUploadAsset(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Assets: Upload From URL
// ---------------------------------------------------------------------------

func TestAPIUploadAssetFromURL_MissingFields(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Missing url field
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/assets/from-url", strings.NewReader(`{"serve_path":"/assets/test.png"}`))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUploadAssetFromURL(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIUploadAssetFromURL_InvalidBody(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/assets/from-url", strings.NewReader(`{broken`))
	req.Header.Set("Content-Type", "application/json")
	ah.APIUploadAssetFromURL(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}
