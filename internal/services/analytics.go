package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const activityCollection = "user_activity"

// Hourly stats are stored in user_activity with user_id="__hourly__" and
// date formatted as "2006-01-02T15" to reuse the existing collection and
// avoid hitting Atlas's 500-collection limit.
const hourlyUserID = "__hourly__"

// AnalyticsService tracks user activity for DAU/MAU metrics.
//
// Deduplication strategy:
//   - visited is an in-process cache keyed by "userID:date".
//     sync.Map.LoadOrStore is atomic — at most one goroutine per (userID, day)
//     will reach the database write, eliminating redundant upserts.
//   - The unique compound index on (user_id, date) is a safety net for
//     multi-instance deployments or post-restart cold caches.
//   - A background goroutine sweeps the cache at midnight UTC so it never
//     accumulates more than one day's worth of entries.
//
// Page views and referrers use a write buffer to avoid per-request DB writes.
// Counters accumulate in memory and flush to MongoDB every 30 seconds.
type AnalyticsService struct {
	db      *database.DB
	visited sync.Map // key: "userID:YYYY-MM-DD" → struct{}{}
	stop    chan struct{}

	// siteHosts contains hostnames that should be treated as same-site
	// (referrers from these are filtered out as internal navigation).
	siteHosts map[string]bool

	// Write buffer for page views, referrers, and user agents.
	bufMu              sync.Mutex
	bufPageViews       map[string]map[string]int // hourKey → escapedPath → count (combined)
	bufPageViewsHuman  map[string]map[string]int // hourKey → escapedPath → count (non-bot)
	bufPageViewsBot    map[string]map[string]int // hourKey → escapedPath → count (bot)
	bufRefHuman        map[string]map[string]int // hourKey → source → count (non-bot)
	bufRefBot          map[string]map[string]int // hourKey → source → count (bot)
	bufPageRefHuman    map[string]map[string]int // hourKey → "path||source" → count (non-bot)
	bufPageRefBot      map[string]map[string]int // hourKey → "path||source" → count (bot)
	bufUserAgents      map[string]map[string]int // hourKey → category → count
}

// NewAnalyticsService creates a new AnalyticsService, ensures required indexes
// exist, and starts background goroutines. baseURL is used to filter out
// same-site referrers (e.g. "https://metavert.io").
func NewAnalyticsService(ctx context.Context, db *database.DB, baseURL string) *AnalyticsService {
	hosts := map[string]bool{"localhost": true}
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		h := strings.ToLower(u.Hostname())
		hosts[h] = true
		hosts["www."+h] = true
	}

	svc := &AnalyticsService{
		db:           db,
		stop:         make(chan struct{}),
		siteHosts:    hosts,
		bufPageViews:      make(map[string]map[string]int),
		bufPageViewsHuman: make(map[string]map[string]int),
		bufPageViewsBot:   make(map[string]map[string]int),
		bufRefHuman:       make(map[string]map[string]int),
		bufRefBot:       make(map[string]map[string]int),
		bufPageRefHuman: make(map[string]map[string]int),
		bufPageRefBot:   make(map[string]map[string]int),
		bufUserAgents:   make(map[string]map[string]int),
	}

	col := db.Collection(activityCollection)

	// Unique compound index on (user_id, date) — DB-level dedup safety net.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "date", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("user_id_date_unique"),
	})

	// Index for MAU range queries on date.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "date", Value: 1}},
		Options: options.Index().SetName("date_1"),
	})

	// TTL index on created_at — MongoDB auto-deletes documents after 90 days.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(90 * 24 * 3600).SetName("created_at_ttl_90d"),
	})

	log.Printf("[analytics] siteHosts for referrer filtering: %v", hosts)

	go svc.runMidnightCleanup()
	go svc.runUptimeHeartbeat()
	go svc.runBufferFlush()
	return svc
}

// Stop shuts down background goroutines and flushes any remaining buffered data.
func (s *AnalyticsService) Stop() {
	close(s.stop)
	s.flushBuffer()
}

// --- Background goroutines ---

// runMidnightCleanup wakes at each UTC midnight and flushes the in-memory
// visited cache so it never holds more than one day's worth of keys.
func (s *AnalyticsService) runMidnightCleanup() {
	for {
		now := time.Now().UTC()
		nextMidnight := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
		t := time.NewTimer(time.Until(nextMidnight))
		select {
		case <-t.C:
			s.visited.Range(func(k, _ any) bool {
				s.visited.Delete(k)
				return true
			})
		case <-s.stop:
			t.Stop()
			return
		}
	}
}

// runUptimeHeartbeat pings the user_activity collection every minute to record uptime.
func (s *AnalyticsService) runUptimeHeartbeat() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			hk := hourKey(time.Now())
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := s.db.Collection(activityCollection).UpdateOne(ctx,
				bson.M{"user_id": hourlyUserID, "date": hk},
				bson.M{
					"$inc":         bson.M{"uptime_pings": 1},
					"$setOnInsert": bson.M{"created_at": time.Now().UTC(), "visitors": bson.A{}},
				},
				options.Update().SetUpsert(true),
			)
			if err != nil {
				log.Printf("[analytics] heartbeat error: %v", err)
			}
			cancel()
		case <-s.stop:
			return
		}
	}
}

// runBufferFlush drains the page-view / referrer write buffer every 30 seconds.
func (s *AnalyticsService) runBufferFlush() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.flushBuffer()
		case <-s.stop:
			return
		}
	}
}

// FlushBufferForTest flushes buffered analytics writes to MongoDB synchronously.
// It is intended for tests that need recorded page views to be queryable
// immediately, without waiting for the background flush ticker.
func (s *AnalyticsService) FlushBufferForTest() {
	s.flushBuffer()
}

// flushBuffer writes all buffered page views and referrers to MongoDB in one
// UpdateOne per hourly bucket. The buffer is swapped under the lock so new
// writes don't block on the DB round-trip.
func (s *AnalyticsService) flushBuffer() {
	s.bufMu.Lock()
	pv := s.bufPageViews
	pvH := s.bufPageViewsHuman
	pvB := s.bufPageViewsBot
	refH := s.bufRefHuman
	refB := s.bufRefBot
	prH := s.bufPageRefHuman
	prB := s.bufPageRefBot
	ua := s.bufUserAgents
	s.bufPageViews = make(map[string]map[string]int)
	s.bufPageViewsHuman = make(map[string]map[string]int)
	s.bufPageViewsBot = make(map[string]map[string]int)
	s.bufRefHuman = make(map[string]map[string]int)
	s.bufRefBot = make(map[string]map[string]int)
	s.bufPageRefHuman = make(map[string]map[string]int)
	s.bufPageRefBot = make(map[string]map[string]int)
	s.bufUserAgents = make(map[string]map[string]int)
	s.bufMu.Unlock()

	// Collect all hour keys across all maps.
	hourKeys := make(map[string]struct{})
	for _, m := range []map[string]map[string]int{pv, pvH, pvB, refH, refB, prH, prB, ua} {
		for hk := range m {
			hourKeys[hk] = struct{}{}
		}
	}

	if len(hourKeys) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	col := s.db.Collection(activityCollection)

	for hk := range hourKeys {
		inc := bson.M{}

		// Page views (combined)
		if m, ok := pv[hk]; ok {
			for path, count := range m {
				inc["page_views."+path] = count
			}
		}
		// Page views (human)
		if m, ok := pvH[hk]; ok {
			for path, count := range m {
				inc["page_views_human."+path] = count
			}
		}
		// Page views (bot)
		if m, ok := pvB[hk]; ok {
			for path, count := range m {
				inc["page_views_bot."+path] = count
			}
		}

		// Human referrers
		if m, ok := refH[hk]; ok {
			for src, count := range m {
				inc["ref_human."+escapeMongoKey(src)] = count
			}
		}

		// Bot referrers
		if m, ok := refB[hk]; ok {
			for src, count := range m {
				inc["ref_bot."+escapeMongoKey(src)] = count
			}
		}

		// Human per-page referrers
		if m, ok := prH[hk]; ok {
			for key, count := range m {
				inc["pref_human."+escapeMongoKey(key)] = count
			}
		}

		// Bot per-page referrers
		if m, ok := prB[hk]; ok {
			for key, count := range m {
				inc["pref_bot."+escapeMongoKey(key)] = count
			}
		}

		// User agents: user_agents.{category} += count
		if m, ok := ua[hk]; ok {
			for cat, count := range m {
				inc["user_agents."+escapeMongoKey(cat)] = count
			}
		}

		if len(inc) == 0 {
			continue
		}

		log.Printf("[analytics] flush hk=%s fields=%d", hk, len(inc))

		_, err := col.UpdateOne(ctx,
			bson.M{"user_id": hourlyUserID, "date": hk},
			bson.M{
				"$inc":         inc,
				"$setOnInsert": bson.M{"created_at": time.Now().UTC(), "uptime_pings": 0, "visitors": bson.A{}},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Printf("[analytics] flushBuffer error for %s: %v", hk, err)
		}
	}
}

// --- Recording methods (hot path, lock-free or minimal locking) ---

// RecordActivity records that userID was active today. Safe to call on every
// request — the in-memory cache ensures at most one DB write per (userID, day).
func (s *AnalyticsService) RecordActivity(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	cacheKey := userID + ":" + today

	if _, loaded := s.visited.LoadOrStore(cacheKey, struct{}{}); loaded {
		return
	}

	col := s.db.Collection(activityCollection)
	col.UpdateOne(
		ctx,
		bson.M{"user_id": userID, "date": today},
		bson.M{
			"$set":         bson.M{"last_seen": time.Now().UTC()},
			"$setOnInsert": bson.M{"created_at": time.Now().UTC()},
		},
		options.Update().SetUpsert(true),
	)
}

// RecordHourlyVisitor records a unique visitor (by hashed IP) for the current hour.
// It classifies the visitor as bot or human based on the User-Agent string and
// stores them in separate arrays for filtered analytics.
func (s *AnalyticsService) RecordHourlyVisitor(ctx context.Context, ipHash, rawUA string) {
	if ipHash == "" {
		return
	}
	now := time.Now().UTC().Truncate(time.Hour)
	hk := hourKey(now)
	cacheKey := "hv:" + ipHash + ":" + hk
	if _, loaded := s.visited.LoadOrStore(cacheKey, struct{}{}); loaded {
		return
	}
	isBot := classifyUserAgent(rawUA) == "Bot"
	field := "visitors_human"
	if isBot {
		field = "visitors_bot"
	}
	col := s.db.Collection(activityCollection)
	_, err := col.UpdateOne(ctx,
		bson.M{"user_id": hourlyUserID, "date": hk},
		bson.M{
			"$addToSet":    bson.M{field: ipHash, "visitors": ipHash},
			"$setOnInsert": bson.M{"created_at": time.Now().UTC(), "uptime_pings": 0},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("[analytics] RecordHourlyVisitor error: %v", err)
	}
}

// RecordPageView buffers a page view, optional referrer, and user agent category for the current hour.
// No DB write happens here — data is flushed every 30 seconds by runBufferFlush.
func (s *AnalyticsService) RecordPageView(ctx context.Context, pagePath, rawReferrer, rawUA string) {
	if pagePath == "" {
		return
	}
	hk := hourKey(time.Now())
	safeKey := escapeMongoKey(pagePath)
	domain := s.extractReferrerDomain(rawReferrer)
	uaCat := classifyUserAgent(rawUA)

	// Classify traffic source
	var source string
	if rawReferrer == "" {
		source = "(direct)"
	} else if domain == "" {
		source = "(internal)"
	} else {
		source = domain
	}

	isBot := uaCat == "Bot"
	prKey := safeKey + "||" + source

	s.bufMu.Lock()
	// Page views (combined + split)
	if s.bufPageViews[hk] == nil {
		s.bufPageViews[hk] = make(map[string]int)
	}
	s.bufPageViews[hk][safeKey]++
	if isBot {
		if s.bufPageViewsBot[hk] == nil {
			s.bufPageViewsBot[hk] = make(map[string]int)
		}
		s.bufPageViewsBot[hk][safeKey]++
	} else {
		if s.bufPageViewsHuman[hk] == nil {
			s.bufPageViewsHuman[hk] = make(map[string]int)
		}
		s.bufPageViewsHuman[hk][safeKey]++
	}

	// Referrers — split into human/bot buckets
	if isBot {
		if s.bufRefBot[hk] == nil {
			s.bufRefBot[hk] = make(map[string]int)
		}
		s.bufRefBot[hk][source]++
		if s.bufPageRefBot[hk] == nil {
			s.bufPageRefBot[hk] = make(map[string]int)
		}
		s.bufPageRefBot[hk][prKey]++
	} else {
		if s.bufRefHuman[hk] == nil {
			s.bufRefHuman[hk] = make(map[string]int)
		}
		s.bufRefHuman[hk][source]++
		if s.bufPageRefHuman[hk] == nil {
			s.bufPageRefHuman[hk] = make(map[string]int)
		}
		s.bufPageRefHuman[hk][prKey]++
	}

	// User agent category
	if uaCat != "" {
		if s.bufUserAgents[hk] == nil {
			s.bufUserAgents[hk] = make(map[string]int)
		}
		s.bufUserAgents[hk][uaCat]++
	}
	s.bufMu.Unlock()
}

// classifyUserAgent returns a simple browser/device category from a User-Agent string.
func classifyUserAgent(ua string) string {
	if ua == "" {
		return "Unknown"
	}
	lower := strings.ToLower(ua)
	// Bots first
	for _, bot := range []string{"bot", "crawler", "spider", "slurp", "wget", "curl", "python", "go-http", "headless"} {
		if strings.Contains(lower, bot) {
			return "Bot"
		}
	}
	// Mobile detection
	isMobile := strings.Contains(lower, "mobile") || strings.Contains(lower, "android") && !strings.Contains(lower, "tablet")
	isTablet := strings.Contains(lower, "tablet") || strings.Contains(lower, "ipad")
	// Browser detection
	switch {
	case strings.Contains(lower, "edg/") || strings.Contains(lower, "edge/"):
		if isMobile {
			return "Edge Mobile"
		}
		return "Edge"
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "chromium"):
		if isMobile {
			return "Chrome Mobile"
		}
		return "Chrome"
	case strings.Contains(lower, "firefox"):
		if isMobile {
			return "Firefox Mobile"
		}
		return "Firefox"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		if isTablet {
			return "Safari iPad"
		}
		if isMobile {
			return "Safari Mobile"
		}
		return "Safari"
	default:
		if isMobile {
			return "Other Mobile"
		}
		if isTablet {
			return "Other Tablet"
		}
		return "Other"
	}
}

// extractReferrerDomain parses a Referer header and returns the hostname.
// Returns "" for empty, unparseable, or same-site referrers.
func (s *AnalyticsService) extractReferrerDomain(rawReferrer string) string {
	if rawReferrer == "" {
		return ""
	}
	u, err := url.Parse(rawReferrer)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if s.siteHosts[host] {
		return ""
	}
	return host
}

// --- Helper functions ---

// HashIP returns a privacy-safe hash of an IP address for visitor counting.
func HashIP(ip string) string {
	h := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("%x", h[:8])
}

// hourKey formats a time as "2006-01-02T15" for use as the date field in hourly docs.
func hourKey(t time.Time) string {
	return t.UTC().Truncate(time.Hour).Format("2006-01-02T15")
}

// escapeMongoKey replaces dots and dollar signs which are invalid in MongoDB field names.
func escapeMongoKey(s string) string {
	r := ""
	for _, c := range s {
		switch c {
		case '.':
			r += "\uff0e" // fullwidth period
		case '$':
			r += "\uff04" // fullwidth dollar
		default:
			r += string(c)
		}
	}
	return r
}

// unescapeMongoKey reverses escapeMongoKey.
func unescapeMongoKey(s string) string {
	r := ""
	for _, c := range s {
		switch c {
		case '\uff0e':
			r += "."
		case '\uff04':
			r += "$"
		default:
			r += string(c)
		}
	}
	return r
}

// --- Read methods (DAU/MAU) ---

// GetDAU returns the number of distinct users active today (UTC).
func (s *AnalyticsService) GetDAU(ctx context.Context) int64 {
	today := time.Now().UTC().Format("2006-01-02")
	n, _ := s.db.Count(ctx, activityCollection, bson.M{"date": today, "user_id": bson.M{"$ne": hourlyUserID}})
	return n
}

// GetMAU returns the number of distinct users active in the last 30 calendar days.
func (s *AnalyticsService) GetMAU(ctx context.Context) int64 {
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	var result []struct {
		Count int64 `bson:"count"`
	}
	pipeline := bson.A{
		bson.M{"$match": bson.M{"date": bson.M{"$gte": cutoff}, "user_id": bson.M{"$ne": hourlyUserID}}},
		bson.M{"$group": bson.M{"_id": "$user_id"}},
		bson.M{"$count": "count"},
	}
	s.db.Aggregate(ctx, activityCollection, pipeline, &result)
	if len(result) > 0 {
		return result[0].Count
	}
	return 0
}

// GetContentCreatedToday returns the number of content items created since midnight UTC.
func (s *AnalyticsService) GetContentCreatedToday(ctx context.Context) int64 {
	midnight := time.Now().UTC().Truncate(24 * time.Hour)
	n, _ := s.db.Count(ctx, "content", bson.M{
		"created_at": bson.M{"$gte": midnight},
		"deleted":    bson.M{"$ne": true},
	})
	return n
}

// --- Read methods (hourly stats, top pages, referrers) ---

// HourlyStat is the read model for hourly analytics data.
type HourlyStat struct {
	Hour              time.Time `bson:"hour" json:"hour"`
	VisitorCount      int       `bson:"visitor_count" json:"visitor_count"`
	VisitorCountHuman int       `bson:"visitor_count_human" json:"visitor_count_human"`
	VisitorCountBot   int       `bson:"visitor_count_bot" json:"visitor_count_bot"`
	UptimePings       int       `bson:"uptime_pings" json:"uptime_pings"`
}

// GetHourlyStats returns stats for the given time range, one entry per hour.
func (s *AnalyticsService) GetHourlyStats(ctx context.Context, since, until time.Time) ([]HourlyStat, error) {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
		}},
		bson.M{"$project": bson.M{
			"date":                1,
			"visitor_count":       bson.M{"$size": bson.M{"$ifNull": bson.A{"$visitors", bson.A{}}}},
			"visitor_count_human": bson.M{"$size": bson.M{"$ifNull": bson.A{"$visitors_human", bson.A{}}}},
			"visitor_count_bot":   bson.M{"$size": bson.M{"$ifNull": bson.A{"$visitors_bot", bson.A{}}}},
			"uptime_pings":       1,
		}},
		bson.M{"$sort": bson.M{"date": 1}},
	}

	var raw []struct {
		Date              string `bson:"date"`
		VisitorCount      int    `bson:"visitor_count"`
		VisitorCountHuman int    `bson:"visitor_count_human"`
		VisitorCountBot   int    `bson:"visitor_count_bot"`
		UptimePings       int    `bson:"uptime_pings"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}

	results := make([]HourlyStat, 0, len(raw))
	for _, r := range raw {
		t, err := time.Parse("2006-01-02T15", r.Date)
		if err != nil {
			continue
		}
		results = append(results, HourlyStat{
			Hour:              t,
			VisitorCount:      r.VisitorCount,
			VisitorCountHuman: r.VisitorCountHuman,
			VisitorCountBot:   r.VisitorCountBot,
			UptimePings:       r.UptimePings,
		})
	}
	return results, nil
}

// GetUptimeSummary returns uptime percentage, total visitors, and human-only visitors for a time range.
func (s *AnalyticsService) GetUptimeSummary(ctx context.Context, since time.Time) (uptimePct float64, totalVisitors, humanVisitors int) {
	now := time.Now().UTC()
	totalHours := int(now.Sub(since).Hours())
	if totalHours <= 0 {
		return 100.0, 0, 0
	}

	stats, err := s.GetHourlyStats(ctx, since, now)
	if err != nil {
		return 0, 0, 0
	}

	upHours := 0
	for _, st := range stats {
		if st.UptimePings > 0 {
			upHours++
		}
		totalVisitors += st.VisitorCount
		humanVisitors += st.VisitorCountHuman
	}
	uptimePct = float64(upHours) / float64(totalHours) * 100
	return uptimePct, totalVisitors, humanVisitors
}

// PageStat represents a page path and its total view count.
type PageStat struct {
	Path     string `json:"path"`
	Views    int    `json:"views"`
	EditID   string `json:"edit_id,omitempty"`
}

// GetTopPages returns the top N pages by view count in the given time range.
func (s *AnalyticsService) GetTopPages(ctx context.Context, since, until time.Time, limit int, filter BotFilter) ([]PageStat, error) {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)

	// For "all", merge human + bot + legacy combined field
	if filter == BotFilterAll {
		h, _ := s.GetTopPages(ctx, since, until, limit*2, BotFilterHuman)
		b, _ := s.GetTopPages(ctx, since, until, limit*2, BotFilterBot)
		legacy := s.queryTopPagesField(ctx, sinceKey, untilKey, "page_views", limit*2)
		merged := make(map[string]int)
		for _, p := range h {
			merged[p.Path] += p.Views
		}
		for _, p := range b {
			merged[p.Path] += p.Views
		}
		// Use legacy data for paths not present in split data
		for _, p := range legacy {
			if _, ok := merged[p.Path]; !ok {
				merged[p.Path] = p.Views
			}
		}
		results := make([]PageStat, 0, len(merged))
		for path, views := range merged {
			results = append(results, PageStat{Path: path, Views: views})
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Views > results[j].Views })
		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}

	field := pvField(filter)
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"pv_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$pv_array"},
		bson.M{"$group": bson.M{
			"_id":   "$pv_array.k",
			"views": bson.M{"$sum": "$pv_array.v"},
		}},
		bson.M{"$sort": bson.M{"views": -1}},
		bson.M{"$limit": limit},
	}

	var raw []struct {
		Key   string `bson:"_id"`
		Views int    `bson:"views"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}

	results := make([]PageStat, 0, len(raw))
	for _, r := range raw {
		results = append(results, PageStat{
			Path:  unescapeMongoKey(r.Key),
			Views: r.Views,
		})
	}
	return results, nil
}

// queryTopPagesField runs the standard page views aggregation on a specific field name.
func (s *AnalyticsService) queryTopPagesField(ctx context.Context, sinceKey, untilKey, field string, limit int) []PageStat {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"pv_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$pv_array"},
		bson.M{"$group": bson.M{
			"_id":   "$pv_array.k",
			"views": bson.M{"$sum": "$pv_array.v"},
		}},
		bson.M{"$sort": bson.M{"views": -1}},
		bson.M{"$limit": limit},
	}
	var raw []struct {
		Key   string `bson:"_id"`
		Views int    `bson:"views"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil
	}
	results := make([]PageStat, 0, len(raw))
	for _, r := range raw {
		results = append(results, PageStat{Path: unescapeMongoKey(r.Key), Views: r.Views})
	}
	return results
}

// ReferrerStat represents a referrer domain and its hit count.
type ReferrerStat struct {
	Domain string `json:"domain"`
	Hits   int    `json:"hits"`
}

// GetTopReferrers returns the top N referrer domains across all pages.
func (s *AnalyticsService) GetTopReferrers(ctx context.Context, since, until time.Time, limit int, filter BotFilter) ([]ReferrerStat, error) {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)

	if filter == BotFilterAll {
		// Merge human + bot + legacy data
		h, _ := s.GetTopReferrers(ctx, since, until, limit*2, BotFilterHuman)
		b, _ := s.GetTopReferrers(ctx, since, until, limit*2, BotFilterBot)
		legacy := s.queryRefField(ctx, sinceKey, untilKey, "referrers", limit*2)
		merged := make(map[string]int)
		for _, r := range h {
			merged[r.Domain] += r.Hits
		}
		for _, r := range b {
			merged[r.Domain] += r.Hits
		}
		for _, r := range legacy {
			merged[r.Domain] += r.Hits
		}
		results := make([]ReferrerStat, 0, len(merged))
		for d, hits := range merged {
			results = append(results, ReferrerStat{Domain: d, Hits: hits})
		}
		for i := 0; i < len(results); i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].Hits > results[i].Hits {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}

	field := refField(filter)
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"ref_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$ref_array"},
		bson.M{"$group": bson.M{
			"_id":  "$ref_array.k",
			"hits": bson.M{"$sum": "$ref_array.v"},
		}},
		bson.M{"$sort": bson.M{"hits": -1}},
		bson.M{"$limit": limit},
	}

	var raw []struct {
		Key  string `bson:"_id"`
		Hits int    `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}

	results := make([]ReferrerStat, 0, len(raw))
	for _, r := range raw {
		results = append(results, ReferrerStat{
			Domain: unescapeMongoKey(r.Key),
			Hits:   r.Hits,
		})
	}
	return results, nil
}

// GetPageReferrers returns the top N referrer domains for a specific page path.
func (s *AnalyticsService) GetPageReferrers(ctx context.Context, since, until time.Time, pagePath string, limit int, filter BotFilter) ([]ReferrerStat, error) {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)
	safeKey := escapeMongoKey(pagePath)
	pathPrefix := escapeMongoKey(safeKey + "||")

	if filter == BotFilterAll {
		h, _ := s.GetPageReferrers(ctx, since, until, pagePath, limit*2, BotFilterHuman)
		b, _ := s.GetPageReferrers(ctx, since, until, pagePath, limit*2, BotFilterBot)
		legacy := s.queryPrefField(ctx, sinceKey, untilKey, "page_refs", pathPrefix, limit*2)
		merged := make(map[string]int)
		for _, r := range h {
			merged[r.Domain] += r.Hits
		}
		for _, r := range b {
			merged[r.Domain] += r.Hits
		}
		for _, r := range legacy {
			merged[r.Domain] += r.Hits
		}
		results := make([]ReferrerStat, 0, len(merged))
		for d, hits := range merged {
			results = append(results, ReferrerStat{Domain: d, Hits: hits})
		}
		for i := 0; i < len(results); i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].Hits > results[i].Hits {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}

	field := prefField(filter)

	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"pr_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$pr_array"},
		bson.M{"$match": bson.M{
			"pr_array.k": bson.M{"$regex": "^" + regexEscape(pathPrefix)},
		}},
		bson.M{"$group": bson.M{
			"_id":  "$pr_array.k",
			"hits": bson.M{"$sum": "$pr_array.v"},
		}},
		bson.M{"$sort": bson.M{"hits": -1}},
		bson.M{"$limit": limit},
	}

	var raw []struct {
		Key  string `bson:"_id"`
		Hits int    `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}

	results := make([]ReferrerStat, 0, len(raw))
	for _, r := range raw {
		parts := strings.SplitN(unescapeMongoKey(r.Key), "||", 2)
		domain := ""
		if len(parts) == 2 {
			domain = parts[1]
		}
		results = append(results, ReferrerStat{
			Domain: domain,
			Hits:   r.Hits,
		})
	}
	return results, nil
}

// GetTopPagesByReferrer returns the top N pages that received traffic from a specific referrer source.
func (s *AnalyticsService) GetTopPagesByReferrer(ctx context.Context, since, until time.Time, referrer string, limit int, filter BotFilter) ([]PageStat, error) {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)
	safeSuffix := escapeMongoKey("||" + referrer)

	if filter == BotFilterAll {
		h, _ := s.GetTopPagesByReferrer(ctx, since, until, referrer, limit*2, BotFilterHuman)
		b, _ := s.GetTopPagesByReferrer(ctx, since, until, referrer, limit*2, BotFilterBot)
		legacy := s.queryPrefFieldByRef(ctx, sinceKey, untilKey, "page_refs", safeSuffix, limit*2)
		merged := make(map[string]int)
		for _, p := range h {
			merged[p.Path] += p.Views
		}
		for _, p := range b {
			merged[p.Path] += p.Views
		}
		for _, p := range legacy {
			merged[p.Path] += p.Views
		}
		results := make([]PageStat, 0, len(merged))
		for path, views := range merged {
			results = append(results, PageStat{Path: path, Views: views})
		}
		for i := 0; i < len(results); i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].Views > results[i].Views {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}
	field := prefField(filter)

	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"pr_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$pr_array"},
		bson.M{"$match": bson.M{
			"pr_array.k": bson.M{"$regex": regexEscape(escapeMongoKey(safeSuffix)) + "$"},
		}},
		bson.M{"$group": bson.M{
			"_id":   "$pr_array.k",
			"views": bson.M{"$sum": "$pr_array.v"},
		}},
		bson.M{"$sort": bson.M{"views": -1}},
		bson.M{"$limit": limit},
	}

	var raw []struct {
		Key   string `bson:"_id"`
		Views int    `bson:"views"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}

	results := make([]PageStat, 0, len(raw))
	for _, r := range raw {
		parts := strings.SplitN(unescapeMongoKey(r.Key), "||", 2)
		path := parts[0]
		results = append(results, PageStat{
			Path:  path,
			Views: r.Views,
		})
	}
	return results, nil
}

// GetReferrerHits returns the total hit count for a specific referrer in the time range.
func (s *AnalyticsService) GetReferrerHits(ctx context.Context, since, until time.Time, referrer string, filter BotFilter) int {
	if filter == BotFilterAll {
		total := s.GetReferrerHits(ctx, since, until, referrer, BotFilterHuman) +
			s.GetReferrerHits(ctx, since, until, referrer, BotFilterBot)
		// Also count legacy "referrers" field
		sinceKey := hourKey(since)
		untilKey := hourKey(until)
		safeKey := escapeMongoKey(referrer)
		legacyHits := s.queryRefFieldSum(ctx, sinceKey, untilKey, "referrers", safeKey)
		return total + legacyHits
	}

	sinceKey := hourKey(since)
	untilKey := hourKey(until)
	field := refField(filter)
	safeKey := escapeMongoKey(referrer)

	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id":                       hourlyUserID,
			"date":                          bson.M{"$gte": sinceKey, "$lt": untilKey},
			field + "." + safeKey: bson.M{"$exists": true},
		}},
		bson.M{"$group": bson.M{
			"_id":  nil,
			"hits": bson.M{"$sum": "$" + field + "." + safeKey},
		}},
	}

	var raw []struct {
		Hits int `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil || len(raw) == 0 {
		return 0
	}
	return raw[0].Hits
}

// GetPageViews returns the total view count for a specific page in the time range.
func (s *AnalyticsService) GetPageViews(ctx context.Context, since, until time.Time, pagePath string) int {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)
	safeKey := escapeMongoKey(pagePath)

	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id":                      hourlyUserID,
			"date":                         bson.M{"$gte": sinceKey, "$lt": untilKey},
			"page_views." + safeKey: bson.M{"$exists": true},
		}},
		bson.M{"$group": bson.M{
			"_id":   nil,
			"views": bson.M{"$sum": "$page_views." + safeKey},
		}},
	}

	var raw []struct {
		Views int `bson:"views"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil || len(raw) == 0 {
		return 0
	}
	return raw[0].Views
}

// regexEscape escapes special regex characters in a string.
// BotFilter specifies which traffic to include in referrer queries.
type BotFilter string

const (
	BotFilterHuman BotFilter = "human"
	BotFilterBot   BotFilter = "bot"
	BotFilterAll   BotFilter = "all"
)

// pvField returns the MongoDB field name for page views based on filter.
func pvField(filter BotFilter) string {
	switch filter {
	case BotFilterBot:
		return "page_views_bot"
	case BotFilterHuman:
		return "page_views_human"
	default:
		return "page_views"
	}
}

// refField returns the MongoDB field name for site-wide referrers based on filter.
func refField(filter BotFilter) string {
	switch filter {
	case BotFilterBot:
		return "ref_bot"
	default:
		return "ref_human"
	}
}

// prefField returns the MongoDB field name for per-page referrers based on filter.
func prefField(filter BotFilter) string {
	switch filter {
	case BotFilterBot:
		return "pref_bot"
	default:
		return "pref_human"
	}
}

// queryRefField runs the standard $objectToArray aggregation on a given referrer field name.
// Used to query both new (ref_human/ref_bot) and legacy (referrers) fields.
func (s *AnalyticsService) queryRefField(ctx context.Context, sinceKey, untilKey, field string, limit int) []ReferrerStat {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"ref_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$ref_array"},
		bson.M{"$group": bson.M{
			"_id":  "$ref_array.k",
			"hits": bson.M{"$sum": "$ref_array.v"},
		}},
		bson.M{"$sort": bson.M{"hits": -1}},
		bson.M{"$limit": limit},
	}
	var raw []struct {
		Key  string `bson:"_id"`
		Hits int    `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil
	}
	results := make([]ReferrerStat, 0, len(raw))
	for _, r := range raw {
		results = append(results, ReferrerStat{Domain: unescapeMongoKey(r.Key), Hits: r.Hits})
	}
	return results
}

// queryPrefField runs $objectToArray on a per-page referrer field, filtered by page path prefix.
func (s *AnalyticsService) queryPrefField(ctx context.Context, sinceKey, untilKey, field, pathPrefix string, limit int) []ReferrerStat {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"pr_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$pr_array"},
		bson.M{"$match": bson.M{
			"pr_array.k": bson.M{"$regex": "^" + regexEscape(pathPrefix)},
		}},
		bson.M{"$group": bson.M{
			"_id":  "$pr_array.k",
			"hits": bson.M{"$sum": "$pr_array.v"},
		}},
		bson.M{"$sort": bson.M{"hits": -1}},
		bson.M{"$limit": limit},
	}
	var raw []struct {
		Key  string `bson:"_id"`
		Hits int    `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil
	}
	results := make([]ReferrerStat, 0, len(raw))
	for _, r := range raw {
		parts := strings.SplitN(unescapeMongoKey(r.Key), "||", 2)
		domain := ""
		if len(parts) == 2 {
			domain = parts[1]
		}
		results = append(results, ReferrerStat{Domain: domain, Hits: r.Hits})
	}
	return results
}

// queryPrefFieldByRef runs $objectToArray on a per-page referrer field, filtered by referrer suffix.
func (s *AnalyticsService) queryPrefFieldByRef(ctx context.Context, sinceKey, untilKey, field, refSuffix string, limit int) []PageStat {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": sinceKey, "$lt": untilKey},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"pr_array": bson.M{"$objectToArray": "$" + field},
		}},
		bson.M{"$unwind": "$pr_array"},
		bson.M{"$match": bson.M{
			"pr_array.k": bson.M{"$regex": regexEscape(refSuffix) + "$"},
		}},
		bson.M{"$group": bson.M{
			"_id":   "$pr_array.k",
			"views": bson.M{"$sum": "$pr_array.v"},
		}},
		bson.M{"$sort": bson.M{"views": -1}},
		bson.M{"$limit": limit},
	}
	var raw []struct {
		Key   string `bson:"_id"`
		Views int    `bson:"views"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil
	}
	results := make([]PageStat, 0, len(raw))
	for _, r := range raw {
		parts := strings.SplitN(unescapeMongoKey(r.Key), "||", 2)
		results = append(results, PageStat{Path: parts[0], Views: r.Views})
	}
	return results
}

// queryRefFieldSum returns the sum of a specific key within a referrer field.
func (s *AnalyticsService) queryRefFieldSum(ctx context.Context, sinceKey, untilKey, field, safeKey string) int {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id":                       hourlyUserID,
			"date":                          bson.M{"$gte": sinceKey, "$lt": untilKey},
			field + "." + safeKey: bson.M{"$exists": true},
		}},
		bson.M{"$group": bson.M{
			"_id":  nil,
			"hits": bson.M{"$sum": "$" + field + "." + safeKey},
		}},
	}
	var raw []struct {
		Hits int `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil || len(raw) == 0 {
		return 0
	}
	return raw[0].Hits
}

func regexEscape(s string) string {
	special := `\.+*?^${}()|[]`
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(special, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// HourlyStatsJSON returns the JSON representation of hourly stats for chart rendering.
func HourlyStatsJSON(stats []HourlyStat) string {
	if stats == nil {
		stats = []HourlyStat{}
	}
	b, _ := json.Marshal(stats)
	return string(b)
}

// UserAgentStat represents a browser/device category and its hit count.
type UserAgentStat struct {
	Category string `json:"category"`
	Hits     int    `json:"hits"`
}

// GetUserAgents returns user agent category breakdown for the given time range.
func (s *AnalyticsService) GetUserAgents(ctx context.Context, since, until time.Time) ([]UserAgentStat, error) {
	sinceKey := hourKey(since)
	untilKey := hourKey(until)

	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id":      hourlyUserID,
			"date":         bson.M{"$gte": sinceKey, "$lt": untilKey},
			"user_agents":  bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{
			"ua_array": bson.M{"$objectToArray": "$user_agents"},
		}},
		bson.M{"$unwind": "$ua_array"},
		bson.M{"$group": bson.M{
			"_id":  "$ua_array.k",
			"hits": bson.M{"$sum": "$ua_array.v"},
		}},
		bson.M{"$sort": bson.M{"hits": -1}},
	}

	var raw []struct {
		Key  string `bson:"_id"`
		Hits int    `bson:"hits"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}

	results := make([]UserAgentStat, 0, len(raw))
	for _, r := range raw {
		results = append(results, UserAgentStat{
			Category: unescapeMongoKey(r.Key),
			Hits:     r.Hits,
		})
	}
	return results, nil
}
