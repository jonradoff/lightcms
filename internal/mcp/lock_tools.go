package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type LockContentInput struct {
	ContentID string `json:"content_id" jsonschema:"Content item ID,required"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerLockTools() {
	// get_content_lock
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_content_lock",
		Title:       "Get Content Lock",
		Description: "Get the current advisory lock status for a content item. Returns lock holder and expiry, or {locked: false} if unlocked.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Content Lock",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LockContentInput) (*mcp.CallToolResult, any, error) {
		if args.ContentID == "" {
			return errorResult(fmt.Errorf("content_id is required")), nil, nil
		}
		lock, err := s.client.GetContentLock(ctx, args.ContentID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(lock), nil, nil
	})

	// acquire_content_lock
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "acquire_content_lock",
		Title:       "Acquire Content Lock",
		Description: "Acquire an advisory lock on a content item for the current API user. Returns conflict if another user holds the lock.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Acquire Content Lock",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LockContentInput) (*mcp.CallToolResult, any, error) {
		if args.ContentID == "" {
			return errorResult(fmt.Errorf("content_id is required")), nil, nil
		}
		lock, err := s.client.AcquireContentLock(ctx, args.ContentID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(lock), nil, nil
	})

	// release_content_lock
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "release_content_lock",
		Title:       "Release Content Lock",
		Description: "Release the advisory lock on a content item held by the current API user.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Release Content Lock",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LockContentInput) (*mcp.CallToolResult, any, error) {
		if args.ContentID == "" {
			return errorResult(fmt.Errorf("content_id is required")), nil, nil
		}
		if err := s.client.ReleaseContentLock(ctx, args.ContentID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Lock released for content %s", args.ContentID)), nil, nil
	})

	// force_unlock_content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "force_unlock_content",
		Title:       "Force Unlock Content",
		Description: "Admin only: force-release any lock on a content item regardless of who holds it.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Force Unlock Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LockContentInput) (*mcp.CallToolResult, any, error) {
		if args.ContentID == "" {
			return errorResult(fmt.Errorf("content_id is required")), nil, nil
		}
		if err := s.client.ForceUnlockContent(ctx, args.ContentID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Lock force-released for content %s", args.ContentID)), nil, nil
	})
}
