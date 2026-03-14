package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"lightcms/internal/auth"
	"lightcms/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// API Settings endpoints (theme, config, redirects, folders, collections)

// Theme

func (a *APIHandler) APIGetTheme(w http.ResponseWriter, r *http.Request) {
	theme, err := a.settingsService.GetTheme(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, theme)
}

func (a *APIHandler) APIUpdateTheme(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	// Get current theme as base
	theme, err := a.settingsService.GetTheme(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		SiteName        *string `json:"site_name"`
		SiteTagline     *string `json:"site_tagline"`
		LogoURL         *string `json:"logo_url"`
		PrimaryColor    *string `json:"primary_color"`
		SecondaryColor  *string `json:"secondary_color"`
		AccentColor     *string `json:"accent_color"`
		BackgroundColor *string `json:"background_color"`
		TextColor       *string `json:"text_color"`
		FontFamily      *string `json:"font_family"`
		HeadingFont     *string `json:"heading_font"`
		BorderRadius    *string `json:"border_radius"`
		CustomCSS       *string `json:"custom_css"`
		HeadHTML        *string `json:"head_html"`
		HeaderHTML      *string `json:"header_html"`
		FooterHTML      *string `json:"footer_html"`
		VersionComment  *string `json:"version_comment"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SiteName != nil {
		theme.SiteName = *req.SiteName
	}
	if req.SiteTagline != nil {
		theme.SiteTagline = *req.SiteTagline
	}
	if req.LogoURL != nil {
		theme.LogoURL = *req.LogoURL
	}
	if req.PrimaryColor != nil {
		theme.PrimaryColor = *req.PrimaryColor
	}
	if req.SecondaryColor != nil {
		theme.SecondaryColor = *req.SecondaryColor
	}
	if req.AccentColor != nil {
		theme.AccentColor = *req.AccentColor
	}
	if req.BackgroundColor != nil {
		theme.BackgroundColor = *req.BackgroundColor
	}
	if req.TextColor != nil {
		theme.TextColor = *req.TextColor
	}
	if req.FontFamily != nil {
		theme.FontFamily = *req.FontFamily
	}
	if req.HeadingFont != nil {
		theme.HeadingFont = *req.HeadingFont
	}
	if req.BorderRadius != nil {
		theme.BorderRadius = *req.BorderRadius
	}
	if req.CustomCSS != nil {
		theme.CustomCSS = *req.CustomCSS
	}
	if req.HeadHTML != nil {
		theme.HeadHTML = *req.HeadHTML
	}
	if req.HeaderHTML != nil {
		theme.HeaderHTML = *req.HeaderHTML
	}
	if req.FooterHTML != nil {
		theme.FooterHTML = *req.FooterHTML
	}

	args := []string{}
	if req.VersionComment != nil && *req.VersionComment != "" {
		args = append(args, *req.VersionComment)
	}

	if err := a.settingsService.UpdateTheme(r.Context(), theme, args...); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "theme.update", "theme", "", nil)
	a.jsonResponse(w, http.StatusOK, theme)
}

func (a *APIHandler) APIListThemeVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := a.settingsService.GetThemeVersions(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, versions)
}

func (a *APIHandler) APIGetThemeVersion(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(mux.Vars(r)["version"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	v, err := a.settingsService.GetThemeVersion(r.Context(), version)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "version not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, v)
}

func (a *APIHandler) APIRevertThemeVersion(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	version, err := strconv.Atoi(mux.Vars(r)["version"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	var req struct {
		VersionComment string `json:"version_comment"`
	}
	a.decodeJSON(r, &req)

	args := []string{}
	if req.VersionComment != "" {
		args = append(args, req.VersionComment)
	}

	if err := a.settingsService.RevertThemeToVersion(r.Context(), version, args...); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "theme.revert", "theme", "", map[string]interface{}{"version": version})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIPinThemeVersion locks or unlocks a theme version.
// POST /api/v1/theme/versions/{version}/pin  → locks
// POST /api/v1/theme/versions/{version}/unpin → unlocks
func (a *APIHandler) APIPinThemeVersion(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	version, err := strconv.Atoi(mux.Vars(r)["version"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid version number")
		return
	}
	locked := !strings.Contains(r.URL.Path, "/unpin")
	if err := a.settingsService.SetThemeVersionLocked(r.Context(), version, locked); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	action := "theme.pin_version"
	if !locked {
		action = "theme.unpin_version"
	}
	a.auditLog(r, action, "theme", "", map[string]interface{}{"version": version})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "version": version, "locked": locked})
}

// Site Config

func (a *APIHandler) APIGetSiteConfig(w http.ResponseWriter, r *http.Request) {
	config, err := a.settingsService.GetSiteConfig(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, config)
}

func (a *APIHandler) APIUpdateSiteConfig(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	var req struct {
		TitleTemplate        *string `json:"title_template"`
		TitleTemplateNoTitle *string `json:"title_template_no_title"`
		MarkdownScriptPolicy *string `json:"markdown_script_policy"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	config, err := a.settingsService.GetSiteConfig(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.TitleTemplate != nil {
		config.TitleTemplate = *req.TitleTemplate
	}
	if req.TitleTemplateNoTitle != nil {
		config.TitleTemplateNoTitle = *req.TitleTemplateNoTitle
	}
	if req.MarkdownScriptPolicy != nil {
		switch *req.MarkdownScriptPolicy {
		case "", "all", "admin_only", "none":
			config.MarkdownScriptPolicy = *req.MarkdownScriptPolicy
		default:
			a.jsonError(w, http.StatusBadRequest, "markdown_script_policy must be 'all', 'admin_only', or 'none'")
			return
		}
	}

	if err := a.settingsService.UpdateSiteConfig(r.Context(), config); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "config.update", "config", "", nil)
	a.jsonResponse(w, http.StatusOK, config)
}

// Redirects

func (a *APIHandler) APIListRedirects(w http.ResponseWriter, r *http.Request) {
	redirects, err := a.settingsService.ListRedirects(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, redirects)
}

func (a *APIHandler) APIGetRedirect(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid redirect ID")
		return
	}

	redirect, err := a.settingsService.GetRedirect(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "redirect not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, redirect)
}

func (a *APIHandler) APICreateRedirect(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	var req struct {
		FromPath    string `json:"from_path"`
		ToPath      string `json:"to_path"`
		StatusCode  int    `json:"status_code"`
		Description string `json:"description"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FromPath == "" || req.ToPath == "" {
		a.jsonError(w, http.StatusBadRequest, "from_path and to_path are required")
		return
	}

	redirect := &models.Redirect{
		FromPath:    req.FromPath,
		ToPath:      req.ToPath,
		StatusCode:  req.StatusCode,
		Description: req.Description,
	}

	if err := a.settingsService.CreateRedirect(r.Context(), redirect); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "redirect.create", "redirect", redirect.ID.Hex(), map[string]interface{}{"from": redirect.FromPath, "to": redirect.ToPath})
	a.jsonResponse(w, http.StatusCreated, redirect)
}

func (a *APIHandler) APIUpdateRedirect(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid redirect ID")
		return
	}

	redirect, err := a.settingsService.GetRedirect(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "redirect not found")
		return
	}

	var req struct {
		FromPath    *string `json:"from_path"`
		ToPath      *string `json:"to_path"`
		StatusCode  *int    `json:"status_code"`
		Description *string `json:"description"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FromPath != nil {
		redirect.FromPath = *req.FromPath
	}
	if req.ToPath != nil {
		redirect.ToPath = *req.ToPath
	}
	if req.StatusCode != nil {
		redirect.StatusCode = *req.StatusCode
	}
	if req.Description != nil {
		redirect.Description = *req.Description
	}

	if err := a.settingsService.UpdateRedirect(r.Context(), redirect); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "redirect.update", "redirect", redirect.ID.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, redirect)
}

func (a *APIHandler) APIDeleteRedirect(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid redirect ID")
		return
	}

	if err := a.settingsService.DeleteRedirect(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "redirect.delete", "redirect", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Folders

func (a *APIHandler) APIListFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := a.settingsService.ListFolders(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, folders)
}

func (a *APIHandler) APIGetFolder(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid folder ID")
		return
	}

	folder, err := a.settingsService.GetFolder(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "folder not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, folder)
}

func (a *APIHandler) APICreateFolder(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	var req struct {
		Name     string `json:"name"`
		Slug     string `json:"slug"`
		ParentID string `json:"parent_id"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" {
		a.jsonError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	folder := &models.Folder{
		Name: req.Name,
		Slug: req.Slug,
	}

	if req.ParentID != "" {
		pid, err := primitive.ObjectIDFromHex(req.ParentID)
		if err != nil {
			a.jsonError(w, http.StatusBadRequest, "invalid parent_id")
			return
		}
		folder.ParentID = &pid
	}

	if err := a.settingsService.CreateFolder(r.Context(), folder); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "folder.create", "folder", folder.ID.Hex(), map[string]interface{}{"name": folder.Name})
	a.jsonResponse(w, http.StatusCreated, folder)
}

func (a *APIHandler) APIDeleteFolder(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid folder ID")
		return
	}

	if err := a.settingsService.DeleteFolder(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.auditLog(r, "folder.delete", "folder", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Collections

func (a *APIHandler) APIListCollections(w http.ResponseWriter, r *http.Request) {
	collections, err := a.settingsService.ListCollections(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, collections)
}

func (a *APIHandler) APIGetCollection(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	collection, err := a.settingsService.GetCollection(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "collection not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, collection)
}

func (a *APIHandler) APICreateCollection(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	var req struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		Description  string `json:"description"`
		Category     string `json:"category"`
		SortField    string `json:"sort_field"`
		SortOrder    string `json:"sort_order"`
		ItemTemplate string `json:"item_template"`
		PageTemplate string `json:"page_template"`
		ItemsPerPage int    `json:"items_per_page"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" {
		a.jsonError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	collection := &models.Collection{
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		Category:     req.Category,
		SortField:    req.SortField,
		SortOrder:    req.SortOrder,
		ItemTemplate: req.ItemTemplate,
		PageTemplate: req.PageTemplate,
		ItemsPerPage: req.ItemsPerPage,
	}

	if err := a.settingsService.CreateCollection(r.Context(), collection); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "collection.create", "collection", collection.ID.Hex(), map[string]interface{}{"name": collection.Name})
	a.jsonResponse(w, http.StatusCreated, collection)
}

func (a *APIHandler) APIUpdateCollection(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	collection, err := a.settingsService.GetCollection(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "collection not found")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Slug         *string `json:"slug"`
		Description  *string `json:"description"`
		Category     *string `json:"category"`
		SortField    *string `json:"sort_field"`
		SortOrder    *string `json:"sort_order"`
		ItemTemplate *string `json:"item_template"`
		PageTemplate *string `json:"page_template"`
		ItemsPerPage *int    `json:"items_per_page"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		collection.Name = *req.Name
	}
	if req.Slug != nil {
		collection.Slug = *req.Slug
	}
	if req.Description != nil {
		collection.Description = *req.Description
	}
	if req.Category != nil {
		collection.Category = *req.Category
	}
	if req.SortField != nil {
		collection.SortField = *req.SortField
	}
	if req.SortOrder != nil {
		collection.SortOrder = *req.SortOrder
	}
	if req.ItemTemplate != nil {
		collection.ItemTemplate = *req.ItemTemplate
	}
	if req.PageTemplate != nil {
		collection.PageTemplate = *req.PageTemplate
	}
	if req.ItemsPerPage != nil {
		collection.ItemsPerPage = *req.ItemsPerPage
	}

	if err := a.settingsService.UpdateCollection(r.Context(), collection); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "collection.update", "collection", collection.ID.Hex(), map[string]interface{}{"name": collection.Name})
	a.jsonResponse(w, http.StatusOK, collection)
}

func (a *APIHandler) APIDeleteCollection(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	if err := a.settingsService.DeleteCollection(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "collection.delete", "collection", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Regenerate all content

func (a *APIHandler) APIRegenerateAllContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}

	if err := a.contentService.RegenerateAllContent(r.Context()); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All published content has been regenerated",
	})
}

