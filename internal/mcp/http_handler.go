package mcp

import (
	"net/http"
	"strings"

	"lightcms/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewHTTPHandler creates an HTTP handler for the MCP Streamable HTTP transport.
// Each session gets its own MCP server backed by an API client using the
// caller's API key (extracted from the Authorization header, already validated
// by the API auth middleware).
func NewHTTPHandler(serverPort string) http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			apiKey := extractBearerToken(r)
			client := apiclient.New("http://localhost:"+serverPort, apiKey)
			server := NewServer(client)
			return server.MCPServer()
		},
		nil,
	)
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
