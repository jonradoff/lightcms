package handlers

import (
	"net/http"
	"strconv"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/services"
)

// SetSearchService sets the search service on the API handler
func (a *APIHandler) SetSearchService(ss *services.SearchService) {
	a.searchService = ss
}

// APIEndUserSearch handles the authenticated API v1 search endpoint
func (a *APIHandler) APIEndUserSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		a.jsonError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "hybrid"
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	results, err := a.searchService.Search(r.Context(), query, mode, limit)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"query":   query,
		"mode":    mode,
		"results": results,
		"total":   len(results),
	})
}

// APIEndUserSearchSuggest handles the authenticated API v1 suggest endpoint
func (a *APIHandler) APIEndUserSearchSuggest(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("q")
	if prefix == "" || len(prefix) < 2 {
		a.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"keywords": []string{},
			"pages":    []interface{}{},
		})
		return
	}

	if len(prefix) > 100 {
		prefix = prefix[:100]
	}

	limit := 8
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}

	result, err := a.searchService.Suggest(r.Context(), prefix, limit)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, result)
}

// APIReindexEmbeddings triggers batch embedding generation via API (admin only)
func (a *APIHandler) APIReindexEmbeddings(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	processed, errCount, err := a.searchService.BatchGenerateEmbeddings(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"processed": processed,
		"errors":    errCount,
		"message":   "Reindex complete",
	})
}
