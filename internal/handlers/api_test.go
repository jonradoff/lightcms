package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/middleware"

	"github.com/gorilla/mux"
)

// ---------------------------------------------------------------------------
// 1. api.go utilities
// ---------------------------------------------------------------------------

func TestJsonResponse(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	ah.jsonResponse(rr, http.StatusOK, map[string]string{"hello": "world"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["hello"] != "world" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestJsonError(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	ah.jsonError(rr, http.StatusBadRequest, "something broke")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "something broke" {
		t.Fatalf("unexpected error field: %v", body)
	}
}

func TestDecodeJSON(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	req := authReq(http.MethodPost, "/", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")

	var target struct {
		Name string `json:"name"`
	}
	if err := ah.decodeJSON(req, &target); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	if target.Name != "test" {
		t.Fatalf("expected 'test', got %q", target.Name)
	}
}

func TestGetAPIUser_NilContext(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := ah.getAPIUser(req)
	if user != nil {
		t.Fatal("expected nil user from empty context")
	}
}

func TestRequirePermission_NilUser(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No user in context => denied (legacy keys without a user are rejected).
	if ah.requirePermission(rr, req, "anything") {
		t.Fatal("expected requirePermission to return false for nil user")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for nil user, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// 2. healthz.go
// ---------------------------------------------------------------------------

func TestHealthz_OK(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/healthz", nil)
	h.Healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("expected status=healthy, got %v", body["status"])
	}
	if body["name"] != "LightCMS" {
		t.Fatalf("expected name=LightCMS, got %v", body["name"])
	}
	deps, ok := body["dependencies"].([]interface{})
	if !ok {
		t.Fatalf("expected dependencies array, got %T", body["dependencies"])
	}
	if len(deps) == 0 {
		t.Fatal("expected at least one dependency")
	}
}

// ---------------------------------------------------------------------------
// 3. api_templates.go
// ---------------------------------------------------------------------------

func TestAPIListTemplates_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/templates", nil)
	ah.APIListTemplates(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestAPIListTemplates_WithData(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	seedTemplate(t, db, "Blog", "blog")

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/templates", nil)
	ah.APIListTemplates(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 1 {
		t.Fatalf("expected 1 template, got %d", len(body))
	}
}

func TestAPICreateTemplate(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"Page","slug":"page","html_layout":"<html></html>"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/templates", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateTemplate(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "Page" {
		t.Fatalf("expected name=Page, got %v", body["name"])
	}
}

func TestAPICreateTemplate_MissingFields(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"OnlyName"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/templates", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPIGetTemplate_ByID(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	id := seedTemplate(t, db, "ByID", "by-id")

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/templates/{id}", ah.APIGetTemplate).Methods(http.MethodGet)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/templates/"+id.Hex(), nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "ByID" {
		t.Fatalf("expected name=ByID, got %v", body["name"])
	}
}

func TestAPIGetTemplate_BySlug(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	seedTemplate(t, db, "BySlug", "by-slug")

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/templates/{id}", ah.APIGetTemplate).Methods(http.MethodGet)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/templates/placeholder?slug=by-slug", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "BySlug" {
		t.Fatalf("expected name=BySlug, got %v", body["name"])
	}
}

func TestAPIUpdateTemplate(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	id := seedTemplate(t, db, "Original", "original")

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/templates/{id}", ah.APIUpdateTemplate).Methods(http.MethodPut)

	payload := `{"name":"Updated"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/templates/"+id.Hex(), strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "Updated" {
		t.Fatalf("expected name=Updated, got %v", body["name"])
	}
}

func TestAPIDeleteTemplate(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()

	id := seedTemplate(t, db, "ToDelete", "to-delete")

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/templates/{id}", ah.APIDeleteTemplate).Methods(http.MethodDelete)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/templates/"+id.Hex(), nil)
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
// 4. api_snippets.go
// ---------------------------------------------------------------------------

func TestAPIListSnippets_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/snippets", nil)
	ah.APIListSnippets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestAPICreateSnippet(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"footer","html":"<footer>bye</footer>"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/snippets", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateSnippet(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["name"] != "footer" {
		t.Fatalf("expected name=footer, got %v", body["name"])
	}
}

func TestAPICreateSnippet_MissingName(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"html":"<p>no name</p>"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/snippets", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateSnippet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPIGetSnippet(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Create first
	payload := `{"name":"getme","html":"<p>hi</p>"}`
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/snippets", strings.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	ah.APICreateSnippet(createRR, createReq)

	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		// Try _id
		id, _ = created["_id"].(string)
	}
	if id == "" {
		t.Fatalf("could not extract snippet ID from create response: %v", created)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/snippets/{id}", ah.APIGetSnippet).Methods(http.MethodGet)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/snippets/"+id, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIUpdateSnippet(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Create
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/snippets", strings.NewReader(`{"name":"updme","html":"<p>old</p>"}`))
	createReq.Header.Set("Content-Type", "application/json")
	ah.APICreateSnippet(createRR, createReq)

	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		id, _ = created["_id"].(string)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/snippets/{id}", ah.APIUpdateSnippet).Methods(http.MethodPut)

	payload := `{"name":"updme","html":"<p>new</p>"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPut, "/api/v1/snippets/"+id, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIDeleteSnippet(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Create
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/snippets", strings.NewReader(`{"name":"delme","html":"<p>bye</p>"}`))
	createReq.Header.Set("Content-Type", "application/json")
	ah.APICreateSnippet(createRR, createReq)

	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		id, _ = created["_id"].(string)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/snippets/{id}", ah.APIDeleteSnippet).Methods(http.MethodDelete)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/snippets/"+id, nil)
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
// 5. api_apikeys.go
// ---------------------------------------------------------------------------

func TestAPIListAPIKeys_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/api-keys", nil)
	ah.APIListAPIKeys(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body []interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestAPICreateAPIKey(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"name":"test-key","description":"for testing"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/api-keys", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateAPIKey(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["key"] == nil || body["key"] == "" {
		t.Fatal("expected key in response")
	}
	if body["name"] != "test-key" {
		t.Fatalf("expected name=test-key, got %v", body["name"])
	}
}

func TestAPICreateAPIKey_MissingName(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	payload := `{"description":"no name"}`
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/api-keys", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	ah.APICreateAPIKey(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPIDeleteAPIKey(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	// Inject an admin user so the delete handler can determine key ownership
	adminUser := &auth.SessionUser{ID: "000000000000000000000001", Email: "admin@test", Role: "admin"}

	// Create first (as admin)
	createRR := httptest.NewRecorder()
	createReq := authReq(http.MethodPost, "/api/v1/api-keys", strings.NewReader(`{"name":"del-key"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(middleware.InjectAPIUser(createReq.Context(), adminUser))
	ah.APICreateAPIKey(createRR, createReq)

	var created map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("could not extract API key ID from create response: %v", created)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/api-keys/{id}", ah.APIDeleteAPIKey).Methods(http.MethodDelete)

	rr := httptest.NewRecorder()
	req := authReq(http.MethodDelete, "/api/v1/api-keys/"+id, nil)
	req = req.WithContext(middleware.InjectAPIUser(req.Context(), adminUser))
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
// 6. api_search.go
// ---------------------------------------------------------------------------

func TestAPIEndUserSearch_MissingQuery(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/search", nil)
	ah.APIEndUserSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] == nil {
		t.Fatal("expected error field in response")
	}
}

func TestAPIEndUserSearch_Empty(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/search?q=nonexistent", nil)
	ah.APIEndUserSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["query"] != "nonexistent" {
		t.Fatalf("expected query=nonexistent, got %v", body["query"])
	}
	results, ok := body["results"].([]interface{})
	if !ok {
		t.Fatalf("expected results array, got %T", body["results"])
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}

func TestAPIReindexEmbeddings_NoVoyageKey(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/search/reindex", nil)
	ah.APIReindexEmbeddings(rr, req)

	// Without a Voyage API key, expect 500
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 without Voyage key, got %d; body: %s", rr.Code, rr.Body.String())
	}
}
