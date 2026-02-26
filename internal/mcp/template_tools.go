package mcp

import (
	"context"
	"fmt"

	"lightcms/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Template tool input types
type TemplateIDInput struct {
	ID string `json:"id" jsonschema:"Template ID (MongoDB ObjectID),required"`
}

type GetTemplateInput struct {
	ID   string `json:"id,omitempty" jsonschema:"Template ID (MongoDB ObjectID)"`
	Slug string `json:"slug,omitempty" jsonschema:"Template slug"`
}

type CreateTemplateInput struct {
	Name        string                   `json:"name" jsonschema:"Template name,required"`
	Slug        string                   `json:"slug" jsonschema:"Template slug for URLs,required"`
	Description string                   `json:"description,omitempty" jsonschema:"Template description"`
	Category    string                   `json:"category,omitempty" jsonschema:"Template category for grouping"`
	Fields      []apiclient.TemplateField `json:"fields" jsonschema:"Template fields definition,required"`
	HTMLLayout  string                   `json:"html_layout" jsonschema:"HTML layout with {{.FieldName}} placeholders,required"`
}

type UpdateTemplateInput struct {
	ID          string                   `json:"id" jsonschema:"Template ID (MongoDB ObjectID),required"`
	Name        string                   `json:"name,omitempty" jsonschema:"Template name"`
	Slug        string                   `json:"slug,omitempty" jsonschema:"Template slug"`
	Description string                   `json:"description,omitempty" jsonschema:"Template description"`
	Category    string                   `json:"category,omitempty" jsonschema:"Template category"`
	Fields      []apiclient.TemplateField `json:"fields,omitempty" jsonschema:"Template fields definition"`
	HTMLLayout  string                   `json:"html_layout,omitempty" jsonschema:"HTML layout (changing this regenerates all content using this template)"`
}

func (s *Server) registerTemplateTools() {
	// List templates
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_templates",
		Title:       "List Templates",
		Description: "List all available templates. Templates define content structure with fields and HTML layout.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "List Templates",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		templates, err := s.client.ListTemplates(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}

		type TemplateSummary struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			Description string `json:"description"`
			Category    string `json:"category"`
			IsSystem    bool   `json:"is_system"`
			FieldCount  int    `json:"field_count"`
		}

		summaries := make([]TemplateSummary, len(templates))
		for i, t := range templates {
			summaries[i] = TemplateSummary{
				ID:          t.ID,
				Name:        t.Name,
				Slug:        t.Slug,
				Description: t.Description,
				Category:    t.Category,
				IsSystem:    t.IsSystem,
				FieldCount:  len(t.Fields),
			}
		}

		return jsonResult(summaries), nil, nil
	})

	// Get template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_template",
		Title:       "Get Template",
		Description: "Get a single template by ID or slug. Returns full template including fields and HTML layout.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "Get Template",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetTemplateInput) (*mcp.CallToolResult, any, error) {
		if args.ID == "" && args.Slug == "" {
			return errorResult(fmt.Errorf("either id or slug is required")), nil, nil
		}

		// The API supports both ID and slug on the same endpoint
		lookup := args.ID
		if lookup == "" {
			lookup = args.Slug
		}

		tmpl, err := s.client.GetTemplate(ctx, lookup)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(tmpl), nil, nil
	})

	// Create template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_template",
		Title:       "Create Template",
		Description: "Create a new template. Define fields with name, label, type (text, textarea, richtext, date, image, select), and options.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create Template",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateTemplateInput) (*mcp.CallToolResult, any, error) {
		createReq := apiclient.CreateTemplateRequest{
			Name:        args.Name,
			Slug:        args.Slug,
			Description: args.Description,
			Category:    args.Category,
			Fields:      args.Fields,
			HTMLLayout:  args.HTMLLayout,
		}

		tmpl, err := s.client.CreateTemplate(ctx, createReq)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      tmpl.ID,
			"message": fmt.Sprintf("Template '%s' created successfully", tmpl.Name),
		}), nil, nil
	})

	// Update template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_template",
		Title:       "Update Template",
		Description: "Update an existing template. Changing the HTML layout will regenerate all content using this template.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Template",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateTemplateInput) (*mcp.CallToolResult, any, error) {
		updates := make(apiclient.UpdateTemplateRequest)

		if args.Name != "" {
			updates["name"] = args.Name
		}
		if args.Slug != "" {
			updates["slug"] = args.Slug
		}
		if args.Description != "" {
			updates["description"] = args.Description
		}
		if args.Category != "" {
			updates["category"] = args.Category
		}
		if args.Fields != nil {
			updates["fields"] = args.Fields
		}
		if args.HTMLLayout != "" {
			updates["html_layout"] = args.HTMLLayout
		}

		tmpl, err := s.client.UpdateTemplate(ctx, args.ID, updates)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      tmpl.ID,
			"message": fmt.Sprintf("Template '%s' updated successfully", tmpl.Name),
		}), nil, nil
	})

	// Delete template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_template",
		Title:       "Delete Template",
		Description: "Delete a template. Cannot delete system templates or templates that have content using them.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Template",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args TemplateIDInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.DeleteTemplate(ctx, args.ID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Template %s deleted successfully", args.ID)), nil, nil
	})
}
