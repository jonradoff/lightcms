package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"lightcms/internal/database"
	"lightcms/internal/services"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server and services
type Server struct {
	mcpServer       *mcp.Server
	db              *database.DB
	contentService  *services.ContentService
	templateService *services.TemplateService
	assetService    *services.AssetService
	settingsService *services.SettingsService
}

// NewServer creates a new MCP server with all tools registered
func NewServer(db *database.DB) *Server {
	// Create services
	contentService := services.NewContentService(db)
	templateService := services.NewTemplateService(db, contentService)
	assetService := services.NewAssetService(db)
	settingsService := services.NewSettingsService(db, contentService)

	// Create MCP server
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "lightcms",
			Version: "1.0.0",
		},
		nil,
	)

	s := &Server{
		mcpServer:       mcpServer,
		db:              db,
		contentService:  contentService,
		templateService: templateService,
		assetService:    assetService,
		settingsService: settingsService,
	}

	// Register all tools
	s.registerContentTools()
	s.registerTemplateTools()
	s.registerAssetTools()
	s.registerSettingsTools()
	s.registerSearchTools()

	return s
}

// Run starts the MCP server using stdio transport
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// Helper to create a text result
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// Helper to create an error result
func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
		},
		IsError: true,
	}
}

// Helper to create a JSON result
func jsonResult(data interface{}) *mcp.CallToolResult {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errorResult(err)
	}
	return textResult(string(jsonBytes))
}
