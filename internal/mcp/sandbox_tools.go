package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sandboxState tracks the agent sandbox for this MCP session. While a
// sandbox is active, all content writes are redirected into its fork —
// live content is untouched until a human reviews and merges the fork.
type sandboxState struct {
	mu       sync.Mutex
	forkID   string
	forkName string
}

// sandboxFork returns the active sandbox fork, if any.
func (s *Server) sandboxFork() (id, name string, active bool) {
	s.sandbox.mu.Lock()
	defer s.sandbox.mu.Unlock()
	return s.sandbox.forkID, s.sandbox.forkName, s.sandbox.forkID != ""
}

// sandboxBlock returns a non-nil error result when a sandbox is active,
// explaining why the tool is unavailable. Tools that publish, delete, or
// bulk-modify live content call this first.
func (s *Server) sandboxBlock(tool string) *mcp.CallToolResult {
	forkID, forkName, active := s.sandboxFork()
	if !active {
		return nil
	}
	return textResult(fmt.Sprintf(
		"BLOCKED: %s is not available while agent sandbox %q (fork %s) is active. "+
			"All changes must stay in the sandbox fork until a human reviews and merges it. "+
			"Use update_content / create_content (they write into the sandbox automatically), "+
			"then end_agent_sandbox to hand the fork off for review.",
		tool, forkName, forkID))
}

// sandboxTargetForContent resolves which content ID an update should hit
// while a sandbox is active: fork copies pass through, live pages get a
// copy-on-write fork copy, and pages from other forks are rejected.
func (s *Server) sandboxTargetForContent(ctx context.Context, contentID string) (string, error) {
	forkID, _, active := s.sandboxFork()
	if !active {
		return contentID, nil
	}
	content, err := s.client.GetContent(ctx, contentID, false)
	if err != nil {
		return "", fmt.Errorf("resolve sandbox target: %w", err)
	}
	if content.ForkID == forkID {
		return contentID, nil // already the sandbox copy
	}
	if content.ForkID != "" {
		return "", fmt.Errorf("content %s belongs to another fork (%s); the active sandbox is %s", contentID, content.ForkID, forkID)
	}
	// Live page: copy-on-write into the sandbox (idempotent).
	page, err := s.client.ForkPage(ctx, forkID, contentID, "")
	if err != nil {
		return "", fmt.Errorf("sandbox copy-on-write failed: %w", err)
	}
	return page.ID, nil
}

// sandboxTargetForPath is the path-based variant of sandboxTargetForContent.
// It returns the ID of the sandbox copy for the page at path ("" when no
// sandbox is active, meaning: use the normal by-path flow).
func (s *Server) sandboxTargetForPath(ctx context.Context, path string) (string, error) {
	forkID, _, active := s.sandboxFork()
	if !active {
		return "", nil
	}
	page, err := s.client.ForkPage(ctx, forkID, "", path)
	if err != nil {
		return "", fmt.Errorf("sandbox copy-on-write failed for %s: %w", path, err)
	}
	return page.ID, nil
}

type StartAgentSandboxInput struct {
	Name        string `json:"name,omitempty" jsonschema:"Short name for the sandbox fork (e.g. 'refresh-pricing-pages'). Auto-generated when omitted."`
	Description string `json:"description,omitempty" jsonschema:"What this agent session intends to change — shown to the human reviewer."`
}

type EndAgentSandboxInput struct {
	Action string `json:"action" jsonschema:"'submit' keeps the fork and hands it to a human for review/merge; 'discard' deletes the fork and every change in it."`
}

func (s *Server) registerSandboxTools() {
	// Start sandbox
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:  "start_agent_sandbox",
		Title: "Start Agent Sandbox",
		Description: `Start a sandboxed editing session backed by a content fork ("pull request for content").

While the sandbox is active:
- update_content / update_content_by_path transparently copy pages into the fork and edit the copies (copy-on-write). Live content is never modified.
- create_content creates new pages inside the fork.
- Publishing, deleting, bulk operations, and search/replace-execute are blocked.
- The site is unaffected until a human reviews the fork at /cm/forks and merges it.

Recommended for any multi-page or risky editing task. End with end_agent_sandbox.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Start Agent Sandbox",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(false),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args StartAgentSandboxInput) (*mcp.CallToolResult, any, error) {
		if _, name, active := s.sandboxFork(); active {
			return textResult(fmt.Sprintf("A sandbox (%q) is already active. End it with end_agent_sandbox before starting another.", name)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "agent-session"
		}
		desc := args.Description
		if desc == "" {
			desc = "Agent sandbox session"
		}
		fork, err := s.client.CreateFork(ctx, name, desc)
		if err != nil {
			return errorResult(err), nil, nil
		}
		forkID, _ := fork["id"].(string)
		if forkID == "" {
			return textResult("fork created but no ID returned"), nil, nil
		}
		s.sandbox.mu.Lock()
		s.sandbox.forkID = forkID
		s.sandbox.forkName = name
		s.sandbox.mu.Unlock()

		return jsonResult(map[string]interface{}{
			"success": true,
			"fork_id": forkID,
			"name":    name,
			"message": "Sandbox active. All content writes now go into this fork; live content is protected. Call end_agent_sandbox when done.",
		}), nil, nil
	})

	// Sandbox status
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_agent_sandbox",
		Title:       "Get Agent Sandbox Status",
		Description: "Show whether an agent sandbox is active, and if so its fork ID, name, and the pages changed so far (with per-field diffs against live).",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Agent Sandbox Status",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		forkID, name, active := s.sandboxFork()
		if !active {
			return textResult("No agent sandbox is active. Content tools operate on live content."), nil, nil
		}
		diff, err := s.client.GetForkDiff(ctx, forkID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(map[string]interface{}{
			"active":  true,
			"fork_id": forkID,
			"name":    name,
			"changes": diff.Pages,
		}), nil, nil
	})

	// End sandbox
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "end_agent_sandbox",
		Title:       "End Agent Sandbox",
		Description: `End the sandbox session. action='submit' keeps the fork for human review and merge (at /cm/forks/{id}); action='discard' permanently deletes the fork and all its changes. Agents cannot merge their own sandbox — merging is a human decision.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "End Agent Sandbox",
			ReadOnlyHint:    false,
			DestructiveHint: boolPtr(true),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(false),
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EndAgentSandboxInput) (*mcp.CallToolResult, any, error) {
		forkID, name, active := s.sandboxFork()
		if !active {
			return textResult("No agent sandbox is active."), nil, nil
		}
		switch args.Action {
		case "submit":
			diff, err := s.client.GetForkDiff(ctx, forkID)
			if err != nil {
				return errorResult(err), nil, nil
			}
			s.sandbox.mu.Lock()
			s.sandbox.forkID, s.sandbox.forkName = "", ""
			s.sandbox.mu.Unlock()
			return jsonResult(map[string]interface{}{
				"success":    true,
				"fork_id":    forkID,
				"name":       name,
				"changes":    diff.Pages,
				"review_url": "/cm/forks/" + forkID,
				"message":    "Sandbox submitted. A human can review the changes and merge the fork from the admin UI.",
			}), nil, nil
		case "discard":
			if err := s.client.DeleteFork(ctx, forkID); err != nil {
				return errorResult(err), nil, nil
			}
			s.sandbox.mu.Lock()
			s.sandbox.forkID, s.sandbox.forkName = "", ""
			s.sandbox.mu.Unlock()
			return textResult(fmt.Sprintf("Sandbox %q discarded — fork and all its changes deleted. Live content was never modified.", name)), nil, nil
		default:
			return textResult("action must be 'submit' or 'discard'"), nil, nil
		}
	})

	// Fork diff (also useful outside sandbox sessions, e.g. for reviewers)
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_fork_diff",
		Title:       "Get Fork Diff",
		Description: "Per-field differences between each page in a fork and its live counterpart — the review surface for merging a fork. Status 'added' means the page exists only in the fork.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Fork Diff",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetForkInput) (*mcp.CallToolResult, any, error) {
		diff, err := s.client.GetForkDiff(ctx, args.ID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(diff), nil, nil
	})
}
