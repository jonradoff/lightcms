package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/testutil"
)

func TestAnalytics_PureHelpers(t *testing.T) {
	// classifyUserAgent
	for _, ua := range []string{
		"Mozilla/5.0 (Macintosh) AppleWebKit Chrome Safari",
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"curl/8.0", "", "python-requests/2.31",
	} {
		_ = classifyUserAgent(ua)
	}

	// HashIP is stable and non-empty.
	if HashIP("203.0.113.1") == "" || HashIP("203.0.113.1") != HashIP("203.0.113.1") {
		t.Error("HashIP should be stable and non-empty")
	}

	// hourKey formats a time bucket.
	if hourKey(time.Now()) == "" {
		t.Error("hourKey empty")
	}

	// escape/unescape round-trip for Mongo-unsafe keys.
	for _, k := range []string{"a.b", "with$dollar", "plain", "x.y.z$"} {
		if got := unescapeMongoKey(escapeMongoKey(k)); got != k {
			t.Errorf("escape round-trip: %q -> %q", k, got)
		}
	}

	// regexEscape neutralizes regex metacharacters.
	if regexEscape("a.b*c") == "a.b*c" {
		t.Error("regexEscape should escape metacharacters")
	}

	// pvField/refField/prefField for each filter.
	for _, f := range []BotFilter{BotFilterHuman, BotFilterBot, BotFilterAll} {
		_ = pvField(f)
		_ = refField(f)
		_ = prefField(f)
	}

	// HourlyStatsJSON serializes.
	if HourlyStatsJSON([]HourlyStat{}) == "" {
		t.Error("HourlyStatsJSON empty for empty slice")
	}
}

func TestAnalytics_ExtractReferrerDomain(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewAnalyticsService(context.Background(), db, "http://localhost:8082")
	defer svc.Stop()
	for _, r := range []string{
		"https://news.ycombinator.com/item?id=1",
		"http://localhost:8082/self", // own site → likely empty/internal
		"", "not a url",
	} {
		_ = svc.extractReferrerDomain(r)
	}
}

func TestAnalytics_RecordAndQuery(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAnalyticsService(context.Background(), db, "http://localhost:8082")
	defer svc.Stop()
	ctx := context.Background()

	ua := "Mozilla/5.0 (Windows NT 10.0) Chrome/120 Safari/537"
	for i := 0; i < 5; i++ {
		svc.RecordPageView(ctx, "/blog/post", "https://news.ycombinator.com/", ua)
		svc.RecordPageView(ctx, "/about", "", "Googlebot/2.1")
	}
	svc.RecordActivity(ctx, "user-1")
	svc.RecordHourlyVisitor(ctx, HashIP("203.0.113.7"), ua)

	// Persist buffered events.
	svc.flushBuffer()

	since := time.Now().Add(-48 * time.Hour)
	until := time.Now().Add(2 * time.Hour)

	for _, f := range []BotFilter{BotFilterAll, BotFilterHuman, BotFilterBot} {
		if _, err := svc.GetTopPages(ctx, since, until, 10, f); err != nil {
			t.Errorf("GetTopPages(%s): %v", f, err)
		}
		if _, err := svc.GetTopReferrers(ctx, since, until, 10, f); err != nil {
			t.Errorf("GetTopReferrers(%s): %v", f, err)
		}
		_, _ = svc.GetPageReferrers(ctx, since, until, "/blog/post", 10, f)
		_, _ = svc.GetTopPagesByReferrer(ctx, since, until, "news.ycombinator.com", 10, f)
		_ = svc.GetReferrerHits(ctx, since, until, "news.ycombinator.com", f)
	}

	_ = svc.GetPageViews(ctx, since, until, "/blog/post")
	_, _ = svc.GetHourlyStats(ctx, since, until)
	_, _, _ = svc.GetUptimeSummary(ctx, since)
	_ = svc.GetDAU(ctx)
	_ = svc.GetMAU(ctx)
	_ = svc.GetContentCreatedToday(ctx)
	_, _ = svc.GetUserAgents(ctx, since, until)
}
