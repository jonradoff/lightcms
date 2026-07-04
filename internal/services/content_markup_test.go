package services

import (
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/models"
)

// ---------------------------------------------------------------------------
// headingSlug
// ---------------------------------------------------------------------------

func TestHeadingSlug_Simple(t *testing.T) {
	if got := headingSlug("Hello World"); got != "hello-world" {
		t.Errorf("got %q", got)
	}
}

func TestHeadingSlug_WithHTML(t *testing.T) {
	if got := headingSlug("<strong>Title</strong>"); got != "title" {
		t.Errorf("got %q", got)
	}
}

func TestHeadingSlug_SpecialChars(t *testing.T) {
	if got := headingSlug("Hello & World!"); got != "hello-world" {
		t.Errorf("got %q", got)
	}
}

func TestHeadingSlug_Empty(t *testing.T) {
	if got := headingSlug(""); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestHeadingSlug_MultipleSpaces(t *testing.T) {
	if got := headingSlug("  Getting  Started  "); got != "getting-started" {
		t.Errorf("got %q", got)
	}
}

func TestHeadingSlug_Numbers(t *testing.T) {
	if got := headingSlug("Step 1: Installation"); got != "step-1-installation" {
		t.Errorf("got %q", got)
	}
}

func TestHeadingSlug_OnlySpecialChars(t *testing.T) {
	if got := headingSlug("!!! ???"); got != "" {
		t.Errorf("expected empty slug for all-special input, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// injectHeadingIDs
// ---------------------------------------------------------------------------

func TestInjectHeadingIDs_AddsID(t *testing.T) {
	html := `<h2>Getting Started</h2>`
	got := injectHeadingIDs(html)
	if !strings.Contains(got, `id="getting-started"`) {
		t.Errorf("expected id attribute, got: %s", got)
	}
}

func TestInjectHeadingIDs_SkipsExistingID(t *testing.T) {
	html := `<h2 id="custom">My Heading</h2>`
	got := injectHeadingIDs(html)
	if got != html {
		t.Errorf("should not modify heading with existing id, got: %s", got)
	}
}

func TestInjectHeadingIDs_PreservesAttrs(t *testing.T) {
	html := `<h3 class="section">Installation Guide</h3>`
	got := injectHeadingIDs(html)
	if !strings.Contains(got, `id="installation-guide"`) {
		t.Errorf("expected id attribute, got: %s", got)
	}
	if !strings.Contains(got, `class="section"`) {
		t.Errorf("expected class attr preserved, got: %s", got)
	}
}

func TestInjectHeadingIDs_MultipleHeadings(t *testing.T) {
	html := "<h1>Intro</h1><h2>Setup</h2><h3>Config</h3>"
	got := injectHeadingIDs(html)
	if !strings.Contains(got, `id="intro"`) {
		t.Errorf("expected h1 id, got: %s", got)
	}
	if !strings.Contains(got, `id="setup"`) {
		t.Errorf("expected h2 id, got: %s", got)
	}
	if !strings.Contains(got, `id="config"`) {
		t.Errorf("expected h3 id, got: %s", got)
	}
}

func TestInjectHeadingIDs_AllLevels(t *testing.T) {
	for _, level := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		html := "<" + level + ">Test</" + level + ">"
		got := injectHeadingIDs(html)
		if !strings.Contains(got, `id="test"`) {
			t.Errorf("%s: expected id=test, got: %s", level, got)
		}
	}
}

func TestInjectHeadingIDs_EmptyContent(t *testing.T) {
	// Heading with empty text should not get an id (slug would be "")
	html := `<h2></h2>`
	got := injectHeadingIDs(html)
	// Should not add id="" for empty slug
	if strings.Contains(got, `id=""`) {
		t.Errorf("should not add empty id attribute, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// buildAndInjectTOC
// ---------------------------------------------------------------------------

func TestBuildAndInjectTOC_NoPlaceholder(t *testing.T) {
	html := "<h2>Section</h2><p>Text</p>"
	got := buildAndInjectTOC(html)
	// No placeholder → returned unchanged
	if got != html {
		t.Errorf("expected unchanged html, got: %s", got)
	}
}

func TestBuildAndInjectTOC_NoHeadings(t *testing.T) {
	html := tocPlaceholder + "<p>No headings here</p>"
	got := buildAndInjectTOC(html)
	if strings.Contains(got, tocPlaceholder) {
		t.Error("placeholder should be removed when no headings")
	}
	if strings.Contains(got, `<nav`) {
		t.Error("should not generate nav when no headings")
	}
}

func TestBuildAndInjectTOC_WithHeadings(t *testing.T) {
	// Headings need id= attributes (added by injectHeadingIDs first)
	html := tocPlaceholder + `<h2 id="intro">Introduction</h2><h3 id="setup">Setup</h3>`
	got := buildAndInjectTOC(html)
	if strings.Contains(got, tocPlaceholder) {
		t.Error("placeholder should be replaced")
	}
	if !strings.Contains(got, `<nav class="lc-toc">`) {
		t.Errorf("expected nav element, got: %s", got)
	}
	if !strings.Contains(got, `href="#intro"`) {
		t.Errorf("expected intro anchor, got: %s", got)
	}
	if !strings.Contains(got, `href="#setup"`) {
		t.Errorf("expected setup anchor, got: %s", got)
	}
	if !strings.Contains(got, "Introduction") {
		t.Errorf("expected heading text, got: %s", got)
	}
}

func TestBuildAndInjectTOC_HeadingLevels(t *testing.T) {
	html := tocPlaceholder +
		`<h1 id="h1">Title</h1>` +
		`<h2 id="h2">Section</h2>` +
		`<h3 id="h3">Sub</h3>`
	got := buildAndInjectTOC(html)
	if !strings.Contains(got, `class="toc-h1"`) {
		t.Errorf("expected toc-h1 class, got: %s", got)
	}
	if !strings.Contains(got, `class="toc-h2"`) {
		t.Errorf("expected toc-h2 class, got: %s", got)
	}
	if !strings.Contains(got, `class="toc-h3"`) {
		t.Errorf("expected toc-h3 class, got: %s", got)
	}
}

func TestBuildAndInjectTOC_EscapesHeadingText(t *testing.T) {
	// Heading text with special chars should be escaped in TOC links
	html := tocPlaceholder + `<h2 id="xss">Hello &amp; World</h2>`
	got := buildAndInjectTOC(html)
	// Should not contain unescaped & in anchor text
	if strings.Contains(got, "&amp;&amp;") {
		t.Errorf("double-escaped: %s", got)
	}
}

// ---------------------------------------------------------------------------
// markdownToHTML
// ---------------------------------------------------------------------------

func TestMarkdownToHTML_Paragraph(t *testing.T) {
	got := markdownToHTML("Hello world.", true)
	if !strings.Contains(got, "<p>") {
		t.Errorf("expected paragraph tag, got: %s", got)
	}
	if !strings.Contains(got, "Hello world.") {
		t.Errorf("expected content, got: %s", got)
	}
}

func TestMarkdownToHTML_Bold(t *testing.T) {
	got := markdownToHTML("**bold**", true)
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("expected bold, got: %s", got)
	}
}

func TestMarkdownToHTML_Heading(t *testing.T) {
	got := markdownToHTML("# My Heading", true)
	if !strings.Contains(got, "<h1") {
		t.Errorf("expected h1, got: %s", got)
	}
	if !strings.Contains(got, "My Heading") {
		t.Errorf("expected heading text, got: %s", got)
	}
}

func TestMarkdownToHTML_GFMTable(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |"
	got := markdownToHTML(md, true)
	if !strings.Contains(got, "<table>") {
		t.Errorf("expected table, got: %s", got)
	}
}

func TestMarkdownToHTML_GFMStrikethrough(t *testing.T) {
	got := markdownToHTML("~~deleted~~", true)
	if !strings.Contains(got, "<del>") {
		t.Errorf("expected del tag, got: %s", got)
	}
}

func TestMarkdownToHTML_AllowUnsafePassesScript(t *testing.T) {
	got := markdownToHTML("<script>alert(1)</script>", true)
	if !strings.Contains(got, "<script>") {
		t.Errorf("expected script to pass through with allowUnsafe=true, got: %s", got)
	}
}

func TestMarkdownToHTML_DenyUnsafeStripsScript(t *testing.T) {
	got := markdownToHTML("<script>alert(1)</script>", false)
	if strings.Contains(got, "<script>") {
		t.Errorf("expected script to be stripped with allowUnsafe=false, got: %s", got)
	}
}

func TestMarkdownToHTML_DenyUnsafeAllowsNormalHTML(t *testing.T) {
	got := markdownToHTML("**bold** and *italic*", false)
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("expected bold to remain, got: %s", got)
	}
	if !strings.Contains(got, "<em>italic</em>") {
		t.Errorf("expected em to remain, got: %s", got)
	}
}

func TestMarkdownToHTML_DenyUnsafeStripsOnclick(t *testing.T) {
	got := markdownToHTML(`<a href="/page" onclick="evil()">link</a>`, false)
	if strings.Contains(got, "onclick") {
		t.Errorf("expected onclick stripped, got: %s", got)
	}
	if !strings.Contains(got, "link") {
		t.Errorf("expected link text preserved, got: %s", got)
	}
}

func TestMarkdownToHTML_Empty(t *testing.T) {
	got := markdownToHTML("", true)
	// Empty input produces no output (or just whitespace)
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty output for empty input, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// mergeInlineTags
// ---------------------------------------------------------------------------

func TestMergeInlineTags_Basic(t *testing.T) {
	c := &models.Content{
		Data: map[string]interface{}{
			"body": "This post is about #golang and #webdev techniques.",
		},
	}
	mergeInlineTags(c)
	if !containsTag(c.Tags, "golang") {
		t.Errorf("expected golang tag, got: %v", c.Tags)
	}
	if !containsTag(c.Tags, "webdev") {
		t.Errorf("expected webdev tag, got: %v", c.Tags)
	}
}

func TestMergeInlineTags_Deduplication(t *testing.T) {
	c := &models.Content{
		Tags: []string{"golang"},
		Data: map[string]interface{}{
			"body": "Talking about #golang again.",
		},
	}
	mergeInlineTags(c)
	count := 0
	for _, tag := range c.Tags {
		if strings.ToLower(tag) == "golang" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected golang to appear exactly once, got count=%d, tags=%v", count, c.Tags)
	}
}

func TestMergeInlineTags_CaseNormalization(t *testing.T) {
	// #Go and #go should be treated as the same tag
	c := &models.Content{
		Tags: []string{"Go"},
		Data: map[string]interface{}{
			"body": "Check out #go for more.",
		},
	}
	mergeInlineTags(c)
	count := 0
	for _, tag := range c.Tags {
		if strings.ToLower(tag) == "go" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected go to appear once, got count=%d, tags=%v", count, c.Tags)
	}
}

func TestMergeInlineTags_NoTagsInData(t *testing.T) {
	c := &models.Content{
		Data: map[string]interface{}{
			"body": "No hashtags in this text at all.",
		},
	}
	mergeInlineTags(c)
	if len(c.Tags) != 0 {
		t.Errorf("expected no tags, got: %v", c.Tags)
	}
}

func TestMergeInlineTags_NonStringField(t *testing.T) {
	c := &models.Content{
		Data: map[string]interface{}{
			"count": 42,
			"body":  "Check out #testing",
		},
	}
	mergeInlineTags(c)
	if !containsTag(c.Tags, "testing") {
		t.Errorf("expected testing tag, got: %v", c.Tags)
	}
}

func TestMergeInlineTags_MultipleFields(t *testing.T) {
	c := &models.Content{
		Data: map[string]interface{}{
			"title":   "#featured article",
			"excerpt": "Read about #golang and #performance.",
		},
	}
	mergeInlineTags(c)
	if !containsTag(c.Tags, "featured") {
		t.Errorf("expected featured tag, got: %v", c.Tags)
	}
	if !containsTag(c.Tags, "golang") {
		t.Errorf("expected golang tag, got: %v", c.Tags)
	}
	if !containsTag(c.Tags, "performance") {
		t.Errorf("expected performance tag, got: %v", c.Tags)
	}
}

func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.ToLower(t) == strings.ToLower(target) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// processWikiLinks
// ---------------------------------------------------------------------------

func TestProcessWikiLinks_TitleLink(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{"my page": "/my-page"},
		pathToTitle: map[string]string{"/my-page": "My Page"},
	}
	html := `<p>See [[My Page]] for details.</p>`
	got := processWikiLinks(idx, html)
	if !strings.Contains(got, `href="/my-page"`) {
		t.Errorf("expected resolved link, got: %s", got)
	}
	if !strings.Contains(got, "My Page") {
		t.Errorf("expected link text, got: %s", got)
	}
}

func TestProcessWikiLinks_TitleLinkCaseInsensitive(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{"my page": "/my-page"},
		pathToTitle: map[string]string{"/my-page": "My Page"},
	}
	// Lowercase [[my page]] should resolve
	html := `[[my page]]`
	got := processWikiLinks(idx, html)
	if !strings.Contains(got, `href="/my-page"`) {
		t.Errorf("expected case-insensitive resolution, got: %s", got)
	}
}

func TestProcessWikiLinks_CustomDisplay(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{"installation guide": "/install"},
		pathToTitle: map[string]string{"/install": "Installation Guide"},
	}
	html := `[[Installation Guide|set it up]]`
	got := processWikiLinks(idx, html)
	if !strings.Contains(got, `href="/install"`) {
		t.Errorf("expected link href, got: %s", got)
	}
	if !strings.Contains(got, "set it up") {
		t.Errorf("expected custom display text, got: %s", got)
	}
	if strings.Contains(got, "Installation Guide") {
		t.Errorf("should use custom display text, not page title, got: %s", got)
	}
}

func TestProcessWikiLinks_PathLink(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{},
		pathToTitle: map[string]string{"/about": "About Us"},
	}
	html := `[[/about]]`
	got := processWikiLinks(idx, html)
	if !strings.Contains(got, `href="/about"`) {
		t.Errorf("expected resolved path link, got: %s", got)
	}
	if !strings.Contains(got, "About Us") {
		t.Errorf("expected page title as display text, got: %s", got)
	}
}

func TestProcessWikiLinks_PathLinkWithDisplay(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{},
		pathToTitle: map[string]string{"/about": "About Us"},
	}
	html := `[[/about|click here]]`
	got := processWikiLinks(idx, html)
	if !strings.Contains(got, `href="/about"`) {
		t.Errorf("expected path href, got: %s", got)
	}
	if !strings.Contains(got, "click here") {
		t.Errorf("expected custom display, got: %s", got)
	}
}

func TestProcessWikiLinks_BrokenTitleLink(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{},
		pathToTitle: map[string]string{},
	}
	html := `[[Nonexistent Page]]`
	got := processWikiLinks(idx, html)
	if strings.Contains(got, `<a `) {
		t.Errorf("should not generate link for broken target, got: %s", got)
	}
	if !strings.Contains(got, `class="broken-link"`) {
		t.Errorf("expected broken-link span, got: %s", got)
	}
	if !strings.Contains(got, "Nonexistent Page") {
		t.Errorf("expected target text in broken link, got: %s", got)
	}
}

func TestProcessWikiLinks_BrokenPathLink(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{},
		pathToTitle: map[string]string{},
	}
	html := `[[/no/such/page]]`
	got := processWikiLinks(idx, html)
	if !strings.Contains(got, `class="broken-link"`) {
		t.Errorf("expected broken-link, got: %s", got)
	}
}

func TestProcessWikiLinks_NoLinks(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{},
		pathToTitle: map[string]string{},
	}
	html := `<p>No wikilinks here.</p>`
	got := processWikiLinks(idx, html)
	if got != html {
		t.Errorf("expected unchanged html, got: %s", got)
	}
}

func TestProcessWikiLinks_EscapesTarget(t *testing.T) {
	idx := &wikilinkIndex{
		titleToPath: map[string]string{},
		pathToTitle: map[string]string{},
	}
	// XSS attempt in target
	html := `[[<script>alert(1)</script>]]`
	got := processWikiLinks(idx, html)
	if strings.Contains(got, "<script>") {
		t.Errorf("expected script escaped in broken-link, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// rewriteWikilinksInText
// ---------------------------------------------------------------------------

func TestRewriteWikilinksInText_Title(t *testing.T) {
	text := `See [[Old Title]] for details.`
	got := rewriteWikilinksInText(text, "Old Title", "New Title", "", "")
	if !strings.Contains(got, "[[New Title]]") {
		t.Errorf("expected rewritten title, got: %s", got)
	}
	if strings.Contains(got, "[[Old Title]]") {
		t.Errorf("old title should be gone, got: %s", got)
	}
}

func TestRewriteWikilinksInText_TitleCaseInsensitive(t *testing.T) {
	text := `Check [[old title]] here.`
	got := rewriteWikilinksInText(text, "Old Title", "New Title", "", "")
	if !strings.Contains(got, "[[New Title]]") {
		t.Errorf("expected case-insensitive match, got: %s", got)
	}
}

func TestRewriteWikilinksInText_Path(t *testing.T) {
	text := `Link to [[/old/path]] here.`
	got := rewriteWikilinksInText(text, "", "", "/old/path", "/new/path")
	if !strings.Contains(got, "[[/new/path]]") {
		t.Errorf("expected rewritten path, got: %s", got)
	}
	if strings.Contains(got, "[[/old/path]]") {
		t.Errorf("old path should be gone, got: %s", got)
	}
}

func TestRewriteWikilinksInText_PreservesDisplay(t *testing.T) {
	text := `[[Old Title|click here]]`
	got := rewriteWikilinksInText(text, "Old Title", "New Title", "", "")
	if !strings.Contains(got, "[[New Title|click here]]") {
		t.Errorf("expected display text preserved, got: %s", got)
	}
}

func TestRewriteWikilinksInText_NoMatch(t *testing.T) {
	text := `[[Some Other Page]]`
	got := rewriteWikilinksInText(text, "Old Title", "New Title", "/old", "/new")
	if got != text {
		t.Errorf("expected unchanged text, got: %s", got)
	}
}

func TestRewriteWikilinksInText_SameTitle(t *testing.T) {
	// oldTitle == newTitle → no-op
	text := `[[My Page]]`
	got := rewriteWikilinksInText(text, "My Page", "My Page", "", "")
	if got != text {
		t.Errorf("expected unchanged text when titles match, got: %s", got)
	}
}

func TestRewriteWikilinksInText_NoLinks(t *testing.T) {
	text := `Plain text with no links.`
	got := rewriteWikilinksInText(text, "Old", "New", "/old", "/new")
	if got != text {
		t.Errorf("expected unchanged text, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// parseDirectiveAttrs
// ---------------------------------------------------------------------------

func TestParseDirectiveAttrs_Single(t *testing.T) {
	attrs := parseDirectiveAttrs(`tag="golang"`)
	if attrs["tag"] != "golang" {
		t.Errorf("expected tag=golang, got: %v", attrs)
	}
}

func TestParseDirectiveAttrs_Multiple(t *testing.T) {
	attrs := parseDirectiveAttrs(`tag="go" snippet="card" limit="10"`)
	if attrs["tag"] != "go" {
		t.Errorf("expected tag=go, got: %v", attrs)
	}
	if attrs["snippet"] != "card" {
		t.Errorf("expected snippet=card, got: %v", attrs)
	}
	if attrs["limit"] != "10" {
		t.Errorf("expected limit=10, got: %v", attrs)
	}
}

func TestParseDirectiveAttrs_Empty(t *testing.T) {
	attrs := parseDirectiveAttrs("")
	if len(attrs) != 0 {
		t.Errorf("expected empty map, got: %v", attrs)
	}
}

func TestParseDirectiveAttrs_NoQuotes(t *testing.T) {
	// Only key="value" pairs are matched; bare key=value should not parse
	attrs := parseDirectiveAttrs(`tag=golang`)
	if _, ok := attrs["tag"]; ok {
		t.Errorf("expected no match for unquoted value, got: %v", attrs)
	}
}

func TestParseDirectiveAttrs_EmptyValue(t *testing.T) {
	attrs := parseDirectiveAttrs(`tag=""`)
	if attrs["tag"] != "" {
		t.Errorf("expected empty string value, got: %v", attrs)
	}
}
