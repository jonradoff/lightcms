package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type ListAuditLogsInput struct {
	Limit    int    `json:"limit,omitempty" jsonschema:"Maximum number of entries to return (default 50, max 200)"`
	Action   string `json:"action,omitempty" jsonschema:"Filter by action (e.g. content.create, login.success)"`
	Resource string `json:"resource,omitempty" jsonschema:"Filter by resource type (e.g. content, template, user)"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerAuditTools() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_audit_logs",
		Title:       "List Audit Logs",
		Description: "List recent audit log entries. Admin only. Supports filtering by action and resource type.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Audit Logs",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListAuditLogsInput) (*mcp.CallToolResult, any, error) {
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		result, err := s.client.ListAuditLogs(ctx, limit, args.Action, args.Resource)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})
}
