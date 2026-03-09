package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"lightcms/internal/auth"
	"lightcms/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// API Content endpoints

func (a *APIHandler) APIListContent(w http.ResponseWriter, r *http.Request) {
	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))
	category := r.URL.Query().Get("category")

	var folderID *primitive.ObjectID
	if fid := r.URL.Query().Get("folder_id"); fid != "" {
		id, err := primitive.ObjectIDFromHex(fid)
		if err != nil {
			a.jsonError(w, http.StatusBadRequest, "invalid folder_id")
			return
		}
		folderID = &id
	}

	contents, err := a.contentService.ListContent(r.Context(), includeDeleted, category, folderID)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, contents)
}

func (a *APIHandler) APIGetContent(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	content, err := a.contentService.GetContent(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "content not found")
		return
	}

	if r.URL.Query().Get("include_rendered") == "true" {
		rendered, warnings, _ := a.renderContentWithWarnings(r, content)
		type ContentWithRendered struct {
			*models.Content
			RenderedHTML string   `json:"rendered_html"`
			Warnings     []string `json:"warnings,omitempty"`
		}
		a.jsonResponse(w, http.StatusOK, ContentWithRendered{Content: content, RenderedHTML: rendered, Warnings: warnings})
		return
	}

	a.jsonResponse(w, http.StatusOK, content)
}

func (a *APIHandler) APIGetContentByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		a.jsonError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	content, err := a.contentService.GetContentByPath(r.Context(), path)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "content not found")
		return
	}

	a.jsonResponse(w, http.StatusOK, content)
}

func (a *APIHandler) APICreateContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentCreate) {
		return
	}

	var req struct {
		TemplateID      string                 `json:"template_id"`
		Title           string                 `json:"title"`
		Slug            string                 `json:"slug"`
		FolderPath      string                 `json:"folder_path"`
		Category        string                 `json:"category"`
		MetaDescription string                 `json:"meta_description"`
		OGImage         string                 `json:"og_image"`
		Data            map[string]interface{} `json:"data"`
		Published       bool                   `json:"published"`
		UseHeader       bool                   `json:"use_header"`
		UseFooter       bool                   `json:"use_footer"`
		UseTheme        bool                   `json:"use_theme"`
		RawMode         bool                   `json:"raw_mode"`
		VersionComment  string                 `json:"version_comment"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TemplateID == "" || req.Title == "" || req.Slug == "" {
		a.jsonError(w, http.StatusBadRequest, "template_id, title, and slug are required")
		return
	}

	templateID, err := primitive.ObjectIDFromHex(req.TemplateID)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid template_id")
		return
	}

	// Get template name
	tmpl, err := a.templateService.GetTemplate(r.Context(), templateID)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "template not found")
		return
	}

	// Resolve folder
	var folderID *primitive.ObjectID
	folderPath := req.FolderPath
	if folderPath != "" {
		folders, _ := a.settingsService.ListFolders(r.Context())
		for _, f := range folders {
			if f.Path == folderPath {
				folderID = &f.ID
				break
			}
		}
	}

	content := &models.Content{
		TemplateID:      templateID,
		TemplateName:    tmpl.Name,
		Title:           req.Title,
		Slug:            req.Slug,
		FolderID:        folderID,
		FolderPath:      folderPath,
		Category:        req.Category,
		MetaDescription: req.MetaDescription,
		OGImage:         req.OGImage,
		Data:            req.Data,
		Published:       req.Published,
		UseHeader:       req.UseHeader,
		UseFooter:       req.UseFooter,
		UseTheme:        req.UseTheme,
		RawMode:         req.RawMode,
	}

	args := []string{}
	if req.VersionComment != "" {
		args = append(args, req.VersionComment)
	}
	if err := a.contentService.CreateContent(r.Context(), content, args...); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.create", "content", content.ID.Hex(), map[string]interface{}{"title": content.Title, "path": content.FullPath})
	a.jsonResponse(w, http.StatusCreated, content)
}

func (a *APIHandler) APIUpdateContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	// Get existing content
	content, err := a.contentService.GetContent(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "content not found")
		return
	}

	// Decode partial update from request body
	var raw map[string]json.RawMessage
	if err := a.decodeJSON(r, &raw); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply fields that are present in the request
	if v, ok := raw["title"]; ok {
		json.Unmarshal(v, &content.Title)
	}
	if v, ok := raw["slug"]; ok {
		json.Unmarshal(v, &content.Slug)
	}
	if v, ok := raw["template_id"]; ok {
		var tidStr string
		json.Unmarshal(v, &tidStr)
		if tid, err := primitive.ObjectIDFromHex(tidStr); err == nil {
			content.TemplateID = tid
			if tmpl, err := a.templateService.GetTemplate(r.Context(), tid); err == nil {
				content.TemplateName = tmpl.Name
			}
		}
	}
	if v, ok := raw["folder_path"]; ok {
		json.Unmarshal(v, &content.FolderPath)
		// Resolve folder ID
		content.FolderID = nil
		if content.FolderPath != "" {
			folders, _ := a.settingsService.ListFolders(r.Context())
			for _, f := range folders {
				if f.Path == content.FolderPath {
					content.FolderID = &f.ID
					break
				}
			}
		}
	}
	if v, ok := raw["category"]; ok {
		json.Unmarshal(v, &content.Category)
	}
	if v, ok := raw["meta_description"]; ok {
		json.Unmarshal(v, &content.MetaDescription)
	}
	if v, ok := raw["og_image"]; ok {
		json.Unmarshal(v, &content.OGImage)
	}
	if v, ok := raw["data"]; ok {
		json.Unmarshal(v, &content.Data)
	}
	if v, ok := raw["published"]; ok {
		json.Unmarshal(v, &content.Published)
	}
	if v, ok := raw["use_header"]; ok {
		json.Unmarshal(v, &content.UseHeader)
	}
	if v, ok := raw["use_footer"]; ok {
		json.Unmarshal(v, &content.UseFooter)
	}
	if v, ok := raw["use_theme"]; ok {
		json.Unmarshal(v, &content.UseTheme)
	}
	if v, ok := raw["raw_mode"]; ok {
		json.Unmarshal(v, &content.RawMode)
	}

	var versionComment string
	if v, ok := raw["version_comment"]; ok {
		json.Unmarshal(v, &versionComment)
	}

	args := []string{}
	if versionComment != "" {
		args = append(args, versionComment)
	}
	if err := a.contentService.UpdateContent(r.Context(), content, args...); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.update", "content", content.ID.Hex(), map[string]interface{}{"title": content.Title})
	a.jsonResponse(w, http.StatusOK, content)
}

func (a *APIHandler) APIDeleteContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentDelete) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.DeleteContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.delete", "content", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIRestoreContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.RestoreContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.restore", "content", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIPublishContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentPublish) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.PublishContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.publish", "content", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIUnpublishContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentPublish) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.UnpublishContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.unpublish", "content", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIListContentVersions(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	versions, err := a.contentService.GetVersions(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, versions)
}

func (a *APIHandler) APIGetContentVersion(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	version, err := strconv.Atoi(mux.Vars(r)["version"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	v, err := a.contentService.GetVersion(r.Context(), id, version)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "version not found")
		return
	}

	a.jsonResponse(w, http.StatusOK, v)
}

func (a *APIHandler) APIRevertContentVersion(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
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

	comment := req.VersionComment
	if comment == "" {
		comment = fmt.Sprintf("Reverted to version %d", version)
	}

	if err := a.contentService.RevertToVersion(r.Context(), id, version, comment); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.revert", "content", id.Hex(), map[string]interface{}{"version": version})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APISearchContent handles content search
func (a *APIHandler) APISearchContent(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		a.jsonError(w, http.StatusBadRequest, "q parameter is required")
		return
	}
	if len(query) > 200 {
		query = query[:200]
	}

	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "fulltext"
	}
	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))

	// Use service to list all content, then filter in-memory (matching MCP behavior)
	contents, err := a.contentService.ListContent(r.Context(), includeDeleted, "", nil)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type SearchMatch struct {
		ID           string   `json:"id"`
		Title        string   `json:"title"`
		FullPath     string   `json:"full_path"`
		TemplateName string   `json:"template_name"`
		Published    bool     `json:"published"`
		MatchedIn    []string `json:"matched_in"`
	}

	queryLower := strings.ToLower(query)
	var matches []SearchMatch

	for _, c := range contents {
		var matchedIn []string

		if strings.Contains(strings.ToLower(c.Title), queryLower) {
			matchedIn = append(matchedIn, "title")
		}

		if searchType == "fulltext" {
			for fieldName, v := range c.Data {
				if strVal, ok := v.(string); ok {
					if strings.Contains(strings.ToLower(strVal), queryLower) {
						matchedIn = append(matchedIn, fieldName)
					}
				}
			}
		}

		if len(matchedIn) > 0 {
			matches = append(matches, SearchMatch{
				ID:           c.ID.Hex(),
				Title:        c.Title,
				FullPath:     c.FullPath,
				TemplateName: c.TemplateName,
				Published:    c.Published,
				MatchedIn:    matchedIn,
			})
		}
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"query":       query,
		"search_type": searchType,
		"total":       len(matches),
		"matches":     matches,
	})
}

// APISearchReplacePreview previews search and replace
func (a *APIHandler) APISearchReplacePreview(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSearchReplace) {
		return
	}

	var req struct {
		Search  string `json:"search"`
		Replace string `json:"replace"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Search == "" {
		a.jsonError(w, http.StatusBadRequest, "search text is required")
		return
	}

	contents, err := a.contentService.ListContent(r.Context(), false, "", nil)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type MatchDetail struct {
		ID           string         `json:"id"`
		Title        string         `json:"title"`
		FullPath     string         `json:"full_path"`
		Published    bool           `json:"published"`
		MatchCount   int            `json:"match_count"`
		FieldMatches map[string]int `json:"field_matches"`
	}

	var matches []MatchDetail
	totalMatchCount := 0

	for _, content := range contents {
		matchCount := 0
		fieldMatches := make(map[string]int)

		for fieldName, value := range content.Data {
			if strVal, ok := value.(string); ok {
				count := strings.Count(strVal, req.Search)
				if count > 0 {
					matchCount += count
					fieldMatches[fieldName] = count
				}
			}
		}

		if strings.Contains(content.Title, req.Search) {
			titleCount := strings.Count(content.Title, req.Search)
			matchCount += titleCount
			fieldMatches["title"] = titleCount
		}

		if matchCount > 0 {
			matches = append(matches, MatchDetail{
				ID:           content.ID.Hex(),
				Title:        content.Title,
				FullPath:     content.FullPath,
				Published:    content.Published,
				MatchCount:   matchCount,
				FieldMatches: fieldMatches,
			})
			totalMatchCount += matchCount
		}
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"search":          req.Search,
		"replace":         req.Replace,
		"total_matches":   totalMatchCount,
		"affected_pages":  len(matches),
		"matches":         matches,
	})
}

// APISearchReplaceExecute executes search and replace
func (a *APIHandler) APISearchReplaceExecute(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSearchReplace) {
		return
	}

	var req struct {
		Search         string `json:"search"`
		Replace        string `json:"replace"`
		VersionComment string `json:"version_comment"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Search == "" {
		a.jsonError(w, http.StatusBadRequest, "search text is required")
		return
	}

	versionComment := req.VersionComment
	if versionComment == "" {
		versionComment = fmt.Sprintf("Bulk search and replace: '%s' → '%s'", req.Search, req.Replace)
	}

	contents, err := a.contentService.ListContent(r.Context(), false, "", nil)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type UpdatedPage struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		FullPath      string   `json:"full_path"`
		MatchCount    int      `json:"match_count"`
		FieldsUpdated []string `json:"fields_updated"`
	}

	var updatedPages []UpdatedPage
	totalReplacements := 0

	for _, content := range contents {
		needsUpdate := false
		matchCount := 0
		var fieldsUpdated []string

		newData := make(map[string]interface{})
		for k, v := range content.Data {
			newData[k] = v
		}

		for fieldName, value := range content.Data {
			if strVal, ok := value.(string); ok {
				if strings.Contains(strVal, req.Search) {
					count := strings.Count(strVal, req.Search)
					matchCount += count
					newData[fieldName] = strings.ReplaceAll(strVal, req.Search, req.Replace)
					needsUpdate = true
					fieldsUpdated = append(fieldsUpdated, fieldName)
				}
			}
		}

		newTitle := content.Title
		if strings.Contains(content.Title, req.Search) {
			count := strings.Count(content.Title, req.Search)
			matchCount += count
			newTitle = strings.ReplaceAll(content.Title, req.Search, req.Replace)
			needsUpdate = true
			fieldsUpdated = append(fieldsUpdated, "title")
		}

		if needsUpdate {
			content.Title = newTitle
			content.Data = newData
			if err := a.contentService.UpdateContent(r.Context(), &content, versionComment); err != nil {
				continue
			}

			updatedPages = append(updatedPages, UpdatedPage{
				ID:            content.ID.Hex(),
				Title:         newTitle,
				FullPath:      content.FullPath,
				MatchCount:    matchCount,
				FieldsUpdated: fieldsUpdated,
			})
			totalReplacements += matchCount
		}
	}

	a.auditLog(r, "content.search_replace", "content", "", map[string]interface{}{
		"search": req.Search, "replace": req.Replace,
		"pages_updated": len(updatedPages), "total_replacements": totalReplacements,
	})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":            true,
		"search":             req.Search,
		"replace":            req.Replace,
		"total_replacements": totalReplacements,
		"pages_updated":      len(updatedPages),
		"updated_pages":      updatedPages,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// renderContentWithWarnings renders content body HTML and returns template
// validation warnings (missing required fields, unresolved placeholders).
func (a *APIHandler) renderContentWithWarnings(r *http.Request, content *models.Content) (string, []string, error) {
	tmpl, err := a.templateService.GetTemplate(r.Context(), content.TemplateID)
	if err != nil {
		return "", nil, err
	}

	var warnings []string

	// Check required fields
	for _, field := range tmpl.Fields {
		if field.Required {
			v, ok := content.Data[field.Name]
			if !ok || fmt.Sprintf("%v", v) == "" {
				warnings = append(warnings, fmt.Sprintf("required field '%s' (%s) is empty", field.Name, field.Label))
			}
		}
	}

	// Render template body
	data := make(map[string]interface{})
	for k, v := range content.Data {
		if str, ok := v.(string); ok {
			data[k] = template.HTML(str)
		} else {
			data[k] = v
		}
	}
	data["title"] = content.Title
	data["slug"] = content.Slug

	t, err := template.New("content").Parse(tmpl.HTMLLayout)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("template parse error: %v", err))
		return "", warnings, nil
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		warnings = append(warnings, fmt.Sprintf("template execute error: %v", err))
	}

	// Check for unresolved Go template placeholders in output ({{.xxx}} not filled)
	unresolvedRe := regexp.MustCompile(`\{\{\.(\w+)\}\}`)
	for _, match := range unresolvedRe.FindAllStringSubmatch(tmpl.HTMLLayout, -1) {
		field := match[1]
		if field != "title" && field != "slug" {
			if _, exists := content.Data[field]; !exists {
				warnings = append(warnings, fmt.Sprintf("placeholder '{{.%s}}' has no matching field in content data", field))
			}
		}
	}

	// Warn about likely unclosed HTML tags
	openTagRe := regexp.MustCompile(`<(div|section|article|aside|main|header|footer|nav|ul|ol|table|tbody|thead|tr|form|select)\b`)
	closeTagRe := regexp.MustCompile(`</(div|section|article|aside|main|header|footer|nav|ul|ol|table|tbody|thead|tr|form|select)>`)
	body := buf.String()
	opens := len(openTagRe.FindAllString(body, -1))
	closes := len(closeTagRe.FindAllString(body, -1))
	if opens > closes {
		warnings = append(warnings, fmt.Sprintf("possible unclosed HTML tags: %d open vs %d close block elements", opens, closes))
	}

	return body, warnings, nil
}

// ─── Batch operations ─────────────────────────────────────────────────────────

// APIBatchPublishContent publishes multiple content items in one call.
// Body: {"ids": ["id1","id2",...]} or {"publish_all_drafts": true}
func (a *APIHandler) APIBatchPublishContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentPublish) {
		return
	}

	var req struct {
		IDs             []string `json:"ids"`
		PublishAllDrafts bool    `json:"publish_all_drafts"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var ids []primitive.ObjectID
	if req.PublishAllDrafts {
		contents, err := a.contentService.ListContent(r.Context(), false, "", nil)
		if err != nil {
			a.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, c := range contents {
			if !c.Published {
				ids = append(ids, c.ID)
			}
		}
	} else {
		for _, sid := range req.IDs {
			id, err := primitive.ObjectIDFromHex(sid)
			if err != nil {
				a.jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid id: %s", sid))
				return
			}
			ids = append(ids, id)
		}
	}

	var published []string
	var failed []map[string]string
	for _, id := range ids {
		if err := a.contentService.PublishContent(r.Context(), id); err != nil {
			failed = append(failed, map[string]string{"id": id.Hex(), "error": err.Error()})
		} else {
			published = append(published, id.Hex())
		}
	}

	a.auditLog(r, "content.batch_publish", "content", "", map[string]interface{}{"count": len(published)})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"published": published,
		"failed":    failed,
	})
}

// ─── Preview ──────────────────────────────────────────────────────────────────

// APIPreviewContent renders a content item's HTML without saving or publishing.
// Returns the rendered body HTML plus any validation warnings.
func (a *APIHandler) APIPreviewContent(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	content, err := a.contentService.GetContent(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "content not found")
		return
	}

	// Allow optional field overrides in the request body (simulate edits without saving)
	var overrides map[string]json.RawMessage
	a.decodeJSON(r, &overrides) // ignore error — overrides are optional
	if v, ok := overrides["title"]; ok {
		json.Unmarshal(v, &content.Title)
	}
	if v, ok := overrides["data"]; ok {
		json.Unmarshal(v, &content.Data)
	}

	rendered, warnings, err := a.renderContentWithWarnings(r, content)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"content_id":    id.Hex(),
		"rendered_html": rendered,
		"warnings":      warnings,
	})
}

// ─── By-path update ───────────────────────────────────────────────────────────

// APIUpdateContentByPath updates content identified by URL path rather than ID.
// PUT /api/v1/content/by-path?path=/some/path
func (a *APIHandler) APIUpdateContentByPath(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		a.jsonError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	content, err := a.contentService.GetContentByPath(r.Context(), path)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "content not found at path: "+path)
		return
	}

	var raw map[string]json.RawMessage
	if err := a.decodeJSON(r, &raw); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if v, ok := raw["title"]; ok {
		json.Unmarshal(v, &content.Title)
	}
	if v, ok := raw["data"]; ok {
		json.Unmarshal(v, &content.Data)
	}
	if v, ok := raw["category"]; ok {
		json.Unmarshal(v, &content.Category)
	}
	if v, ok := raw["meta_description"]; ok {
		json.Unmarshal(v, &content.MetaDescription)
	}
	if v, ok := raw["og_image"]; ok {
		json.Unmarshal(v, &content.OGImage)
	}
	if v, ok := raw["published"]; ok {
		json.Unmarshal(v, &content.Published)
	}
	if v, ok := raw["use_header"]; ok {
		json.Unmarshal(v, &content.UseHeader)
	}
	if v, ok := raw["use_footer"]; ok {
		json.Unmarshal(v, &content.UseFooter)
	}
	if v, ok := raw["use_theme"]; ok {
		json.Unmarshal(v, &content.UseTheme)
	}
	if v, ok := raw["raw_mode"]; ok {
		json.Unmarshal(v, &content.RawMode)
	}

	var versionComment string
	if v, ok := raw["version_comment"]; ok {
		json.Unmarshal(v, &versionComment)
	}

	args := []string{}
	if versionComment != "" {
		args = append(args, versionComment)
	}
	if err := a.contentService.UpdateContent(r.Context(), content, args...); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "content.update", "content", content.ID.Hex(), map[string]interface{}{"title": content.Title, "path": content.FullPath})
	a.jsonResponse(w, http.StatusOK, content)
}

// ─── Scoped search & replace ──────────────────────────────────────────────────

// scopeFilter defines optional filters for scoped search-and-replace.
type scopeFilter struct {
	ContentIDs   []string `json:"content_ids"`
	FolderPath   string   `json:"folder_path"`
	TemplateName string   `json:"template_name"`
	Category     string   `json:"category"`
}

func (f *scopeFilter) matches(c models.Content) bool {
	if len(f.ContentIDs) > 0 {
		for _, sid := range f.ContentIDs {
			if c.ID.Hex() == sid {
				return true
			}
		}
		return false
	}
	if f.FolderPath != "" && !strings.HasPrefix(c.FullPath, f.FolderPath) {
		return false
	}
	if f.TemplateName != "" && !strings.EqualFold(c.TemplateName, f.TemplateName) {
		return false
	}
	if f.Category != "" && !strings.EqualFold(c.Category, f.Category) {
		return false
	}
	return true
}

// APIScopedSearchReplacePreview previews a scoped search-and-replace.
func (a *APIHandler) APIScopedSearchReplacePreview(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSearchReplace) {
		return
	}

	var req struct {
		Search  string      `json:"search"`
		Replace string      `json:"replace"`
		Scope   scopeFilter `json:"scope"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Search == "" {
		a.jsonError(w, http.StatusBadRequest, "search text is required")
		return
	}

	contents, err := a.contentService.ListContent(r.Context(), false, "", nil)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type MatchDetail struct {
		ID           string         `json:"id"`
		Title        string         `json:"title"`
		FullPath     string         `json:"full_path"`
		Published    bool           `json:"published"`
		MatchCount   int            `json:"match_count"`
		FieldMatches map[string]int `json:"field_matches"`
	}

	var matches []MatchDetail
	totalMatchCount := 0

	for _, content := range contents {
		if !req.Scope.matches(content) {
			continue
		}
		matchCount := 0
		fieldMatches := make(map[string]int)
		for fieldName, value := range content.Data {
			if strVal, ok := value.(string); ok {
				count := strings.Count(strVal, req.Search)
				if count > 0 {
					matchCount += count
					fieldMatches[fieldName] = count
				}
			}
		}
		if strings.Contains(content.Title, req.Search) {
			n := strings.Count(content.Title, req.Search)
			matchCount += n
			fieldMatches["title"] = n
		}
		if matchCount > 0 {
			matches = append(matches, MatchDetail{
				ID: content.ID.Hex(), Title: content.Title,
				FullPath: content.FullPath, Published: content.Published,
				MatchCount: matchCount, FieldMatches: fieldMatches,
			})
			totalMatchCount += matchCount
		}
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"search":         req.Search,
		"replace":        req.Replace,
		"scope":          req.Scope,
		"total_matches":  totalMatchCount,
		"affected_pages": len(matches),
		"matches":        matches,
	})
}

// APIScopedSearchReplaceExecute executes a scoped search-and-replace.
func (a *APIHandler) APIScopedSearchReplaceExecute(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSearchReplace) {
		return
	}

	var req struct {
		Search         string      `json:"search"`
		Replace        string      `json:"replace"`
		VersionComment string      `json:"version_comment"`
		Scope          scopeFilter `json:"scope"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Search == "" {
		a.jsonError(w, http.StatusBadRequest, "search text is required")
		return
	}

	versionComment := req.VersionComment
	if versionComment == "" {
		versionComment = fmt.Sprintf("Scoped search and replace: '%s' → '%s'", req.Search, req.Replace)
	}

	contents, err := a.contentService.ListContent(r.Context(), false, "", nil)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type UpdatedPage struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		FullPath      string   `json:"full_path"`
		MatchCount    int      `json:"match_count"`
		FieldsUpdated []string `json:"fields_updated"`
	}

	var updatedPages []UpdatedPage
	totalReplacements := 0

	for _, content := range contents {
		if !req.Scope.matches(content) {
			continue
		}
		needsUpdate := false
		matchCount := 0
		var fieldsUpdated []string
		newData := make(map[string]interface{})
		for k, v := range content.Data {
			newData[k] = v
		}
		for fieldName, value := range content.Data {
			if strVal, ok := value.(string); ok && strings.Contains(strVal, req.Search) {
				count := strings.Count(strVal, req.Search)
				matchCount += count
				newData[fieldName] = strings.ReplaceAll(strVal, req.Search, req.Replace)
				needsUpdate = true
				fieldsUpdated = append(fieldsUpdated, fieldName)
			}
		}
		newTitle := content.Title
		if strings.Contains(content.Title, req.Search) {
			n := strings.Count(content.Title, req.Search)
			matchCount += n
			newTitle = strings.ReplaceAll(content.Title, req.Search, req.Replace)
			needsUpdate = true
			fieldsUpdated = append(fieldsUpdated, "title")
		}
		if needsUpdate {
			content.Title = newTitle
			content.Data = newData
			if err := a.contentService.UpdateContent(r.Context(), &content, versionComment); err != nil {
				continue
			}
			updatedPages = append(updatedPages, UpdatedPage{
				ID: content.ID.Hex(), Title: newTitle,
				FullPath: content.FullPath, MatchCount: matchCount,
				FieldsUpdated: fieldsUpdated,
			})
			totalReplacements += matchCount
		}
	}

	a.auditLog(r, "content.search_replace", "content", "", map[string]interface{}{
		"search": req.Search, "replace": req.Replace,
		"pages_updated": len(updatedPages), "total_replacements": totalReplacements,
	})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":            true,
		"search":             req.Search,
		"replace":            req.Replace,
		"total_replacements": totalReplacements,
		"pages_updated":      len(updatedPages),
		"updated_pages":      updatedPages,
	})
}
