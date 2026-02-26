package mcp

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Asset tool input types
type ListAssetsInput struct {
	Folder string `json:"folder,omitempty" jsonschema:"Filter by folder path (e.g., /images)"`
}

type GetAssetInput struct {
	ID   string `json:"id,omitempty" jsonschema:"Asset ID (MongoDB ObjectID)"`
	Path string `json:"path,omitempty" jsonschema:"Asset serve path (e.g., /images/logo.png)"`
}

type UploadAssetInput struct {
	Filename    string `json:"filename" jsonschema:"Original filename with extension,required"`
	ServePath   string `json:"serve_path" jsonschema:"URL path where file will be served (e.g., /images/logo.png),required"`
	Description string `json:"description,omitempty" jsonschema:"Optional description of the asset"`
	DataBase64  string `json:"data_base64" jsonschema:"Base64-encoded file content,required"`
}

type AssetIDInput struct {
	ID string `json:"id" jsonschema:"Asset ID (MongoDB ObjectID),required"`
}

func (s *Server) registerAssetTools() {
	// List assets
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_assets",
		Title:       "List Assets",
		Description: "List all assets in the asset library. Assets are files like images, documents, CSS, JS, etc.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "List Assets",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListAssetsInput) (*mcp.CallToolResult, any, error) {
		assets, err := s.assetService.ListAssets(ctx, args.Folder)
		if err != nil {
			return errorResult(err), nil, nil
		}

		// Return summary for each asset
		type AssetSummary struct {
			ID          string `json:"id"`
			Filename    string `json:"filename"`
			Folder      string `json:"folder"`
			ServePath   string `json:"serve_path"`
			MimeType    string `json:"mime_type"`
			Size        int64  `json:"size"`
			Description string `json:"description"`
			CreatedAt   string `json:"created_at"`
		}

		summaries := make([]AssetSummary, len(assets))
		for i, a := range assets {
			summaries[i] = AssetSummary{
				ID:          a.ID.Hex(),
				Filename:    a.Filename,
				Folder:      a.Folder,
				ServePath:   a.ServePath,
				MimeType:    a.MimeType,
				Size:        a.Size,
				Description: a.Description,
				CreatedAt:   a.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}

		return jsonResult(summaries), nil, nil
	})

	// List asset folders
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_asset_folders",
		Title:       "List Asset Folders",
		Description: "List all unique folder paths in the asset library.",
		Annotations: &mcp.ToolAnnotations{
			Title:         "List Asset Folders",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		folders, err := s.assetService.ListFolders(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(folders), nil, nil
	})

	// Get asset
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_asset",
		Title:       "Get Asset",
		Description: "Get asset metadata by ID or path. Does not return file content (use the serve path to access the file).",
		Annotations: &mcp.ToolAnnotations{
			Title:         "Get Asset",
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetAssetInput) (*mcp.CallToolResult, any, error) {
		if args.ID != "" {
			id, err := primitive.ObjectIDFromHex(args.ID)
			if err != nil {
				return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
			}
			asset, err := s.assetService.GetAsset(ctx, id)
			if err != nil {
				return errorResult(err), nil, nil
			}
			return jsonResult(asset), nil, nil
		} else if args.Path != "" {
			asset, err := s.assetService.GetAssetByPath(ctx, args.Path)
			if err != nil {
				return errorResult(err), nil, nil
			}
			if asset == nil {
				return errorResult(fmt.Errorf("asset not found")), nil, nil
			}
			return jsonResult(asset), nil, nil
		}
		return errorResult(fmt.Errorf("either id or path is required")), nil, nil
	})

	// Upload asset
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "upload_asset",
		Title:       "Upload Asset",
		Description: "Upload a new asset to the asset library. Provide file content as base64. Validates file type and MIME type for security.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Upload Asset",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UploadAssetInput) (*mcp.CallToolResult, any, error) {
		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(args.DataBase64)
		if err != nil {
			return errorResult(fmt.Errorf("invalid base64 data: %w", err)), nil, nil
		}

		asset, err := s.assetService.UploadAsset(ctx, data, args.Filename, args.ServePath, args.Description)
		if err != nil {
			return errorResult(err), nil, nil
		}

		return jsonResult(map[string]interface{}{
			"success":    true,
			"id":         asset.ID.Hex(),
			"serve_path": asset.ServePath,
			"mime_type":  asset.MimeType,
			"size":       asset.Size,
			"message":    fmt.Sprintf("Asset '%s' uploaded successfully", args.Filename),
		}), nil, nil
	})

	// Delete asset
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_asset",
		Title:       "Delete Asset",
		Description: "Delete an asset from the library. Removes both the file and database record.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Asset",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args AssetIDInput) (*mcp.CallToolResult, any, error) {
		id, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errorResult(fmt.Errorf("invalid id: %w", err)), nil, nil
		}

		if err := s.assetService.DeleteAsset(ctx, id); err != nil {
			return errorResult(err), nil, nil
		}

		return textResult(fmt.Sprintf("Asset %s deleted successfully", args.ID)), nil, nil
	})
}
