package importer

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FeedItem is a parsed RSS/Atom item
type FeedItem struct {
	Title       string
	URL         string    // link (used as source_url for dedup)
	Description string    // content/summary
	Author      string
	PublishedAt *time.Time
	GUID        string
}

// ParseFeed fetches and parses an RSS or Atom feed URL, returning items
func ParseFeed(ctx context.Context, feedURL string) ([]FeedItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "LightCMS-Importer/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching feed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading feed body: %w", err)
	}

	// Try RSS first, then Atom
	items, err := parseRSS(body)
	if err != nil || len(items) == 0 {
		items, err = parseAtom(body)
		if err != nil {
			return nil, fmt.Errorf("parsing feed (tried RSS and Atom): %w", err)
		}
	}
	return items, nil
}

// RSS 2.0 parsing
type rssRoot struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}
type rssChannel struct {
	Items []rssItem `xml:"item"`
}
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Author      string `xml:"author"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
	Content     string `xml:"encoded"` // content:encoded
}

func parseRSS(body []byte) ([]FeedItem, error) {
	var root rssRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	var items []FeedItem
	for _, it := range root.Channel.Items {
		item := FeedItem{
			Title:       strings.TrimSpace(it.Title),
			URL:         strings.TrimSpace(it.Link),
			Description: strings.TrimSpace(it.Content),
			Author:      strings.TrimSpace(it.Author),
			GUID:        strings.TrimSpace(it.GUID),
		}
		if item.Description == "" {
			item.Description = strings.TrimSpace(it.Description)
		}
		if it.PubDate != "" {
			t := parseTime(it.PubDate)
			item.PublishedAt = t
		}
		if item.URL == "" {
			item.URL = item.GUID
		}
		items = append(items, item)
	}
	return items, nil
}

// Atom parsing
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Author    atomAuthor `xml:"author"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	ID        string     `xml:"id"`
}
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}
type atomAuthor struct {
	Name string `xml:"name"`
}

func parseAtom(body []byte) ([]FeedItem, error) {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	var items []FeedItem
	for _, e := range feed.Entries {
		url := ""
		for _, l := range e.Links {
			if l.Rel == "alternate" || l.Rel == "" {
				url = l.Href
				break
			}
		}
		content := e.Content
		if content == "" {
			content = e.Summary
		}
		item := FeedItem{
			Title:       strings.TrimSpace(e.Title),
			URL:         url,
			Description: strings.TrimSpace(content),
			Author:      strings.TrimSpace(e.Author.Name),
			GUID:        e.ID,
		}
		pubStr := e.Published
		if pubStr == "" {
			pubStr = e.Updated
		}
		if pubStr != "" {
			item.PublishedAt = parseTime(pubStr)
		}
		items = append(items, item)
	}
	return items, nil
}

// parseTime tries several common feed date formats
func parseTime(s string) *time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return &t
		}
	}
	return nil
}
