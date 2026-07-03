package mcp

import (
	"context"
	"fmt"

	"github.com/jonradoff/lightcms/v6/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type ListWebhooksInput struct{}

type CreateWebhookInput struct {
	Name   string   `json:"name" jsonschema:"Name for the webhook,required"`
	URL    string   `json:"url" jsonschema:"Endpoint URL to deliver events to,required"`
	Events []string `json:"events" jsonschema:"Event types to subscribe to (e.g. content.publish),required"`
	Active bool     `json:"active,omitempty" jsonschema:"Whether the webhook is active (default true)"`
}

type UpdateWebhookInput struct {
	ID     string   `json:"id" jsonschema:"Webhook ID,required"`
	Name   *string  `json:"name,omitempty" jsonschema:"New name"`
	URL    *string  `json:"url,omitempty" jsonschema:"New endpoint URL"`
	Events []string `json:"events,omitempty" jsonschema:"New event list"`
	Active *bool    `json:"active,omitempty" jsonschema:"Enable or disable the webhook"`
}

type WebhookIDInput struct {
	ID string `json:"id" jsonschema:"Webhook ID,required"`
}

type ListWebhookDeliveriesInput struct {
	ID    string `json:"id" jsonschema:"Webhook ID,required"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of deliveries to return (default 50)"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerWebhookTools() {
	// list_webhooks
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_webhooks",
		Title:       "List Webhooks",
		Description: "List all registered webhooks. Secrets are never returned.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Webhooks",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListWebhooksInput) (*mcp.CallToolResult, any, error) {
		hooks, err := s.client.ListWebhooks(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(hooks), nil, nil
	})

	// create_webhook
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:  "create_webhook",
		Title: "Create Webhook",
		Description: `Create a new webhook. Returns the generated HMAC-SHA256 secret ONCE — save it immediately.

Valid event types: content.create, content.update, content.publish, content.unpublish, content.delete, comment.created, content.pending_approval, asset.pending_review`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create Webhook",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(true),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateWebhookInput) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return errorResult(fmt.Errorf("name is required")), nil, nil
		}
		if args.URL == "" {
			return errorResult(fmt.Errorf("url is required")), nil, nil
		}
		if len(args.Events) == 0 {
			return errorResult(fmt.Errorf("events is required")), nil, nil
		}
		active := args.Active
		// Default active to true when not specified (zero value is false, treat as "unset" → true)
		// Since we can't distinguish false from unset, we keep the passed value.
		createReq := apiclient.CreateWebhookRequest{
			Name:   args.Name,
			URL:    args.URL,
			Events: args.Events,
			Active: active,
		}
		created, err := s.client.CreateWebhook(ctx, createReq)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(created), nil, nil
	})

	// update_webhook
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_webhook",
		Title:       "Update Webhook",
		Description: "Update an existing webhook (name, URL, events, active). Partial update — only provided fields change.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Webhook",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateWebhookInput) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return errorResult(fmt.Errorf("id is required")), nil, nil
		}
		updateReq := apiclient.UpdateWebhookRequest{
			Name:   args.Name,
			URL:    args.URL,
			Events: args.Events,
			Active: args.Active,
		}
		result, err := s.client.UpdateWebhook(ctx, args.ID, updateReq)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// delete_webhook
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_webhook",
		Title:       "Delete Webhook",
		Description: "Permanently delete a webhook.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Webhook",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args WebhookIDInput) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return errorResult(fmt.Errorf("id is required")), nil, nil
		}
		if err := s.client.DeleteWebhook(ctx, args.ID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Webhook %s deleted successfully", args.ID)), nil, nil
	})

	// regenerate_webhook_secret
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "regenerate_webhook_secret",
		Title:       "Regenerate Webhook Secret",
		Description: "Generate a new HMAC-SHA256 signing secret for a webhook. The old secret stops working immediately. The new secret is returned ONCE — save it.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Regenerate Webhook Secret",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  false,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args WebhookIDInput) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return errorResult(fmt.Errorf("id is required")), nil, nil
		}
		secret, err := s.client.RegenerateWebhookSecret(ctx, args.ID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]string{
			"webhook_id": args.ID,
			"secret":     secret,
			"note":       "Save this secret immediately — it will not be shown again.",
		}), nil, nil
	})

	// list_webhook_deliveries
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_webhook_deliveries",
		Title:       "List Webhook Deliveries",
		Description: "List recent delivery attempts for a webhook.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Webhook Deliveries",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListWebhookDeliveriesInput) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return errorResult(fmt.Errorf("id is required")), nil, nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}
		deliveries, err := s.client.ListWebhookDeliveries(ctx, args.ID, limit)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(deliveries), nil, nil
	})
}
