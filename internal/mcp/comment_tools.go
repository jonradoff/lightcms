package mcp

import (
	"context"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type ListCommentsInput struct {
	ContentID string `json:"content_id" jsonschema:"required,Content ID to list comments for"`
}

type CreateCommentInput struct {
	ContentID string   `json:"content_id" jsonschema:"required,Content ID to post a comment on"`
	Text      string   `json:"text" jsonschema:"required,Comment text"`
	Mentions  []string `json:"mentions,omitempty" jsonschema:"Optional list of user ID strings to mention"`
}

type DeleteCommentInput struct {
	ContentID string `json:"content_id" jsonschema:"required,Content ID the comment belongs to"`
	CommentID string `json:"comment_id" jsonschema:"required,Comment ID to delete"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerCommentTools() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_comments",
		Title:       "List Comments",
		Description: "List all discussion comments on a content item.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Comments",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListCommentsInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.ListComments(ctx, args.ContentID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_comment",
		Title:       "Post Comment",
		Description: "Post a discussion comment on a content item. Requires discussion.post permission.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Post Comment",
			DestructiveHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateCommentInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.CreateComment(ctx, args.ContentID, apiclient.CreateCommentRequest{
			Text:     args.Text,
			Mentions: args.Mentions,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_comment",
		Title:       "Delete Comment",
		Description: "Delete a discussion comment. Requires comment.delete permission (admin only).",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Comment",
			DestructiveHint: boolPtr(true),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DeleteCommentInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.DeleteComment(ctx, args.ContentID, args.CommentID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("Comment deleted successfully"), nil, nil
	})
}
