package services

import (
	"context"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// RecordCrawlerHit buffers a hit from a known crawler (AI or search) on a
// site path — a page, or an AI-facing endpoint like /llms.txt. Non-crawler
// requests are ignored. No DB write happens here (see flushBuffer).
func (s *AnalyticsService) RecordCrawlerHit(path, rawUA string) {
	if s == nil || path == "" {
		return
	}
	c := IdentifyCrawler(rawUA)
	if c == nil {
		return
	}
	hk := hourKey(time.Now())
	s.bufMu.Lock()
	defer s.bufMu.Unlock()
	if s.bufAIBots[hk] == nil {
		s.bufAIBots[hk] = make(map[string]int)
	}
	s.bufAIBots[hk][c.Token]++
	if s.bufAIBotPages[hk] == nil {
		s.bufAIBotPages[hk] = make(map[string]int)
	}
	s.bufAIBotPages[hk][path+"||"+c.Token]++
}

// CrawlerStat is hit volume for one known crawler.
type CrawlerStat struct {
	Token   string `json:"token"`
	Vendor  string `json:"vendor"`
	Purpose string `json:"purpose"`
	Label   string `json:"purpose_label"`
	Hits    int    `json:"hits"`
}

// PurposeStat is hit volume for one crawler purpose.
type PurposeStat struct {
	Purpose string `json:"purpose"`
	Label   string `json:"label"`
	Hits    int    `json:"hits"`
}

// CrawledPage is a path with its crawler hits, split by purpose.
type CrawledPage struct {
	Path      string         `json:"path"`
	Hits      int            `json:"hits"`
	ByPurpose map[string]int `json:"by_purpose"`
	EditID    string         `json:"edit_id,omitempty"`
}

// AIReferralStat is human visits referred by one AI assistant.
type AIReferralStat struct {
	Assistant string `json:"assistant"`
	Hits      int    `json:"hits"`
}

// DailyPurposeStat is one day's crawler hits by purpose.
type DailyPurposeStat struct {
	Day       string         `json:"day"` // YYYY-MM-DD (UTC)
	ByPurpose map[string]int `json:"by_purpose"`
}

// AITrafficReport is the AI-visibility view of the site's traffic.
type AITrafficReport struct {
	Since            time.Time          `json:"since"`
	Until            time.Time          `json:"until"`
	ByPurpose        []PurposeStat      `json:"by_purpose"`
	Crawlers         []CrawlerStat      `json:"crawlers"`
	TopCrawledPages  []CrawledPage      `json:"top_crawled_pages"`
	Referrals        []AIReferralStat   `json:"ai_referrals"`
	ReferralTotal    int                `json:"ai_referral_total"`
	TopLandingPages  []PageStat         `json:"top_ai_landing_pages"`
	Daily            []DailyPurposeStat `json:"daily"`
	AICrawlerHits    int                `json:"ai_crawler_hits"`    // training + AI search + user fetch
	SearchEngineHits int                `json:"search_engine_hits"` // classic search crawlers
}

// sumObjectField sums an object-of-counters field across hourly docs in range.
func (s *AnalyticsService) sumObjectField(ctx context.Context, since, until time.Time, field string) (map[string]int, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": hourKey(since), "$lt": hourKey(until)},
			field:     bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{"arr": bson.M{"$objectToArray": "$" + field}}},
		bson.M{"$unwind": "$arr"},
		bson.M{"$group": bson.M{"_id": "$arr.k", "n": bson.M{"$sum": "$arr.v"}}},
	}
	var raw []struct {
		Key string `bson:"_id"`
		N   int    `bson:"n"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]int, len(raw))
	for _, r := range raw {
		out[unescapeMongoKey(r.Key)] += r.N
	}
	return out, nil
}

// GetAITraffic builds the AI-visibility report for [since, until).
func (s *AnalyticsService) GetAITraffic(ctx context.Context, since, until time.Time, limit int) (*AITrafficReport, error) {
	if limit <= 0 {
		limit = 20
	}
	rep := &AITrafficReport{Since: since, Until: until}

	// Crawlers and purposes
	bots, err := s.sumObjectField(ctx, since, until, "ai_bots")
	if err != nil {
		return nil, err
	}
	purpose := map[string]int{}
	for token, n := range bots {
		c := CrawlerByToken(token)
		if c == nil {
			continue
		}
		rep.Crawlers = append(rep.Crawlers, CrawlerStat{Token: c.Token, Vendor: c.Vendor, Purpose: c.Purpose, Label: CrawlerPurposeLabels[c.Purpose], Hits: n})
		purpose[c.Purpose] += n
		if c.Purpose == CrawlerPurposeSearch {
			rep.SearchEngineHits += n
		} else {
			rep.AICrawlerHits += n
		}
	}
	sort.Slice(rep.Crawlers, func(i, j int) bool { return rep.Crawlers[i].Hits > rep.Crawlers[j].Hits })
	for _, p := range []string{CrawlerPurposeTraining, CrawlerPurposeAISearch, CrawlerPurposeUserFetch, CrawlerPurposeSearch} {
		rep.ByPurpose = append(rep.ByPurpose, PurposeStat{Purpose: p, Label: CrawlerPurposeLabels[p], Hits: purpose[p]})
	}

	// Pages crawled by AI crawlers (classic search excluded)
	pages, err := s.sumObjectField(ctx, since, until, "ai_bot_pages")
	if err != nil {
		return nil, err
	}
	byPath := map[string]*CrawledPage{}
	for key, n := range pages {
		i := strings.LastIndex(key, "||")
		if i < 0 {
			continue
		}
		c := CrawlerByToken(key[i+2:])
		if c == nil || c.Purpose == CrawlerPurposeSearch {
			continue
		}
		p := byPath[key[:i]]
		if p == nil {
			p = &CrawledPage{Path: key[:i], ByPurpose: map[string]int{}}
			byPath[key[:i]] = p
		}
		p.Hits += n
		p.ByPurpose[c.Purpose] += n
	}
	for _, p := range byPath {
		rep.TopCrawledPages = append(rep.TopCrawledPages, *p)
	}
	sort.Slice(rep.TopCrawledPages, func(i, j int) bool {
		a, b := rep.TopCrawledPages[i], rep.TopCrawledPages[j]
		if a.Hits != b.Hits {
			return a.Hits > b.Hits
		}
		return a.Path < b.Path
	})
	if len(rep.TopCrawledPages) > limit {
		rep.TopCrawledPages = rep.TopCrawledPages[:limit]
	}

	// AI referrals: human referrers whose host is an AI assistant
	refs, err := s.sumObjectField(ctx, since, until, "ref_human")
	if err != nil {
		return nil, err
	}
	assist := map[string]int{}
	for host, n := range refs {
		if name := AIAssistantForHost(host); name != "" {
			assist[name] += n
			rep.ReferralTotal += n
		}
	}
	for name, n := range assist {
		rep.Referrals = append(rep.Referrals, AIReferralStat{Assistant: name, Hits: n})
	}
	sort.Slice(rep.Referrals, func(i, j int) bool { return rep.Referrals[i].Hits > rep.Referrals[j].Hits })

	// Landing pages for AI referrals
	prefs, err := s.sumObjectField(ctx, since, until, "pref_human")
	if err != nil {
		return nil, err
	}
	landing := map[string]int{}
	for key, n := range prefs {
		i := strings.LastIndex(key, "||")
		if i < 0 {
			continue
		}
		if AIAssistantForHost(key[i+2:]) != "" {
			landing[key[:i]] += n
		}
	}
	for path, n := range landing {
		rep.TopLandingPages = append(rep.TopLandingPages, PageStat{Path: path, Views: n})
	}
	sort.Slice(rep.TopLandingPages, func(i, j int) bool { return rep.TopLandingPages[i].Views > rep.TopLandingPages[j].Views })
	if len(rep.TopLandingPages) > limit {
		rep.TopLandingPages = rep.TopLandingPages[:limit]
	}

	// Daily series by purpose
	daily, err := s.dailyCrawlerHits(ctx, since, until)
	if err != nil {
		return nil, err
	}
	rep.Daily = daily
	return rep, nil
}

func (s *AnalyticsService) dailyCrawlerHits(ctx context.Context, since, until time.Time) ([]DailyPurposeStat, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": hourlyUserID,
			"date":    bson.M{"$gte": hourKey(since), "$lt": hourKey(until)},
			"ai_bots": bson.M{"$exists": true},
		}},
		bson.M{"$project": bson.M{"day": bson.M{"$substrBytes": bson.A{"$date", 0, 10}}, "arr": bson.M{"$objectToArray": "$ai_bots"}}},
		bson.M{"$unwind": "$arr"},
		bson.M{"$group": bson.M{"_id": bson.M{"day": "$day", "k": "$arr.k"}, "n": bson.M{"$sum": "$arr.v"}}},
	}
	var raw []struct {
		ID struct {
			Day string `bson:"day"`
			K   string `bson:"k"`
		} `bson:"_id"`
		N int `bson:"n"`
	}
	if err := s.db.Aggregate(ctx, activityCollection, pipeline, &raw); err != nil {
		return nil, err
	}
	days := map[string]map[string]int{}
	for _, r := range raw {
		c := CrawlerByToken(unescapeMongoKey(r.ID.K))
		if c == nil {
			continue
		}
		if days[r.ID.Day] == nil {
			days[r.ID.Day] = map[string]int{}
		}
		days[r.ID.Day][c.Purpose] += r.N
	}
	// Emit every day in range so charts have no gaps.
	var out []DailyPurposeStat
	for d := since.UTC().Truncate(24 * time.Hour); d.Before(until); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		m := days[key]
		if m == nil {
			m = map[string]int{}
		}
		out = append(out, DailyPurposeStat{Day: key, ByPurpose: m})
	}
	return out, nil
}
