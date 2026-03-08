package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"lightcms/internal/services"
)

// APIHandler handles REST API endpoints (JSON-only, no sessions/templates)
type APIHandler struct {
	contentService  *services.ContentService
	templateService *services.TemplateService
	assetService    *services.AssetService
	settingsService *services.SettingsService
	apiKeyService   *services.APIKeyService
	searchService   *services.SearchService
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(
	contentService *services.ContentService,
	templateService *services.TemplateService,
	assetService *services.AssetService,
	settingsService *services.SettingsService,
	apiKeyService *services.APIKeyService,
) *APIHandler {
	return &APIHandler{
		contentService:  contentService,
		templateService: templateService,
		assetService:    assetService,
		settingsService: settingsService,
		apiKeyService:   apiKeyService,
	}
}

// jsonResponse writes a JSON response with the given status code
func (a *APIHandler) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// jsonError writes a JSON error response
func (a *APIHandler) jsonError(w http.ResponseWriter, statusCode int, message string) {
	a.jsonResponse(w, statusCode, map[string]interface{}{
		"error": message,
	})
}

// decodeJSON reads and decodes the request body into the given target
func (a *APIHandler) decodeJSON(r *http.Request, target interface{}) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
