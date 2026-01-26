package mcp

import (
	"context"
	"fmt"

	"lightcms/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	Name        string                `json:"name" jsonschema:"Template name,required"`
	Slug        string                `json:"slug" jsonschema:"Template slug for URLs,required"`
	Description string                `json:"description,omitempty" jsonschema:"Template description"`
	Category    string                `json:"category,omitempty" jsonschema:"Template category for grouping"`
	Fields      []models.TemplateField `json:"fields" jsonschema:"Template fields definition,required"`
	HTMLLayout  string                `json:"html_layout" jsonschema:"HTML layout with {{.FieldName}} placeholders,required"`
}

type UpdateTemplateInput struct {
	ID          string                `json:"id" jsonschema:"Template ID (MongoDB ObjectID),required"`
	Name        string                `json:"name,omitempty" jsonschema:"Template name"`
	Slug        string                `json:"slug,omitempty" jsonschema:"Template slug"`
	Description string                `json:"description,omitempty" jsonschema:"Template description"`
	Category    string                `json:"category,omitempty" jsonschema:"Template category"`
	Fields      []models.TemplateField `json:"fields,omitempty" jsonschema:"Template fields definition"`
	HTMLLayout  string                `json:"html_layout,omitempty" jsonschema:"HTML layout (changing this regenerates all content using this template)"`
}

func (s *Server) registerTemplateTools() {
	// List templates
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_templates",
		Description: "List all available templates. Templates define content structure with fields and HTML layout.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		templates, err := s.templateService.ListTemplates(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Return summary for each template
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
				ID:          t.ID.Hex(),
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
		Description: "Get a single template by ID or slug. Returns full template including fields and HTML layout.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetTemplateInput) (*mcp.CallToolResult, any, error) {
		var tmpl *models.Template
		var err error

		if args.ID != "" {
			id, err := primitive.ObjectIDFromHex(args.ID)
			if err != nil {
				return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
			}
			tmpl, err = s.templateService.GetTemplate(ctx, id)
		} else if args.Slug != "" {
			tmpl, err = s.templateService.GetTemplateBySlug(ctx, args.Slug)
		} else {
			return errorResult(fmt.Errorf("either id or slug is required")), nil, nil
		}

		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(tmpl), nil, nil
	})

	// Create template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_template",
		Description: "Create a new template. Define fields with name, label, type (text, textarea, richtext, date, image, select), and options.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateTemplateInput) (*mcp.CallToolResult, any, error) {
		tmpl := &models.Template{
			Name:        args.Name,
			Slug:        args.Slug,
			Description: args.Description,
			Category:    args.Category,
			Fields:      args.Fields,
			HTMLLayout:  args.HTMLLayout,
			IsSystem:    false,
		}

		if err := s.templateService.CreateTemplate(ctx, tmpl); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      tmpl.ID.Hex(),
			"message": fmt.Sprintf("Template '%s' created successfully", tmpl.Name),
		}), nil, nil
	})

	// Update template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_template",
		Description: "Update an existing template. Changing the HTML layout will regenerate all content using this template.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateTemplateInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		// Get existing template
		tmpl, err := s.templateService.GetTemplate(ctx, id)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Update fields if provided
		if args.Name != "" {
			tmpl.Name = args.Name
		}
		if args.Slug != "" {
			tmpl.Slug = args.Slug
		}
		if args.Description != "" {
			tmpl.Description = args.Description
		}
		if args.Category != "" {
			tmpl.Category = args.Category
		}
		if args.Fields != nil {
			tmpl.Fields = args.Fields
		}
		if args.HTMLLayout != "" {
			tmpl.HTMLLayout = args.HTMLLayout
		}

		if err := s.templateService.UpdateTemplate(ctx, tmpl); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      tmpl.ID.Hex(),
			"message": fmt.Sprintf("Template '%s' updated successfully", tmpl.Name),
		}), nil, nil
	})

	// Delete template
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_template",
		Description: "Delete a template. Cannot delete system templates or templates that have content using them.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args TemplateIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.templateService.DeleteTemplate(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Template %s deleted successfully", args.ID)), nil, nil
	})
}
