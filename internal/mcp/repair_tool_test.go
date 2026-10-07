package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"
)

// repair_fork_damage posts to the maintenance endpoint, passes dry_run
// through as the query parameter, and returns the server's report.
func TestRepairForkDamageTool(t *testing.T) {
	var calls []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/maintenance/repair-fork-damage" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}
		dry := r.URL.Query().Get("dry_run") == "true"
		report := map[string]interface{}{
			"dry_run":               dry,
			"fork_copies_published": []map[string]interface{}{{"id": "c1", "path": "/a", "fork_id": "f1", "has_live_page": true}},
			"missing_static":        []map[string]interface{}{{"id": "c2", "path": "/b"}},
			"fork_copies_cleared":   0, "regenerated": 0, "pages_checked": 7,
		}
		if !dry {
			report["fork_copies_cleared"], report["regenerated"] = 1, 1
		}
		json.NewEncoder(w).Encode(report)
	}))
	defer ts.Close()
	s := NewServer(apiclient.New(ts.URL, "k"))

	out := resultText(t, callTool(t, s, "repair_fork_damage", map[string]bool{"dry_run": true}))
	if !strings.Contains(out, `"dry_run": true`) && !strings.Contains(out, `"dry_run":true`) {
		t.Errorf("dry run result: %s", out)
	}
	if !strings.Contains(out, "has_live_page") || !strings.Contains(out, "/b") {
		t.Errorf("dry run result lacks the lists: %s", out)
	}
	out = resultText(t, callTool(t, s, "repair_fork_damage", struct{}{}))
	if !strings.Contains(out, `"regenerated": 1`) && !strings.Contains(out, `"regenerated":1`) {
		t.Errorf("real run result: %s", out)
	}
	want := []string{
		"POST /api/v1/maintenance/repair-fork-damage?dry_run=true",
		"POST /api/v1/maintenance/repair-fork-damage",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests = %q, want %q", calls, want)
	}

	// A refusal from the server (not an admin) comes back as a tool error
	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "insufficient permissions"})
	}))
	defer denied.Close()
	res := callTool(t, NewServer(apiclient.New(denied.URL, "k")), "repair_fork_damage", struct{}{})
	if !res.IsError || !strings.Contains(resultText(t, res), "insufficient permissions") {
		t.Errorf("403 from the server: isError=%v %s", res.IsError, resultText(t, res))
	}
}
