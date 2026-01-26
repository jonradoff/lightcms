package mcp

import (
	"context"
	"fmt"
	"strings"

	"lightcms/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson"
)

// Search tool input types
type SearchContentInput struct {
	Query          string `json:"query" jsonschema:"Search query string,required"`
	SearchType     string `json:"search_type,omitempty" jsonschema:"Search type: 'name' (title only) or 'fulltext' (all fields). Defaults to 'fulltext'"`
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"Include soft-deleted content in results"`
}

type SearchReplacePreviewInput struct {
	Search  string `json:"search" jsonschema:"Text to search for,required"`
	Replace string `json:"replace" jsonschema:"Text to replace with,required"`
}

type SearchReplaceExecuteInput struct {
	Search         string `json:"search" jsonschema:"Text to search for,required"`
	Replace        string `json:"replace" jsonschema:"Text to replace with,required"`
	VersionComment string `json:"version_comment,omitempty" jsonschema:"Comment for version history (defaults to 'Bulk search and replace')"`
}

func (s *Server) registerSearchTools() {
	// Search content
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_content",
		Description: "Search across all content items by title or full text. Returns matching content with paths and match context.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchContentInput) (*mcp.CallToolResult, any, error) {
		if args.Query == "" {
			return errorResult(fmt.Errorf("query is required")), nil, nil
		}

		// Sanitize and limit query length
		query := strings.TrimSpace(args.Query)
		if len(query) > 200 {
			query = query[:200]
		}

		searchType := args.SearchType
		if searchType == "" {
			searchType = "fulltext"
		}

		// Build filter
		filter := bson.M{}
		if !args.IncludeDeleted {
			filter["deleted"] = bson.M{"$ne": true}
		}

		// Add search criteria
		if searchType == "fulltext" {
			filter["$or"] = []bson.M{
				{"title": bson.M{"$regex": query, "$options": "i"}},
				{"data": bson.M{"$regex": query, "$options": "i"}},
			}
		} else {
			filter["title"] = bson.M{"$regex": query, "$options": "i"}
		}

		// Execute search
		cursor, err := s.db.FindMany(ctx, "content", filter, nil)
		if err != nil {
			return errorResult(err), nil, nil
		}

		var results []models.Content
		if err := cursor.All(ctx, &results); err != nil {
			return errorResult(err), nil, nil
		}

		// For fulltext search, if MongoDB regex on data object didn't work, search manually
		if searchType == "fulltext" && len(results) == 0 {
			fallbackFilter := bson.M{}
			if !args.IncludeDeleted {
				fallbackFilter["deleted"] = bson.M{"$ne": true}
			}

			cursor, err := s.db.FindMany(ctx, "content", fallbackFilter, nil)
			if err != nil {
				return errorResult(err), nil, nil
			}

			var allContent []models.Content
			if err := cursor.All(ctx, &allContent); err != nil {
				return errorResult(err), nil, nil
			}

			queryLower := strings.ToLower(query)
			for _, c := range allContent {
				if strings.Contains(strings.ToLower(c.Title), queryLower) {
					results = append(results, c)
					continue
				}
				for _, v := range c.Data {
					if strVal, ok := v.(string); ok {
						if strings.Contains(strings.ToLower(strVal), queryLower) {
							results = append(results, c)
							break
						}
					}
				}
			}
		}

		// Build response with match details
		type SearchMatch struct {
			ID           string   `json:"id"`
			Title        string   `json:"title"`
			FullPath     string   `json:"full_path"`
			TemplateName string   `json:"template_name"`
			Published    bool     `json:"published"`
			MatchedIn    []string `json:"matched_in"`
		}

		matches := make([]SearchMatch, 0, len(results))
		queryLower := strings.ToLower(query)

		for _, c := range results {
			match := SearchMatch{
				ID:           c.ID.Hex(),
				Title:        c.Title,
				FullPath:     c.FullPath,
				TemplateName: c.TemplateName,
				Published:    c.Published,
				MatchedIn:    []string{},
			}

			// Determine where the match was found
			if strings.Contains(strings.ToLower(c.Title), queryLower) {
				match.MatchedIn = append(match.MatchedIn, "title")
			}
			for fieldName, v := range c.Data {
				if strVal, ok := v.(string); ok {
					if strings.Contains(strings.ToLower(strVal), queryLower) {
						match.MatchedIn = append(match.MatchedIn, fieldName)
					}
				}
			}

			matches = append(matches, match)
		}

		return jsonResult(map[string]interface{}{
			"query":       query,
			"search_type": searchType,
			"total":       len(matches),
			"matches":     matches,
		}), nil, nil
	})

	// Search and replace preview
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_replace_preview",
		Description: "Preview search and replace results without making changes. Shows which content items would be affected and where matches occur. ALWAYS use this before search_replace_execute to understand the impact.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchReplacePreviewInput) (*mcp.CallToolResult, any, error) {
		if args.Search == "" {
			return errorResult(fmt.Errorf("search text is required")), nil, nil
		}

		// Find all non-deleted content
		cursor, err := s.db.FindMany(ctx, "content", bson.M{"deleted": bson.M{"$ne": true}}, nil)
		if err != nil {
			return errorResult(err), nil, nil
		}

		var allContent []models.Content
		if err := cursor.All(ctx, &allContent); err != nil {
			return errorResult(err), nil, nil
		}

		type MatchDetail struct {
			ID            string            `json:"id"`
			Title         string            `json:"title"`
			FullPath      string            `json:"full_path"`
			Published     bool              `json:"published"`
			MatchCount    int               `json:"match_count"`
			FieldMatches  map[string]int    `json:"field_matches"`
			SampleExcerpt string            `json:"sample_excerpt,omitempty"`
		}

		matches := []MatchDetail{}
		totalMatchCount := 0

		for _, content := range allContent {
			matchCount := 0
			fieldMatches := make(map[string]int)
			var sampleExcerpt string

			// Search in all data fields
			for fieldName, value := range content.Data {
				if strVal, ok := value.(string); ok {
					count := strings.Count(strVal, args.Search)
					if count > 0 {
						matchCount += count
						fieldMatches[fieldName] = count

						// Generate sample excerpt if we don't have one yet
						if sampleExcerpt == "" {
							sampleExcerpt = generateExcerpt(strVal, args.Search, args.Replace)
						}
					}
				}
			}

			// Also check title
			if strings.Contains(content.Title, args.Search) {
				titleCount := strings.Count(content.Title, args.Search)
				matchCount += titleCount
				fieldMatches["title"] = titleCount
				if sampleExcerpt == "" {
					sampleExcerpt = fmt.Sprintf("Title: '%s' → '%s'",
						content.Title,
						strings.ReplaceAll(content.Title, args.Search, args.Replace))
				}
			}

			if matchCount > 0 {
				matches = append(matches, MatchDetail{
					ID:            content.ID.Hex(),
					Title:         content.Title,
					FullPath:      content.FullPath,
					Published:     content.Published,
					MatchCount:    matchCount,
					FieldMatches:  fieldMatches,
					SampleExcerpt: sampleExcerpt,
				})
				totalMatchCount += matchCount
			}
		}

		publishedCount := 0
		for _, m := range matches {
			if m.Published {
				publishedCount++
			}
		}

		return jsonResult(map[string]interface{}{
			"search":               args.Search,
			"replace":              args.Replace,
			"total_matches":        totalMatchCount,
			"affected_pages":       len(matches),
			"published_pages":      publishedCount,
			"draft_pages":          len(matches) - publishedCount,
			"matches":              matches,
			"warning":              "This is a PREVIEW only. Use search_replace_execute to apply changes.",
			"destructive_warning":  "Search and replace will modify content permanently. Review the matches carefully before executing.",
		}), nil, nil
	})

	// Search and replace execute
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_replace_execute",
		Description: "Execute search and replace across all content. WARNING: This is a destructive operation that modifies content permanently. Always run search_replace_preview first to review what will be changed.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchReplaceExecuteInput) (*mcp.CallToolResult, any, error) {
		if args.Search == "" {
			return errorResult(fmt.Errorf("search text is required")), nil, nil
		}

		versionComment := args.VersionComment
		if versionComment == "" {
			versionComment = fmt.Sprintf("Bulk search and replace: '%s' → '%s'", args.Search, args.Replace)
		}

		// Find all non-deleted content
		cursor, err := s.db.FindMany(ctx, "content", bson.M{"deleted": bson.M{"$ne": true}}, nil)
		if err != nil {
			return errorResult(err), nil, nil
		}

		var allContent []models.Content
		if err := cursor.All(ctx, &allContent); err != nil {
			return errorResult(err), nil, nil
		}

		type UpdatedPage struct {
			ID            string   `json:"id"`
			Title         string   `json:"title"`
			FullPath      string   `json:"full_path"`
			Published     bool     `json:"published"`
			MatchCount    int      `json:"match_count"`
			FieldsUpdated []string `json:"fields_updated"`
		}

		updatedPages := []UpdatedPage{}
		totalReplacements := 0

		for _, content := range allContent {
			needsUpdate := false
			matchCount := 0
			fieldsUpdated := []string{}

			newData := make(map[string]interface{})
			for k, v := range content.Data {
				newData[k] = v
			}

			// Replace in all data fields
			for fieldName, value := range content.Data {
				if strVal, ok := value.(string); ok {
					if strings.Contains(strVal, args.Search) {
						count := strings.Count(strVal, args.Search)
						matchCount += count
						newData[fieldName] = strings.ReplaceAll(strVal, args.Search, args.Replace)
						needsUpdate = true
						fieldsUpdated = append(fieldsUpdated, fieldName)
					}
				}
			}

			// Replace in title
			newTitle := content.Title
			if strings.Contains(content.Title, args.Search) {
				count := strings.Count(content.Title, args.Search)
				matchCount += count
				newTitle = strings.ReplaceAll(content.Title, args.Search, args.Replace)
				needsUpdate = true
				fieldsUpdated = append(fieldsUpdated, "title")
			}

			if needsUpdate {
				// Update content
				content.Title = newTitle
				content.Data = newData

				if err := s.contentService.UpdateContent(ctx, &content, versionComment); err != nil {
					// Log error but continue with other content
					continue
				}

				updatedPages = append(updatedPages, UpdatedPage{
					ID:            content.ID.Hex(),
					Title:         newTitle,
					FullPath:      content.FullPath,
					Published:     content.Published,
					MatchCount:    matchCount,
					FieldsUpdated: fieldsUpdated,
				})
				totalReplacements += matchCount
			}
		}

		publishedCount := 0
		for _, p := range updatedPages {
			if p.Published {
				publishedCount++
			}
		}

		return jsonResult(map[string]interface{}{
			"success":            true,
			"search":             args.Search,
			"replace":            args.Replace,
			"total_replacements": totalReplacements,
			"pages_updated":      len(updatedPages),
			"published_pages":    publishedCount,
			"version_comment":    versionComment,
			"updated_pages":      updatedPages,
		}), nil, nil
	})
}

// generateExcerpt creates a short excerpt showing the replacement context
func generateExcerpt(text, search, replace string) string {
	idx := strings.Index(text, search)
	if idx == -1 {
		return ""
	}

	// Get surrounding context
	start := idx - 30
	if start < 0 {
		start = 0
	}
	end := idx + len(search) + 30
	if end > len(text) {
		end = len(text)
	}

	excerpt := ""
	if start > 0 {
		excerpt = "..."
	}
	excerpt += text[start:idx] + "[" + search + " → " + replace + "]" + text[idx+len(search):end]
	if end < len(text) {
		excerpt += "..."
	}

	return excerpt
}
