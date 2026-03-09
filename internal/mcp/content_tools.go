package mcp

import (
	"context"
	"fmt"

	"lightcms/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	UseHeader       bool                   `json:"use_header,omitempty" jsonschema:"Include site header"`
	UseFooter       bool                   `json:"use_footer,omitempty" jsonschema:"Include site footer"`
	UseTheme        bool                   `json:"use_theme,omitempty" jsonschema:"Apply site theme/layout"`
	RawMode         bool                   `json:"raw_mode,omitempty" jsonschema:"Use raw HTML mode"`
	SetUseHeader    bool                   `json:"set_use_header,omitempty" jsonschema:"Set to true to explicitly update use_header (needed to set it to false)"`
	SetUseFooter    bool                   `json:"set_use_footer,omitempty" jsonschema:"Set to true to explicitly update use_footer (needed to set it to false)"`
	SetUseTheme     bool                   `json:"set_use_theme,omitempty" jsonschema:"Set to true to explicitly update use_theme (needed to set it to false)"`
	SetRawMode      bool                   `json:"set_raw_mode,omitempty" jsonschema:"Set to true to explicitly update raw_mode (needed to set it to false)"`
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

type BatchPublishInput struct {
	IDs              []string `json:"ids,omitempty" jsonschema:"List of content IDs to publish. Mutually exclusive with publish_all_drafts."`
	PublishAllDrafts bool     `json:"publish_all_drafts,omitempty" jsonschema:"If true, publish every unpublished (draft) content item in the site"`
}

type PreviewContentInput struct {
	ID    string                 `json:"id" jsonschema:"Content ID (MongoDB ObjectID),required"`
	Title string                 `json:"title,omitempty" jsonschema:"Override title for the preview (not saved)"`
	Data  map[string]interface{} `json:"data,omitempty" jsonschema:"Override field data for the preview (not saved). Merged on top of existing data."`
}

type UpdateContentByPathInput struct {
	Path           string                 `json:"path" jsonschema:"URL path of the content to update (e.g. /about or /blog/my-post),required"`
	Title          string                 `json:"title,omitempty" jsonschema:"New title"`
	Data           map[string]interface{} `json:"data,omitempty" jsonschema:"Field values to update"`
	Category       string                 `json:"category,omitempty" jsonschema:"Content category"`
	MetaDescription string                `json:"meta_description,omitempty" jsonschema:"SEO meta description"`
	OGImage        string                 `json:"og_image,omitempty" jsonschema:"Open Graph image URL"`
	Published      *bool                  `json:"published,omitempty" jsonschema:"Publish state"`
	VersionComment string                 `json:"version_comment,omitempty" jsonschema:"Version comment"`
}

type ScopedSearchReplaceInput struct {
	Search         string `json:"search" jsonschema:"Text to search for,required"`
	Replace        string `json:"replace" jsonschema:"Replacement text (empty string to delete)"`
	VersionComment string `json:"version_comment,omitempty" jsonschema:"Version comment for updated pages"`
	// Scope filters — leave all empty to match everything (same as global S&R)
	ContentIDs   []string `json:"content_ids,omitempty" jsonschema:"Limit to specific content IDs"`
	FolderPath   string   `json:"folder_path,omitempty" jsonschema:"Limit to pages whose URL starts with this path (e.g. /blog)"`
	TemplateName string   `json:"template_name,omitempty" jsonschema:"Limit to pages using this template name (e.g. 'Concept Page')"`
	Category     string   `json:"category,omitempty" jsonschema:"Limit to pages in this category"`
}

func (s *Server) registerContentTools() {
	// List content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_content",
		Title:       "List Content",
		Description: "List all content items with optional filters. Returns content metadata including title, path, publish status, and timestamps.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "List Content",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListContentInput) (*mcp.CallToolResult, any, error) {
		contents, err := s.client.ListContent(ctx, args.IncludeDeleted, args.Category, args.FolderID)
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
				ID:        c.ID,
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

			if c.Deleted {
				deletedCount++
			} else if c.Published {
				publishedCount++
			} else {
				draftCount++
			}
		}

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
		Name:  "get_content",
		Title: "Get Content",
		Description: `Get a single content item by ID or path. Returns full content including all field data (title, slug, full_path, data fields, published state).

Prefer path when you know the URL: {"path": "/about"}
Use id when you have the MongoDB ObjectID: {"id": "abc123"}

Tip: to verify what a page looks like before publishing, use preview_content instead.`,
		Annotations: &mcp.ToolAnnotations{
			Title:         "Get Content",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetContentInput) (*mcp.CallToolResult, any, error) {
		var content *apiclient.Content
		var err error

		if args.ID != "" {
			content, err = s.client.GetContent(ctx, args.ID)
		} else if args.Path != "" {
			content, err = s.client.GetContentByPath(ctx, args.Path)
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
		Name:  "create_content",
		Title: "Create Content",
		Description: `Create a new content item. Requires a template_id, title, slug, and the data fields defined by the template.

Workflow:
1. Call list_templates to find the right template and its field names.
2. Create the content with data matching those fields.
3. Call publish_content to make it live (or set published=true here to do both in one step).

Set use_header=true, use_footer=true, use_theme=true for pages that should use the site layout.
Always include version_comment to make history readable.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateContentInput) (*mcp.CallToolResult, any, error) {
		createReq := apiclient.CreateContentRequest{
			TemplateID:      args.TemplateID,
			Title:           args.Title,
			Slug:            args.Slug,
			FolderPath:      args.FolderPath,
			Category:        args.Category,
			MetaDescription: args.MetaDescription,
			OGImage:         args.OGImage,
			Data:            args.Data,
			Published:       args.Published,
			UseHeader:       args.UseHeader,
			UseFooter:       args.UseFooter,
			UseTheme:        args.UseTheme,
			RawMode:         args.RawMode,
			VersionComment:  args.VersionComment,
		}

		content, err := s.client.CreateContent(ctx, createReq)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success":   true,
			"id":        content.ID,
			"full_path": content.FullPath,
			"message":   fmt.Sprintf("Content '%s' created successfully", content.Title),
		}), nil, nil
	})

	// Update content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:  "update_content",
		Title: "Update Content",
		Description: `Update an existing content item by ID. Creates a new version automatically. Only send fields you want to change.

For partial data updates, only the keys you include in "data" are changed — existing keys are preserved (merge semantics).
To update by URL path instead of ID, use update_content_by_path.
Always include version_comment so the version history is useful.

Example: {"id": "abc123", "data": {"body": "<p>Updated text</p>"}, "version_comment": "Revised intro paragraph"}`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Content",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateContentInput) (*mcp.CallToolResult, any, error) {
		updates := make(apiclient.UpdateContentRequest)

		if args.TemplateID != "" {
			updates["template_id"] = args.TemplateID
		}
		if args.Title != "" {
			updates["title"] = args.Title
		}
		if args.Slug != "" {
			updates["slug"] = args.Slug
		}
		if args.FolderPath != "" {
			updates["folder_path"] = args.FolderPath
		}
		if args.Category != "" {
			updates["category"] = args.Category
		}
		if args.MetaDescription != "" {
			updates["meta_description"] = args.MetaDescription
		}
		if args.OGImage != "" {
			updates["og_image"] = args.OGImage
		}
		if args.UseHeader || args.SetUseHeader {
			updates["use_header"] = args.UseHeader
		}
		if args.UseFooter || args.SetUseFooter {
			updates["use_footer"] = args.UseFooter
		}
		if args.UseTheme || args.SetUseTheme {
			updates["use_theme"] = args.UseTheme
		}
		if args.RawMode || args.SetRawMode {
			updates["raw_mode"] = args.RawMode
		}
		if args.VersionComment != "" {
			updates["version_comment"] = args.VersionComment
		}

		// For data fields, merge with existing content data
		if args.Data != nil {
			existing, err := s.client.GetContent(ctx, args.ID)
			if err != nil {
				return errorResult(err), nil, nil
			}
			mergedData := existing.Data
			if mergedData == nil {
				mergedData = make(map[string]interface{})
			}
			for k, v := range args.Data {
				mergedData[k] = v
			}
			updates["data"] = mergedData
		}

		content, err := s.client.UpdateContent(ctx, args.ID, updates)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success":   true,
			"id":        content.ID,
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
		if err := s.client.PublishContent(ctx, args.ID); err != nil {
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
		if err := s.client.UnpublishContent(ctx, args.ID); err != nil {
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
		if err := s.client.DeleteContent(ctx, args.ID); err != nil {
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
		if err := s.client.RestoreContent(ctx, args.ID); err != nil {
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
			Title:         "Get Content Versions",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetVersionsInput) (*mcp.CallToolResult, any, error) {
		versions, err := s.client.GetContentVersions(ctx, args.ContentID)
		if err != nil {
			return errorResult(err), nil, nil
		}

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
			Title:         "Get Content Version",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetVersionInput) (*mcp.CallToolResult, any, error) {
		version, err := s.client.GetContentVersion(ctx, args.ContentID, args.Version)
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
		if err := s.client.RevertContentVersion(ctx, args.ContentID, args.Version, args.VersionComment); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Content reverted to version %d successfully", args.Version)), nil, nil
	})

	// Batch publish
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "publish_multiple",
		Title: "Publish Multiple",
		Description: `Publish multiple content items in a single call. Use this instead of calling publish_content in a loop.

Examples:
- Publish specific pages: {"ids": ["abc123", "def456"]}
- Publish all drafts at once: {"publish_all_drafts": true}

Returns a list of published IDs and any failures.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Publish Multiple",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BatchPublishInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.BatchPublishContent(ctx, args.IDs, args.PublishAllDrafts)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// Preview content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "preview_content",
		Title: "Preview Content",
		Description: `Render a content item's HTML without saving or publishing. Use this to verify what a page will look like before publishing.

Also accepts optional title/data overrides to preview unsaved edits:
{"id": "abc123", "data": {"body": "<p>New text</p>"}}

Returns rendered_html and any warnings (missing required fields, unclosed tags, unresolved placeholders).`,
		Annotations: &mcp.ToolAnnotations{
			Title:         "Preview Content",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args PreviewContentInput) (*mcp.CallToolResult, any, error) {
		overrides := map[string]interface{}{}
		if args.Title != "" {
			overrides["title"] = args.Title
		}
		if args.Data != nil {
			overrides["data"] = args.Data
		}
		result, err := s.client.PreviewContent(ctx, args.ID, overrides)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// Update content by path
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "update_content_by_path",
		Title: "Update Content by Path",
		Description: `Update content identified by its URL path instead of its ID. Useful when you know the page URL but not the MongoDB ID.

Example: {"path": "/about", "title": "About Us", "data": {"body": "<p>Updated content</p>"}}

Only the fields you provide are changed. Always include a version_comment describing what changed.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Content by Path",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateContentByPathInput) (*mcp.CallToolResult, any, error) {
		updates := map[string]interface{}{}
		if args.Title != "" {
			updates["title"] = args.Title
		}
		if args.Data != nil {
			// Merge data on top of existing content
			existing, err := s.client.GetContentByPath(ctx, args.Path)
			if err != nil {
				return errorResult(err), nil, nil
			}
			mergedData := existing.Data
			if mergedData == nil {
				mergedData = make(map[string]interface{})
			}
			for k, v := range args.Data {
				mergedData[k] = v
			}
			updates["data"] = mergedData
		}
		if args.Category != "" {
			updates["category"] = args.Category
		}
		if args.MetaDescription != "" {
			updates["meta_description"] = args.MetaDescription
		}
		if args.OGImage != "" {
			updates["og_image"] = args.OGImage
		}
		if args.Published != nil {
			updates["published"] = *args.Published
		}
		if args.VersionComment != "" {
			updates["version_comment"] = args.VersionComment
		}
		content, err := s.client.UpdateContentByPath(ctx, args.Path, updates)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]interface{}{
			"success":   true,
			"id":        content.ID,
			"full_path": content.FullPath,
			"message":   fmt.Sprintf("Content at '%s' updated successfully", args.Path),
		}), nil, nil
	})

	// Scoped search-and-replace preview
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "scoped_search_replace_preview",
		Title: "Scoped Search Replace Preview",
		Description: `Preview a search-and-replace limited to a subset of pages. Safer than site-wide replacement.

Scope options (all optional — leave blank to match all pages):
- content_ids: specific page IDs
- folder_path: pages under /blog, /docs, etc.
- template_name: pages using "Concept Page", "Blog Post", etc.
- category: pages with a matching category

Example: {"search": "old text", "replace": "new text", "folder_path": "/blog"}

Always run preview before execute.`,
		Annotations: &mcp.ToolAnnotations{
			Title:         "Scoped Search Replace Preview",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ScopedSearchReplaceInput) (*mcp.CallToolResult, any, error) {
		scope := apiclient.ScopedSearchReplaceScope{
			ContentIDs:   args.ContentIDs,
			FolderPath:   args.FolderPath,
			TemplateName: args.TemplateName,
			Category:     args.Category,
		}
		result, err := s.client.ScopedSearchReplacePreview(ctx, args.Search, args.Replace, scope)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// Scoped search-and-replace execute
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "scoped_search_replace_execute",
		Title: "Scoped Search Replace Execute",
		Description: `Execute a search-and-replace limited to a subset of pages. ALWAYS run scoped_search_replace_preview first and show results to the user before executing.

Scope options (all optional):
- content_ids, folder_path, template_name, category

Example: {"search": "old text", "replace": "new text", "folder_path": "/blog", "version_comment": "Updated old references"}`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scoped Search Replace Execute",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ScopedSearchReplaceInput) (*mcp.CallToolResult, any, error) {
		scope := apiclient.ScopedSearchReplaceScope{
			ContentIDs:   args.ContentIDs,
			FolderPath:   args.FolderPath,
			TemplateName: args.TemplateName,
			Category:     args.Category,
		}
		result, err := s.client.ScopedSearchReplaceExecute(ctx, args.Search, args.Replace, args.VersionComment, scope)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})
}
