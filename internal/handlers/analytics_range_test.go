package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseAnalyticsRange(t *testing.T) {
	now := time.Now().UTC()

	get := func(url string) (time.Time, time.Time, string, string, string) {
		return parseAnalyticsRange(httptest.NewRequest("GET", url, nil))
	}

	// Presets
	for _, tc := range []struct {
		q    string
		want time.Duration
		name string
	}{
		{"/cm/analytics", 24 * time.Hour, "24h"},
		{"/cm/analytics?range=7d", 7 * 24 * time.Hour, "7d"},
		{"/cm/analytics?range=30d", 30 * 24 * time.Hour, "30d"},
		{"/cm/analytics?range=60d", 60 * 24 * time.Hour, "60d"},
		{"/cm/analytics?range=90d", 90 * 24 * time.Hour, "90d"},
		{"/cm/analytics?range=bogus", 24 * time.Hour, "24h"},
	} {
		since, until, name, _, _ := get(tc.q)
		if name != tc.name {
			t.Errorf("%s: range = %q, want %q", tc.q, name, tc.name)
		}
		if d := now.Sub(since); d < tc.want-time.Minute || d > tc.want+time.Minute {
			t.Errorf("%s: since window = %v, want ~%v", tc.q, d, tc.want)
		}
		if now.Sub(until) > time.Minute {
			t.Errorf("%s: until should be ~now", tc.q)
		}
	}

	// Custom range, inclusive end date.
	since, until, name, s, e := get("/cm/analytics?range=custom&start=2026-06-01&end=2026-06-15")
	if name != "custom" || s != "2026-06-01" || e != "2026-06-15" {
		t.Errorf("custom: name=%q start=%q end=%q", name, s, e)
	}
	if since.Format("2006-01-02") != "2026-06-01" {
		t.Errorf("custom since = %v", since)
	}
	if until.Format("2006-01-02") != "2026-06-16" { // end day fully included
		t.Errorf("custom until = %v, want start of 06-16", until)
	}

	// Custom end in the future clamps to now.
	_, until, _, _, _ = get("/cm/analytics?range=custom&start=2026-06-01&end=2199-01-01")
	if now.Sub(until) > time.Minute || until.After(now.Add(time.Minute)) {
		t.Errorf("future end should clamp to now, got %v", until)
	}

	// Invalid custom input falls back to 24h.
	for _, q := range []string{
		"/cm/analytics?range=custom",
		"/cm/analytics?range=custom&start=junk&end=2026-06-15",
		"/cm/analytics?range=custom&start=2026-06-15&end=2026-06-01", // end before start
	} {
		_, _, name, _, _ := get(q)
		if name != "24h" {
			t.Errorf("%s: fallback range = %q, want 24h", q, name)
		}
	}
}

func TestAnalyticsPage_RangeControls(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := sessionReq("GET", "/cm/analytics?range=60d", nil, nil)
	rr := httptest.NewRecorder()
	h.AnalyticsPage(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"60 Days", "90 Days", `name="start"`, `name="end"`, `value="custom"`} {
		if !strings.Contains(body, want) {
			t.Errorf("analytics page missing %q", want)
		}
	}
	// 60d is the active button.
	if !strings.Contains(body, `range=60d" class="btn btn-primary`) {
		t.Errorf("60d button not marked active")
	}
}
