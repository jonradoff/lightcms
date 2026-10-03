package services

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// FAQItem is one question/answer pair found in a page.
type FAQItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

const maxFAQAnswer = 2000

// ExtractFAQ finds explicitly structured FAQs in rendered page HTML, for
// FAQPage structured data. Only two unambiguous patterns count, to avoid
// marking up ordinary content as Q&A:
//   - <details><summary>Question?</summary>answer…</details>
//   - a heading "FAQ" / "FAQs" / "Frequently Asked Questions", followed by
//     lower-level headings ending in "?", each answered by the content up to
//     the next heading.
func ExtractFAQ(src string) []FAQItem {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return nil
	}
	var items []FAQItem
	seen := map[string]bool{}
	add := func(q, a string) {
		q = strings.TrimSpace(collapseSpace(q))
		a = strings.TrimSpace(collapseSpace(a))
		if q == "" || a == "" || !strings.HasSuffix(q, "?") || seen[q] {
			return
		}
		if len(a) > maxFAQAnswer {
			a = a[:maxFAQAnswer] + "…"
		}
		seen[q] = true
		items = append(items, FAQItem{Question: q, Answer: a})
	}

	// Pattern 1: <details><summary>
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Details {
			var q string
			var a strings.Builder
			for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
				if ch.Type == html.ElementNode && ch.DataAtom == atom.Summary && q == "" {
					q = visibleText(ch)
					continue
				}
				a.WriteString(" " + visibleText(ch))
			}
			add(q, a.String())
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	// Pattern 2: FAQ section of headings. Flatten the document into a
	// sequence of headings and text so sections spanning wrapper divs work.
	type seg struct {
		level int // 1-6 for headings, 0 for text
		text  string
	}
	var segs []seg
	var flatten func(*html.Node)
	flatten = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Nav, atom.Details:
				return
			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
				segs = append(segs, seg{level: int(n.Data[1] - '0'), text: visibleText(n)})
				return
			}
		}
		if n.Type == html.TextNode {
			if t := strings.TrimSpace(n.Data); t != "" {
				segs = append(segs, seg{text: t})
			}
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			flatten(ch)
		}
	}
	flatten(doc)

	for i := 0; i < len(segs); i++ {
		if segs[i].level == 0 || !isFAQHeading(segs[i].text) {
			continue
		}
		faqLevel := segs[i].level
		var q string
		var a strings.Builder
		flush := func() {
			if q != "" {
				add(q, a.String())
			}
			q = ""
			a.Reset()
		}
		for j := i + 1; j < len(segs); j++ {
			s := segs[j]
			if s.level > 0 && s.level <= faqLevel {
				break // end of the FAQ section
			}
			if s.level > 0 {
				flush()
				q = s.text
				continue
			}
			a.WriteString(" " + s.text)
		}
		flush()
	}
	return items
}

func isFAQHeading(t string) bool {
	t = strings.ToLower(strings.Trim(strings.TrimSpace(t), ":"))
	switch t {
	case "faq", "faqs", "frequently asked questions", "frequently asked questions (faq)", "common questions":
		return true
	}
	return false
}

func visibleText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && (x.DataAtom == atom.Script || x.DataAtom == atom.Style) {
			return
		}
		if x.Type == html.TextNode {
			sb.WriteString(x.Data + " ")
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.TrimSpace(collapseSpace(sb.String()))
}
