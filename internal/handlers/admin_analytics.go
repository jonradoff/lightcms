package handlers

import (
	"net/http"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/services"
)

// AnalyticsPage shows the uptime and visitor analytics dashboard (admin only).
func (h *Handler) AnalyticsPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	now := time.Now().UTC()

	rangeParam := r.URL.Query().Get("range")
	var since time.Time
	switch rangeParam {
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-24 * time.Hour)
		rangeParam = "24h"
	}

	stats, _ := h.analyticsService.GetHourlyStats(ctx, since, now)
	uptimePct, totalVisitors := h.analyticsService.GetUptimeSummary(ctx, since)

	// Find peak hour
	peakHour := ""
	peakVisitors := 0
	for _, s := range stats {
		if s.VisitorCount > peakVisitors {
			peakVisitors = s.VisitorCount
			peakHour = s.Hour.Format("Jan 2 15:00 UTC")
		}
	}

	h.renderAdmin(w, r, "analytics", map[string]interface{}{
		"Stats":         stats,
		"StatsJSON":     services.HourlyStatsJSON(stats),
		"Range":         rangeParam,
		"UptimePercent": uptimePct,
		"TotalVisitors": totalVisitors,
		"PeakHour":      peakHour,
		"PeakVisitors":  peakVisitors,
	})
}
