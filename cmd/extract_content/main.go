package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	// Extract content from homepage
	homepageHTML, _ := os.ReadFile("/tmp/metavert_pages/homepage.html")
	fmt.Println("=== HOMEPAGE ===")
	extractHomepage(string(homepageHTML))

	// Extract content from concept pages
	conceptPages := []string{
		"virtual-world",
		"blockchain",
		"artificial-intelligence",
		"decentralization",
		"gametech",
		"3d-engine",
		"spatial-computing",
		"virtual-reality",
		"augmented-reality",
		"games",
		"smart-contract",
		"non-fungible-token",
		"live-services",
		"low-code-platform",
		"ray-tracing",
		"distributed-network",
		"immersive-social",
		"infrastructure",
		"creator-economy",
		"megatrends",
		"market-layer",
		"esports",
		"experiences",
		"discovery",
		"human-interface",
	}

	for _, page := range conceptPages {
		html, err := os.ReadFile("/tmp/metavert_pages/" + page + ".html")
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", page, err)
			continue
		}
		fmt.Printf("\n=== %s ===\n", strings.ToUpper(page))
		extractConceptPage(string(html))
	}
}

func extractHomepage(html string) {
	// Find the main content sections
	// Extract text between pre-wrap style tags
	re := regexp.MustCompile(`style="white-space:pre-wrap;">([^<]+)`)
	matches := re.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 && len(strings.TrimSpace(m[1])) > 0 {
			fmt.Println(strings.TrimSpace(m[1]))
		}
	}
}

func extractConceptPage(html string) {
	// Find content in paragraph tags with white-space:pre-wrap
	re := regexp.MustCompile(`<p[^>]*style="white-space:pre-wrap;"[^>]*>([^<]*(?:<[^/][^>]*>[^<]*</[^>]*>)*[^<]*)</p>`)
	matches := re.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			// Clean HTML tags but keep the text
			text := stripTags(m[1])
			if len(strings.TrimSpace(text)) > 0 {
				fmt.Println(strings.TrimSpace(text))
			}
		}
	}
}

func stripTags(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(html, "")
}
