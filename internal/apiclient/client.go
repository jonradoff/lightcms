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
	"strings"
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

func (c *Client) GetBacklinks(ctx context.Context, path string) ([]Content, error) {
	var result []Content
	if err := c.do(ctx, "GET", "/content/backlinks?path="+url.QueryEscape(path), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
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

func (c *Client) SearchReplacePreview(ctx context.Context, search, replace string, regex bool) (*SearchReplaceResult, error) {
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/preview", map[string]interface{}{
		"search":  search,
		"replace": replace,
		"regex":   regex,
	}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SearchReplaceExecute(ctx context.Context, search, replace, comment string, regex, autoRepublish bool) (*SearchReplaceResult, error) {
	req := map[string]interface{}{
		"search":         search,
		"replace":        replace,
		"regex":          regex,
		"auto_republish": autoRepublish,
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
func (c *Client) ScopedSearchReplacePreview(ctx context.Context, search, replace string, regex bool, scope ScopedSearchReplaceScope) (*SearchReplaceResult, error) {
	req := map[string]interface{}{
		"search":  search,
		"replace": replace,
		"regex":   regex,
		"scope":   scope,
	}
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/scoped/preview", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ScopedSearchReplaceExecute runs a scoped search-and-replace.
func (c *Client) ScopedSearchReplaceExecute(ctx context.Context, search, replace, comment string, regex, autoRepublish bool, scope ScopedSearchReplaceScope) (*SearchReplaceResult, error) {
	req := map[string]interface{}{
		"search":          search,
		"replace":         replace,
		"regex":           regex,
		"version_comment": comment,
		"auto_republish":  autoRepublish,
		"scope":           scope,
	}
	var result SearchReplaceResult
	if err := c.do(ctx, "POST", "/search-replace/scoped/execute", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListContentOptions provides optional parameters for listing content.
type ListContentOptions struct {
	IncludeDeleted bool
	Category       string
	FolderID       string
	IncludeData    bool
	IncludeFields  []string
}

// ListContentWithOptions lists content with extended filter options.
func (c *Client) ListContentWithOptions(ctx context.Context, opts ListContentOptions) ([]Content, error) {
	params := url.Values{}
	if opts.IncludeDeleted {
		params.Set("include_deleted", "true")
	}
	if opts.Category != "" {
		params.Set("category", opts.Category)
	}
	if opts.FolderID != "" {
		params.Set("folder_id", opts.FolderID)
	}
	if opts.IncludeData {
		params.Set("include_data", "true")
	}
	if len(opts.IncludeFields) > 0 {
		params.Set("include_fields", strings.Join(opts.IncludeFields, ","))
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

// BulkUpdateContent updates multiple content items in a single call.
func (c *Client) BulkUpdateContent(ctx context.Context, req interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.do(ctx, "POST", "/content/bulk-update", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BulkFieldOperation applies a field operation to all matching content.
func (c *Client) BulkFieldOperation(ctx context.Context, req interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.do(ctx, "POST", "/content/bulk-field-op", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ExportContent exports content items with their field data.
func (c *Client) ExportContent(ctx context.Context, req interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.do(ctx, "POST", "/content/export", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListSnippets returns all snippets.
func (c *Client) ListSnippets(ctx context.Context) ([]Snippet, error) {
	var snippets []Snippet
	if err := c.do(ctx, "GET", "/snippets", nil, &snippets); err != nil {
		return nil, err
	}
	return snippets, nil
}

// GetSnippet returns a snippet by ID.
func (c *Client) GetSnippet(ctx context.Context, id string) (*Snippet, error) {
	var snip Snippet
	if err := c.do(ctx, "GET", "/snippets/"+id, nil, &snip); err != nil {
		return nil, err
	}
	return &snip, nil
}

// CreateSnippet creates a new snippet.
func (c *Client) CreateSnippet(ctx context.Context, req CreateSnippetRequest) (*Snippet, error) {
	var snip Snippet
	if err := c.do(ctx, "POST", "/snippets", req, &snip); err != nil {
		return nil, err
	}
	return &snip, nil
}

// UpdateSnippet updates an existing snippet.
func (c *Client) UpdateSnippet(ctx context.Context, id string, req UpdateSnippetRequest) (*Snippet, error) {
	var snip Snippet
	if err := c.do(ctx, "PUT", "/snippets/"+id, req, &snip); err != nil {
		return nil, err
	}
	return &snip, nil
}

// DeleteSnippet deletes a snippet by ID.
func (c *Client) DeleteSnippet(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/snippets/"+id, nil, nil)
}

// Fork operations

// ListForks returns all fork workspaces.
func (c *Client) ListForks(ctx context.Context) ([]Fork, error) {
	var forks []Fork
	if err := c.do(ctx, "GET", "/forks", nil, &forks); err != nil {
		return nil, err
	}
	return forks, nil
}

// CreateFork creates a new fork workspace.
func (c *Client) CreateFork(ctx context.Context, name, description string) (map[string]interface{}, error) {
	body := map[string]string{"name": name, "description": description}
	var result map[string]interface{}
	if err := c.do(ctx, "POST", "/forks", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetFork returns a fork with its page list.
func (c *Client) GetFork(ctx context.Context, id string) (*ForkDetail, error) {
	var detail ForkDetail
	if err := c.do(ctx, "GET", "/forks/"+id, nil, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// ForkPage copies a live page into a fork by content ID.
func (c *Client) ForkPage(ctx context.Context, forkID, contentID, path string) (*ForkPageResult, error) {
	body := map[string]string{}
	if contentID != "" {
		body["content_id"] = contentID
	} else {
		body["path"] = path
	}
	var result ForkPageResult
	if err := c.do(ctx, "POST", "/forks/"+forkID+"/fork-page", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListForkPages returns all pages in a fork.
func (c *Client) ListForkPages(ctx context.Context, forkID string) ([]ForkPageRef, error) {
	var pages []ForkPageRef
	if err := c.do(ctx, "GET", "/forks/"+forkID+"/pages", nil, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

// RemoveForkPage removes a page from a fork.
func (c *Client) RemoveForkPage(ctx context.Context, forkID, pageID string) error {
	return c.do(ctx, "DELETE", "/forks/"+forkID+"/pages/"+pageID, nil, nil)
}

// MergeFork merges all fork pages into live content.
func (c *Client) MergeFork(ctx context.Context, forkID string) (*ForkMergeResult, error) {
	var result ForkMergeResult
	if err := c.do(ctx, "POST", "/forks/"+forkID+"/merge", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ArchiveFork archives a fork without merging.
func (c *Client) ArchiveFork(ctx context.Context, forkID string) error {
	return c.do(ctx, "POST", "/forks/"+forkID+"/archive", nil, nil)
}

// DeleteFork permanently deletes a fork and all its pages.
func (c *Client) DeleteFork(ctx context.Context, forkID string) error {
	return c.do(ctx, "DELETE", "/forks/"+forkID, nil, nil)
}
