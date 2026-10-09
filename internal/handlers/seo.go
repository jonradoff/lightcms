package handlers

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"html/template"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SetSEOService wires the SEO & AI settings service into the handler.
func (h *Handler) SetSEOService(s *services.SEOService) { h.seoService = s }

// seoConfig returns the SEO settings with defaults (safe when unwired).
func (h *Handler) seoConfig(ctx context.Context) services.SEOConfig {
	return h.seoService.Get(ctx)
}

// recordCrawler counts a known crawler's hit on path (no-op for other agents).
func (h *Handler) recordCrawler(r *http.Request, path string) {
	if h.analyticsService != nil {
		h.analyticsService.RecordCrawlerHit(path, r.UserAgent())
	}
}

// ==================== robots.txt ====================

// ServeRobotsTxt serves robots.txt generated from the AI crawler policy.
func (h *Handler) ServeRobotsTxt(w http.ResponseWriter, r *http.Request) {
	h.recordCrawler(r, "/robots.txt")
	cfg := h.seoConfig(r.Context())
	base := h.resolveBaseURL(r)
	var feeds []string
	if !cfg.FeedDisabled {
		feeds = []string{base + "/feed.xml", base + "/atom.xml"}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(cfg.RobotsTxt(base, feeds)))
}

// ==================== Markdown copies ====================

// markdownPath is the URL of a page's Markdown copy.
func markdownPath(fullPath string) string {
	if fullPath == "" || fullPath == "/" {
		return "/index.md"
	}
	return fullPath + ".md"
}

// findLivePage loads a published, live (non-fork, non-deleted) page by path,
// falling back to a case-insensitive match.
func (h *Handler) findLivePage(ctx context.Context, fullPath string) (*models.Content, error) {
	var c models.Content
	base := bson.M{"published": true, "deleted": bson.M{"$ne": true}, "fork_id": nil}
	f := bson.M{"full_path": fullPath}
	for k, v := range base {
		f[k] = v
	}
	if err := h.db.FindOne(ctx, "content", f, &c); err == nil {
		return &c, nil
	}
	f["full_path"] = bson.M{"$regex": "^" + regexp.QuoteMeta(fullPath) + "$", "$options": "i"}
	if err := h.db.FindOne(ctx, "content", f, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// serveMarkdownAlternate serves /<path>.md (and /index.md for the homepage)
// as a Markdown copy of the page. Called only after normal page lookup has
// failed, so a real page whose path ends in .md always wins.
func (h *Handler) serveMarkdownAlternate(w http.ResponseWriter, r *http.Request, fullPath string) bool {
	if !strings.HasSuffix(strings.ToLower(fullPath), ".md") {
		return false
	}
	ctx := r.Context()
	if h.seoConfig(ctx).MarkdownDisabled {
		return false
	}
	pagePath := fullPath[:len(fullPath)-3]
	if strings.EqualFold(pagePath, "/index") {
		pagePath = "/"
	}
	content, err := h.findLivePage(ctx, pagePath)
	if err != nil || content.NoIndex {
		return false
	}
	md, err := h.pageMarkdown(ctx, r, content)
	if err != nil {
		return false
	}
	h.recordCrawler(r, fullPath)
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300, s-maxage=3600")
	// The Markdown copy is for agents; keep it out of search indexes and
	// point at the HTML page as the canonical version.
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Header().Set("Link", `<`+h.resolveBaseURL(r)+content.FullPath+`>; rel="canonical"`)
	w.Write([]byte(md))
	return true
}

// pageBodyHTML returns the page's rendered body (template output, without
// the theme's header/footer).
func (h *Handler) pageBodyHTML(ctx context.Context, content *models.Content) string {
	if !content.UseTheme && content.TemplateName == "Blank Page" {
		if v, ok := content.Data["content"].(string); ok {
			return v
		}
	}
	if b, err := os.ReadFile(h.getStaticFilePath(content.FullPath)); err == nil {
		return string(b)
	}
	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err != nil {
		return ""
	}
	return h.renderContent(content, &tmpl)
}

// pageMarkdown builds the Markdown copy: a title block with source metadata,
// then the converted page body.
func (h *Handler) pageMarkdown(ctx context.Context, r *http.Request, content *models.Content) (string, error) {
	base := h.resolveBaseURL(r)
	body := services.HTMLToMarkdown(h.pageBodyHTML(ctx, content), base)

	// Drop a leading H1 that repeats the title.
	trimmed := strings.TrimSpace(body)
	if first, rest, ok := strings.Cut(trimmed, "\n"); ok || strings.HasPrefix(trimmed, "# ") {
		if !ok {
			first, rest = trimmed, ""
		}
		if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(first, "# ")), strings.TrimSpace(content.Title)) && strings.HasPrefix(first, "# ") {
			body = strings.TrimSpace(rest) + "\n"
		}
	}

	var sb strings.Builder
	sb.WriteString("# " + content.Title + "\n\n")
	if content.MetaDescription != "" {
		sb.WriteString("> " + content.MetaDescription + "\n\n")
	}
	sb.WriteString("Source: " + base + content.FullPath + "  \n")
	if content.PublishedAt != nil && !content.PublishedAt.IsZero() {
		sb.WriteString("Published: " + content.PublishedAt.UTC().Format("2006-01-02") + "  \n")
	}
	sb.WriteString("Updated: " + content.ModifiedAt().UTC().Format("2006-01-02") + "\n\n")
	sb.WriteString(body)
	return sb.String(), nil
}

// ==================== <head> additions ====================

// pageHeadExtras returns per-page <head> tags: robots noindex for hidden
// pages, otherwise the Markdown alternate link.
func (h *Handler) pageHeadExtras(r *http.Request, content *models.Content, cfg services.SEOConfig) string {
	if content == nil {
		return ""
	}
	if content.NoIndex {
		return `<meta name="robots" content="noindex">`
	}
	if cfg.MarkdownDisabled {
		return ""
	}
	return `<link rel="alternate" type="text/markdown" title="Markdown" href="` +
		template.HTMLEscapeString(h.resolveBaseURL(r)+markdownPath(content.FullPath)) + `">`
}

// feedLinks returns feed autodiscovery <link> tags for every public page.
func (h *Handler) feedLinks(r *http.Request, cfg services.SEOConfig, siteName string) string {
	if cfg.FeedDisabled {
		return ""
	}
	base := h.resolveBaseURL(r)
	t := template.HTMLEscapeString(siteName)
	return `<link rel="alternate" type="application/rss+xml" title="` + t + ` (RSS)" href="` + base + `/feed.xml">` +
		`<link rel="alternate" type="application/atom+xml" title="` + t + ` (Atom)" href="` + base + `/atom.xml">`
}

// injectHead inserts tags before </head> of a full HTML document (raw pages).
func injectHead(doc, tags string) string {
	if tags == "" {
		return doc
	}
	if idx := strings.Index(strings.ToLower(doc), "</head>"); idx >= 0 {
		return doc[:idx] + tags + doc[idx:]
	}
	return doc
}

// ==================== Feeds ====================

type feedItem struct {
	Title       string
	URL         string
	Summary     string
	ContentHTML string
	Author      string
	Published   time.Time
	Updated     time.Time
}

// feedItems selects the newest feed-eligible pages. category != "" restricts
// to one collection's category (collection feeds ignore the template filter).
func (h *Handler) feedItems(ctx context.Context, r *http.Request, cfg services.SEOConfig, category string) ([]feedItem, error) {
	filter := bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
		"fork_id":   nil,
		"noindex":   bson.M{"$ne": true},
	}
	if category != "" {
		filter["category"] = category
	} else if !cfg.FeedAllPages {
		var or bson.A
		if len(cfg.FeedTemplates) > 0 {
			or = append(or, bson.M{"template_name": bson.M{"$in": cfg.FeedTemplates}})
		}
		if len(cfg.FeedCategories) > 0 {
			or = append(or, bson.M{"category": bson.M{"$in": cfg.FeedCategories}})
		}
		if len(or) > 0 {
			filter["$or"] = or
		}
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "published_at", Value: -1}, {Key: "created_at", Value: -1}}).
		SetLimit(int64(cfg.FeedLimit)).
		SetProjection(bson.M{
			"title": 1, "full_path": 1, "slug": 1, "meta_description": 1, "published_at": 1,
			"created_at": 1, "updated_at": 1, "content_modified_at": 1, "author_name": 1,
			"template_name": 1, "template_id": 1, "use_theme": 1,
		})
	cursor, err := h.db.FindMany(ctx, "content", filter, opts)
	if err != nil {
		return nil, err
	}
	var pages []models.Content
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, err
	}

	base := h.resolveBaseURL(r)
	items := make([]feedItem, 0, len(pages))
	for i := range pages {
		p := &pages[i]
		pub := p.CreatedAt
		if p.PublishedAt != nil && !p.PublishedAt.IsZero() {
			pub = *p.PublishedAt
		}
		author := p.AuthorName
		if author == "" {
			author = cfg.AuthorName
		}
		body := ""
		if b, err := os.ReadFile(h.getStaticFilePath(p.FullPath)); err == nil {
			body = absolutizeHTML(string(b), base)
		}
		items = append(items, feedItem{
			Title: p.Title, URL: base + p.FullPath, Summary: p.MetaDescription,
			ContentHTML: body, Author: author, Published: pub, Updated: p.ModifiedAt(),
		})
	}
	return items, nil
}

var rootRelAttr = regexp.MustCompile(`(\s(?:href|src))="/([^/"][^"]*)?"`)

// absolutizeHTML rewrites root-relative href/src attributes so feed readers
// (which have no page base URL) resolve links and images correctly.
func absolutizeHTML(s, base string) string {
	return rootRelAttr.ReplaceAllString(s, `$1="`+strings.TrimRight(base, "/")+`/$2"`)
}

type rssCDATA struct {
	Text string `xml:",cdata"`
}

type rssItem struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	GUID        rssGUID   `xml:"guid"`
	PubDate     string    `xml:"pubDate"`
	Author      string    `xml:"dc:creator,omitempty"`
	Description string    `xml:"description,omitempty"`
	Content     *rssCDATA `xml:"content:encoded,omitempty"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type rssAtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssDoc struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	NSAtom  string   `xml:"xmlns:atom,attr"`
	NSCont  string   `xml:"xmlns:content,attr"`
	NSDC    string   `xml:"xmlns:dc,attr"`
	Channel struct {
		Title         string      `xml:"title"`
		Link          string      `xml:"link"`
		Description   string      `xml:"description"`
		Self          rssAtomLink `xml:"atom:link"`
		LastBuildDate string      `xml:"lastBuildDate,omitempty"`
		Generator     string      `xml:"generator"`
		Items         []rssItem   `xml:"item"`
	} `xml:"channel"`
}

type atomPerson struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomText struct {
	Type string `xml:"type,attr,omitempty"`
	Body string `xml:",chardata"`
}

type atomEntry struct {
	Title     string      `xml:"title"`
	Link      atomLink    `xml:"link"`
	ID        string      `xml:"id"`
	Published string      `xml:"published"`
	Updated   string      `xml:"updated"`
	Author    *atomPerson `xml:"author,omitempty"`
	Summary   *atomText   `xml:"summary,omitempty"`
	Content   *atomText   `xml:"content,omitempty"`
}

type atomFeed struct {
	XMLName  xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle,omitempty"`
	Links    []atomLink  `xml:"link"`
	ID       string      `xml:"id"`
	Updated  string      `xml:"updated"`
	Author   *atomPerson `xml:"author,omitempty"`
	Entries  []atomEntry `xml:"entry"`
}

// feedScope resolves which feed a request is for: site-wide, or a
// collection's (/<collection>/feed.xml). ok=false means 404.
func (h *Handler) feedScope(r *http.Request) (title, desc, category, selfPath string, ok bool) {
	ctx := r.Context()
	theme, _ := h.db.GetThemeSettings(ctx)
	siteName, tagline := "", ""
	if theme != nil {
		siteName, tagline = theme.SiteName, theme.SiteTagline
	}
	if slug := mux.Vars(r)["collection"]; slug != "" {
		var coll models.Collection
		if err := h.db.FindOne(ctx, "collections", bson.M{"slug": slug}, &coll); err != nil {
			return "", "", "", "", false
		}
		return coll.Name + " — " + siteName, coll.Description, coll.Category, r.URL.Path, true
	}
	return siteName, tagline, "", r.URL.Path, true
}

// ServeRSSFeed serves /feed.xml and /<collection>/feed.xml (RSS 2.0).
func (h *Handler) ServeRSSFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.seoConfig(ctx)
	title, desc, category, selfPath, ok := h.feedScope(r)
	if cfg.FeedDisabled || !ok {
		h.servePagePath(w, r)
		return
	}
	h.recordCrawler(r, r.URL.Path)
	items, err := h.feedItems(ctx, r, cfg, category)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	base := h.resolveBaseURL(r)
	var doc rssDoc
	doc.Version, doc.NSAtom = "2.0", "http://www.w3.org/2005/Atom"
	doc.NSCont, doc.NSDC = "http://purl.org/rss/1.0/modules/content/", "http://purl.org/dc/elements/1.1/"
	doc.Channel.Title = title
	doc.Channel.Link = base + "/"
	doc.Channel.Description = desc
	if doc.Channel.Description == "" {
		doc.Channel.Description = title
	}
	doc.Channel.Self = rssAtomLink{Href: base + selfPath, Rel: "self", Type: "application/rss+xml"}
	doc.Channel.Generator = "LightCMS"
	var newest time.Time
	for _, it := range items {
		ri := rssItem{
			Title: it.Title, Link: it.URL, GUID: rssGUID{IsPermaLink: "true", Value: it.URL},
			PubDate: it.Published.UTC().Format(time.RFC1123Z), Author: it.Author, Description: it.Summary,
		}
		if it.ContentHTML != "" {
			ri.Content = &rssCDATA{Text: it.ContentHTML}
		}
		doc.Channel.Items = append(doc.Channel.Items, ri)
		if it.Updated.After(newest) {
			newest = it.Updated
		}
	}
	if !newest.IsZero() {
		doc.Channel.LastBuildDate = newest.UTC().Format(time.RFC1123Z)
	}
	writeXML(w, "application/rss+xml; charset=utf-8", doc)
}

// ServeAtomFeed serves /atom.xml (Atom 1.0).
func (h *Handler) ServeAtomFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.seoConfig(ctx)
	title, desc, category, selfPath, ok := h.feedScope(r)
	if cfg.FeedDisabled || !ok {
		h.servePagePath(w, r)
		return
	}
	h.recordCrawler(r, r.URL.Path)
	items, err := h.feedItems(ctx, r, cfg, category)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	base := h.resolveBaseURL(r)
	feed := atomFeed{
		Title: title, Subtitle: desc, ID: base + "/",
		Links: []atomLink{{Href: base + "/"}, {Href: base + selfPath, Rel: "self", Type: "application/atom+xml"}},
	}
	if cfg.AuthorName != "" {
		feed.Author = &atomPerson{Name: cfg.AuthorName}
	}
	var newest time.Time
	for _, it := range items {
		e := atomEntry{
			Title: it.Title, Link: atomLink{Href: it.URL}, ID: it.URL,
			Published: it.Published.UTC().Format(time.RFC3339), Updated: it.Updated.UTC().Format(time.RFC3339),
		}
		if it.Author != "" {
			e.Author = &atomPerson{Name: it.Author}
		} else if feed.Author == nil {
			e.Author = &atomPerson{Name: title} // Atom requires an author on feed or entry
		}
		if it.Summary != "" {
			e.Summary = &atomText{Type: "text", Body: it.Summary}
		}
		if it.ContentHTML != "" {
			e.Content = &atomText{Type: "html", Body: it.ContentHTML}
		}
		feed.Entries = append(feed.Entries, e)
		if it.Updated.After(newest) {
			newest = it.Updated
		}
	}
	if newest.IsZero() {
		newest = time.Now()
	}
	feed.Updated = newest.UTC().Format(time.RFC3339)
	writeXML(w, "application/atom+xml; charset=utf-8", feed)
}

// servePagePath hands a request on a dedicated route (feeds) to normal page
// serving, which resolves the page from the "slug" route var.
func (h *Handler) servePagePath(w http.ResponseWriter, r *http.Request) {
	h.ServePage(w, mux.SetURLVars(r, map[string]string{"slug": strings.TrimPrefix(r.URL.Path, "/")}))
}

func writeXML(w http.ResponseWriter, contentType string, v interface{}) {
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=300, s-maxage=1800")
	w.Write([]byte(xml.Header))
	w.Write(out)
}

// ==================== Structured data ====================

// pageLD is everything needed for a page's schema.org JSON-LD.
type pageLD struct {
	Content     *models.Content
	Tmpl        *models.Template
	SiteName    string
	Tagline     string
	BaseURL     string
	OGImage     string
	LogoURL     string
	SEO         *services.SEOConfig
	Breadcrumbs []crumb
	FAQ         []services.FAQItem
	ExistingLD  string // the page's own JSON-LD, to avoid duplicating types
}

type crumb struct{ Name, URL string }

func ldScript(doc map[string]interface{}) string {
	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	// </script> inside JSON strings would terminate the script element early.
	return `<script type="application/ld+json">` + strings.ReplaceAll(string(b), "</", `<\/`) + `</script>`
}

var ldTypeRE = regexp.MustCompile(`"@type"\s*:\s*"([A-Za-z]+)"`)

// existingLDTypes lists @type values already present in authored JSON-LD.
func existingLDTypes(src string) map[string]bool {
	out := map[string]bool{}
	if !strings.Contains(src, "application/ld+json") {
		return out
	}
	for _, m := range ldTypeRE.FindAllStringSubmatch(src, -1) {
		out[m[1]] = true
	}
	return out
}

func absURL(base, u string) string {
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return strings.TrimRight(base, "/") + u
	}
	return u
}

// authorNode builds the schema.org author for a page: the page's own author
// override, else the site default. Never derived from user accounts.
func authorNode(p *pageLD) map[string]interface{} {
	if p.Content != nil && p.Content.AuthorName != "" {
		n := map[string]interface{}{"@type": "Person", "name": p.Content.AuthorName}
		if p.Content.AuthorURL != "" {
			n["url"] = p.Content.AuthorURL
		}
		return n
	}
	if p.SEO == nil || p.SEO.AuthorName == "" {
		return nil
	}
	t := p.SEO.AuthorType
	if t == "" {
		t = "Person"
	}
	n := map[string]interface{}{"@type": t, "name": p.SEO.AuthorName}
	if p.SEO.AuthorURL != "" {
		n["url"] = p.SEO.AuthorURL
	}
	if len(p.SEO.AuthorSameAs) > 0 {
		n["sameAs"] = p.SEO.AuthorSameAs
	}
	return n
}

func publisherNode(p *pageLD) map[string]interface{} {
	if p.SiteName == "" {
		return nil
	}
	n := map[string]interface{}{"@type": "Organization", "name": p.SiteName}
	if p.LogoURL != "" {
		n["logo"] = map[string]interface{}{"@type": "ImageObject", "url": absURL(p.BaseURL, p.LogoURL)}
	}
	if p.SEO != nil && len(p.SEO.PublisherSameAs) > 0 {
		n["sameAs"] = p.SEO.PublisherSameAs
	}
	return n
}

// buildPageJSONLD returns the page's JSON-LD script tags: the main page node
// (WebPage / BlogPosting / NewsArticle), plus BreadcrumbList, FAQPage, and on
// the homepage WebSite + Organization. Types the page already declares in its
// own authored JSON-LD are skipped.
func buildPageJSONLD(p *pageLD) string {
	if p == nil || p.Content == nil {
		return ""
	}
	c := p.Content
	base := strings.TrimRight(p.BaseURL, "/")
	path := c.FullPath
	if path == "" {
		path = "/" + c.Slug
	}
	skip := existingLDTypes(p.ExistingLD)
	var out strings.Builder

	schemaType := "WebPage"
	if p.Tmpl != nil {
		name := strings.ToLower(p.Tmpl.Name + " " + p.Tmpl.Category)
		switch {
		case strings.Contains(name, "blog"):
			schemaType = "BlogPosting"
		case strings.Contains(name, "press"):
			schemaType = "NewsArticle"
		}
	}
	if !skip[schemaType] {
		doc := map[string]interface{}{
			"@context": "https://schema.org",
			"@type":    schemaType,
			"headline": c.Title,
			"url":      base + path,
		}
		if schemaType != "WebPage" {
			doc["mainEntityOfPage"] = base + path
		}
		if c.MetaDescription != "" {
			doc["description"] = c.MetaDescription
		}
		if p.OGImage != "" {
			doc["image"] = absURL(base, p.OGImage)
		}
		if c.PublishedAt != nil && !c.PublishedAt.IsZero() {
			doc["datePublished"] = c.PublishedAt.UTC().Format(time.RFC3339)
		}
		if m := c.ModifiedAt(); !m.IsZero() {
			doc["dateModified"] = m.UTC().Format(time.RFC3339)
		}
		if a := authorNode(p); a != nil {
			doc["author"] = a
		}
		if pub := publisherNode(p); pub != nil {
			doc["publisher"] = pub
		}
		out.WriteString(ldScript(doc))
	}

	if len(p.Breadcrumbs) > 0 && !skip["BreadcrumbList"] {
		var list []interface{}
		for i, b := range p.Breadcrumbs {
			list = append(list, map[string]interface{}{"@type": "ListItem", "position": i + 1, "name": b.Name, "item": b.URL})
		}
		out.WriteString(ldScript(map[string]interface{}{"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": list}))
	}

	if len(p.FAQ) > 0 && !skip["FAQPage"] {
		var qs []interface{}
		for _, f := range p.FAQ {
			qs = append(qs, map[string]interface{}{
				"@type": "Question", "name": f.Question,
				"acceptedAnswer": map[string]interface{}{"@type": "Answer", "text": f.Answer},
			})
		}
		out.WriteString(ldScript(map[string]interface{}{"@context": "https://schema.org", "@type": "FAQPage", "mainEntity": qs}))
	}

	if path == "/" {
		if !skip["WebSite"] {
			out.WriteString(buildWebsiteJSONLD(p.SiteName, p.Tagline, base))
		}
		if org := publisherNode(p); org != nil && !skip["Organization"] {
			org["@context"] = "https://schema.org"
			org["url"] = base + "/"
			out.WriteString(ldScript(org))
		}
	}
	return out.String()
}

// breadcrumbsFor builds Home → folders → page for pages inside folders.
func (h *Handler) breadcrumbsFor(ctx context.Context, base string, c *models.Content) []crumb {
	if c.FolderPath == "" || c.FolderPath == "/" {
		return nil
	}
	base = strings.TrimRight(base, "/")
	var paths []string
	parts := strings.Split(strings.Trim(c.FolderPath, "/"), "/")
	for i := range parts {
		paths = append(paths, "/"+strings.Join(parts[:i+1], "/"))
	}
	names := map[string]string{}
	cursor, err := h.db.FindMany(ctx, "folders", bson.M{"path": bson.M{"$in": paths}},
		options.Find().SetProjection(bson.M{"path": 1, "name": 1}))
	if err == nil {
		var folders []models.Folder
		if cursor.All(ctx, &folders) == nil {
			for _, f := range folders {
				names[f.Path] = f.Name
			}
		}
	}
	crumbs := []crumb{{Name: "Home", URL: base + "/"}}
	for i, p := range paths {
		name := names[p]
		if name == "" {
			name = parts[i]
		}
		crumbs = append(crumbs, crumb{Name: name, URL: base + p})
	}
	return append(crumbs, crumb{Name: c.Title, URL: base + c.FullPath})
}

// pageSEOHead assembles all SEO <head> additions for a content page:
// JSON-LD, robots/markdown tags, and feed autodiscovery.
func (h *Handler) pageSEOHead(r *http.Request, content *models.Content, tmpl *models.Template, ogImage, bodyHTML string) string {
	ctx := r.Context()
	cfg := h.seoConfig(ctx)
	theme, _ := h.db.GetThemeSettings(ctx)
	p := &pageLD{
		Content: content, Tmpl: tmpl, BaseURL: h.resolveBaseURL(r), OGImage: ogImage,
		SEO: &cfg, ExistingLD: bodyHTML,
	}
	if theme != nil {
		p.SiteName, p.Tagline, p.LogoURL = theme.SiteName, theme.SiteTagline, theme.LogoURL
	}
	p.Breadcrumbs = h.breadcrumbsFor(ctx, p.BaseURL, content)
	if strings.Contains(bodyHTML, "<details") || strings.Contains(strings.ToLower(bodyHTML), "question") || strings.Contains(strings.ToLower(bodyHTML), "faq") {
		p.FAQ = services.ExtractFAQ(bodyHTML)
	}
	head := buildPageJSONLD(p) + h.pageHeadExtras(r, content, cfg)
	return head
}
