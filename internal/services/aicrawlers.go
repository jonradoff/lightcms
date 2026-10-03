package services

import (
	"net/url"
	"strings"
)

// Crawler purposes. Policy (robots.txt) is set per purpose; analytics groups by it.
const (
	CrawlerPurposeTraining  = "training"   // collects content to train models
	CrawlerPurposeAISearch  = "ai_search"  // indexes content for AI answers / AI search
	CrawlerPurposeUserFetch = "user_fetch" // fetches a page because a person asked an assistant
	CrawlerPurposeSearch    = "search"     // classic search engine
)

// CrawlerPurposeLabels are human-readable names for the purposes.
var CrawlerPurposeLabels = map[string]string{
	CrawlerPurposeTraining:  "AI training",
	CrawlerPurposeAISearch:  "AI search",
	CrawlerPurposeUserFetch: "AI user fetch",
	CrawlerPurposeSearch:    "Search engine",
}

// KnownCrawler is one crawler LightCMS recognizes by name.
type KnownCrawler struct {
	Token   string `json:"token"`   // robots.txt user-agent token
	Vendor  string `json:"vendor"`  // company operating it
	Purpose string `json:"purpose"` // one of the CrawlerPurpose* constants
	// UAMatch is a lowercase substring identifying the crawler in a
	// User-Agent header. Empty for robots-only tokens (e.g. Google-Extended),
	// which never appear in requests — the vendor's regular crawler fetches
	// and the token only controls how the content may be used.
	UAMatch string `json:"-"`
}

// KnownCrawlers is the single registry used by robots.txt generation and
// analytics. Keep more specific UAMatch strings before less specific ones
// that they contain (e.g. "claude-searchbot" is distinct from "claudebot",
// but "applebot-extended" must be checked before "applebot").
var KnownCrawlers = []KnownCrawler{
	// Training
	{Token: "GPTBot", Vendor: "OpenAI", Purpose: CrawlerPurposeTraining, UAMatch: "gptbot"},
	{Token: "ClaudeBot", Vendor: "Anthropic", Purpose: CrawlerPurposeTraining, UAMatch: "claudebot"},
	{Token: "Google-Extended", Vendor: "Google", Purpose: CrawlerPurposeTraining},
	{Token: "Applebot-Extended", Vendor: "Apple", Purpose: CrawlerPurposeTraining},
	{Token: "CCBot", Vendor: "Common Crawl", Purpose: CrawlerPurposeTraining, UAMatch: "ccbot"},
	{Token: "Meta-ExternalAgent", Vendor: "Meta", Purpose: CrawlerPurposeTraining, UAMatch: "meta-externalagent"},
	{Token: "Bytespider", Vendor: "ByteDance", Purpose: CrawlerPurposeTraining, UAMatch: "bytespider"},
	{Token: "Amazonbot", Vendor: "Amazon", Purpose: CrawlerPurposeTraining, UAMatch: "amazonbot"},
	{Token: "cohere-training-data-crawler", Vendor: "Cohere", Purpose: CrawlerPurposeTraining, UAMatch: "cohere-training-data-crawler"},
	{Token: "Diffbot", Vendor: "Diffbot", Purpose: CrawlerPurposeTraining, UAMatch: "diffbot"},

	// AI search / answer indexing
	{Token: "OAI-SearchBot", Vendor: "OpenAI", Purpose: CrawlerPurposeAISearch, UAMatch: "oai-searchbot"},
	{Token: "Claude-SearchBot", Vendor: "Anthropic", Purpose: CrawlerPurposeAISearch, UAMatch: "claude-searchbot"},
	{Token: "PerplexityBot", Vendor: "Perplexity", Purpose: CrawlerPurposeAISearch, UAMatch: "perplexitybot"},
	{Token: "DuckAssistBot", Vendor: "DuckDuckGo", Purpose: CrawlerPurposeAISearch, UAMatch: "duckassistbot"},

	// User-initiated fetches (a person asked an assistant to read the page)
	{Token: "ChatGPT-User", Vendor: "OpenAI", Purpose: CrawlerPurposeUserFetch, UAMatch: "chatgpt-user"},
	{Token: "Claude-User", Vendor: "Anthropic", Purpose: CrawlerPurposeUserFetch, UAMatch: "claude-user"},
	{Token: "Perplexity-User", Vendor: "Perplexity", Purpose: CrawlerPurposeUserFetch, UAMatch: "perplexity-user"},
	{Token: "Meta-ExternalFetcher", Vendor: "Meta", Purpose: CrawlerPurposeUserFetch, UAMatch: "meta-externalfetcher"},
	{Token: "MistralAI-User", Vendor: "Mistral", Purpose: CrawlerPurposeUserFetch, UAMatch: "mistralai-user"},

	// Classic search (not governed by the AI policy; tracked for comparison)
	{Token: "Googlebot", Vendor: "Google", Purpose: CrawlerPurposeSearch, UAMatch: "googlebot"},
	{Token: "Bingbot", Vendor: "Microsoft", Purpose: CrawlerPurposeSearch, UAMatch: "bingbot"},
	{Token: "Applebot", Vendor: "Apple", Purpose: CrawlerPurposeSearch, UAMatch: "applebot"},
	{Token: "DuckDuckBot", Vendor: "DuckDuckGo", Purpose: CrawlerPurposeSearch, UAMatch: "duckduckbot"},
	{Token: "YandexBot", Vendor: "Yandex", Purpose: CrawlerPurposeSearch, UAMatch: "yandexbot"},
	{Token: "Baiduspider", Vendor: "Baidu", Purpose: CrawlerPurposeSearch, UAMatch: "baiduspider"},
}

var crawlersByToken = func() map[string]*KnownCrawler {
	m := make(map[string]*KnownCrawler, len(KnownCrawlers))
	for i := range KnownCrawlers {
		m[KnownCrawlers[i].Token] = &KnownCrawlers[i]
	}
	return m
}()

// CrawlerByToken looks up a crawler by its robots.txt token.
func CrawlerByToken(token string) *KnownCrawler { return crawlersByToken[token] }

// IdentifyCrawler returns the known crawler a User-Agent belongs to, or nil.
func IdentifyCrawler(ua string) *KnownCrawler {
	if ua == "" {
		return nil
	}
	lower := strings.ToLower(ua)
	for i := range KnownCrawlers {
		c := &KnownCrawlers[i]
		if c.UAMatch != "" && strings.Contains(lower, c.UAMatch) {
			return c
		}
	}
	return nil
}

// aiAssistantHosts maps referrer hosts of AI assistants to display names.
var aiAssistantHosts = map[string]string{
	"chatgpt.com":             "ChatGPT",
	"chat.openai.com":         "ChatGPT",
	"perplexity.ai":           "Perplexity",
	"claude.ai":               "Claude",
	"gemini.google.com":       "Gemini",
	"bard.google.com":         "Gemini",
	"copilot.microsoft.com":   "Copilot",
	"copilot.cloud.microsoft": "Copilot",
	"chat.mistral.ai":         "Le Chat",
	"meta.ai":                 "Meta AI",
	"grok.com":                "Grok",
	"chat.deepseek.com":       "DeepSeek",
	"you.com":                 "You.com",
	"phind.com":               "Phind",
	"kagi.com":                "Kagi",
}

// aiAssistantBareNames maps utm_source values that aren't hostnames.
var aiAssistantBareNames = map[string]string{
	"chatgpt":    "chatgpt.com",
	"openai":     "chatgpt.com",
	"perplexity": "perplexity.ai",
	"claude":     "claude.ai",
	"gemini":     "gemini.google.com",
	"copilot":    "copilot.microsoft.com",
}

// AIAssistantForHost returns the assistant name for a referrer host
// ("www." and subdomains of listed hosts included), or "".
func AIAssistantForHost(host string) string {
	h := strings.TrimPrefix(strings.ToLower(strings.TrimSuffix(host, ".")), "www.")
	if name, ok := aiAssistantHosts[h]; ok {
		return name
	}
	for known, name := range aiAssistantHosts {
		if strings.HasSuffix(h, "."+known) {
			return name
		}
	}
	return ""
}

// AIReferrerFromQuery recovers an AI assistant referral that arrived without a
// Referer header. Assistants often strip the referrer but tag outbound links,
// e.g. ChatGPT appends ?utm_source=chatgpt.com. Returns a synthetic referrer
// URL ("https://chatgpt.com/") or "".
func AIReferrerFromQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ""
	}
	src := strings.ToLower(strings.TrimSpace(q.Get("utm_source")))
	if src == "" {
		return ""
	}
	if AIAssistantForHost(src) != "" {
		return "https://" + strings.TrimPrefix(src, "www.") + "/"
	}
	// Bare names some assistants use instead of a host.
	if host, ok := aiAssistantBareNames[src]; ok {
		return "https://" + host + "/"
	}
	return ""
}
