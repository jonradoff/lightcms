package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

type StartLinkCheckInput struct{}

type GetLinkCheckResultsInput struct {
	JobID string `json:"job_id" jsonschema:"Link check job ID returned by start_link_check,required"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func (s *Server) registerLinkCheckTools() {
	// start_link_check
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:  "start_link_check",
		Title: "Start Link Check",
		Description: `Start an asynchronous broken-link check across all published content.

Returns a job_id immediately. Poll get_link_check_results(job_id) until status is "done".

Example workflow:
  start_link_check() → {job_id: "abc123"}
  get_link_check_results("abc123") → if status=="running", wait and retry; if "done", review broken_links`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Start Link Check",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  false,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args StartLinkCheckInput) (*mcp.CallToolResult, any, error) {
		result, err := s.client.StartLinkCheck(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(result), nil, nil
	})

	// get_link_check_results
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_link_check_results",
		Title:       "Get Link Check Results",
		Description: `Get the status and results of a link check job. Status is "running", "done", or "failed".`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Link Check Results",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetLinkCheckResultsInput) (*mcp.CallToolResult, any, error) {
		if args.JobID == "" {
			return errorResult(fmt.Errorf("job_id is required")), nil, nil
		}
		job, err := s.client.GetLinkCheckJob(ctx, args.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(job), nil, nil
	})
}
