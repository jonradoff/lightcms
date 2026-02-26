package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

	a.jsonResponse(w, http.StatusCreated, content)
}

func (a *APIHandler) APIUpdateContent(w http.ResponseWriter, r *http.Request) {
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

	a.jsonResponse(w, http.StatusOK, content)
}

func (a *APIHandler) APIDeleteContent(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.DeleteContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIRestoreContent(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.RestoreContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIPublishContent(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.PublishContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIUnpublishContent(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}

	if err := a.contentService.UnpublishContent(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

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

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":            true,
		"total_replacements": totalReplacements,
		"pages_updated":      len(updatedPages),
		"updated_pages":      updatedPages,
	})
}
