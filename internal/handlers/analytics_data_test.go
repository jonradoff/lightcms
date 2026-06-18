package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAnalyticsHandlers_WithData seeds analytics page views and content, then
// drives the analytics admin handlers so their data-processing branches (and
// resolveEditIDs) execute.
func TestAnalyticsHandlers_WithData(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmpl := seedTemplate(t, db, "Page", "page")
	seedContent(t, db, tmpl, "Blog Post", "blog-post", "/blog/post")

	ctx := context.Background()
	ua := "Mozilla/5.0 (Windows NT 10.0) Chrome/120 Safari/537"
	for i := 0; i < 8; i++ {
		h.analyticsService.RecordPageView(ctx, "/blog/post", "https://news.ycombinator.com/", ua)
		h.analyticsService.RecordPageView(ctx, "/about", "https://google.com/", ua)
	}
	h.analyticsService.RecordPageView(ctx, "/about", "", "Googlebot/2.1")
	h.analyticsService.RecordActivity(ctx, "u1")
	h.analyticsService.RecordHourlyVisitor(ctx, "iphash", ua)
	h.analyticsService.FlushBufferForTest()

	get := func(name string, handler http.HandlerFunc, rawQuery string) {
		r := sessionReq(http.MethodGet, "/cm/analytics?"+rawQuery, nil, nil)
		rr := httptest.NewRecorder()
		handler(rr, r)
		if rr.Code >= 500 {
			t.Errorf("%s?%s: %d (%s)", name, rawQuery, rr.Code, rr.Body.String())
		}
	}

	for _, rng := range []string{"24h", "7d", "30d"} {
		get("AnalyticsPage", h.AnalyticsPage, "range="+rng)
	}
	get("AnalyticsPageDetail", h.AnalyticsPageDetail, "path=/blog/post&range=7d")
	get("AnalyticsReferrerReport", h.AnalyticsReferrerReport, "referrer=news.ycombinator.com&range=7d")
}
