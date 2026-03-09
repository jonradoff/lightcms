package handlers

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"lightcms/internal/auth"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// API Asset endpoints

func (a *APIHandler) APIListAssets(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")

	assets, err := a.assetService.ListAssets(r.Context(), folder)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Strip binary data from response
	type AssetSummary struct {
		ID          string  `json:"id"`
		Filename    string  `json:"filename"`
		Folder      string  `json:"folder"`
		FullPath    string  `json:"full_path"`
		ServePath   string  `json:"serve_path"`
		MimeType    string  `json:"mime_type"`
		Size        int64   `json:"size"`
		Description string  `json:"description"`
		CreatedAt   string  `json:"created_at"`
	}

	result := make([]AssetSummary, 0, len(assets))
	for _, asset := range assets {
		result = append(result, AssetSummary{
			ID:          asset.ID.Hex(),
			Filename:    asset.Filename,
			Folder:      asset.Folder,
			FullPath:    asset.FullPath,
			ServePath:   asset.ServePath,
			MimeType:    asset.MimeType,
			Size:        asset.Size,
			Description: asset.Description,
			CreatedAt:   asset.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	a.jsonResponse(w, http.StatusOK, result)
}

func (a *APIHandler) APIGetAsset(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid asset ID")
		return
	}

	asset, err := a.assetService.GetAsset(r.Context(), id)
	if err != nil || asset == nil {
		a.jsonError(w, http.StatusNotFound, "asset not found")
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":          asset.ID.Hex(),
		"filename":    asset.Filename,
		"folder":      asset.Folder,
		"full_path":   asset.FullPath,
		"serve_path":  asset.ServePath,
		"mime_type":   asset.MimeType,
		"size":        asset.Size,
		"description": asset.Description,
		"created_at":  asset.CreatedAt,
	})
}

func (a *APIHandler) APIGetAssetByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		a.jsonError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	asset, err := a.assetService.GetAssetByPath(r.Context(), path)
	if err != nil || asset == nil {
		a.jsonError(w, http.StatusNotFound, "asset not found")
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":          asset.ID.Hex(),
		"filename":    asset.Filename,
		"folder":      asset.Folder,
		"full_path":   asset.FullPath,
		"serve_path":  asset.ServePath,
		"mime_type":   asset.MimeType,
		"size":        asset.Size,
		"description": asset.Description,
		"created_at":  asset.CreatedAt,
	})
}

func (a *APIHandler) APIUploadAsset(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAssetUpload) {
		return
	}

	var req struct {
		Filename    string `json:"filename"`
		ServePath   string `json:"serve_path"`
		DataBase64  string `json:"data_base64"`
		Description string `json:"description"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Filename == "" || req.ServePath == "" || req.DataBase64 == "" {
		a.jsonError(w, http.StatusBadRequest, "filename, serve_path, and data_base64 are required")
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.DataBase64)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid base64 data")
		return
	}

	asset, err := a.assetService.UploadAsset(r.Context(), data, req.Filename, req.ServePath, req.Description)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.auditLog(r, "asset.upload", "asset", asset.ID.Hex(), map[string]interface{}{"filename": asset.Filename, "serve_path": asset.ServePath})
	a.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":         asset.ID.Hex(),
		"filename":   asset.Filename,
		"serve_path": asset.ServePath,
		"mime_type":  asset.MimeType,
		"size":       asset.Size,
	})
}

func (a *APIHandler) APIDeleteAsset(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAssetDelete) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid asset ID")
		return
	}

	if err := a.assetService.DeleteAsset(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "asset.delete", "asset", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (a *APIHandler) APIListAssetFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := a.assetService.ListFolders(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, folders)
}

// APIUploadAssetFromURL fetches a remote URL and stores it as an asset.
// Body: {"url": "https://...", "serve_path": "/assets/foo.png", "description": "..."}
func (a *APIHandler) APIUploadAssetFromURL(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAssetUpload) {
		return
	}

	var req struct {
		URL         string `json:"url"`
		ServePath   string `json:"serve_path"`
		Description string `json:"description"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		a.jsonError(w, http.StatusBadRequest, "url is required")
		return
	}

	// Reject non-http(s) schemes
	lower := strings.ToLower(req.URL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		a.jsonError(w, http.StatusBadRequest, "url must use http or https scheme")
		return
	}

	resp, err := http.Get(req.URL) //nolint:gosec — URL is user-supplied but scheme-validated above
	if err != nil {
		a.jsonError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch URL: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.jsonError(w, http.StatusBadGateway, fmt.Sprintf("remote server returned %d", resp.StatusCode))
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MB cap
	if err != nil {
		a.jsonError(w, http.StatusBadGateway, fmt.Sprintf("failed to read response: %v", err))
		return
	}

	// Derive filename from URL path if serve_path not provided
	servePath := req.ServePath
	if servePath == "" {
		urlPath := req.URL
		if idx := strings.Index(urlPath, "?"); idx != -1 {
			urlPath = urlPath[:idx]
		}
		filename := filepath.Base(urlPath)
		if filename == "" || filename == "." || filename == "/" {
			filename = "asset"
		}
		servePath = "/assets/" + filename
	}
	filename := filepath.Base(servePath)

	asset, err := a.assetService.UploadAsset(r.Context(), data, filename, servePath, req.Description)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.auditLog(r, "asset.upload", "asset", asset.ID.Hex(), map[string]interface{}{
		"filename": asset.Filename, "serve_path": asset.ServePath, "source_url": req.URL,
	})
	a.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":         asset.ID.Hex(),
		"filename":   asset.Filename,
		"serve_path": asset.ServePath,
		"mime_type":  asset.MimeType,
		"size":       asset.Size,
	})
}
