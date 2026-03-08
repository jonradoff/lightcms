package handlers

import (
	"net/http"
	"strconv"

	"lightcms/internal/services"
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

// APIReindexEmbeddings triggers batch embedding generation via API
func (a *APIHandler) APIReindexEmbeddings(w http.ResponseWriter, r *http.Request) {
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
