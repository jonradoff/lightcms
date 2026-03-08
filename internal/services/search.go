package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	voyageEmbedURL    = "https://api.voyageai.com/v1/embeddings"
	voyageModel       = "voyage-4-lite"
	embeddingDims     = 1024
	vectorSearchIndex = "content_vector_search"
	maxSnippetLen     = 200
	maxBatchSize      = 128
)

// SearchResult represents a single search result
type SearchResult struct {
	ContentID string  `json:"id"`
	Title     string  `json:"title"`
	FullPath  string  `json:"full_path"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
	MatchType string  `json:"match_type"`
}

// SearchService handles end-user search with full-text and semantic (vector) search
type SearchService struct {
	db           *database.DB
	voyageAPIKey string
	httpClient   *http.Client
}

// NewSearchService creates a new search service
func NewSearchService(db *database.DB, voyageAPIKey string) *SearchService {
	return &SearchService{
		db:           db,
		voyageAPIKey: voyageAPIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HasVoyageKey returns true if a Voyage API key is configured
func (s *SearchService) HasVoyageKey() bool {
	return s.voyageAPIKey != ""
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

	results := make([]SearchResult, 0, len(contents))
	lowerQuery := strings.ToLower(query)

	for i, c := range contents {
		if i >= limit {
			break
		}
		snippet := extractSnippet(c.PlainText, lowerQuery)
		results = append(results, SearchResult{
			ContentID: c.ID.Hex(),
			Title:     c.Title,
			FullPath:  c.FullPath,
			Snippet:   snippet,
			Score:     1.0 - float64(i)*0.01, // Simple rank score
			MatchType: "exact",
		})
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
	const k = 60.0 // RRF constant
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
