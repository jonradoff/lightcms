package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/services"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PublicServer is the read-only MCP surface for site *visitors*: any MCP
// client (a reader's Claude, a research agent, …) can search and read the
// published site with no authentication. It never exposes drafts, forks,
// or any mutation.
type PublicServer struct {
	db      *database.DB
	search  *services.SearchService
	baseURL string
	server  *mcp.Server
}

// NewPublicServer builds the public MCP server with its read-only tools.
func NewPublicServer(db *database.DB, search *services.SearchService, baseURL string) *PublicServer {
	s := &PublicServer{
		db:      db,
		search:  search,
		baseURL: strings.TrimRight(baseURL, "/"),
		server: mcp.NewServer(&mcp.Implementation{
			Name:    "lightcms-public",
			Version: "6.0.0",
		}, nil),
	}
	s.registerTools()
	return s
}

// Handler returns the Streamable HTTP transport handler for mounting.
func (s *PublicServer) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.server
	}, nil)
}

// publishedFilter restricts queries to live, published, non-deleted pages.
func publishedFilter() bson.M {
	return bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
		"fork_id":   bson.M{"$exists": false},
	}
}

type PublicSearchInput struct {
	Query string `json:"query" jsonschema:"Search terms,required"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max results (default 10, max 25)"`
}

type PublicGetPageInput struct {
	Path string `json:"path" jsonschema:"URL path of the page (e.g. /about),required"`
}

func (s *PublicServer) registerTools() {
	// get_site_info
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_site_info",
		Title:       "Get Site Info",
		Description: "Name, tagline, and base URL of this site, plus pointers to llms.txt and the sitemap.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Site Info", ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		theme, err := s.db.GetThemeSettings(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]interface{}{
			"name":     theme.SiteName,
			"tagline":  theme.SiteTagline,
			"base_url": s.baseURL,
			"llms_txt": s.baseURL + "/llms.txt",
			"sitemap":  s.baseURL + "/sitemap.xml",
		}), nil, nil
	})

	// list_pages
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_pages",
		Title:       "List Pages",
		Description: "All published pages on this site: title, path, and meta description.",
		Annotations: &mcp.ToolAnnotations{Title: "List Pages", ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		cursor, err := s.db.FindMany(ctx, "content", publishedFilter(),
			options.Find().SetSort(bson.D{{Key: "full_path", Value: 1}}).SetLimit(1000))
		if err != nil {
			return errorResult(err), nil, nil
		}
		var items []models.Content
		if err := cursor.All(ctx, &items); err != nil {
			return errorResult(err), nil, nil
		}
		out := make([]map[string]interface{}, 0, len(items))
		for _, c := range items {
			out = append(out, map[string]interface{}{
				"title":       c.Title,
				"path":        c.FullPath,
				"url":         s.baseURL + c.FullPath,
				"description": c.MetaDescription,
			})
		}
		return jsonResult(out), nil, nil
	})

	// search_site
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_site",
		Title:       "Search Site",
		Description: "Full-text search across the published site. Returns matching pages with snippets.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Site", ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args PublicSearchInput) (*mcp.CallToolResult, any, error) {
		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}
		if limit > 25 {
			limit = 25
		}
		results, err := s.search.Search(ctx, args.Query, "", limit)
		if err != nil {
			return errorResult(err), nil, nil
		}
		out := make([]map[string]interface{}, 0, len(results))
		for _, r := range results {
			out = append(out, map[string]interface{}{
				"title":   r.Title,
				"path":    r.FullPath,
				"url":     s.baseURL + r.FullPath,
				"snippet": r.Snippet,
			})
		}
		return jsonResult(out), nil, nil
	})

	// get_page
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_page",
		Title:       "Get Page",
		Description: "Full plain-text content of one published page by URL path, with title, description, and publish date.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Page", ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args PublicGetPageInput) (*mcp.CallToolResult, any, error) {
		path := args.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		filter := publishedFilter()
		filter["full_path"] = path
		var c models.Content
		if err := s.db.FindOne(ctx, "content", filter, &c); err != nil {
			return textResult(fmt.Sprintf("no published page at %s", path)), nil, nil
		}
		out := map[string]interface{}{
			"title":       c.Title,
			"path":        c.FullPath,
			"url":         s.baseURL + c.FullPath,
			"description": c.MetaDescription,
			"tags":        c.Tags,
			"content":     strings.TrimSpace(c.PlainText),
		}
		if c.PublishedAt != nil {
			out["published_at"] = c.PublishedAt.UTC().Format("2006-01-02")
		}
		return jsonResult(out), nil, nil
	})
}

// MCPServer exposes the underlying SDK server (used by transports and tests).
func (s *PublicServer) MCPServer() *mcp.Server { return s.server }
