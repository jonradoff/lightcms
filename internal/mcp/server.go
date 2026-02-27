package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"lightcms/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server and API client
type Server struct {
	mcpServer *mcp.Server
	client    *apiclient.Client
}

// NewServer creates a new MCP server with all tools registered
func NewServer(client *apiclient.Client) *Server {
	// Create MCP server
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "lightcms",
			Version: "1.1.0",
		},
		nil,
	)

	s := &Server{
		mcpServer: mcpServer,
		client:    client,
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

// MCPServer returns the underlying SDK server for use with HTTP transport
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}

// boolPtr returns a pointer to a bool value.
// Needed for ToolAnnotations fields where nil means "use spec default" (true).
func boolPtr(b bool) *bool {
	return &b
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
