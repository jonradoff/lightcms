package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type ScheduleContentPublishInput struct {
	ContentID string `json:"content_id" jsonschema:"Content item ID,required"`
	PublishAt string `json:"publish_at" jsonschema:"ISO 8601 datetime when to publish (e.g. 2026-03-24T15:00:00Z),required"`
}

type CancelScheduledPublishInput struct {
	ContentID string `json:"content_id" jsonschema:"Content item ID,required"`
}

type ListScheduledContentInput struct {
	Folder string `json:"folder,omitempty" jsonschema:"Optional folder path filter (e.g. /blog)"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerScheduleTools() {
	// schedule_content_publish
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "schedule_content_publish",
		Title:       "Schedule Content Publish",
		Description: "Set a future publish date/time for a content item. The scheduler automatically publishes it when the time arrives.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Schedule Content Publish",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ScheduleContentPublishInput) (*mcp.CallToolResult, any, error) {
		if args.ContentID == "" {
			return errorResult(fmt.Errorf("content_id is required")), nil, nil
		}
		if args.PublishAt == "" {
			return errorResult(fmt.Errorf("publish_at is required")), nil, nil
		}
		result, err := s.client.ScheduleContentPublish(ctx, args.ContentID, &args.PublishAt)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// cancel_scheduled_publish
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "cancel_scheduled_publish",
		Title:       "Cancel Scheduled Publish",
		Description: "Clear the scheduled publish time for a content item.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Cancel Scheduled Publish",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CancelScheduledPublishInput) (*mcp.CallToolResult, any, error) {
		if args.ContentID == "" {
			return errorResult(fmt.Errorf("content_id is required")), nil, nil
		}
		result, err := s.client.ScheduleContentPublish(ctx, args.ContentID, nil)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// list_scheduled_content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_scheduled_content",
		Title:       "List Scheduled Content",
		Description: "List all unpublished content items that have a scheduled publish time set.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Scheduled Content",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListScheduledContentInput) (*mcp.CallToolResult, any, error) {
		items, err := s.client.ListScheduledContent(ctx, args.Folder)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(items), nil, nil
	})
}
