package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jonradoff/lightcms/v6/internal/apiclient"
)

// sandboxAPI is a minimal fake REST API tracking fork state, for exercising
// the agent-sandbox flow end to end at the MCP layer.
type sandboxAPI struct {
	mu          sync.Mutex
	forkCreated bool
	forkDeleted bool
	forkPages   map[string]string // path -> fork page ID
	updates     map[string]int    // content ID -> update count
}

func newSandboxServer(t *testing.T) (*Server, *sandboxAPI, func()) {
	t.Helper()
	state := &sandboxAPI{forkPages: map[string]string{}, updates: map[string]int{}}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/forks", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		state.forkCreated = true
		state.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "fork1", "name": "agent-session", "status": "active"})
	})
	mux.HandleFunc("DELETE /api/v1/forks/fork1", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		state.forkDeleted = true
		state.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})
	mux.HandleFunc("GET /api/v1/forks/fork1/diff", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"fork_id": "fork1",
			"pages": []map[string]interface{}{
				{"path": "/live-page", "title": "Live Page", "status": "modified",
					"fields": []map[string]string{{"name": "title", "live": "Old", "fork": "New"}}},
			},
		})
	})
	mux.HandleFunc("POST /api/v1/forks/fork1/fork-page", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		path := body["path"]
		if path == "" {
			path = "/live-page" // fork by content_id resolves to the live page
		}
		state.mu.Lock()
		id, ok := state.forkPages[path]
		if !ok {
			id = "fp-" + strings.TrimPrefix(path, "/")
			state.forkPages[path] = id
		}
		state.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "full_path": path, "title": "Fork copy"})
	})
	mux.HandleFunc("GET /api/v1/content/live1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "live1", "title": "Live Page", "full_path": "/live-page", "published": true,
			"data": map[string]interface{}{"body": "old"},
		})
	})
	mux.HandleFunc("GET /api/v1/content/fp-live-page", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "fp-live-page", "title": "Live Page", "full_path": "/live-page",
			"fork_id": "fork1", "data": map[string]interface{}{"body": "old"},
		})
	})
	mux.HandleFunc("PUT /api/v1/content/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/content/")
		state.mu.Lock()
		state.updates[id]++
		state.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "title": "Updated", "full_path": "/live-page"})
	})
	mux.HandleFunc("POST /api/v1/content", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		forkID, _ := body["fork_id"].(string)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "new1", "title": body["title"], "full_path": "/new-page", "fork_id": forkID,
		})
	})

	ts := httptest.NewServer(mux)
	client := apiclient.New(ts.URL, "test-key")
	s := NewServer(client)
	return s, state, ts.Close
}

func TestSandbox_Lifecycle(t *testing.T) {
	s, state, cleanup := newSandboxServer(t)
	defer cleanup()

	// No sandbox: status reports inactive.
	res := callTool(t, s, "get_agent_sandbox", struct{}{})
	if !strings.Contains(resultText(t, res), "No agent sandbox") {
		t.Errorf("expected inactive status, got %s", resultText(t, res))
	}

	// Start.
	res = callTool(t, s, "start_agent_sandbox", map[string]string{"name": "test-session"})
	if !strings.Contains(resultText(t, res), "fork1") {
		t.Fatalf("start: %s", resultText(t, res))
	}
	if !state.forkCreated {
		t.Fatal("fork was not created")
	}
	if _, _, active := s.sandboxFork(); !active {
		t.Fatal("sandbox not active after start")
	}

	// Double-start is refused.
	res = callTool(t, s, "start_agent_sandbox", map[string]string{})
	if !strings.Contains(resultText(t, res), "already active") {
		t.Errorf("double start: %s", resultText(t, res))
	}

	// Status includes diff.
	res = callTool(t, s, "get_agent_sandbox", struct{}{})
	if !strings.Contains(resultText(t, res), "modified") {
		t.Errorf("status should include changes: %s", resultText(t, res))
	}

	// Submit ends the session and returns the review URL.
	res = callTool(t, s, "end_agent_sandbox", map[string]string{"action": "submit"})
	txt := resultText(t, res)
	if !strings.Contains(txt, "/cm/forks/fork1") {
		t.Errorf("submit should include review URL: %s", txt)
	}
	if _, _, active := s.sandboxFork(); active {
		t.Fatal("sandbox still active after submit")
	}
}

func TestSandbox_CopyOnWriteUpdate(t *testing.T) {
	s, state, cleanup := newSandboxServer(t)
	defer cleanup()

	callTool(t, s, "start_agent_sandbox", map[string]string{})

	// Updating a live page must hit the fork copy, not the live document.
	res := callTool(t, s, "update_content", map[string]interface{}{
		"id": "live1", "title": "New Title", "version_comment": "sandbox edit",
	})
	if strings.Contains(resultText(t, res), "error") {
		t.Fatalf("update failed: %s", resultText(t, res))
	}
	state.mu.Lock()
	liveUpdates, forkUpdates := state.updates["live1"], state.updates["fp-live-page"]
	state.mu.Unlock()
	if liveUpdates != 0 {
		t.Errorf("live content was updated %d times — sandbox must protect it", liveUpdates)
	}
	if forkUpdates != 1 {
		t.Errorf("fork copy updates = %d, want 1", forkUpdates)
	}

	// By-path update also lands on the fork copy.
	callTool(t, s, "update_content_by_path", map[string]interface{}{
		"path": "/live-page", "title": "Another Title",
	})
	state.mu.Lock()
	liveUpdates, forkUpdates = state.updates["live1"], state.updates["fp-live-page"]
	state.mu.Unlock()
	if liveUpdates != 0 || forkUpdates != 2 {
		t.Errorf("after by-path: live=%d fork=%d, want 0/2", liveUpdates, forkUpdates)
	}
}

func TestSandbox_CreateGoesIntoFork(t *testing.T) {
	s, _, cleanup := newSandboxServer(t)
	defer cleanup()

	callTool(t, s, "start_agent_sandbox", map[string]string{})
	res := callTool(t, s, "create_content", map[string]interface{}{
		"template_id": "t1", "title": "Brand New", "slug": "new-page",
		"data": map[string]interface{}{"body": "x"},
	})
	if !strings.Contains(resultText(t, res), "new1") {
		t.Fatalf("create: %s", resultText(t, res))
	}

	// Upsert is refused in sandbox mode.
	res = callTool(t, s, "create_content", map[string]interface{}{
		"template_id": "t1", "title": "Up", "slug": "up", "upsert": true,
		"data": map[string]interface{}{},
	})
	if !strings.Contains(resultText(t, res), "not available") {
		t.Errorf("upsert should be blocked: %s", resultText(t, res))
	}
}

func TestSandbox_BlocksDestructiveTools(t *testing.T) {
	s, _, cleanup := newSandboxServer(t)
	defer cleanup()

	callTool(t, s, "start_agent_sandbox", map[string]string{})

	blocked := []struct {
		tool string
		args interface{}
	}{
		{"publish_content", map[string]string{"id": "live1"}},
		{"unpublish_content", map[string]string{"id": "live1"}},
		{"delete_content", map[string]string{"id": "live1"}},
		{"restore_content", map[string]string{"id": "live1"}},
		{"publish_multiple", map[string]interface{}{"ids": []string{"live1"}}},
		{"bulk_update_content", map[string]interface{}{"updates": []map[string]interface{}{}}},
		{"bulk_field_operation", map[string]interface{}{"operation": "set", "field": "x"}},
		{"search_replace_execute", map[string]interface{}{"search": "a", "replace": "b"}},
		{"scoped_search_replace_execute", map[string]interface{}{"search": "a", "replace": "b"}},
		{"regenerate_all_content", struct{}{}},
	}
	for _, b := range blocked {
		res := callTool(t, s, b.tool, b.args)
		if !strings.Contains(resultText(t, res), "BLOCKED") {
			t.Errorf("%s should be blocked in sandbox, got: %s", b.tool, resultText(t, res))
		}
	}

	// Fork tools targeting the sandbox's own fork are blocked too.
	for _, tool := range []string{"merge_fork", "archive_fork", "delete_fork"} {
		res := callTool(t, s, tool, map[string]string{"fork_id": "fork1"})
		if !strings.Contains(resultText(t, res), "BLOCKED") {
			t.Errorf("%s on own sandbox should be blocked, got: %s", tool, resultText(t, res))
		}
	}
}

func TestSandbox_DiscardDeletesFork(t *testing.T) {
	s, state, cleanup := newSandboxServer(t)
	defer cleanup()

	callTool(t, s, "start_agent_sandbox", map[string]string{})
	res := callTool(t, s, "end_agent_sandbox", map[string]string{"action": "discard"})
	if !strings.Contains(resultText(t, res), "discarded") {
		t.Fatalf("discard: %s", resultText(t, res))
	}
	if !state.forkDeleted {
		t.Error("fork was not deleted on discard")
	}
	if _, _, active := s.sandboxFork(); active {
		t.Error("sandbox still active after discard")
	}
}

func TestGovernanceAndMaintenanceTools(t *testing.T) {
	state := &sandboxAPI{forkPages: map[string]string{}, updates: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/agent-sessions/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": "s1", "entries": 2,
			"content_items": []map[string]interface{}{{"content_id": "c1", "actions": []string{"content.update"}}},
		})
	})
	mux.HandleFunc("POST /api/v1/agent-sessions/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"session_id": "s1", "reverted": []string{"c1"}})
	})
	mux.HandleFunc("GET /api/v1/maintenance/report", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"page_count": 5, "stale_pages": []interface{}{}})
	})
	mux.HandleFunc("POST /api/v1/maintenance/scan", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"page_count": 5, "link_job_id": "j1"})
	})
	_ = state

	ts := httptest.NewServer(mux)
	defer ts.Close()
	client := apiclient.New(ts.URL, "k")
	client.SetAgentSession("agent-default-1")
	s := NewServer(client)

	// Session tools default to the client's own session.
	out := resultText(t, callTool(t, s, "get_agent_session_changes", struct{}{}))
	if !strings.Contains(out, "content.update") {
		t.Errorf("get_agent_session_changes: %s", out)
	}
	out = resultText(t, callTool(t, s, "rollback_agent_session", struct{}{}))
	if !strings.Contains(out, "reverted") {
		t.Errorf("rollback_agent_session: %s", out)
	}
	// Explicit session ID also works.
	out = resultText(t, callTool(t, s, "get_agent_session_changes", map[string]string{"session_id": "s1"}))
	if !strings.Contains(out, "s1") {
		t.Errorf("explicit session: %s", out)
	}

	// Maintenance tools.
	out = resultText(t, callTool(t, s, "get_maintenance_report", struct{}{}))
	if !strings.Contains(out, "page_count") {
		t.Errorf("get_maintenance_report: %s", out)
	}
	out = resultText(t, callTool(t, s, "run_maintenance_scan", map[string]bool{"link_check": true}))
	if !strings.Contains(out, "j1") {
		t.Errorf("run_maintenance_scan: %s", out)
	}

	// No session anywhere → helpful message.
	client2 := apiclient.New(ts.URL, "k")
	s2 := NewServer(client2)
	out = resultText(t, callTool(t, s2, "get_agent_session_changes", struct{}{}))
	if !strings.Contains(out, "no session_id") {
		t.Errorf("missing session message: %s", out)
	}
	out = resultText(t, callTool(t, s2, "rollback_agent_session", struct{}{}))
	if !strings.Contains(out, "no session_id") {
		t.Errorf("rollback missing session message: %s", out)
	}
}
