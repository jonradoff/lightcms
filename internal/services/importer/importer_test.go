package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	in := "title, body\nHello, World\nFoo, Bar\n"
	headers, records, err := ParseCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(headers) != 2 || headers[0] != "title" || headers[1] != "body" {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Fields["title"] != "Hello" || records[0].Fields["body"] != "World" {
		t.Errorf("unexpected record 0: %+v", records[0].Fields)
	}
	if records[1].Row != 3 {
		t.Errorf("expected row 3 for second record, got %d", records[1].Row)
	}
}

func TestParseCSV_BadInput(t *testing.T) {
	// Mismatched field counts make encoding/csv error.
	if _, _, err := ParseCSV(strings.NewReader("a,b\n1,2,3\n")); err == nil {
		t.Error("expected error for ragged CSV")
	}
}

func TestParseMarkdownFile_WithFrontmatter(t *testing.T) {
	content := "---\ntitle: My Page\nslug: \"my-page\"\n---\n# Heading\n\nBody text."
	page := ParseMarkdownFile("post.md", content)
	if page.Frontmatter["title"] != "My Page" {
		t.Errorf("title = %q", page.Frontmatter["title"])
	}
	if page.Frontmatter["slug"] != "my-page" {
		t.Errorf("slug (quotes stripped) = %q", page.Frontmatter["slug"])
	}
	if !strings.HasPrefix(page.Body, "# Heading") {
		t.Errorf("body = %q", page.Body)
	}
}

func TestParseMarkdownFile_NoFrontmatter(t *testing.T) {
	page := ParseMarkdownFile("plain.md", "just body, no frontmatter")
	if len(page.Frontmatter) != 0 {
		t.Errorf("expected empty frontmatter, got %v", page.Frontmatter)
	}
	if page.Body != "just body, no frontmatter" {
		t.Errorf("body = %q", page.Body)
	}
}

func TestFrontmatterGet_CaseInsensitive(t *testing.T) {
	fm := map[string]string{"Title": "X"}
	if FrontmatterGet(fm, "title") != "X" {
		t.Error("expected case-insensitive match")
	}
	if FrontmatterGet(fm, "missing") != "" {
		t.Error("expected empty for missing key")
	}
}

func TestParseMarkdownZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"a.md":       "---\ntitle: A\n---\nBody A",
		"b.markdown": "Body B",
		"skip.txt":   "ignored",
	} {
		f, _ := zw.Create(name)
		f.Write([]byte(body))
	}
	zw.Close()

	pages, err := ParseMarkdownZip(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseMarkdownZip: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 markdown pages (.txt skipped), got %d", len(pages))
	}
}

func TestParseMarkdownZip_BadZip(t *testing.T) {
	if _, err := ParseMarkdownZip([]byte("not a zip")); err == nil {
		t.Error("expected error for invalid zip")
	}
}

func TestParseTimeStr(t *testing.T) {
	if ParseTimeStr("2026-01-15") == nil {
		t.Error("expected parseable date")
	}
	if ParseTimeStr("definitely not a date") != nil {
		t.Error("expected nil for unparseable date")
	}
}

func TestParseFeed_RSS(t *testing.T) {
	rss := `<?xml version="1.0"?>
<rss version="2.0"><channel>
<item><title>Post One</title><link>https://x.com/1</link><description>Desc</description><pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate><guid>g1</guid></item>
<item><title>Post Two</title><link>https://x.com/2</link></item>
</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(rss))
	}))
	defer srv.Close()

	items, err := ParseFeed(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ParseFeed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Title != "Post One" || items[0].URL != "https://x.com/1" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestParseFeed_Atom(t *testing.T) {
	atom := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example</title>
  <entry>
    <title>Atom Post One</title>
    <link rel="alternate" href="https://example.com/a1"/>
    <id>urn:a1</id>
    <author><name>Jane</name></author>
    <published>2026-01-02T15:04:05Z</published>
    <content>Full content one</content>
  </entry>
  <entry>
    <title>Atom Post Two</title>
    <link href="https://example.com/a2"/>
    <id>urn:a2</id>
    <updated>2026-01-03T00:00:00Z</updated>
    <summary>Summary two</summary>
  </entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(atom))
	}))
	defer srv.Close()

	items, err := ParseFeed(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ParseFeed(atom): %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 atom items, got %d", len(items))
	}
	if items[0].Title != "Atom Post One" || items[0].URL != "https://example.com/a1" {
		t.Errorf("unexpected atom item 0: %+v", items[0])
	}
	if items[0].Author != "Jane" || items[0].PublishedAt == nil {
		t.Errorf("atom author/date not parsed: %+v", items[0])
	}
	if items[1].Description != "Summary two" {
		t.Errorf("atom summary fallback failed: %+v", items[1])
	}
}

func TestParseFeed_BadURL(t *testing.T) {
	if _, err := ParseFeed(context.Background(), "http://127.0.0.1:0/nope"); err == nil {
		t.Error("expected error for unreachable feed")
	}
}
