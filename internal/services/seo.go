package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Crawler policy values.
const (
	CrawlerAllow    = "allow"
	CrawlerDisallow = "disallow"
)

const (
	seoConfigType     = "seo_config"
	seoConfigCacheTTL = 30 * time.Second
	DefaultFeedLimit  = 50
	MaxFeedLimit      = 500
)

// DefaultFeedTemplates are the template names included in feeds unless configured.
var DefaultFeedTemplates = []string{"Blog Post", "Press Release"}

// DefaultFeedCategories are the categories included in feeds unless configured.
var DefaultFeedCategories = []string{"blog"}

// SEOConfig holds the site's search & AI settings. Stored in the settings
// collection as type "seo_config" — separate from SiteConfig, whose save
// $sets the whole struct.
//
// Zero values mean "today's behavior": every crawler allowed, no extra
// robots.txt lines, Markdown copies on, default feed selection.
type SEOConfig struct {
	// robots.txt crawler policy, per purpose: "allow" | "disallow".
	TrainingPolicy  string `bson:"training_policy" json:"training_policy"`
	AISearchPolicy  string `bson:"ai_search_policy" json:"ai_search_policy"`
	UserFetchPolicy string `bson:"user_fetch_policy" json:"user_fetch_policy"`
	// Per-crawler overrides by robots token, e.g. {"GPTBot": "disallow"}.
	CrawlerOverrides map[string]string `bson:"crawler_overrides,omitempty" json:"crawler_overrides,omitempty"`
	// Emit a Content-Signal line (search / ai-input / ai-train) derived from the policy.
	ContentSignals bool `bson:"content_signals" json:"content_signals"`
	// Extra lines appended to robots.txt verbatim.
	RobotsExtra string `bson:"robots_extra,omitempty" json:"robots_extra,omitempty"`

	// Markdown copies of pages at /<path>.md.
	MarkdownDisabled bool `bson:"markdown_disabled" json:"markdown_disabled"`

	// Default author for structured data and feeds ("Person" | "Organization").
	AuthorType   string   `bson:"author_type,omitempty" json:"author_type,omitempty"`
	AuthorName   string   `bson:"author_name,omitempty" json:"author_name,omitempty"`
	AuthorURL    string   `bson:"author_url,omitempty" json:"author_url,omitempty"`
	AuthorSameAs []string `bson:"author_same_as,omitempty" json:"author_same_as,omitempty"`
	// Publisher (the site's organization) profile links, used on the homepage Organization.
	PublisherSameAs []string `bson:"publisher_same_as,omitempty" json:"publisher_same_as,omitempty"`

	// Feeds (/feed.xml, /atom.xml, /<collection>/feed.xml).
	FeedDisabled   bool     `bson:"feed_disabled" json:"feed_disabled"`
	FeedAllPages   bool     `bson:"feed_all_pages" json:"feed_all_pages"` // include every page, ignoring templates/categories
	FeedTemplates  []string `bson:"feed_templates,omitempty" json:"feed_templates,omitempty"`
	FeedCategories []string `bson:"feed_categories,omitempty" json:"feed_categories,omitempty"`
	FeedLimit      int      `bson:"feed_limit,omitempty" json:"feed_limit,omitempty"`

	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// withDefaults fills unset fields with their defaults (for reading).
func (c SEOConfig) withDefaults() SEOConfig {
	if c.TrainingPolicy == "" {
		c.TrainingPolicy = CrawlerAllow
	}
	if c.AISearchPolicy == "" {
		c.AISearchPolicy = CrawlerAllow
	}
	if c.UserFetchPolicy == "" {
		c.UserFetchPolicy = CrawlerAllow
	}
	if c.FeedLimit <= 0 {
		c.FeedLimit = DefaultFeedLimit
	}
	if !c.FeedAllPages && len(c.FeedTemplates) == 0 && len(c.FeedCategories) == 0 {
		c.FeedTemplates = append([]string(nil), DefaultFeedTemplates...)
		c.FeedCategories = append([]string(nil), DefaultFeedCategories...)
	}
	return c
}

// Validate checks values and normalizes lists.
func (c *SEOConfig) Validate() error {
	for name, v := range map[string]*string{"training_policy": &c.TrainingPolicy, "ai_search_policy": &c.AISearchPolicy, "user_fetch_policy": &c.UserFetchPolicy} {
		*v = strings.ToLower(strings.TrimSpace(*v))
		if *v != "" && *v != CrawlerAllow && *v != CrawlerDisallow {
			return fmt.Errorf("%s must be \"allow\" or \"disallow\"", name)
		}
	}
	for token, v := range c.CrawlerOverrides {
		if CrawlerByToken(token) == nil {
			return fmt.Errorf("unknown crawler %q in crawler_overrides", token)
		}
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || v == "default" {
			delete(c.CrawlerOverrides, token)
			continue
		}
		if v != CrawlerAllow && v != CrawlerDisallow {
			return fmt.Errorf("crawler_overrides[%s] must be \"allow\" or \"disallow\"", token)
		}
		c.CrawlerOverrides[token] = v
	}
	c.AuthorType = strings.TrimSpace(c.AuthorType)
	if c.AuthorType != "" && c.AuthorType != "Person" && c.AuthorType != "Organization" {
		return fmt.Errorf("author_type must be \"Person\" or \"Organization\"")
	}
	c.AuthorName = strings.TrimSpace(c.AuthorName)
	c.AuthorURL = strings.TrimSpace(c.AuthorURL)
	c.AuthorSameAs = cleanList(c.AuthorSameAs)
	c.PublisherSameAs = cleanList(c.PublisherSameAs)
	for _, u := range append([]string{c.AuthorURL}, append(c.AuthorSameAs, c.PublisherSameAs...)...) {
		if u == "" {
			continue
		}
		pu, err := url.Parse(u)
		if err != nil || (pu.Scheme != "https" && pu.Scheme != "http") || pu.Host == "" {
			return fmt.Errorf("%q is not an absolute http(s) URL", u)
		}
	}
	if c.FeedLimit < 0 || c.FeedLimit > MaxFeedLimit {
		return fmt.Errorf("feed_limit must be between 1 and %d", MaxFeedLimit)
	}
	c.FeedTemplates = cleanList(c.FeedTemplates)
	c.FeedCategories = cleanList(c.FeedCategories)
	c.RobotsExtra = strings.TrimSpace(strings.ReplaceAll(c.RobotsExtra, "\r\n", "\n"))
	return nil
}

func cleanList(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// CrawlerAllowed reports whether a known crawler is allowed under this policy.
// Classic search crawlers are only restricted by an explicit override.
func (c SEOConfig) CrawlerAllowed(k *KnownCrawler) bool {
	if v, ok := c.CrawlerOverrides[k.Token]; ok {
		return v != CrawlerDisallow
	}
	switch k.Purpose {
	case CrawlerPurposeTraining:
		return c.TrainingPolicy != CrawlerDisallow
	case CrawlerPurposeAISearch:
		return c.AISearchPolicy != CrawlerDisallow
	case CrawlerPurposeUserFetch:
		return c.UserFetchPolicy != CrawlerDisallow
	}
	return true
}

// RobotsTxt renders robots.txt for this policy.
func (c SEOConfig) RobotsTxt(baseURL string, feedURLs []string) string {
	c = c.withDefaults()
	var sb strings.Builder

	var blocked []string
	for i := range KnownCrawlers {
		k := &KnownCrawlers[i]
		if !c.CrawlerAllowed(k) {
			blocked = append(blocked, k.Token)
		}
	}
	sort.Strings(blocked)

	sb.WriteString("User-agent: *\n")
	if c.ContentSignals {
		yn := func(b bool) string {
			if b {
				return "yes"
			}
			return "no"
		}
		// Content Signals (contentsignals.org): how content may be used once accessed.
		sb.WriteString(fmt.Sprintf("Content-Signal: search=yes, ai-input=%s, ai-train=%s\n",
			yn(c.AISearchPolicy != CrawlerDisallow || c.UserFetchPolicy != CrawlerDisallow),
			yn(c.TrainingPolicy != CrawlerDisallow)))
	}
	sb.WriteString("Allow: /\n")

	if len(blocked) > 0 {
		sb.WriteString("\n# AI crawler policy (LightCMS: Tools → SEO & AI)\n")
		for _, token := range blocked {
			sb.WriteString("User-agent: " + token + "\n")
		}
		sb.WriteString("Disallow: /\n")
	}

	if c.RobotsExtra != "" {
		sb.WriteString("\n" + c.RobotsExtra + "\n")
	}

	sb.WriteString("\nSitemap: " + strings.TrimRight(baseURL, "/") + "/sitemap.xml\n")
	for _, f := range feedURLs {
		sb.WriteString("# Feed: " + f + "\n")
	}
	return sb.String()
}

// SEOService loads and saves SEOConfig with a short in-memory cache (it is
// read on every page render and every robots.txt / feed request).
type SEOService struct {
	db *database.DB

	mu       sync.Mutex
	cached   *SEOConfig
	cachedAt time.Time
}

// NewSEOService creates the service.
func NewSEOService(db *database.DB) *SEOService { return &SEOService{db: db} }

// Get returns the config with defaults applied. Never fails: on a DB error
// it returns the last cached value or the defaults.
func (s *SEOService) Get(ctx context.Context) SEOConfig {
	if s == nil {
		return SEOConfig{}.withDefaults()
	}
	s.mu.Lock()
	if s.cached != nil && time.Since(s.cachedAt) < seoConfigCacheTTL {
		c := *s.cached
		s.mu.Unlock()
		return c.withDefaults()
	}
	s.mu.Unlock()

	var doc struct {
		Config SEOConfig `bson:"config"`
	}
	err := s.db.Settings().FindOne(ctx, bson.M{"type": seoConfigType}).Decode(&doc)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.cached != nil {
			return s.cached.withDefaults()
		}
		return SEOConfig{}.withDefaults()
	}
	s.mu.Lock()
	s.cached, s.cachedAt = &doc.Config, time.Now()
	s.mu.Unlock()
	return doc.Config.withDefaults()
}

// GetRaw returns the stored config without defaults (for editing forms).
func (s *SEOService) GetRaw(ctx context.Context) SEOConfig {
	var doc struct {
		Config SEOConfig `bson:"config"`
	}
	s.db.Settings().FindOne(ctx, bson.M{"type": seoConfigType}).Decode(&doc) //nolint:errcheck
	return doc.Config
}

// Save validates and stores the config.
func (s *SEOService) Save(ctx context.Context, cfg SEOConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg.UpdatedAt = time.Now()
	_, err := s.db.Settings().UpdateOne(ctx,
		bson.M{"type": seoConfigType},
		bson.M{"$set": bson.M{"config": cfg}, "$setOnInsert": bson.M{"type": seoConfigType}},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("save seo config: %w", err)
	}
	s.mu.Lock()
	s.cached, s.cachedAt = &cfg, time.Now()
	s.mu.Unlock()
	return nil
}
