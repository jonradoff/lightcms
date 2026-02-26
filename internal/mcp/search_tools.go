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

type SearchReplacePreviewInput struct {
	Search  string `json:"search" jsonschema:"Text to search for,required"`
	Replace string `json:"replace" jsonschema:"Text to replace with,required"`
}

type SearchReplaceExecuteInput struct {
	Search         string `json:"search" jsonschema:"Text to search for,required"`
	Replace        string `json:"replace" jsonschema:"Text to replace with,required"`
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
		Name:        "search_replace_preview",
		Title:       "Search Replace Preview",
		Description: "Preview search and replace results without making changes. Shows which content items would be affected and where matches occur. ALWAYS use this before search_replace_execute to understand the impact.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "Search Replace Preview",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchReplacePreviewInput) (*mcp.CallToolResult, any, error) {
		if args.Search == "" {
			return errorResult(fmt.Errorf("search text is required")), nil, nil
		}

		result, err := s.client.SearchReplacePreview(ctx, args.Search, args.Replace)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(result), nil, nil
	})

	// Search and replace execute
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_replace_execute",
		Title:       "Search Replace Execute",
		Description: "Execute search and replace across all content. WARNING: This is a destructive operation that modifies content permanently. Always run search_replace_preview first to review what will be changed.",
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

		result, err := s.client.SearchReplaceExecute(ctx, args.Search, args.Replace, args.VersionComment)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(result), nil, nil
	})
}
