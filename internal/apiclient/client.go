package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is an HTTP client for the LightCMS REST API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new API client
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// do executes an HTTP request with auth and JSON handling
func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// Content operations

func (c *Client) ListContent(ctx context.Context, includeDeleted bool, category, folderID string) ([]Content, error) {
	params := url.Values{}
	if includeDeleted {
		params.Set("include_deleted", "true")
	}
	if category != "" {
		params.Set("category", category)
	}
	if folderID != "" {
		params.Set("folder_id", folderID)
	}

	path := "/content"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result []Content
	if err := c.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetContent(ctx context.Context, id string, includeRendered bool) (*Content, error) {
	var result Content
	apiPath := "/content/" + id
	if includeRendered {
		apiPath += "?include_rendered=true"
	}
	if err := c.do(ctx, "GET", apiPath, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetContentByPath(ctx context.Context, path string, includeRendered bool) (*Content, error) {
	var result Content
	apiPath := "/content/by-path?path=" + url.QueryEscape(path)
	if includeRendered {
		apiPath += "&include_rendered=true"
	}
	if err := c.do(ctx, "GET", apiPath, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateContent(ctx context.Context, req CreateContentRequest) (*Content, error) {
	var result Content
	if err := c.do(ctx, "POST", "/content", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateContent(ctx context.Context, id string, req UpdateContentRequest) (*Content, error) {
	var result Content
	if err := c.do(ctx, "PUT", "/content/"+id, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteContent(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/content/"+id, nil, nil)
}

func (c *Client) RestoreContent(ctx context.Context, id string) error {
	return c.do(ctx, "POST", "/content/"+id+"/restore", nil, nil)
}

func (c *Client) PublishContent(ctx context.Context, id string) error {
	return c.do(ctx, "POST", "/content/"+id+"/publish", nil, nil)
}

func (c *Client) UnpublishContent(ctx context.Context, id string) error {
	return c.do(ctx, "POST", "/content/"+id+"/unpublish", nil, nil)
}

func (c *Client) GetContentVersions(ctx context.Context, contentID string) ([]ContentVersion, error) {
	var result []ContentVersion
	if err := c.do(ctx, "GET", "/content/"+contentID+"/versions", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetContentVersion(ctx context.Context, contentID string, version int) (*ContentVersion, error) {
	var result ContentVersion
	if err := c.do(ctx, "GET", "/content/"+contentID+"/versions/"+strconv.Itoa(version), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) RevertContentVersion(ctx context.Context, contentID string, version int, comment string) error {
	req := map[string]string{}
	if comment != "" {
		req["version_comment"] = comment
	}
	return c.do(ctx, "POST", "/content/"+contentID+"/versions/"+strconv.Itoa(version)+"/revert", req, nil)
}

// Template operations

func (c *Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var result []Template
	if err := c.do(ctx, "GET", "/templates", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetTemplate(ctx context.Context, id string) (*Template, error) {
	var result Template
	if err := c.do(ctx, "GET", "/templates/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateTemplate(ctx context.Context, req CreateTemplateRequest) (*Template, error) {
	var result Template
	if err := c.do(ctx, "POST", "/templates", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateTemplate(ctx context.Context, id string, req UpdateTemplateRequest) (*Template, error) {
	var result Template
	if err := c.do(ctx, "PUT", "/templates/"+id, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteTemplate(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/templates/"+id, nil, nil)
}

// Asset operations

func (c *Client) ListAssets(ctx context.Context, folder string) ([]AssetSummary, error) {
	path := "/assets"
	if folder != "" {
		path += "?folder=" + url.QueryEscape(folder)
	}
	var result []AssetSummary
	if err := c.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetAsset(ctx context.Context, id string) (*AssetSummary, error) {
	var result AssetSummary
	if err := c.do(ctx, "GET", "/assets/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAssetByPath(ctx context.Context, path string) (*AssetSummary, error) {
	var result AssetSummary
	if err := c.do(ctx, "GET", "/assets/by-path?path="+url.QueryEscape(path), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UploadAsset(ctx context.Context, req UploadAssetRequest) (*AssetSummary, error) {
	var result AssetSummary
	if err := c.do(ctx, "POST", "/assets", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAsset(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/assets/"+id, nil, nil)
}

func (c *Client) ListAssetFolders(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.do(ctx, "GET", "/assets/folders", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Theme operations

func (c *Client) GetTheme(ctx context.Context) (*ThemeSettings, error) {
	var result ThemeSettings
	if err := c.do(ctx, "GET", "/theme", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateTheme(ctx context.Context, req map[string]interface{}) (*ThemeSettings, error) {
	var result ThemeSettings
	if err := c.do(ctx, "PUT", "/theme", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListThemeVersions(ctx context.Context) ([]ThemeVersion, error) {
	var result []ThemeVersion
	if err := c.do(ctx, "GET", "/theme/versions", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetThemeVersion(ctx context.Context, version int) (*ThemeVersion, error) {
	var result ThemeVersion
	if err := c.do(ctx, "GET", "/theme/versions/"+strconv.Itoa(version), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) RevertThemeVersion(ctx context.Context, version int, comment string) error {
	req := map[string]string{}
	if comment != "" {
		req["version_comment"] = comment
	}
	return c.do(ctx, "POST", "/theme/versions/"+strconv.Itoa(version)+"/revert", req, nil)
}

// Site Config

func (c *Client) GetSiteConfig(ctx context.Context) (*SiteConfig, error) {
	var result SiteConfig
	if err := c.do(ctx, "GET", "/config", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateSiteConfig(ctx context.Context, req map[string]interface{}) (*SiteConfig, error) {
	var result SiteConfig
	if err := c.do(ctx, "PUT", "/config", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Redirects

func (c *Client) ListRedirects(ctx context.Context) ([]Redirect, error) {
	var result []Redirect
	if err := c.do(ctx, "GET", "/redirects", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetRedirect(ctx context.Context, id string) (*Redirect, error) {
	var result Redirect
	if err := c.do(ctx, "GET", "/redirects/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateRedirect(ctx context.Context, req CreateRedirectRequest) (*Redirect, error) {
	var result Redirect
	if err := c.do(ctx, "POST", "/redirects", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateRedirect(ctx context.Context, id string, req map[string]interface{}) (*Redirect, error) {
	var result Redirect
	if err := c.do(ctx, "PUT", "/redirects/"+id, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteRedirect(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/redirects/"+id, nil, nil)
}

// Folders

func (c *Client) ListFolders(ctx context.Context) ([]Folder, error) {
	var result []Folder
	if err := c.do(ctx, "GET", "/folders", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetFolder(ctx context.Context, id string) (*Folder, error) {
	var result Folder
	if err := c.do(ctx, "GET", "/folders/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateFolder(ctx context.Context, req CreateFolderRequest) (*Folder, error) {
	var result Folder
	if err := c.do(ctx, "POST", "/folders", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteFolder(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/folders/"+id, nil, nil)
}

// Collections

func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	var result []Collection
	if err := c.do(ctx, "GET", "/collections", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetCollection(ctx context.Context, id string) (*Collection, error) {
	var result Collection
	if err := c.do(ctx, "GET", "/collections/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateCollection(ctx context.Context, req map[string]interface{}) (*Collection, error) {
	var result Collection
	if err := c.do(ctx, "POST", "/collections", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateCollection(ctx context.Context, id string, req map[string]interface{}) (*Collection, error) {
	var result Collection
	if err := c.do(ctx, "PUT", "/collections/"+id, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteCollection(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/collections/"+id, nil, nil)
}

// Search

func (c *Client) SearchContent(ctx context.Context, query, searchType string, includeDeleted bool) (*SearchResult, error) {
	params := url.Values{"q": {query}}
	if searchType != "" {
		params.Set("type", searchType)
	}
	if includeDeleted {
		params.Set("include_deleted", "true")
	}

	var result SearchResult
	if err := c.do(ctx, "GET", "/search?"+params.Encode(), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SearchReplacePreview(ctx context.Context, search, replace string) (*SearchReplaceResult, error) {
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/preview", map[string]string{
		"search":  search,
		"replace": replace,
	}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SearchReplaceExecute(ctx context.Context, search, replace, comment string) (*SearchReplaceResult, error) {
	req := map[string]string{
		"search":  search,
		"replace": replace,
	}
	if comment != "" {
		req["version_comment"] = comment
	}
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/execute", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// API Keys

func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	var result []APIKey
	if err := c.do(ctx, "GET", "/api-keys", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CreateAPIKey(ctx context.Context, name, description string) (*APIKeyCreated, error) {
	var result APIKeyCreated
	if err := c.do(ctx, "POST", "/api-keys", map[string]string{
		"name":        name,
		"description": description,
	}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api-keys/"+id, nil, nil)
}

// Regenerate

func (c *Client) RegenerateAllContent(ctx context.Context) error {
	return c.do(ctx, "POST", "/regenerate", nil, nil)
}

// End-user search

type EndUserSearchResult struct {
	Query   string        `json:"query"`
	Mode    string        `json:"mode"`
	Results []interface{} `json:"results"`
	Total   int           `json:"total"`
}

func (c *Client) EndUserSearch(ctx context.Context, query, mode string, limit int) (*EndUserSearchResult, error) {
	path := "/end-user-search?q=" + url.QueryEscape(query)
	if mode != "" {
		path += "&mode=" + url.QueryEscape(mode)
	}
	if limit > 0 {
		path += "&limit=" + strconv.Itoa(limit)
	}
	var result EndUserSearchResult
	if err := c.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReindexEmbeddings(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.do(ctx, "POST", "/reindex-embeddings", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BatchPublishContent publishes multiple content items at once.
// Pass a list of IDs, or set PublishAllDrafts = true to publish every draft.
func (c *Client) BatchPublishContent(ctx context.Context, ids []string, publishAllDrafts bool) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"ids":                ids,
		"publish_all_drafts": publishAllDrafts,
	}
	var result map[string]interface{}
	if err := c.do(ctx, "POST", "/content/batch-publish", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// PreviewContent returns the rendered HTML for a content item without publishing.
// Pass optional overrides (title, data map) to preview unsaved edits.
type PreviewContentResult struct {
	ContentID    string      `json:"content_id"`
	RenderedHTML string      `json:"rendered_html"`
	Warnings     []string    `json:"warnings"`
}

func (c *Client) PreviewContent(ctx context.Context, id string, overrides map[string]interface{}) (*PreviewContentResult, error) {
	var result PreviewContentResult
	if err := c.do(ctx, "POST", "/content/"+id+"/preview", overrides, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateContentByPath updates content identified by URL path.
func (c *Client) UpdateContentByPath(ctx context.Context, path string, req map[string]interface{}) (*Content, error) {
	var result Content
	if err := c.do(ctx, "PUT", "/content/by-path?path="+url.QueryEscape(path), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadAssetFromURL fetches a remote URL and stores it as a LightCMS asset.
func (c *Client) UploadAssetFromURL(ctx context.Context, assetURL, servePath, description string) (*AssetSummary, error) {
	req := map[string]interface{}{
		"url":         assetURL,
		"serve_path":  servePath,
		"description": description,
	}
	var result AssetSummary
	if err := c.do(ctx, "POST", "/assets/from-url", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PinThemeVersion locks a theme version so it is protected from auto-pruning.
func (c *Client) PinThemeVersion(ctx context.Context, version int) error {
	return c.do(ctx, "POST", fmt.Sprintf("/theme/versions/%d/pin", version), nil, nil)
}

// UnpinThemeVersion unlocks a previously pinned theme version.
func (c *Client) UnpinThemeVersion(ctx context.Context, version int) error {
	return c.do(ctx, "POST", fmt.Sprintf("/theme/versions/%d/unpin", version), nil, nil)
}

// ScopedSearchReplaceScope defines the optional scope filter for scoped S&R.
type ScopedSearchReplaceScope struct {
	ContentIDs   []string `json:"content_ids,omitempty"`
	FolderPath   string   `json:"folder_path,omitempty"`
	TemplateName string   `json:"template_name,omitempty"`
	Category     string   `json:"category,omitempty"`
}

// ScopedSearchReplacePreview previews a search-and-replace limited to a scope.
func (c *Client) ScopedSearchReplacePreview(ctx context.Context, search, replace string, scope ScopedSearchReplaceScope) (*SearchReplaceResult, error) {
	req := map[string]interface{}{
		"search":  search,
		"replace": replace,
		"scope":   scope,
	}
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/scoped/preview", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ScopedSearchReplaceExecute runs a scoped search-and-replace.
func (c *Client) ScopedSearchReplaceExecute(ctx context.Context, search, replace, comment string, scope ScopedSearchReplaceScope) (*SearchReplaceResult, error) {
	req := map[string]interface{}{
		"search":          search,
		"replace":         replace,
		"version_comment": comment,
		"scope":           scope,
	}
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/scoped/execute", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
