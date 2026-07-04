package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/net/proxy"
)

const (
	voyageEmbedURL    = "https://api.voyageai.com/v1/embeddings"
	voyageModel       = "voyage-4-lite"
	embeddingDims     = 1024
	vectorSearchIndex = "content_vector_search"
	maxSnippetLen     = 200
	maxBatchSize      = 128
)

// warpClient returns an *http.Client that routes through the Cloudflare WARP SOCKS5 proxy
// at 127.0.0.1:40000 (set up in start.sh on Fly.io). Returns nil if WARP is unavailable.
var warpClient = sync.OnceValue(func() *http.Client {
	const warpAddr = "127.0.0.1:40000"
	conn, err := net.DialTimeout("tcp", warpAddr, 2*time.Second)
	if err != nil {
		log.Printf("WARP proxy not available at %s: %v", warpAddr, err)
		return nil
	}
	conn.Close()

	dialer, err := proxy.SOCKS5("tcp", warpAddr, nil, proxy.Direct)
	if err != nil {
		log.Printf("WARP SOCKS5 setup failed: %v", err)
		return nil
	}
	log.Printf("WARP proxy available at %s — Voyage API calls will route through Cloudflare", warpAddr)
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
	}
})

// SearchResult represents a single search result
type SearchResult struct {
	ContentID string  `json:"id"`
	Title     string  `json:"title"`
	FullPath  string  `json:"full_path"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
	MatchType string  `json:"match_type"`
}

// SuggestResult holds typeahead suggestions
type SuggestResult struct {
	Keywords []string      `json:"keywords"`
	Pages    []SuggestPage `json:"pages"`
}

// SuggestPage is a title+path pair for direct navigation
type SuggestPage struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// SearchService handles end-user search with full-text and semantic (vector) search
type SearchService struct {
	db           *database.DB
	voyageAPIKey string
	// Local/self-hosted embedding provider (Ollama). When configured via
	// LIGHTCMS_EMBEDDINGS_PROVIDER=ollama, embeddings are generated locally
	// with no external API or per-call cost.
	embedProvider string // "voyage" (default) or "ollama"
	ollamaURL     string
	ollamaModel   string
	httpClient    *http.Client

	keywordsMu sync.RWMutex
	keywords   []string // cached extracted keywords, sorted by frequency desc

	navPathsMu       sync.RWMutex
	navPaths         []string // cached internal paths from site nav header HTML
	navPathsCachedAt time.Time

	searchConfigMu       sync.RWMutex
	cachedSearchConfig   *database.SearchConfig
	searchConfigCachedAt time.Time
}

// NewSearchService creates a new search service
func NewSearchService(db *database.DB, voyageAPIKey string) *SearchService {
	// Use WARP proxy for Voyage API calls if available (avoids IP-based WAF blocks)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if wc := warpClient(); wc != nil {
		httpClient = wc
	}
	return &SearchService{
		db:            db,
		voyageAPIKey:  voyageAPIKey,
		embedProvider: strings.ToLower(os.Getenv("LIGHTCMS_EMBEDDINGS_PROVIDER")),
		ollamaURL:     strings.TrimRight(getenvDefault("OLLAMA_URL", "http://localhost:11434"), "/"),
		ollamaModel:   getenvDefault("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		httpClient:    httpClient,
	}
}

// HasVoyageKey returns true if a Voyage API key is configured
func (s *SearchService) HasVoyageKey() bool {
	return s.voyageAPIKey != ""
}

// EmbeddingsEnabled reports whether any embedding provider is configured:
// Voyage AI (hosted) or Ollama (local, LIGHTCMS_EMBEDDINGS_PROVIDER=ollama).
// NOTE: the MongoDB Atlas vector index dimensions must match the model
// (voyage-4-lite: 1024, nomic-embed-text: 768).
func (s *SearchService) EmbeddingsEnabled() bool {
	if s.embedProvider == "ollama" {
		return true
	}
	return s.voyageAPIKey != ""
}

// getenvDefault returns the env var value or a default when unset.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var hrefRe = regexp.MustCompile(`href="(/[^"#?][^"]*)"`)

// getNavPaths returns a set of internal paths linked directly from the site navigation header.
// Results are cached for 5 minutes.
func (s *SearchService) getNavPaths(ctx context.Context) map[string]bool {
	s.navPathsMu.RLock()
	cached := s.navPaths
	fresh := time.Since(s.navPathsCachedAt) < 5*time.Minute
	s.navPathsMu.RUnlock()

	if fresh {
		set := make(map[string]bool, len(cached))
		for _, p := range cached {
			set[p] = true
		}
		return set
	}

	theme, err := s.db.GetThemeSettings(ctx)
	if err != nil || theme == nil {
		return nil
	}

	matches := hrefRe.FindAllStringSubmatch(theme.HeaderHTML, -1)
	paths := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		p := m[1]
		if idx := strings.IndexAny(p, "?#"); idx >= 0 {
			p = p[:idx]
		}
		if p != "" && !seen[p] {
			paths = append(paths, p)
			seen[p] = true
		}
	}

	s.navPathsMu.Lock()
	s.navPaths = paths
	s.navPathsCachedAt = time.Now()
	s.navPathsMu.Unlock()

	return seen
}

// getSearchConfig returns the search ranking config, using a 5-minute in-memory cache.
func (s *SearchService) getSearchConfig(ctx context.Context) *database.SearchConfig {
	s.searchConfigMu.RLock()
	cfg := s.cachedSearchConfig
	fresh := cfg != nil && time.Since(s.searchConfigCachedAt) < 5*time.Minute
	s.searchConfigMu.RUnlock()
	if fresh {
		return cfg
	}
	loaded, err := s.db.GetSearchConfig(ctx)
	if err != nil || loaded == nil {
		loaded = database.DefaultSearchConfig()
	}
	// Pre-normalize path/template lists so per-query comparisons skip redundant ToLower/TrimSpace calls.
	for i, p := range loaded.BoostPaths {
		loaded.BoostPaths[i] = strings.ToLower(strings.TrimSpace(p))
	}
	for i, p := range loaded.DemotePaths {
		loaded.DemotePaths[i] = strings.ToLower(strings.TrimSpace(p))
	}
	for i, p := range loaded.DemotePathPrefixes {
		loaded.DemotePathPrefixes[i] = strings.ToLower(strings.TrimSpace(p))
	}
	for i, t := range loaded.BoostTemplates {
		loaded.BoostTemplates[i] = strings.ToLower(strings.TrimSpace(t))
	}
	s.searchConfigMu.Lock()
	s.cachedSearchConfig = loaded
	s.searchConfigCachedAt = time.Now()
	s.searchConfigMu.Unlock()
	return loaded
}

// InvalidateSearchConfigCache clears the cached search config so the next request reloads it from DB.
func (s *SearchService) InvalidateSearchConfigCache() {
	s.searchConfigMu.Lock()
	s.cachedSearchConfig = nil
	s.searchConfigMu.Unlock()
}

// pathBoost returns a score bonus based on structural importance using the provided config.
// Nav-linked pages rank highest; template-boosted pages get a moderate boost;
// demoted path-prefix pages are penalised.
func pathBoost(fullPath, templateName string, navPaths map[string]bool, cfg *database.SearchConfig) float64 {
	if navPaths[fullPath] {
		return cfg.NavBoost
	}
	// cfg paths/templates are pre-lowercased by getSearchConfig, so only lowercase the inputs.
	lower := strings.ToLower(fullPath)
	// Explicitly boosted pages (exact path match)
	for _, p := range cfg.BoostPaths {
		if lower == p {
			return cfg.BoostPathScore
		}
	}
	// Explicitly demoted pages (exact path match)
	for _, p := range cfg.DemotePaths {
		if lower == p {
			return cfg.DemoteScore
		}
	}
	for _, prefix := range cfg.DemotePathPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return cfg.DemoteScore
		}
	}
	lowerTemplate := strings.ToLower(templateName)
	for _, tmpl := range cfg.BoostTemplates {
		if strings.Contains(lowerTemplate, tmpl) {
			return cfg.BoostTemplateScore
		}
	}
	return 0
}

// isDemotedPath returns true if fullPath starts with any configured demote prefix.
func isDemotedPath(fullPath string, cfg *database.SearchConfig) bool {
	lower := strings.ToLower(fullPath)
	for _, prefix := range cfg.DemotePathPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isBoostedTemplate returns true if templateName matches any configured boost template.
func isBoostedTemplate(templateName string, cfg *database.SearchConfig) bool {
	lower := strings.ToLower(templateName)
	for _, tmpl := range cfg.BoostTemplates {
		if strings.Contains(lower, tmpl) {
			return true
		}
	}
	return false
}

// Search performs a combined search using the specified mode
// Falls back to exact search when semantic search is unavailable (no Voyage API key)
func (s *SearchService) Search(ctx context.Context, query, mode string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	switch mode {
	case "exact":
		return s.SearchFullText(ctx, query, limit)
	case "semantic":
		if !s.EmbeddingsEnabled() {
			return s.SearchFullText(ctx, query, limit)
		}
		return s.SearchSemantic(ctx, query, limit)
	default: // "hybrid"
		return s.SearchHybrid(ctx, query, limit)
	}
}

// SearchFullText performs regex-based full-text search on plain_text field
func (s *SearchService) SearchFullText(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// Escape regex special characters for literal matching
	escaped := regexp.QuoteMeta(query)

	filter := bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
		"plain_text": bson.M{
			"$regex":   escaped,
			"$options": "i",
		},
	}

	// Impose a hard deadline on the regex scan to prevent CPU exhaustion on
	// large corpora. 5 s is generous for normal queries but stops pathological cases.
	qCtx, qCancel := context.WithTimeout(ctx, 5*time.Second)
	defer qCancel()

	var contents []models.Content
	if err := s.db.FindAll(qCtx, "content", filter, &contents); err != nil {
		if qCtx.Err() != nil {
			return nil, fmt.Errorf("search query timed out")
		}
		return nil, fmt.Errorf("full-text search failed: %w", err)
	}

	lowerQuery := strings.ToLower(query)
	navPaths := s.getNavPaths(ctx)
	cfg := s.getSearchConfig(ctx)

	// Collect all matches with tier: 0=title+nav, 1=title, 2=nav, 3=boosted-template, 4=body-only, 5=demoted
	type rankedResult struct {
		result SearchResult
		tier   int
	}
	var ranked []rankedResult

	for _, c := range contents {
		snippet := extractSnippet(c.PlainText, lowerQuery)
		titleMatch := strings.Contains(strings.ToLower(c.Title), lowerQuery)
		nav := navPaths[c.FullPath]
		isBoosted := isBoostedTemplate(c.TemplateName, cfg)
		isDemoted := isDemotedPath(c.FullPath, cfg)

		// Tiers: 0=title+nav, 1=title, 2=nav, 3=boosted-template, 4=body-only, 5=demoted
		tier := 4
		switch {
		case titleMatch && nav:
			tier = 0
		case titleMatch:
			tier = 1
		case nav:
			tier = 2
		case isBoosted:
			tier = 3
		case isDemoted:
			tier = 5
		}

		ranked = append(ranked, rankedResult{
			result: SearchResult{
				ContentID: c.ID.Hex(),
				Title:     c.Title,
				FullPath:  c.FullPath,
				Snippet:   snippet,
				MatchType: "exact",
			},
			tier: tier,
		})
	}

	// Sort by tier (lower = better), stable
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].tier < ranked[j].tier
	})

	results := make([]SearchResult, 0, limit)
	for i, r := range ranked {
		if i >= limit {
			break
		}
		r.result.Score = 1.0 - float64(i)*0.01
		results = append(results, r.result)
	}

	return results, nil
}

// SearchSemantic performs vector similarity search using MongoDB Atlas $vectorSearch
func (s *SearchService) SearchSemantic(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if !s.EmbeddingsEnabled() {
		return nil, fmt.Errorf("semantic search unavailable: no embedding provider configured (set VOYAGE_API_KEY or LIGHTCMS_EMBEDDINGS_PROVIDER=ollama)")
	}

	queryEmbedding, err := s.generateEmbedding(ctx, query, "query")
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Convert []float32 to []interface{} for bson
	queryVec := make([]interface{}, len(queryEmbedding))
	for i, v := range queryEmbedding {
		queryVec[i] = v
	}

	pipeline := []bson.M{
		{
			"$vectorSearch": bson.M{
				"index":         vectorSearchIndex,
				"path":          "embedding",
				"queryVector":   queryEmbedding,
				"numCandidates": limit * 10,
				"limit":         limit,
				"filter": bson.M{
					"published": true,
					"deleted":   bson.M{"$ne": true},
				},
			},
		},
		{
			"$project": bson.M{
				"title":      1,
				"full_path":  1,
				"plain_text": 1,
				"score":      bson.M{"$meta": "vectorSearchScore"},
			},
		},
	}

	type vectorResult struct {
		ID        primitive.ObjectID `bson:"_id"`
		Title     string             `bson:"title"`
		FullPath  string             `bson:"full_path"`
		PlainText string             `bson:"plain_text"`
		Score     float64            `bson:"score"`
	}

	var vResults []vectorResult
	if err := s.db.Aggregate(ctx, "content", pipeline, &vResults); err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(vResults))
	for _, vr := range vResults {
		snippet := truncateText(vr.PlainText, maxSnippetLen)
		results = append(results, SearchResult{
			ContentID: vr.ID.Hex(),
			Title:     vr.Title,
			FullPath:  vr.FullPath,
			Snippet:   snippet,
			Score:     vr.Score,
			MatchType: "semantic",
		})
	}

	return results, nil
}

// SearchHybrid merges exact and semantic results using reciprocal rank fusion
func (s *SearchService) SearchHybrid(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	type searchOutcome struct {
		results []SearchResult
		err     error
	}
	exactCh := make(chan searchOutcome, 1)
	semanticCh := make(chan searchOutcome, 1)

	go func() {
		r, e := s.SearchFullText(ctx, query, limit*2)
		exactCh <- searchOutcome{r, e}
	}()
	go func() {
		r, e := s.SearchSemantic(ctx, query, limit*2)
		semanticCh <- searchOutcome{r, e}
	}()

	exactOut := <-exactCh
	semanticOut := <-semanticCh
	exactResults, exactErr := exactOut.results, exactOut.err
	semanticResults, semanticErr := semanticOut.results, semanticOut.err

	// If both fail, return the first error
	if exactErr != nil && semanticErr != nil {
		return nil, exactErr
	}

	// Merge with reciprocal rank fusion
	const k = 60.0 // RRF constant
	lowerQuery := strings.ToLower(query)
	navPaths := s.getNavPaths(ctx)
	cfg := s.getSearchConfig(ctx)
	scores := make(map[string]float64)
	resultMap := make(map[string]*SearchResult)

	for i, r := range exactResults {
		scores[r.ContentID] += 1.0 / (k + float64(i+1))
		copy := r
		resultMap[r.ContentID] = &copy
	}

	for i, r := range semanticResults {
		scores[r.ContentID] += 1.0 / (k + float64(i+1))
		if existing, ok := resultMap[r.ContentID]; ok {
			existing.MatchType = "both"
		} else {
			copy := r
			resultMap[r.ContentID] = &copy
		}
	}

	// Apply title boost and structural boost (nav up, demoted paths down, template boost)
	for id, r := range resultMap {
		if strings.Contains(strings.ToLower(r.Title), lowerQuery) {
			scores[id] += cfg.TitleBoost
		}
		scores[id] += pathBoost(r.FullPath, "", navPaths, cfg)
	}

	// Sort by RRF score
	type scored struct {
		id    string
		score float64
	}
	var sortable []scored
	for id, score := range scores {
		sortable = append(sortable, scored{id, score})
	}
	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].score > sortable[j].score
	})

	results := make([]SearchResult, 0, limit)
	for _, s := range sortable {
		if len(results) >= limit {
			break
		}
		r := resultMap[s.id]
		r.Score = s.score
		results = append(results, *r)
	}

	return results, nil
}

// UpdateContentEmbedding generates and stores an embedding for the given content
func (s *SearchService) UpdateContentEmbedding(ctx context.Context, contentID primitive.ObjectID) error {
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": contentID}, &content); err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	plainText := ExtractPlainText(&content)
	if plainText == "" {
		return nil // Nothing to embed
	}

	embedding, err := s.generateEmbedding(ctx, plainText, "document")
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"embedding":    embedding,
			"embedding_at": now,
			"plain_text":   plainText,
		},
	}

	return s.db.UpdateOne(ctx, "content", bson.M{"_id": contentID}, update)
}

// BatchGenerateEmbeddings generates embeddings for all published content that needs them
func (s *SearchService) BatchGenerateEmbeddings(ctx context.Context) (processed, errCount int, err error) {
	if !s.EmbeddingsEnabled() {
		return 0, 0, fmt.Errorf("no embedding provider configured (set VOYAGE_API_KEY or LIGHTCMS_EMBEDDINGS_PROVIDER=ollama)")
	}

	// Find all published, non-deleted content
	filter := bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
	}

	var contents []models.Content
	if err := s.db.FindAll(ctx, "content", filter, &contents); err != nil {
		return 0, 0, fmt.Errorf("failed to list content: %w", err)
	}

	for _, c := range contents {
		plainText := ExtractPlainText(&c)
		if plainText == "" {
			continue
		}

		// Skip if embedding is up to date
		if c.EmbeddingAt != nil && c.EmbeddingAt.After(c.UpdatedAt) && c.PlainText == plainText {
			continue
		}

		embedding, genErr := s.generateEmbedding(ctx, plainText, "document")
		if genErr != nil {
			log.Printf("Failed to generate embedding for %s: %v", c.FullPath, genErr)
			errCount++
			continue
		}

		now := time.Now()
		update := bson.M{
			"$set": bson.M{
				"embedding":    embedding,
				"embedding_at": now,
				"plain_text":   plainText,
			},
		}

		if updateErr := s.db.UpdateOne(ctx, "content", bson.M{"_id": c.ID}, update); updateErr != nil {
			log.Printf("Failed to save embedding for %s: %v", c.FullPath, updateErr)
			errCount++
			continue
		}

		processed++
	}

	return processed, errCount, nil
}

// EmbeddingStats returns counts of content with and without embeddings
func (s *SearchService) EmbeddingStats(ctx context.Context) (total, withEmbedding int64, err error) {
	publishedFilter := bson.M{"published": true, "deleted": bson.M{"$ne": true}}
	total, err = s.db.Count(ctx, "content", publishedFilter)
	if err != nil {
		return
	}

	embeddedFilter := bson.M{
		"published":    true,
		"deleted":      bson.M{"$ne": true},
		"embedding_at": bson.M{"$exists": true},
	}
	withEmbedding, err = s.db.Count(ctx, "content", embeddedFilter)
	return
}

// Suggest returns typeahead suggestions: matching keywords + title prefix matches
func (s *SearchService) Suggest(ctx context.Context, prefix string, limit int) (*SuggestResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	lowerPrefix := strings.ToLower(prefix)

	// 1. Match cached keywords
	s.keywordsMu.RLock()
	allKeywords := s.keywords
	s.keywordsMu.RUnlock()

	var matchedKeywords []string
	for _, kw := range allKeywords {
		if strings.Contains(kw, lowerPrefix) {
			matchedKeywords = append(matchedKeywords, kw)
			if len(matchedKeywords) >= limit {
				break
			}
		}
	}

	// 2. Title prefix match from DB
	escaped := regexp.QuoteMeta(prefix)
	filter := bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
		"title": bson.M{
			"$regex":   escaped,
			"$options": "i",
		},
	}

	var contents []models.Content
	if err := s.db.FindAll(ctx, "content", filter, &contents); err != nil {
		return nil, fmt.Errorf("suggest title lookup failed: %w", err)
	}

	// Sort page suggestions:
	// 0 = nav-linked + starts-with, 1 = nav-linked + contains
	// 2 = concept page + starts-with, 3 = concept page + contains
	// 4 = other + starts-with, 5 = other + contains
	// 6 = demoted path (any)
	navPaths := s.getNavPaths(ctx)
	cfg := s.getSearchConfig(ctx)
	type scoredPage struct {
		page SuggestPage
		rank int
	}
	var scored []scoredPage
	for _, c := range contents {
		nav := navPaths[c.FullPath]
		isBoosted := isBoostedTemplate(c.TemplateName, cfg)
		isDemoted := isDemotedPath(c.FullPath, cfg)
		startsWith := strings.HasPrefix(strings.ToLower(c.Title), lowerPrefix)

		rank := 5
		switch {
		case isDemoted:
			rank = 6
		case nav && startsWith:
			rank = 0
		case nav:
			rank = 1
		case isBoosted && startsWith:
			rank = 2
		case isBoosted:
			rank = 3
		case startsWith:
			rank = 4
		}
		scored = append(scored, scoredPage{
			page: SuggestPage{Title: c.Title, Path: c.FullPath},
			rank: rank,
		})
	}
	// Stable sort by rank (lower = better)
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].rank < scored[j].rank
	})
	var pages []SuggestPage
	for i, sp := range scored {
		if i >= limit {
			break
		}
		pages = append(pages, sp.page)
	}

	return &SuggestResult{
		Keywords: matchedKeywords,
		Pages:    pages,
	}, nil
}

// RebuildKeywords scans all published content and extracts common keywords.
// Call this on startup and after content publish/unpublish/delete.
func (s *SearchService) RebuildKeywords(ctx context.Context) error {
	filter := bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
	}

	var contents []models.Content
	if err := s.db.FindAll(ctx, "content", filter, &contents); err != nil {
		return fmt.Errorf("failed to list content for keywords: %w", err)
	}

	// Count frequency of 1-3 word phrases from titles and meta descriptions
	freq := make(map[string]int)
	for _, c := range contents {
		extractPhrases(c.Title, freq)
		extractPhrases(c.MetaDescription, freq)
	}

	// Filter to phrases that appear in at least 2 pieces of content, or are from titles
	// Sort by frequency descending
	type kf struct {
		keyword string
		count   int
	}
	var sorted []kf
	for phrase, count := range freq {
		if count >= 2 && len(phrase) > 2 {
			sorted = append(sorted, kf{phrase, count})
		}
	}
	// Sort by frequency desc, then alphabetically
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].keyword < sorted[j].keyword
	})

	keywords := make([]string, 0, len(sorted))
	for _, kv := range sorted {
		keywords = append(keywords, kv.keyword)
		if len(keywords) >= 500 { // cap at 500 keywords
			break
		}
	}

	s.keywordsMu.Lock()
	s.keywords = keywords
	s.keywordsMu.Unlock()

	log.Printf("Search keywords rebuilt: %d terms from %d pages", len(keywords), len(contents))
	return nil
}

// stopWords are common English words to exclude from keyword extraction
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "is": true, "it": true, "as": true,
	"be": true, "was": true, "are": true, "were": true, "been": true, "has": true,
	"have": true, "had": true, "do": true, "does": true, "did": true, "will": true,
	"would": true, "could": true, "should": true, "may": true, "might": true,
	"not": true, "no": true, "if": true, "then": true, "than": true, "so": true,
	"up": true, "out": true, "about": true, "into": true, "over": true, "after": true,
	"this": true, "that": true, "these": true, "those": true, "what": true, "which": true,
	"who": true, "how": true, "when": true, "where": true, "why": true, "all": true,
	"each": true, "every": true, "both": true, "few": true, "more": true, "most": true,
	"other": true, "some": true, "such": true, "only": true, "own": true, "same": true,
	"also": true, "can": true, "just": true, "your": true, "you": true, "we": true,
	"our": true, "its": true, "his": true, "her": true, "my": true, "their": true,
	"i": true, "me": true, "he": true, "she": true, "they": true, "them": true,
	"us": true, "him": true, "am": true, "get": true, "new": true,
}

// wordSplitter splits text on non-alphanumeric characters
var wordSplitter = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// extractPhrases extracts 1-3 word phrases from text and adds them to the frequency map
func extractPhrases(text string, freq map[string]int) {
	if text == "" {
		return
	}
	// Clean and split into words
	cleaned := wordSplitter.ReplaceAllString(text, " ")
	words := strings.Fields(strings.ToLower(cleaned))

	// Extract single words (excluding stop words and short words)
	for _, w := range words {
		if len(w) > 2 && !stopWords[w] {
			freq[w]++
		}
	}

	// Extract 2-word phrases
	for i := 0; i < len(words)-1; i++ {
		if stopWords[words[i]] && stopWords[words[i+1]] {
			continue
		}
		phrase := words[i] + " " + words[i+1]
		if len(phrase) > 4 {
			freq[phrase]++
		}
	}

	// Extract 3-word phrases
	for i := 0; i < len(words)-2; i++ {
		// Skip if all three are stop words
		if stopWords[words[i]] && stopWords[words[i+1]] && stopWords[words[i+2]] {
			continue
		}
		phrase := words[i] + " " + words[i+1] + " " + words[i+2]
		if len(phrase) > 6 {
			freq[phrase]++
		}
	}
}

// generateEmbedding calls Voyage AI API to generate a vector embedding
func (s *SearchService) generateEmbedding(ctx context.Context, text, inputType string) ([]float32, error) {
	// Truncate very long text (providers have token limits)
	if len(text) > 32000 {
		text = text[:32000]
	}

	if s.embedProvider == "ollama" {
		return s.generateEmbeddingOllama(ctx, text)
	}

	reqBody := map[string]interface{}{
		"model":      voyageModel,
		"input":      []string{text},
		"input_type": inputType,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", voyageEmbedURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.voyageAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read voyage response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voyage API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse voyage response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("voyage API returned no embeddings")
	}

	return result.Data[0].Embedding, nil
}

// htmlTagRegex matches HTML tags for stripping
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// ExtractPlainText strips HTML and concatenates all text content from a Content item
func ExtractPlainText(content *models.Content) string {
	var parts []string

	if content.Title != "" {
		parts = append(parts, content.Title)
	}
	if content.MetaDescription != "" {
		parts = append(parts, content.MetaDescription)
	}

	// Extract text from all data fields
	for _, v := range content.Data {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		// Strip HTML tags
		stripped := htmlTagRegex.ReplaceAllString(s, " ")
		// Collapse whitespace
		stripped = strings.Join(strings.Fields(stripped), " ")
		if stripped != "" {
			parts = append(parts, stripped)
		}
	}

	return strings.Join(parts, " ")
}

// extractSnippet returns a snippet around the first match of query in text
func extractSnippet(text, lowerQuery string) string {
	if text == "" {
		return ""
	}
	lowerText := strings.ToLower(text)
	idx := strings.Index(lowerText, lowerQuery)
	if idx < 0 {
		return truncateText(text, maxSnippetLen)
	}

	// Show context around the match
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(lowerQuery) + 120
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}
	return snippet
}

// truncateText truncates text to maxLen characters at a word boundary
func truncateText(text string, maxLen int) string {
	if utf8.RuneCountInString(text) <= maxLen {
		return text
	}
	runes := []rune(text)
	truncated := string(runes[:maxLen])
	// Try to break at last space
	if idx := strings.LastIndex(truncated, " "); idx > maxLen/2 {
		truncated = truncated[:idx]
	}
	return truncated + "..."
}

// generateEmbeddingOllama generates an embedding with a local Ollama server
// (POST /api/embeddings). Self-hosted, no external API or per-call cost.
func (s *SearchService) generateEmbeddingOllama(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":  s.ollamaModel,
		"prompt": text,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.ollamaURL+"/api/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed (is Ollama running at %s?): %w", s.ollamaURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ollama response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned an empty embedding (model %s)", s.ollamaModel)
	}
	return result.Embedding, nil
}
