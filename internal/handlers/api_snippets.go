package handlers

import (
	"net/http"

	"lightcms/internal/auth"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *APIHandler) APIListSnippets(w http.ResponseWriter, r *http.Request) {
	snippets, err := a.snippetService.ListSnippets(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, snippets)
}

func (a *APIHandler) APIGetSnippet(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid snippet ID")
		return
	}
	snip, err := a.snippetService.GetSnippet(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "snippet not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, snip)
}

func (a *APIHandler) APICreateSnippet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermTemplateEdit) {
		return
	}
	var req struct {
		Name string `json:"name"`
		HTML string `json:"html"`
	}
	if err := a.decodeJSON(r, &req); err != nil || req.Name == "" {
		a.jsonError(w, http.StatusBadRequest, "name and html are required")
		return
	}
	snip, err := a.snippetService.CreateSnippet(r.Context(), req.Name, req.HTML)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.auditLog(r, "snippet.create", "snippet", snip.ID.Hex(), map[string]interface{}{"name": snip.Name})
	a.jsonResponse(w, http.StatusCreated, snip)
}

func (a *APIHandler) APIUpdateSnippet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermTemplateEdit) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid snippet ID")
		return
	}
	var req struct {
		Name string `json:"name"`
		HTML string `json:"html"`
	}
	if err := a.decodeJSON(r, &req); err != nil || req.Name == "" {
		a.jsonError(w, http.StatusBadRequest, "name and html are required")
		return
	}
	snip, err := a.snippetService.UpdateSnippet(r.Context(), id, req.Name, req.HTML)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "snippet.update", "snippet", id.Hex(), map[string]interface{}{"name": snip.Name})
	a.jsonResponse(w, http.StatusOK, snip)
}

func (a *APIHandler) APIDeleteSnippet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermTemplateEdit) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid snippet ID")
		return
	}
	if err := a.snippetService.DeleteSnippet(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "snippet.delete", "snippet", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}
