package mcp

import (
	"context"
	"fmt"

	"github.com/jonradoff/lightcms/v6/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SnippetIDInput struct {
	ID string `json:"id" jsonschema:"Snippet ID (MongoDB ObjectID),required"`
}

type CreateSnippetInput struct {
	Name string `json:"name" jsonschema:"Snippet name (used in lc:query directives),required"`
	HTML string `json:"html" jsonschema:"Go template HTML. Available fields: .Title .FullPath .Tags .MetaDescription .Category .Data,required"`
}

type UpdateSnippetInput struct {
	ID   string `json:"id" jsonschema:"Snippet ID (MongoDB ObjectID),required"`
	Name string `json:"name" jsonschema:"Snippet name,required"`
	HTML string `json:"html" jsonschema:"Go template HTML,required"`
}

func (s *Server) registerSnippetTools() {
	// List snippets
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_snippets",
		Description: "List all snippets. Snippets are reusable Go-template HTML fragments used in lc:query index page directives.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		snippets, err := s.client.ListSnippets(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(snippets), nil, nil
	})

	// Get snippet
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_snippet",
		Description: "Get a snippet by ID.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SnippetIDInput) (*mcp.CallToolResult, any, error) {
		snip, err := s.client.GetSnippet(ctx, args.ID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(snip), nil, nil
	})

	// Create snippet
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_snippet",
		Description: "Create a new snippet. Snippets are Go templates that render one content item in an lc:query index page. Reference by name: <!-- lc:query filter=\"tag:MyTag\" snippet=\"your-snippet-name\" -->",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateSnippetInput) (*mcp.CallToolResult, any, error) {
		snip, err := s.client.CreateSnippet(ctx, apiclient.CreateSnippetRequest{
			Name: args.Name,
			HTML: args.HTML,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      snip.ID,
			"name":    snip.Name,
			"message": fmt.Sprintf("Snippet '%s' created successfully", snip.Name),
		}), nil, nil
	})

	// Update snippet
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_snippet",
		Description: "Update an existing snippet's name and/or HTML template.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateSnippetInput) (*mcp.CallToolResult, any, error) {
		snip, err := s.client.UpdateSnippet(ctx, args.ID, apiclient.UpdateSnippetRequest{
			Name: args.Name,
			HTML: args.HTML,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      snip.ID,
			"name":    snip.Name,
			"message": fmt.Sprintf("Snippet '%s' updated successfully", snip.Name),
		}), nil, nil
	})

	// Delete snippet
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_snippet",
		Description: "Delete a snippet by ID.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SnippetIDInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.DeleteSnippet(ctx, args.ID); err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]interface{}{
			"success": true,
			"message": "Snippet deleted successfully",
		}), nil, nil
	})
}
