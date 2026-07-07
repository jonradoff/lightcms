package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// resolveEditIDs looks up content IDs by full_path for the given page stat
// lists so the template can link to the edit page. All lists are resolved
// with a single projected query over the union of their paths.
func (h *Handler) resolveEditIDs(ctx context.Context, pageLists ...[]services.PageStat) {
	pathSet := make(map[string]struct{})
	for _, pages := range pageLists {
		for _, p := range pages {
			pathSet[p.Path] = struct{}{}
		}
	}
	if len(pathSet) == 0 {
		return
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	cursor, err := h.db.FindMany(ctx, "content", bson.M{
		"full_path": bson.M{"$in": paths},
		"deleted":   bson.M{"$ne": true},
	}, options.Find().SetProjection(bson.M{"full_path": 1}))
	if err != nil {
		return
	}
	var docs []models.Content
	if err := cursor.All(ctx, &docs); err != nil {
		return
	}
	byPath := make(map[string]string, len(docs))
	for _, d := range docs {
		byPath[d.FullPath] = d.ID.Hex()
	}
	for _, pages := range pageLists {
		for i := range pages {
			pages[i].EditID = byPath[pages[i].Path]
		}
	}
}

// AnalyticsPage shows the uptime and visitor analytics dashboard (admin only).
func (h *Handler) AnalyticsPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	since, until, rangeParam, rangeStart, rangeEnd := parseAnalyticsRange(r)

	stats, err := h.analyticsService.GetHourlyStats(ctx, since, until)
	if err != nil {
		log.Printf("[analytics] GetHourlyStats error: %v", err)
	}
	uptimePct, totalVisitors, humanVisitors := h.analyticsService.GetUptimeSummary(ctx, since)

	// Find peak hour
	peakHour := ""
	peakVisitors := 0
	for _, s := range stats {
		if s.VisitorCount > peakVisitors {
			peakVisitors = s.VisitorCount
			peakHour = s.Hour.Format("Jan 2 15:00 UTC")
		}
	}

	// Top pages (with edit IDs resolved) — all three views for client-side tab switching
	topPagesHuman, err := h.analyticsService.GetTopPages(ctx, since, until, 20, services.BotFilterHuman)
	if err != nil {
		log.Printf("[analytics] GetTopPages error: %v", err)
	}
	topPagesBot, _ := h.analyticsService.GetTopPages(ctx, since, until, 20, services.BotFilterBot)
	topPagesAll, _ := h.analyticsService.GetTopPages(ctx, since, until, 20, services.BotFilterAll)
	h.resolveEditIDs(ctx, topPagesHuman, topPagesBot, topPagesAll)
	topPagesHumanJSON, _ := json.Marshal(topPagesHuman)
	topPagesBotJSON, _ := json.Marshal(topPagesBot)
	topPagesAllJSON, _ := json.Marshal(topPagesAll)

	// Top referrers — all three views for client-side tab switching
	refHuman, _ := h.analyticsService.GetTopReferrers(ctx, since, until, 20, services.BotFilterHuman)
	refBot, _ := h.analyticsService.GetTopReferrers(ctx, since, until, 20, services.BotFilterBot)
	refAll, _ := h.analyticsService.GetTopReferrers(ctx, since, until, 20, services.BotFilterAll)
	refHumanJSON, _ := json.Marshal(refHuman)
	refBotJSON, _ := json.Marshal(refBot)
	refAllJSON, _ := json.Marshal(refAll)

	// User agent breakdown
	userAgents, err := h.analyticsService.GetUserAgents(ctx, since, until)
	if err != nil {
		log.Printf("[analytics] GetUserAgents error: %v", err)
	}
	userAgentsJSON, _ := json.Marshal(userAgents)

	h.renderAdmin(w, r, "analytics", map[string]interface{}{
		"Stats":             stats,
		"StatsJSON":         services.HourlyStatsJSON(stats),
		"Range":             rangeParam,
		"RangeStart":        rangeStart,
		"RangeEnd":          rangeEnd,
		"UptimePercent":     uptimePct,
		"TotalVisitors":     totalVisitors,
		"HumanVisitors":     humanVisitors,
		"PeakHour":          peakHour,
		"PeakVisitors":      peakVisitors,
		"TopPagesHumanJSON": string(topPagesHumanJSON),
		"TopPagesBotJSON":   string(topPagesBotJSON),
		"TopPagesAllJSON":   string(topPagesAllJSON),
		"RefHumanJSON":      string(refHumanJSON),
		"RefBotJSON":        string(refBotJSON),
		"RefAllJSON":        string(refAllJSON),
		"UserAgentsJSON":    string(userAgentsJSON),
	})
}

// AnalyticsPageDetail shows per-page analytics including referrers.
func (h *Handler) AnalyticsPageDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	pagePath := r.URL.Query().Get("path")
	if pagePath == "" {
		http.Redirect(w, r, "/cm/analytics", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	since, until, rangeParam, rangeStart, rangeEnd := parseAnalyticsRange(r)

	totalViews := h.analyticsService.GetPageViews(ctx, since, until, pagePath)

	prHuman, _ := h.analyticsService.GetPageReferrers(ctx, since, until, pagePath, 20, services.BotFilterHuman)
	prBot, _ := h.analyticsService.GetPageReferrers(ctx, since, until, pagePath, 20, services.BotFilterBot)
	prAll, _ := h.analyticsService.GetPageReferrers(ctx, since, until, pagePath, 20, services.BotFilterAll)
	prHumanJSON, _ := json.Marshal(prHuman)
	prBotJSON, _ := json.Marshal(prBot)
	prAllJSON, _ := json.Marshal(prAll)

	// Look up content ID for edit link
	editID := ""
	var content models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"full_path": pagePath, "deleted": bson.M{"$ne": true}}, &content); err == nil {
		editID = content.ID.Hex()
	}

	h.renderAdmin(w, r, "analytics_page", map[string]interface{}{
		"PagePath":     pagePath,
		"Range":        rangeParam,
		"RangeStart":   rangeStart,
		"RangeEnd":     rangeEnd,
		"TotalViews":   totalViews,
		"RefHumanJSON": string(prHumanJSON),
		"RefBotJSON":   string(prBotJSON),
		"RefAllJSON":   string(prAllJSON),
		"EditID":       editID,
	})
}

// AnalyticsReferrerReport shows top pages from a specific referrer source.
func (h *Handler) AnalyticsReferrerReport(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	referrer := r.URL.Query().Get("referrer")
	if referrer == "" {
		http.Redirect(w, r, "/cm/analytics", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	since, until, rangeParam, rangeStart, rangeEnd := parseAnalyticsRange(r)

	hitsHuman := h.analyticsService.GetReferrerHits(ctx, since, until, referrer, services.BotFilterHuman)
	hitsBot := h.analyticsService.GetReferrerHits(ctx, since, until, referrer, services.BotFilterBot)
	hitsAll := hitsHuman + hitsBot

	pagesHuman, _ := h.analyticsService.GetTopPagesByReferrer(ctx, since, until, referrer, 50, services.BotFilterHuman)
	pagesBot, _ := h.analyticsService.GetTopPagesByReferrer(ctx, since, until, referrer, 50, services.BotFilterBot)
	pagesAll, _ := h.analyticsService.GetTopPagesByReferrer(ctx, since, until, referrer, 50, services.BotFilterAll)
	h.resolveEditIDs(ctx, pagesHuman, pagesBot, pagesAll)
	pagesHumanJSON, _ := json.Marshal(pagesHuman)
	pagesBotJSON, _ := json.Marshal(pagesBot)
	pagesAllJSON, _ := json.Marshal(pagesAll)

	h.renderAdmin(w, r, "analytics_referrer", map[string]interface{}{
		"Referrer":       referrer,
		"Range":          rangeParam,
		"RangeStart":     rangeStart,
		"RangeEnd":       rangeEnd,
		"HitsHuman":      hitsHuman,
		"HitsBot":        hitsBot,
		"HitsAll":        hitsAll,
		"PagesHumanJSON": string(pagesHumanJSON),
		"PagesBotJSON":   string(pagesBotJSON),
		"PagesAllJSON":   string(pagesAllJSON),
	})
}

// parseAnalyticsRange interprets range/start/end query parameters shared by
// the analytics pages. Presets: 24h (default), 7d, 30d, 60d, 90d.
// range=custom reads start/end as YYYY-MM-DD with an inclusive end date.
func parseAnalyticsRange(r *http.Request) (since, until time.Time, rangeParam, startStr, endStr string) {
	now := time.Now().UTC()
	until = now
	rangeParam = r.URL.Query().Get("range")
	switch rangeParam {
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
	case "60d":
		since = now.Add(-60 * 24 * time.Hour)
	case "90d":
		since = now.Add(-90 * 24 * time.Hour)
	case "custom":
		s, errS := time.Parse("2006-01-02", r.URL.Query().Get("start"))
		e, errE := time.Parse("2006-01-02", r.URL.Query().Get("end"))
		if errS != nil || errE != nil || e.Before(s) {
			since = now.Add(-24 * time.Hour)
			rangeParam = "24h"
			return
		}
		since = s
		until = e.Add(24 * time.Hour) // include the whole end day
		if until.After(now) {
			until = now
		}
		startStr = r.URL.Query().Get("start")
		endStr = r.URL.Query().Get("end")
	default:
		since = now.Add(-24 * time.Hour)
		rangeParam = "24h"
	}
	return
}
