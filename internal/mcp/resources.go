package mcp

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerResources registers MCP resources that expose live site data.
func (s *Server) registerResources() {
	s.mcpServer.AddResource(
		&mcp.Resource{
			URI:         "lightcms://site/structure",
			Name:        "Site Structure",
			Description: "Current site structure including pages, folders, and collections",
			MIMEType:    "application/json",
		},
		s.handleSiteStructure,
	)

	s.mcpServer.AddResource(
		&mcp.Resource{
			URI:         "lightcms://content/recent",
			Name:        "Recent Content",
			Description: "Recently modified content items",
			MIMEType:    "application/json",
		},
		s.handleRecentContent,
	)

	s.mcpServer.AddResource(
		&mcp.Resource{
			URI:         "lightcms://theme/config",
			Name:        "Theme Configuration",
			Description: "Current theme settings including colors, fonts, and header/footer",
			MIMEType:    "application/json",
		},
		s.handleThemeConfig,
	)
}

func (s *Server) handleSiteStructure(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	ctx := context.Background()

	folders, err := s.client.ListFolders(ctx)
	if err != nil {
		return nil, err
	}

	content, err := s.client.ListContent(ctx, false, "", "")
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"folders":       folders,
		"content_count": len(content),
		"content":       content,
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "lightcms://site/structure",
				MIMEType: "application/json",
				Text:     string(jsonBytes),
			},
		},
	}, nil
}

func (s *Server) handleRecentContent(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	ctx := context.Background()

	content, err := s.client.ListContent(ctx, false, "", "")
	if err != nil {
		return nil, err
	}

	// Return the most recent 20 items
	limit := 20
	if len(content) < limit {
		limit = len(content)
	}
	recent := content[:limit]

	jsonBytes, err := json.MarshalIndent(recent, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "lightcms://content/recent",
				MIMEType: "application/json",
				Text:     string(jsonBytes),
			},
		},
	}, nil
}

func (s *Server) handleThemeConfig(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	ctx := context.Background()

	theme, err := s.client.GetTheme(ctx)
	if err != nil {
		return nil, err
	}

	jsonBytes, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "lightcms://theme/config",
				MIMEType: "application/json",
				Text:     string(jsonBytes),
			},
		},
	}, nil
}
