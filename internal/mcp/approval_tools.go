package mcp

import (
	"context"

	"lightcms/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type ListApprovalWorkflowsInput struct{}

type GetApprovalWorkflowInput struct {
	ID string `json:"id" jsonschema:"required,Workflow ID"`
}

type CreateApprovalWorkflowInput struct {
	Name         string                      `json:"name" jsonschema:"required,Workflow name"`
	Description  string                      `json:"description,omitempty" jsonschema:"Optional description"`
	Trigger      string                      `json:"trigger" jsonschema:"required,Trigger type: all_contributor | folder_path | template_id | tag"`
	TriggerValue string                      `json:"trigger_value,omitempty" jsonschema:"Value for the trigger (e.g. folder path or template ID)"`
	Approvers    []apiclient.WorkflowApprover `json:"approvers,omitempty" jsonschema:"Ordered list of approvers"`
	Mode         string                      `json:"mode" jsonschema:"required,sequential or concurrent"`
}

type UpdateApprovalWorkflowInput struct {
	ID           string                      `json:"id" jsonschema:"required,Workflow ID to update"`
	Name         string                      `json:"name" jsonschema:"required,Workflow name"`
	Description  string                      `json:"description,omitempty" jsonschema:"Optional description"`
	Trigger      string                      `json:"trigger" jsonschema:"required,Trigger type: all_contributor | folder_path | template_id | tag"`
	TriggerValue string                      `json:"trigger_value,omitempty" jsonschema:"Value for the trigger"`
	Approvers    []apiclient.WorkflowApprover `json:"approvers,omitempty" jsonschema:"Ordered list of approvers"`
	Mode         string                      `json:"mode" jsonschema:"required,sequential or concurrent"`
}

type DeleteApprovalWorkflowInput struct {
	ID string `json:"id" jsonschema:"required,Workflow ID to delete"`
}

type ListApprovalRequestsInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"mine (my queue) or all (default: all pending)"`
}

type GetApprovalRequestInput struct {
	ID string `json:"id" jsonschema:"required,Approval request ID"`
}

type SubmitForApprovalInput struct {
	ContentID string `json:"content_id" jsonschema:"required,Content ID to submit for approval"`
}

type ApproveRequestInput struct {
	ID      string `json:"id" jsonschema:"required,Approval request ID"`
	Comment string `json:"comment,omitempty" jsonschema:"Optional approval comment"`
}

type RejectRequestInput struct {
	ID      string `json:"id" jsonschema:"required,Approval request ID"`
	Comment string `json:"comment" jsonschema:"required,Rejection reason (required)"`
}

type CancelApprovalRequestInput struct {
	ID string `json:"id" jsonschema:"required,Approval request ID to cancel"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerApprovalTools() {
	// Workflow management
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_approval_workflows",
		Title:       "List Approval Workflows",
		Description: "List all configured approval workflows. Requires approval.manage_workflows permission.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Approval Workflows",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListApprovalWorkflowsInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.ListApprovalWorkflows(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_approval_workflow",
		Title:       "Get Approval Workflow",
		Description: "Get a single approval workflow by ID.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Approval Workflow",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetApprovalWorkflowInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.GetApprovalWorkflow(ctx, args.ID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_approval_workflow",
		Title:       "Create Approval Workflow",
		Description: "Create a new approval workflow. Trigger types: all_contributor, folder_path, template_id, tag. Mode: sequential or concurrent.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create Approval Workflow",
			DestructiveHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateApprovalWorkflowInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.CreateApprovalWorkflow(ctx, apiclient.CreateWorkflowRequest{
			Name:         args.Name,
			Description:  args.Description,
			Trigger:      args.Trigger,
			TriggerValue: args.TriggerValue,
			Approvers:    args.Approvers,
			Mode:         args.Mode,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_approval_workflow",
		Title:       "Update Approval Workflow",
		Description: "Update an existing approval workflow.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Approval Workflow",
			DestructiveHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateApprovalWorkflowInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.UpdateApprovalWorkflow(ctx, args.ID, apiclient.CreateWorkflowRequest{
			Name:         args.Name,
			Description:  args.Description,
			Trigger:      args.Trigger,
			TriggerValue: args.TriggerValue,
			Approvers:    args.Approvers,
			Mode:         args.Mode,
		}); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("Workflow updated successfully"), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_approval_workflow",
		Title:       "Delete Approval Workflow",
		Description: "Delete an approval workflow.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Approval Workflow",
			DestructiveHint: boolPtr(true),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DeleteApprovalWorkflowInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.DeleteApprovalWorkflow(ctx, args.ID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("Workflow deleted successfully"), nil, nil
	})

	// Approval request operations
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_approval_requests",
		Title:       "List Approval Requests",
		Description: "List pending approval requests. Use filter=mine to see only requests in your queue.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Approval Requests",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListApprovalRequestsInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.ListApprovalRequests(ctx, args.Filter)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_approval_request",
		Title:       "Get Approval Request",
		Description: "Get details of a single approval request including decisions so far.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Approval Request",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetApprovalRequestInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.GetApprovalRequest(ctx, args.ID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "submit_for_approval",
		Title:       "Submit Content for Approval",
		Description: "Explicitly submit a content item for editorial approval. Contributors must submit content for approval before it can be published.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Submit Content for Approval",
			DestructiveHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SubmitForApprovalInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.SubmitForApproval(ctx, args.ContentID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "approve_request",
		Title:       "Approve Request",
		Description: "Approve an approval request. When all required approvals are collected the content is automatically published.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Approve Request",
			DestructiveHint: boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ApproveRequestInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.ApproveRequest(ctx, args.ID, apiclient.ApproveRejectRequest{
			Comment: args.Comment,
		}); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("Request approved"), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "reject_request",
		Title:       "Reject Request",
		Description: "Reject an approval request. A comment explaining the rejection is required and will be posted to the content's discussion thread.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Reject Request",
			DestructiveHint: boolPtr(true),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args RejectRequestInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.RejectRequest(ctx, args.ID, apiclient.ApproveRejectRequest{
			Comment: args.Comment,
		}); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("Request rejected"), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "cancel_approval_request",
		Title:       "Cancel Approval Request",
		Description: "Cancel a pending approval request.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Cancel Approval Request",
			DestructiveHint: boolPtr(true),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CancelApprovalRequestInput) (*mcp.CallToolResult, any, error) {
		if err := s.client.CancelApprovalRequest(ctx, args.ID); err != nil {
			return errorResult(err), nil, nil
		}
		return textResult("Approval request cancelled"), nil, nil
	})
}
