package mcp

import (
	"context"
	"fmt"

	"lightcms/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Content tool input types
type ListContentInput struct {
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"Include soft-deleted content in results"`
	Category       string `json:"category,omitempty" jsonschema:"Filter by content category"`
	FolderID       string `json:"folder_id,omitempty" jsonschema:"Filter by folder ID (MongoDB ObjectID)"`
}

type GetContentInput struct {
	ID   string `json:"id,omitempty" jsonschema:"Content ID (MongoDB ObjectID)"`
	Path string `json:"path,omitempty" jsonschema:"Content path (e.g., /about or /blog/my-post)"`
}

type CreateContentInput struct {
	TemplateID      string                 `json:"template_id" jsonschema:"Template ID (MongoDB ObjectID),required"`
	Title           string                 `json:"title" jsonschema:"Content title,required"`
	Slug            string                 `json:"slug" jsonschema:"URL slug for the content,required"`
	FolderPath      string                 `json:"folder_path,omitempty" jsonschema:"Folder path (e.g., /blog)"`
	Category        string                 `json:"category,omitempty" jsonschema:"Content category for collections"`
	MetaDescription string                 `json:"meta_description,omitempty" jsonschema:"SEO meta description"`
	OGImage         string                 `json:"og_image,omitempty" jsonschema:"Open Graph image URL"`
	Data            map[string]interface{} `json:"data" jsonschema:"Template field values,required"`
	UseHeader       bool                   `json:"use_header,omitempty" jsonschema:"Include site header"`
	UseFooter       bool                   `json:"use_footer,omitempty" jsonschema:"Include site footer"`
	UseTheme        bool                   `json:"use_theme,omitempty" jsonschema:"Apply site theme/layout"`
	RawMode         bool                   `json:"raw_mode,omitempty" jsonschema:"Use raw HTML mode"`
	Published       bool                   `json:"published,omitempty" jsonschema:"Publish immediately"`
	VersionComment  string                 `json:"version_comment,omitempty" jsonschema:"Optional comment describing this version"`
}

type UpdateContentInput struct {
	ID              string                 `json:"id" jsonschema:"Content ID (MongoDB ObjectID),required"`
	TemplateID      string                 `json:"template_id,omitempty" jsonschema:"Template ID (MongoDB ObjectID)"`
	Title           string                 `json:"title,omitempty" jsonschema:"Content title"`
	Slug            string                 `json:"slug,omitempty" jsonschema:"URL slug"`
	FolderPath      string                 `json:"folder_path,omitempty" jsonschema:"Folder path"`
	Category        string                 `json:"category,omitempty" jsonschema:"Content category"`
	MetaDescription string                 `json:"meta_description,omitempty" jsonschema:"SEO meta description"`
	OGImage         string                 `json:"og_image,omitempty" jsonschema:"Open Graph image URL"`
	Data            map[string]interface{} `json:"data,omitempty" jsonschema:"Template field values"`
	UseHeader       *bool                  `json:"use_header,omitempty" jsonschema:"Include site header"`
	UseFooter       *bool                  `json:"use_footer,omitempty" jsonschema:"Include site footer"`
	UseTheme        *bool                  `json:"use_theme,omitempty" jsonschema:"Apply site theme/layout"`
	RawMode         *bool                  `json:"raw_mode,omitempty" jsonschema:"Use raw HTML mode"`
	VersionComment  string                 `json:"version_comment,omitempty" jsonschema:"Optional comment describing this version change"`
}

type ContentIDInput struct {
	ID string `json:"id" jsonschema:"Content ID (MongoDB ObjectID),required"`
}

type GetVersionsInput struct {
	ContentID string `json:"content_id" jsonschema:"Content ID (MongoDB ObjectID),required"`
}

type GetVersionInput struct {
	ContentID string `json:"content_id" jsonschema:"Content ID (MongoDB ObjectID),required"`
	Version   int    `json:"version" jsonschema:"Version number,required"`
}

type RevertToVersionInput struct {
	ContentID      string `json:"content_id" jsonschema:"Content ID (MongoDB ObjectID),required"`
	Version        int    `json:"version" jsonschema:"Version number to revert to,required"`
	VersionComment string `json:"version_comment,omitempty" jsonschema:"Optional comment for the revert (e.g., 'Reverted to v3')"`
}

func (s *Server) registerContentTools() {
	// List content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_content",
		Title:       "List Content",
		Description: "List all content items with optional filters. Returns content metadata including title, path, publish status, and timestamps.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Content",
			ReadOnlyHint: true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListContentInput) (*mcp.CallToolResult, any, error) {
		var folderID *primitive.ObjectID
		if args.FolderID != "" {
			id, err := primitive.ObjectIDFromHex(args.FolderID)
			if err != nil {
				return errorResult(fmt.Errorf("invalid folder_id: %w", err)), nil, nil
			}
			folderID = &id
		}

		contents, err := s.contentService.ListContent(ctx, args.IncludeDeleted, args.Category, folderID)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Return summary for each content
		type ContentSummary struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Slug        string `json:"slug"`
			FullPath    string `json:"full_path"`
			Category    string `json:"category"`
			Published   bool   `json:"published"`
			Deleted     bool   `json:"deleted"`
			UpdatedAt   string `json:"updated_at"`
			PublishedAt string `json:"published_at,omitempty"`
		}

		summaries := make([]ContentSummary, len(contents))
		publishedCount := 0
		deletedCount := 0
		draftCount := 0
		for i, c := range contents {
			summary := ContentSummary{
				ID:        c.ID.Hex(),
				Title:     c.Title,
				Slug:      c.Slug,
				FullPath:  c.FullPath,
				Category:  c.Category,
				Published: c.Published,
				Deleted:   c.Deleted,
				UpdatedAt: c.UpdatedAt.Format("2006-01-02 15:04:05"),
			}
			if c.PublishedAt != nil {
				summary.PublishedAt = c.PublishedAt.Format("2006-01-02 15:04:05")
			}
			summaries[i] = summary

			// Count stats
			if c.Deleted {
				deletedCount++
			} else if c.Published {
				publishedCount++
			} else {
				draftCount++
			}
		}

		// Return response with summary stats
		type ListContentResponse struct {
			Total           int              `json:"total"`
			Published       int              `json:"published"`
			Drafts          int              `json:"drafts"`
			Deleted         int              `json:"deleted"`
			IncludesDeleted bool             `json:"includes_deleted"`
			Items           []ContentSummary `json:"items"`
		}

		response := ListContentResponse{
			Total:           len(contents),
			Published:       publishedCount,
			Drafts:          draftCount,
			Deleted:         deletedCount,
			IncludesDeleted: args.IncludeDeleted,
			Items:           summaries,
		}

		return jsonResult(response), nil, nil
	})

	// Get content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_content",
		Title:       "Get Content",
		Description: "Get a single content item by ID or path. Returns full content including all field data.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Content",
			ReadOnlyHint: true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetContentInput) (*mcp.CallToolResult, any, error) {
		var content *models.Content
		var err error

		if args.ID != "" {
			id, err := primitive.ObjectIDFromHex(args.ID)
			if err != nil {
				return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
			}
			content, err = s.contentService.GetContent(ctx, id)
		} else if args.Path != "" {
			content, err = s.contentService.GetContentByPath(ctx, args.Path)
		} else {
			return errorResult(fmt.Errorf("either id or path is required")), nil, nil
		}

		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(content), nil, nil
	})

	// Create content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_content",
		Title:       "Create Content",
		Description: "Create a new content item. Requires a template ID and field data. Creates initial version automatically.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateContentInput) (*mcp.CallToolResult, any, error) {
		templateID, err := primitive.ObjectIDFromHex(args.TemplateID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid template_id: %w", err)), nil, nil
		}

		// Get template name
		tmpl, err := s.templateService.GetTemplate(ctx, templateID)
		if err != nil {
			return errorResult(fmt.Errorf("template not found: %w", err)), nil, nil
		}

		content := &models.Content{
			TemplateID:      templateID,
			TemplateName:    tmpl.Name,
			Title:           args.Title,
			Slug:            args.Slug,
			FolderPath:      args.FolderPath,
			Category:        args.Category,
			MetaDescription: args.MetaDescription,
			OGImage:         args.OGImage,
			Data:            args.Data,
			UseHeader:       args.UseHeader,
			UseFooter:       args.UseFooter,
			UseTheme:        args.UseTheme,
			RawMode:         args.RawMode,
			Published:       args.Published,
		}

		if err := s.contentService.CreateContent(ctx, content, args.VersionComment); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success":   true,
			"id":        content.ID.Hex(),
			"full_path": content.FullPath,
			"message":   fmt.Sprintf("Content '%s' created successfully", content.Title),
		}), nil, nil
	})

	// Update content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_content",
		Title:       "Update Content",
		Description: "Update an existing content item. Creates a new version automatically. Only include fields you want to change.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateContentInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		// Get existing content
		content, err := s.contentService.GetContent(ctx, id)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Update fields if provided
		if args.TemplateID != "" {
			templateID, err := primitive.ObjectIDFromHex(args.TemplateID)
			if err != nil {
				return errorResult(fmt.Errorf("invalid template_id: %w", err)), nil, nil
			}
			tmpl, err := s.templateService.GetTemplate(ctx, templateID)
			if err != nil {
				return errorResult(fmt.Errorf("template not found: %w", err)), nil, nil
			}
			content.TemplateID = templateID
			content.TemplateName = tmpl.Name
		}
		if args.Title != "" {
			content.Title = args.Title
		}
		if args.Slug != "" {
			content.Slug = args.Slug
		}
		if args.FolderPath != "" {
			content.FolderPath = args.FolderPath
		}
		if args.Category != "" {
			content.Category = args.Category
		}
		if args.MetaDescription != "" {
			content.MetaDescription = args.MetaDescription
		}
		if args.OGImage != "" {
			content.OGImage = args.OGImage
		}
		if args.Data != nil {
			// Merge data fields
			for k, v := range args.Data {
				content.Data[k] = v
			}
		}
		if args.UseHeader != nil {
			content.UseHeader = *args.UseHeader
		}
		if args.UseFooter != nil {
			content.UseFooter = *args.UseFooter
		}
		if args.UseTheme != nil {
			content.UseTheme = *args.UseTheme
		}
		if args.RawMode != nil {
			content.RawMode = *args.RawMode
		}

		if err := s.contentService.UpdateContent(ctx, content, args.VersionComment); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success":   true,
			"id":        content.ID.Hex(),
			"full_path": content.FullPath,
			"message":   fmt.Sprintf("Content '%s' updated successfully", content.Title),
		}), nil, nil
	})

	// Publish content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "publish_content",
		Title:       "Publish Content",
		Description: "Publish a content item, making it visible on the public site. Generates the static HTML page.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Publish Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ContentIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.contentService.PublishContent(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Content %s published successfully", args.ID)), nil, nil
	})

	// Unpublish content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "unpublish_content",
		Title:       "Unpublish Content",
		Description: "Unpublish a content item, removing it from the public site. Removes the static HTML page.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Unpublish Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ContentIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.contentService.UnpublishContent(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Content %s unpublished successfully", args.ID)), nil, nil
	})

	// Delete content (soft delete)
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_content",
		Title:       "Delete Content",
		Description: "Soft-delete a content item. The content can be restored later. Removes the static HTML page.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ContentIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.contentService.DeleteContent(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Content %s deleted successfully", args.ID)), nil, nil
	})

	// Restore content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "restore_content",
		Title:       "Restore Content",
		Description: "Restore a soft-deleted content item. Regenerates static page if content was published.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Restore Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ContentIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.contentService.RestoreContent(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Content %s restored successfully", args.ID)), nil, nil
	})

	// Get versions
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_content_versions",
		Title:       "Get Content Versions",
		Description: "Get the version history for a content item. Returns list of versions with timestamps.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Content Versions",
			ReadOnlyHint: true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetVersionsInput) (*mcp.CallToolResult, any, error) {
		contentID, err := primitive.ObjectIDFromHex(args.ContentID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid content_id: %w", err)), nil, nil
		}

		versions, err := s.contentService.GetVersions(ctx, contentID)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Return summary for each version
		type VersionSummary struct {
			Version   int    `json:"version"`
			Title     string `json:"title"`
			Slug      string `json:"slug"`
			FullPath  string `json:"full_path"`
			Published bool   `json:"published"`
			Comment   string `json:"comment,omitempty"`
			CreatedAt string `json:"created_at"`
		}

		summaries := make([]VersionSummary, len(versions))
		for i, v := range versions {
			summaries[i] = VersionSummary{
				Version:   v.Version,
				Title:     v.Title,
				Slug:      v.Slug,
				FullPath:  v.FullPath,
				Published: v.Published,
				Comment:   v.Comment,
				CreatedAt: v.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}

		return jsonResult(summaries), nil, nil
	})

	// Get specific version
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_content_version",
		Title:       "Get Content Version",
		Description: "Get a specific version of a content item with full field data.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Content Version",
			ReadOnlyHint: true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetVersionInput) (*mcp.CallToolResult, any, error) {
		contentID, err := primitive.ObjectIDFromHex(args.ContentID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid content_id: %w", err)), nil, nil
		}

		version, err := s.contentService.GetVersion(ctx, contentID, args.Version)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(version), nil, nil
	})

	// Revert to version
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "revert_to_version",
		Title:       "Revert to Version",
		Description: "Revert content to a previous version. Creates a new version with the old data.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Revert to Version",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args RevertToVersionInput) (*mcp.CallToolResult, any, error) {
		contentID, err := primitive.ObjectIDFromHex(args.ContentID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid content_id: %w", err)), nil, nil
		}

		if err := s.contentService.RevertToVersion(ctx, contentID, args.Version, args.VersionComment); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Content reverted to version %d successfully", args.Version)), nil, nil
	})
}
