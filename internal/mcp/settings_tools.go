package mcp

import (
	"context"
	"fmt"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Theme settings input types
type UpdateThemeInput struct {
	PrimaryColor    string `json:"primary_color,omitempty" jsonschema:"Primary theme color (hex)"`
	SecondaryColor  string `json:"secondary_color,omitempty" jsonschema:"Secondary theme color (hex)"`
	AccentColor     string `json:"accent_color,omitempty" jsonschema:"Accent theme color (hex)"`
	BackgroundColor string `json:"background_color,omitempty" jsonschema:"Background color (hex)"`
	TextColor       string `json:"text_color,omitempty" jsonschema:"Text color (hex)"`
	FontFamily      string `json:"font_family,omitempty" jsonschema:"Body font family CSS value"`
	HeadingFont     string `json:"heading_font,omitempty" jsonschema:"Heading font family CSS value"`
	BorderRadius    string `json:"border_radius,omitempty" jsonschema:"Border radius CSS value"`
	CustomCSS       string `json:"custom_css,omitempty" jsonschema:"Additional custom CSS"`
	SiteName        string `json:"site_name,omitempty" jsonschema:"Site name"`
	SiteTagline     string `json:"site_tagline,omitempty" jsonschema:"Site tagline"`
	LogoURL         string `json:"logo_url,omitempty" jsonschema:"Logo image URL"`
	HeadHTML        string `json:"head_html,omitempty" jsonschema:"Custom HTML for <head> section"`
	HeaderHTML      string `json:"header_html,omitempty" jsonschema:"Custom header HTML (changing regenerates all content)"`
	FooterHTML      string `json:"footer_html,omitempty" jsonschema:"Custom footer HTML (changing regenerates all content)"`
}

// Site config input types
type UpdateSiteConfigInput struct {
	TitleTemplate        string `json:"title_template,omitempty" jsonschema:"Page title template with {{title}} and {{site_name}} placeholders"`
	TitleTemplateNoTitle string `json:"title_template_no_title,omitempty" jsonschema:"Title template when page has no title"`
}

// Redirect input types
type CreateRedirectInput struct {
	FromPath    string `json:"from_path" jsonschema:"Source path (e.g., /old-page),required"`
	ToPath      string `json:"to_path" jsonschema:"Destination path or URL (e.g., /new-page),required"`
	StatusCode  int    `json:"status_code,omitempty" jsonschema:"301 (permanent) or 302 (temporary), defaults to 301"`
	Description string `json:"description,omitempty" jsonschema:"Optional description/note"`
}

type UpdateRedirectInput struct {
	ID          string `json:"id" jsonschema:"Redirect ID (MongoDB ObjectID),required"`
	FromPath    string `json:"from_path,omitempty" jsonschema:"Source path"`
	ToPath      string `json:"to_path,omitempty" jsonschema:"Destination path or URL"`
	StatusCode  int    `json:"status_code,omitempty" jsonschema:"301 or 302"`
	Description string `json:"description,omitempty" jsonschema:"Optional description"`
}

type RedirectIDInput struct {
	ID string `json:"id" jsonschema:"Redirect ID (MongoDB ObjectID),required"`
}

// Folder input types
type CreateFolderInput struct {
	Name     string `json:"name" jsonschema:"Display name for the folder,required"`
	Slug     string `json:"slug" jsonschema:"URL segment for the folder,required"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"Parent folder ID for nested folders"`
}

type FolderIDInput struct {
	ID string `json:"id" jsonschema:"Folder ID (MongoDB ObjectID),required"`
}

// Collection input types
type CreateCollectionInput struct {
	Name         string `json:"name" jsonschema:"Collection name,required"`
	Slug         string `json:"slug" jsonschema:"Collection URL slug,required"`
	Description  string `json:"description,omitempty" jsonschema:"Collection description"`
	Category     string `json:"category,omitempty" jsonschema:"Content category to include"`
	SortField    string `json:"sort_field,omitempty" jsonschema:"Field to sort by"`
	SortOrder    string `json:"sort_order,omitempty" jsonschema:"Sort order: asc or desc"`
	ItemTemplate string `json:"item_template,omitempty" jsonschema:"HTML template for each item"`
	PageTemplate string `json:"page_template,omitempty" jsonschema:"HTML template for collection page"`
	ItemsPerPage int    `json:"items_per_page,omitempty" jsonschema:"Items per page for pagination"`
}

type UpdateCollectionInput struct {
	ID           string `json:"id" jsonschema:"Collection ID (MongoDB ObjectID),required"`
	Name         string `json:"name,omitempty" jsonschema:"Collection name"`
	Slug         string `json:"slug,omitempty" jsonschema:"Collection URL slug"`
	Description  string `json:"description,omitempty" jsonschema:"Collection description"`
	Category     string `json:"category,omitempty" jsonschema:"Content category to include"`
	SortField    string `json:"sort_field,omitempty" jsonschema:"Field to sort by"`
	SortOrder    string `json:"sort_order,omitempty" jsonschema:"Sort order: asc or desc"`
	ItemTemplate string `json:"item_template,omitempty" jsonschema:"HTML template for each item"`
	PageTemplate string `json:"page_template,omitempty" jsonschema:"HTML template for collection page"`
	ItemsPerPage int    `json:"items_per_page,omitempty" jsonschema:"Items per page for pagination"`
}

type CollectionIDInput struct {
	ID string `json:"id" jsonschema:"Collection ID (MongoDB ObjectID),required"`
}

func (s *Server) registerSettingsTools() {
	// === Theme Settings ===

	// Get theme
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_theme",
		Description: "Get current theme settings including colors, fonts, and custom HTML for header/footer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		theme, err := s.settingsService.GetTheme(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(theme), nil, nil
	})

	// Update theme
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_theme",
		Description: "Update theme settings. Changing header or footer HTML will regenerate all published content.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateThemeInput) (*mcp.CallToolResult, any, error) {
		theme, err := s.settingsService.GetTheme(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Update fields if provided
		if args.PrimaryColor != "" {
			theme.PrimaryColor = args.PrimaryColor
		}
		if args.SecondaryColor != "" {
			theme.SecondaryColor = args.SecondaryColor
		}
		if args.AccentColor != "" {
			theme.AccentColor = args.AccentColor
		}
		if args.BackgroundColor != "" {
			theme.BackgroundColor = args.BackgroundColor
		}
		if args.TextColor != "" {
			theme.TextColor = args.TextColor
		}
		if args.FontFamily != "" {
			theme.FontFamily = args.FontFamily
		}
		if args.HeadingFont != "" {
			theme.HeadingFont = args.HeadingFont
		}
		if args.BorderRadius != "" {
			theme.BorderRadius = args.BorderRadius
		}
		if args.CustomCSS != "" {
			theme.CustomCSS = args.CustomCSS
		}
		if args.SiteName != "" {
			theme.SiteName = args.SiteName
		}
		if args.SiteTagline != "" {
			theme.SiteTagline = args.SiteTagline
		}
		if args.LogoURL != "" {
			theme.LogoURL = args.LogoURL
		}
		if args.HeadHTML != "" {
			theme.HeadHTML = args.HeadHTML
		}
		if args.HeaderHTML != "" {
			theme.HeaderHTML = args.HeaderHTML
		}
		if args.FooterHTML != "" {
			theme.FooterHTML = args.FooterHTML
		}

		if err := s.settingsService.UpdateTheme(ctx, theme); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Theme updated successfully"), nil, nil
	})

	// === Site Config ===

	// Get site config
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_site_config",
		Description: "Get site configuration including title templates.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		config, err := s.settingsService.GetSiteConfig(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(config), nil, nil
	})

	// Update site config
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_site_config",
		Description: "Update site configuration. Title templates support {{title}} and {{site_name}} placeholders.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateSiteConfigInput) (*mcp.CallToolResult, any, error) {
		config, err := s.settingsService.GetSiteConfig(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}

		if args.TitleTemplate != "" {
			config.TitleTemplate = args.TitleTemplate
		}
		if args.TitleTemplateNoTitle != "" {
			config.TitleTemplateNoTitle = args.TitleTemplateNoTitle
		}

		if err := s.settingsService.UpdateSiteConfig(ctx, config); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Site config updated successfully"), nil, nil
	})

	// === Redirects ===

	// List redirects
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_redirects",
		Description: "List all URL redirects configured for the site.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		redirects, err := s.settingsService.ListRedirects(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(redirects), nil, nil
	})

	// Create redirect
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_redirect",
		Description: "Create a new URL redirect. Use 301 for permanent redirects, 302 for temporary.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateRedirectInput) (*mcp.CallToolResult, any, error) {
		redirect := &models.Redirect{
			FromPath:    args.FromPath,
			ToPath:      args.ToPath,
			StatusCode:  args.StatusCode,
			Description: args.Description,
		}

		if err := s.settingsService.CreateRedirect(ctx, redirect); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      redirect.ID.Hex(),
			"message": fmt.Sprintf("Redirect from %s to %s created", args.FromPath, args.ToPath),
		}), nil, nil
	})

	// Update redirect
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_redirect",
		Description: "Update an existing redirect.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateRedirectInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		redirect, err := s.settingsService.GetRedirect(ctx, id)
		if err != nil {
			return errorResult(err), nil, nil
		}

		if args.FromPath != "" {
			redirect.FromPath = args.FromPath
		}
		if args.ToPath != "" {
			redirect.ToPath = args.ToPath
		}
		if args.StatusCode != 0 {
			redirect.StatusCode = args.StatusCode
		}
		if args.Description != "" {
			redirect.Description = args.Description
		}

		if err := s.settingsService.UpdateRedirect(ctx, redirect); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Redirect updated successfully"), nil, nil
	})

	// Delete redirect
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_redirect",
		Description: "Delete a redirect.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args RedirectIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.settingsService.DeleteRedirect(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Redirect deleted successfully"), nil, nil
	})

	// === Folders ===

	// List folders
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_folders",
		Description: "List all content folders. Folders organize content into URL path segments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		folders, err := s.settingsService.ListFolders(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(folders), nil, nil
	})

	// Create folder
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_folder",
		Description: "Create a new content folder. Folders create URL path segments for organizing content.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateFolderInput) (*mcp.CallToolResult, any, error) {
		folder := &models.Folder{
			Name: args.Name,
			Slug: args.Slug,
		}

		if args.ParentID != "" {
			parentID, err := primitive.ObjectIDFromHex(args.ParentID)
			if err != nil {
				return errorResult(fmt.Errorf("invalid parent_id: %w", err)), nil, nil
			}
			folder.ParentID = &parentID
		}

		if err := s.settingsService.CreateFolder(ctx, folder); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      folder.ID.Hex(),
			"path":    folder.Path,
			"message": fmt.Sprintf("Folder '%s' created at %s", folder.Name, folder.Path),
		}), nil, nil
	})

	// Get folder
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_folder",
		Description: "Get a folder by ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args FolderIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		folder, err := s.settingsService.GetFolder(ctx, id)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(folder), nil, nil
	})

	// Delete folder
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_folder",
		Description: "Delete an empty folder. Cannot delete folders that contain content or subfolders.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args FolderIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.settingsService.DeleteFolder(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Folder deleted successfully"), nil, nil
	})

	// === Collections ===

	// List collections
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_collections",
		Description: "List all content collections. Collections group and display content by category.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		collections, err := s.settingsService.ListCollections(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(collections), nil, nil
	})

	// Create collection
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_collection",
		Description: "Create a new content collection. Collections display grouped content with custom templates.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateCollectionInput) (*mcp.CallToolResult, any, error) {
		collection := &models.Collection{
			Name:         args.Name,
			Slug:         args.Slug,
			Description:  args.Description,
			Category:     args.Category,
			SortField:    args.SortField,
			SortOrder:    args.SortOrder,
			ItemTemplate: args.ItemTemplate,
			PageTemplate: args.PageTemplate,
			ItemsPerPage: args.ItemsPerPage,
		}

		if err := s.settingsService.CreateCollection(ctx, collection); err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success": true,
			"id":      collection.ID.Hex(),
			"message": fmt.Sprintf("Collection '%s' created", collection.Name),
		}), nil, nil
	})

	// Get collection
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_collection",
		Description: "Get a collection by ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CollectionIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		collection, err := s.settingsService.GetCollection(ctx, id)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(collection), nil, nil
	})

	// Update collection
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_collection",
		Description: "Update a collection's settings.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateCollectionInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		collection, err := s.settingsService.GetCollection(ctx, id)
		if err != nil {
			return errorResult(err), nil, nil
		}

		if args.Name != "" {
			collection.Name = args.Name
		}
		if args.Slug != "" {
			collection.Slug = args.Slug
		}
		if args.Description != "" {
			collection.Description = args.Description
		}
		if args.Category != "" {
			collection.Category = args.Category
		}
		if args.SortField != "" {
			collection.SortField = args.SortField
		}
		if args.SortOrder != "" {
			collection.SortOrder = args.SortOrder
		}
		if args.ItemTemplate != "" {
			collection.ItemTemplate = args.ItemTemplate
		}
		if args.PageTemplate != "" {
			collection.PageTemplate = args.PageTemplate
		}
		if args.ItemsPerPage != 0 {
			collection.ItemsPerPage = args.ItemsPerPage
		}

		if err := s.settingsService.UpdateCollection(ctx, collection); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Collection updated successfully"), nil, nil
	})

	// Delete collection
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_collection",
		Description: "Delete a collection. This does not delete the content in the collection.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CollectionIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.settingsService.DeleteCollection(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult("Collection deleted successfully"), nil, nil
	})

	// === Utility ===

	// Regenerate all content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "regenerate_all_content",
		Description: "Regenerate all published static HTML pages. Use after major theme or template changes.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		if err := s.contentService.RegenerateAllContent(ctx); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("All published content regenerated successfully"), nil, nil
	})
}

// Helper to get UpdateThemeInput fields that need special handling
func setThemeFieldIfProvided(theme *database.ThemeSettings, field, value string) {
	if value != "" {
		switch field {
		case "custom_css":
			theme.CustomCSS = value
		}
	}
}
