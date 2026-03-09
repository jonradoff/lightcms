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
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"lightcms/internal/database"
	"lightcms/internal/models"

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
	Keywords []string       `json:"keywords"`
	Pages    []SuggestPage  `json:"pages"`
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
	httpClient   *http.Client

	keywordsMu sync.RWMutex
	keywords   []string // cached extracted keywords, sorted by frequency desc

	navPathsMu      sync.RWMutex
	navPaths        []string  // cached internal paths from site nav header HTML
	navPathsCachedAt time.Time
}

// NewSearchService creates a new search service
func NewSearchService(db *database.DB, voyageAPIKey string) *SearchService {
	// Use WARP proxy for Voyage API calls if available (avoids IP-based WAF blocks)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if wc := warpClient(); wc != nil {
		httpClient = wc
	}
	return &SearchService{
		db:           db,
		voyageAPIKey: voyageAPIKey,
		httpClient:   httpClient,
	}
}

// HasVoyageKey returns true if a Voyage API key is configured
func (s *SearchService) HasVoyageKey() bool {
	return s.voyageAPIKey != ""
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

// pathBoost returns a score bonus based on structural importance.
// Nav-linked pages rank highest; concept pages get a moderate boost;
// video transcript pages are deprioritised.
func pathBoost(fullPath, templateName string, navPaths map[string]bool) float64 {
	if navPaths[fullPath] {
		return 0.15 // direct nav link — highest structural boost
	}
	lower := strings.ToLower(fullPath)
	if strings.HasPrefix(lower, "/videos/") || strings.HasPrefix(lower, "/video/") {
		return -0.05 // video transcripts — deprioritise
	}
	if strings.Contains(strings.ToLower(templateName), "concept") {
		return 0.05 // concept pages — moderate boost over generic content
	}
	return 0
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
		if !s.HasVoyageKey() {
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

	var contents []models.Content
	if err := s.db.FindAll(ctx, "content", filter, &contents); err != nil {
		return nil, fmt.Errorf("full-text search failed: %w", err)
	}

	lowerQuery := strings.ToLower(query)
	navPaths := s.getNavPaths(ctx)

	// Collect all matches with tier: 0=title+nav, 1=title, 2=nav, 3=other, 4=video transcript
	type rankedResult struct {
		result SearchResult
		tier   int
	}
	var ranked []rankedResult

	for _, c := range contents {
		snippet := extractSnippet(c.PlainText, lowerQuery)
		titleMatch := strings.Contains(strings.ToLower(c.Title), lowerQuery)
		nav := navPaths[c.FullPath]
		lower := strings.ToLower(c.FullPath)
		isVideo := strings.HasPrefix(lower, "/videos/") || strings.HasPrefix(lower, "/video/")

		isConcept := strings.Contains(strings.ToLower(c.TemplateName), "concept")
		// Tiers: 0=title+nav, 1=title, 2=nav, 3=concept, 4=body-only, 5=video transcript
		tier := 4
		switch {
		case titleMatch && nav:
			tier = 0
		case titleMatch:
			tier = 1
		case nav:
			tier = 2
		case isConcept:
			tier = 3
		case isVideo:
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

	// Stable insertion sort by tier (lower = better)
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].tier < ranked[j-1].tier; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}

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
	if !s.HasVoyageKey() {
		return nil, fmt.Errorf("semantic search unavailable: no Voyage API key configured")
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
	exactResults, exactErr := s.SearchFullText(ctx, query, limit)
	semanticResults, semanticErr := s.SearchSemantic(ctx, query, limit)

	// If both fail, return the first error
	if exactErr != nil && semanticErr != nil {
		return nil, exactErr
	}

	// Merge with reciprocal rank fusion
	const k = 60.0             // RRF constant
	const titleBoost = 1.0 / 5 // Significant boost for title matches
	lowerQuery := strings.ToLower(query)
	navPaths := s.getNavPaths(ctx)
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

	// Boost results where the query appears in the title
	// Also apply structural boost: nav-linked pages up, video transcripts down
	for id, r := range resultMap {
		if strings.Contains(strings.ToLower(r.Title), lowerQuery) {
			scores[id] += titleBoost
		}
		scores[id] += pathBoost(r.FullPath, "", navPaths)
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
	// Simple insertion sort (small lists)
	for i := 1; i < len(sortable); i++ {
		for j := i; j > 0 && sortable[j].score > sortable[j-1].score; j-- {
			sortable[j], sortable[j-1] = sortable[j-1], sortable[j]
		}
	}

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
	if !s.HasVoyageKey() {
		return 0, 0, fmt.Errorf("no Voyage API key configured")
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
	// 6 = video transcript (any)
	navPaths := s.getNavPaths(ctx)
	type scoredPage struct {
		page SuggestPage
		rank int
	}
	var scored []scoredPage
	for _, c := range contents {
		nav := navPaths[c.FullPath]
		lowerPath := strings.ToLower(c.FullPath)
		isVideo := strings.HasPrefix(lowerPath, "/videos/") || strings.HasPrefix(lowerPath, "/video/")
		isConcept := strings.Contains(strings.ToLower(c.TemplateName), "concept")
		startsWith := strings.HasPrefix(strings.ToLower(c.Title), lowerPrefix)

		rank := 5
		switch {
		case isVideo:
			rank = 6
		case nav && startsWith:
			rank = 0
		case nav:
			rank = 1
		case isConcept && startsWith:
			rank = 2
		case isConcept:
			rank = 3
		case startsWith:
			rank = 4
		}
		scored = append(scored, scoredPage{
			page: SuggestPage{Title: c.Title, Path: c.FullPath},
			rank: rank,
		})
	}
	// Stable sort by rank
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j].rank < scored[j-1].rank; j-- {
			scored[j], scored[j-1] = scored[j-1], scored[j]
		}
	}
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
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && (sorted[j].count > sorted[j-1].count ||
			(sorted[j].count == sorted[j-1].count && sorted[j].keyword < sorted[j-1].keyword)); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

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
	// Truncate very long text (Voyage has token limits)
	if len(text) > 32000 {
		text = text[:32000]
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
