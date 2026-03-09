package handlers

import (
	"net/http"

	"lightcms/internal/auth"
	"lightcms/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// API Template endpoints

func (a *APIHandler) APIListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := a.templateService.ListTemplates(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, templates)
}

func (a *APIHandler) APIGetTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]

	// Try as slug first via query param
	if slug := r.URL.Query().Get("slug"); slug != "" {
		tmpl, err := a.templateService.GetTemplateBySlug(r.Context(), slug)
		if err != nil {
			a.jsonError(w, http.StatusNotFound, "template not found")
			return
		}
		a.jsonResponse(w, http.StatusOK, tmpl)
		return
	}

	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		// Try as slug
		tmpl, err := a.templateService.GetTemplateBySlug(r.Context(), idStr)
		if err != nil {
			a.jsonError(w, http.StatusNotFound, "template not found")
			return
		}
		a.jsonResponse(w, http.StatusOK, tmpl)
		return
	}

	tmpl, err := a.templateService.GetTemplate(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "template not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, tmpl)
}

func (a *APIHandler) APICreateTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermTemplateCreate) {
		return
	}

	var req struct {
		Name        string                `json:"name"`
		Slug        string                `json:"slug"`
		Description string                `json:"description"`
		Fields      []models.TemplateField `json:"fields"`
		HTMLLayout  string                `json:"html_layout"`
		Category    string                `json:"category"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" {
		a.jsonError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	tmpl := &models.Template{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Fields:      req.Fields,
		HTMLLayout:  req.HTMLLayout,
		Category:    req.Category,
	}

	if err := a.templateService.CreateTemplate(r.Context(), tmpl); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "template.create", "template", tmpl.ID.Hex(), map[string]interface{}{"name": tmpl.Name})
	a.jsonResponse(w, http.StatusCreated, tmpl)
}

func (a *APIHandler) APIUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermTemplateEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	// Get existing
	tmpl, err := a.templateService.GetTemplate(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "template not found")
		return
	}

	var req struct {
		Name        *string                `json:"name"`
		Slug        *string                `json:"slug"`
		Description *string                `json:"description"`
		Fields      []models.TemplateField `json:"fields"`
		HTMLLayout  *string                `json:"html_layout"`
		Category    *string                `json:"category"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		tmpl.Name = *req.Name
	}
	if req.Slug != nil {
		tmpl.Slug = *req.Slug
	}
	if req.Description != nil {
		tmpl.Description = *req.Description
	}
	if req.Fields != nil {
		tmpl.Fields = req.Fields
	}
	if req.HTMLLayout != nil {
		tmpl.HTMLLayout = *req.HTMLLayout
	}
	if req.Category != nil {
		tmpl.Category = *req.Category
	}

	if err := a.templateService.UpdateTemplate(r.Context(), tmpl); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "template.update", "template", tmpl.ID.Hex(), map[string]interface{}{"name": tmpl.Name})
	a.jsonResponse(w, http.StatusOK, tmpl)
}

func (a *APIHandler) APIDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermTemplateDelete) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	if err := a.templateService.DeleteTemplate(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.auditLog(r, "template.delete", "template", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}
