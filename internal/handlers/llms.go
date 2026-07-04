package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jonradoff/lightcms/v7/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// resolveBaseURL returns the configured base URL, falling back to the
// request host when unset (mirrors ServeRobotsTxt behavior).
func (h *Handler) resolveBaseURL(r *http.Request) string {
	if h.baseURL != "" {
		return strings.TrimRight(h.baseURL, "/")
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

// listLLMContent returns all live published pages sorted by path.
// includeText controls whether plain_text is fetched (llms-full.txt only).
// The projection matters: full documents carry embeddings (~4KB/page) and
// data maps, which made these endpoints take tens of seconds on large sites.
func (h *Handler) listLLMContent(ctx context.Context, includeText bool) ([]models.Content, error) {
	filter := bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
		"fork_id":   bson.M{"$exists": false},
	}
	projection := bson.M{
		"title": 1, "slug": 1, "full_path": 1, "meta_description": 1, "published_at": 1,
	}
	if includeText {
		projection["plain_text"] = 1
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "full_path", Value: 1}}).
		SetProjection(projection)
	cursor, err := h.db.FindMany(ctx, "content", filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return nil, err
	}
	sort.Slice(contents, func(i, j int) bool { return contents[i].FullPath < contents[j].FullPath })
	return contents, nil
}

// ServeLlmsTxt serves /llms.txt — a Markdown index of the site for AI
// crawlers and agents, per the llms.txt proposal (llmstxt.org).
func (h *Handler) ServeLlmsTxt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	baseURL := h.resolveBaseURL(r)

	theme, err := h.db.GetThemeSettings(ctx)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	contents, err := h.listLLMContent(ctx, false)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	sb.WriteString("# " + theme.SiteName + "\n")
	if theme.SiteTagline != "" {
		sb.WriteString("\n> " + theme.SiteTagline + "\n")
	}
	sb.WriteString("\n## Pages\n\n")
	for _, c := range contents {
		path := c.FullPath
		if path == "" {
			path = "/" + c.Slug
		}
		line := fmt.Sprintf("- [%s](%s%s)", c.Title, baseURL, path)
		if c.MetaDescription != "" {
			line += ": " + c.MetaDescription
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n## Optional\n\n")
	sb.WriteString(fmt.Sprintf("- [Full content](%s/llms-full.txt): complete text of every page\n", baseURL))
	sb.WriteString(fmt.Sprintf("- [Sitemap](%s/sitemap.xml)\n", baseURL))
	sb.WriteString(fmt.Sprintf("- MCP endpoint (read-only, no auth): %s/mcp-public — tools: search_site, get_page, list_pages, get_site_info\n", baseURL))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300, s-maxage=3600")
	w.Write([]byte(sb.String()))
}

// ServeLlmsFullTxt serves /llms-full.txt — the complete plain-text content
// of every published page, for LLM/RAG consumption.
func (h *Handler) ServeLlmsFullTxt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	baseURL := h.resolveBaseURL(r)

	theme, err := h.db.GetThemeSettings(ctx)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	contents, err := h.listLLMContent(ctx, true)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	sb.WriteString("# " + theme.SiteName + "\n")
	if theme.SiteTagline != "" {
		sb.WriteString("\n> " + theme.SiteTagline + "\n")
	}
	for _, c := range contents {
		path := c.FullPath
		if path == "" {
			path = "/" + c.Slug
		}
		sb.WriteString("\n---\n\n## " + c.Title + "\n\n")
		sb.WriteString("URL: " + baseURL + path + "\n")
		if c.PublishedAt != nil && !c.PublishedAt.IsZero() {
			sb.WriteString("Published: " + c.PublishedAt.UTC().Format("2006-01-02") + "\n")
		}
		if c.MetaDescription != "" {
			sb.WriteString("Description: " + c.MetaDescription + "\n")
		}
		text := strings.TrimSpace(c.PlainText)
		if text != "" {
			sb.WriteString("\n" + text + "\n")
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300, s-maxage=3600")
	w.Write([]byte(sb.String()))
}

// buildJSONLD returns a schema.org JSON-LD document for a content page, or
// "" if it cannot be built. The schema type is inferred from the template:
// blog posts become BlogPosting, press releases NewsArticle, all else WebPage.
func buildJSONLD(content *models.Content, tmpl *models.Template, siteName, baseURL, ogImage string) string {
	if content == nil {
		return ""
	}
	path := content.FullPath
	if path == "" {
		path = "/" + content.Slug
	}

	schemaType := "WebPage"
	if tmpl != nil {
		name := strings.ToLower(tmpl.Name + " " + tmpl.Category)
		switch {
		case strings.Contains(name, "blog"):
			schemaType = "BlogPosting"
		case strings.Contains(name, "press"):
			schemaType = "NewsArticle"
		}
	}

	doc := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    schemaType,
		"headline": content.Title,
		"url":      strings.TrimRight(baseURL, "/") + path,
	}
	if content.MetaDescription != "" {
		doc["description"] = content.MetaDescription
	}
	if ogImage != "" {
		doc["image"] = ogImage
	}
	if content.PublishedAt != nil && !content.PublishedAt.IsZero() {
		doc["datePublished"] = content.PublishedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if !content.UpdatedAt.IsZero() {
		doc["dateModified"] = content.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if siteName != "" {
		doc["publisher"] = map[string]interface{}{
			"@type": "Organization",
			"name":  siteName,
		}
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	// </script> inside JSON strings would terminate the script element early.
	safe := strings.ReplaceAll(string(b), "</", `<\/`)
	return `<script type="application/ld+json">` + safe + `</script>`
}

// buildWebsiteJSONLD returns the schema.org WebSite document for the
// homepage, including a SearchAction advertising the public search endpoint
// (used by search engines for sitelinks search and by visiting agents).
func buildWebsiteJSONLD(siteName, tagline, baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	doc := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     siteName,
		"url":      base,
		"potentialAction": map[string]interface{}{
			"@type":       "SearchAction",
			"target":      base + "/api/search?q={search_term_string}",
			"query-input": "required name=search_term_string",
		},
	}
	if tagline != "" {
		doc["description"] = tagline
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	safe := strings.ReplaceAll(string(b), "</", `<\/`)
	return `<script type="application/ld+json">` + safe + `</script>`
}
