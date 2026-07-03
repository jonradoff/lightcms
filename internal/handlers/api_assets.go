package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/auth"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ssrfBlockedCIDRs are IP ranges that must never be contacted via user-supplied URLs.
var ssrfBlockedCIDRs = func() []*net.IPNet {
	var blocks []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8",      // "this" network
		"10.0.0.0/8",     // RFC1918 private
		"100.64.0.0/10",  // CGNAT shared address space
		"127.0.0.0/8",    // IPv4 loopback
		"169.254.0.0/16", // link-local / AWS EC2 metadata
		"172.16.0.0/12",  // RFC1918 private
		"192.168.0.0/16", // RFC1918 private
		"198.18.0.0/15",  // benchmarking
		"240.0.0.0/4",    // reserved
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA (includes fd00::/8)
		"fe80::/10",      // IPv6 link-local
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}()

// isPrivateOrReservedIP returns true if ip falls in any SSRF-blocked range.
func isPrivateOrReservedIP(ip net.IP) bool {
	for _, block := range ssrfBlockedCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// ssrfSafeClient is an http.Client whose dialer rejects private/reserved IP ranges.
// It resolves the destination hostname at dial time and checks every returned IP,
// preventing SSRF and DNS-rebinding attacks.
var ssrfSafeClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address")
			}
			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("could not resolve host")
			}
			for _, rawIP := range ips {
				ip := net.ParseIP(rawIP)
				if ip == nil || isPrivateOrReservedIP(ip) {
					return nil, fmt.Errorf("URL resolves to a private or restricted address")
				}
			}
			// Connect only to the first resolved public IP
			dialer := &net.Dialer{Timeout: 10 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
		},
	},
}

// validAssetServePrefixes are the only path prefixes allowed for asset serve_path.
// This prevents callers from writing assets to arbitrary locations (e.g. /static/css/).
var validAssetServePrefixes = []string{"/assets/", "/images/", "/docs/", "/media/", "/files/"}

// isValidAssetServePath returns true when path begins with an allowed prefix.
func isValidAssetServePath(path string) bool {
	for _, prefix := range validAssetServePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// API Asset endpoints

func (a *APIHandler) APIListAssets(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAssetView) {
		return
	}
	folder := r.URL.Query().Get("folder")

	assets, err := a.assetService.ListAssets(r.Context(), folder)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Strip binary data from response
	type AssetSummary struct {
		ID          string `json:"id"`
		Filename    string `json:"filename"`
		Folder      string `json:"folder"`
		FullPath    string `json:"full_path"`
		ServePath   string `json:"serve_path"`
		MimeType    string `json:"mime_type"`
		Size        int64  `json:"size"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
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
	if !a.requirePermission(w, r, auth.PermAssetView) {
		return
	}
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
	if !a.requirePermission(w, r, auth.PermAssetView) {
		return
	}
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
		FilePath    string `json:"file_path"`
		Description string `json:"description"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Filename == "" || req.ServePath == "" {
		a.jsonError(w, http.StatusBadRequest, "filename and serve_path are required")
		return
	}
	if req.DataBase64 == "" && req.FilePath == "" {
		a.jsonError(w, http.StatusBadRequest, "either data_base64 or file_path is required")
		return
	}
	if !isValidAssetServePath(req.ServePath) {
		a.jsonError(w, http.StatusBadRequest, "serve_path must begin with /assets/, /images/, /docs/, /media/, or /files/")
		return
	}

	var data []byte
	if req.FilePath != "" {
		// Read file directly from local filesystem (avoids base64 size limits in MCP transport)
		var err error
		data, err = os.ReadFile(req.FilePath)
		if err != nil {
			a.jsonError(w, http.StatusBadRequest, fmt.Sprintf("failed to read file_path: %v", err))
			return
		}
	} else {
		var err error
		data, err = base64.StdEncoding.DecodeString(req.DataBase64)
		if err != nil {
			a.jsonError(w, http.StatusBadRequest, "invalid base64 data")
			return
		}
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
	if !a.requirePermission(w, r, auth.PermAssetView) {
		return
	}
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

	// Reject non-http(s) schemes before any DNS resolution
	lower := strings.ToLower(req.URL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		a.jsonError(w, http.StatusBadRequest, "url must use http or https scheme")
		return
	}

	// ssrfSafeClient resolves the host and blocks private/reserved IPs before connecting
	resp, err := ssrfSafeClient.Get(req.URL)
	if err != nil {
		// Return a generic error — never echo network internals back to the caller
		a.jsonError(w, http.StatusBadGateway, "failed to fetch remote URL")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.jsonError(w, http.StatusBadGateway, fmt.Sprintf("remote server returned %d", resp.StatusCode))
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MB cap
	if err != nil {
		a.jsonError(w, http.StatusBadGateway, "failed to read remote response")
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
	if !isValidAssetServePath(servePath) {
		a.jsonError(w, http.StatusBadRequest, "serve_path must begin with /assets/, /images/, /docs/, /media/, or /files/")
		return
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
