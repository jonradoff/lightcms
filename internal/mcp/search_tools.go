package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Search tool input types
type SearchContentInput struct {
	Query          string `json:"query" jsonschema:"Search query string,required"`
	SearchType     string `json:"search_type,omitempty" jsonschema:"Search type: 'name' (title only) or 'fulltext' (all fields). Defaults to 'fulltext'"`
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"Include soft-deleted content in results"`
}

type EndUserSearchInput struct {
	Query string `json:"query" jsonschema:"Search query,required"`
	Mode  string `json:"mode,omitempty" jsonschema:"Search mode: exact, semantic, or hybrid (default hybrid)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max results 1-50 (default 10)"`
}

type SearchReplacePreviewInput struct {
	Search  string `json:"search" jsonschema:"Text to search for,required"`
	Replace string `json:"replace" jsonschema:"Text to replace with,required"`
	Regex   bool   `json:"regex,omitempty" jsonschema:"If true, treat search as a Go regular expression. Use $1, $2 for capture group references in replace."`
}

type SearchReplaceExecuteInput struct {
	Search         string `json:"search" jsonschema:"Text to search for,required"`
	Replace        string `json:"replace" jsonschema:"Text to replace with,required"`
	Regex          bool   `json:"regex,omitempty" jsonschema:"If true, treat search as a Go regular expression. Use $1, $2 for capture group references in replace."`
	VersionComment string `json:"version_comment,omitempty" jsonschema:"Comment for version history (defaults to 'Bulk search and replace')"`
}

func (s *Server) registerSearchTools() {
	// Search content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_content",
		Title:       "Search Content",
		Description: "Search across all content items by title or full text. Returns matching content with paths and match context.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "Search Content",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchContentInput) (*mcp.CallToolResult, any, error) {
		if args.Query == "" {
			return errorResult(fmt.Errorf("query is required")), nil, nil
		}

		result, err := s.client.SearchContent(ctx, args.Query, args.SearchType, args.IncludeDeleted)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(result), nil, nil
	})

	// Search and replace preview
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:  "search_replace_preview",
		Title: "Search Replace Preview",
		Description: `Preview a site-wide search-and-replace without making any changes. ALWAYS run this before search_replace_execute.

Returns: affected page count, total match count, and per-page field breakdown.
For targeted replacements (a folder, template, or category), use scoped_search_replace_preview instead.`,
		Annotations: &mcp.ToolAnnotations{
			Title:         "Search Replace Preview",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchReplacePreviewInput) (*mcp.CallToolResult, any, error) {
		if args.Search == "" {
			return errorResult(fmt.Errorf("search text is required")), nil, nil
		}

		result, err := s.client.SearchReplacePreview(ctx, args.Search, args.Replace, args.Regex)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(result), nil, nil
	})

	// Search and replace execute
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:  "search_replace_execute",
		Title: "Search Replace Execute",
		Description: `Execute a site-wide search-and-replace across all content. Modifies every matching page permanently.

MANDATORY workflow:
1. Run search_replace_preview and show the user which pages will be affected.
2. Get explicit user confirmation before executing.
3. Run search_replace_execute with a clear version_comment.

For targeted replacements, use scoped_search_replace_execute instead.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Search Replace Execute",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchReplaceExecuteInput) (*mcp.CallToolResult, any, error) {
		if args.Search == "" {
			return errorResult(fmt.Errorf("search text is required")), nil, nil
		}

		result, err := s.client.SearchReplaceExecute(ctx, args.Search, args.Replace, args.VersionComment, args.Regex)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(result), nil, nil
	})

	// End-user search
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "end_user_search",
		Title:       "End User Search",
		Description: "Search published content using full-text exact match, semantic (AI) similarity, or hybrid mode. Returns page titles, paths, and snippets.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "End User Search",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EndUserSearchInput) (*mcp.CallToolResult, any, error) {
		if args.Query == "" {
			return errorResult(fmt.Errorf("query is required")), nil, nil
		}
		if args.Limit <= 0 {
			args.Limit = 10
		}
		result, err := s.client.EndUserSearch(ctx, args.Query, args.Mode, args.Limit)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// Reindex embeddings
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "reindex_embeddings",
		Title:       "Reindex Embeddings",
		Description: "Regenerate vector embeddings for all published content. Required after initial setup or if embeddings become stale.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Reindex Embeddings",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(true),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		result, err := s.client.ReindexEmbeddings(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})
}
