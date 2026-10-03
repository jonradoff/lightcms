package services

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLToMarkdown converts rendered page HTML into clean GitHub-flavored
// Markdown for AI agents and crawlers. It keeps document structure
// (headings, paragraphs, lists, links, images, code, tables, quotes) and
// drops presentation and chrome (scripts, styles, navigation, forms).
// Relative links and images are made absolute against baseURL.
func HTMLToMarkdown(src, baseURL string) string {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return ""
	}
	root := findElement(doc, atom.Body)
	if root == nil {
		root = doc
	}
	base, _ := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	c := &mdConverter{base: base}
	c.block(root)
	return tidyMarkdown(c.sb.String())
}

func findElement(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if f := findElement(ch, a); f != nil {
			return f
		}
	}
	return nil
}

type mdConverter struct {
	sb   strings.Builder
	last byte // last byte written (avoids re-reading the buffer)
	base *url.URL
}

func (c *mdConverter) write(s string) {
	if s == "" {
		return
	}
	c.sb.WriteString(s)
	c.last = s[len(s)-1]
}

// skipped elements contribute nothing.
var mdSkip = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true,
	atom.Nav: true, atom.Form: true, atom.Button: true, atom.Input: true,
	atom.Select: true, atom.Textarea: true, atom.Iframe: true, atom.Svg: true,
	atom.Canvas: true, atom.Head: true, atom.Object: true, atom.Embed: true,
}

func (c *mdConverter) abs(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || c.base == nil || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "tel:") {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return c.base.ResolveReference(u).String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hidden(n *html.Node) bool {
	if attr(n, "hidden") != "" || attr(n, "aria-hidden") == "true" {
		return true
	}
	style := strings.ReplaceAll(strings.ToLower(attr(n, "style")), " ", "")
	return strings.Contains(style, "display:none")
}

// block renders children of n as block-level Markdown.
func (c *mdConverter) block(n *html.Node) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.node(ch)
	}
}

func (c *mdConverter) para(s string) {
	s = strings.TrimSpace(s)
	if s != "" {
		c.write("\n\n" + s + "\n\n")
	}
}

func (c *mdConverter) node(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		t := collapseSpace(n.Data)
		if strings.TrimSpace(t) != "" {
			c.write(escapeMDText(t))
		} else if t != "" {
			// Whitespace between inline siblings (e.g. two links) must survive
			// as a single space, or they run together.
			if c.last != 0 && c.last != ' ' && c.last != '\n' {
				c.write(" ")
			}
		}
		return
	case html.ElementNode:
	default:
		return
	}
	if mdSkip[n.DataAtom] || hidden(n) {
		return
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := int(n.Data[1] - '0')
		c.para(strings.Repeat("#", level) + " " + strings.TrimSpace(c.inline(n)))
	case atom.P:
		c.para(c.inline(n))
	case atom.Br:
		c.write("  \n")
	case atom.Hr:
		c.para("---")
	case atom.Pre:
		c.pre(n)
	case atom.Blockquote:
		sub := &mdConverter{base: c.base}
		sub.block(n)
		body := strings.TrimSpace(tidyMarkdown(sub.sb.String()))
		if body != "" {
			lines := strings.Split(body, "\n")
			for i, l := range lines {
				lines[i] = strings.TrimRight("> "+l, " ")
			}
			c.para(strings.Join(lines, "\n"))
		}
	case atom.Ul, atom.Ol:
		c.para(c.list(n, 0))
	case atom.Table:
		c.para(c.table(n))
	case atom.Img:
		c.para(c.img(n))
	case atom.Details:
		c.block(n)
	case atom.Summary:
		c.para("**" + strings.TrimSpace(c.inline(n)) + "**")
	case atom.Figure:
		c.block(n)
	case atom.Figcaption:
		c.para("*" + strings.TrimSpace(c.inline(n)) + "*")
	case atom.Dl:
		c.block(n)
	case atom.Dt:
		c.para("**" + strings.TrimSpace(c.inline(n)) + "**")
	case atom.Dd:
		c.para(c.inline(n))
	default:
		if isInlineAtom(n.DataAtom) {
			// Inline content directly inside a block container: treat as a paragraph run.
			c.write(c.inlineNode(n))
			return
		}
		// div, section, article, main, header, footer, span-as-block, etc.
		c.write("\n\n")
		c.block(n)
		c.write("\n\n")
	}
}

func isInlineAtom(a atom.Atom) bool {
	switch a {
	case atom.A, atom.Strong, atom.B, atom.Em, atom.I, atom.Code, atom.Span, atom.Small,
		atom.Sub, atom.Sup, atom.Mark, atom.Abbr, atom.Cite, atom.Q, atom.U, atom.S, atom.Del,
		atom.Ins, atom.Time, atom.Kbd, atom.Label:
		return true
	}
	return false
}

// inline renders n's children as a single line of inline Markdown.
func (c *mdConverter) inline(n *html.Node) string {
	var sb strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		sb.WriteString(c.inlineNode(ch))
	}
	out := strings.TrimSpace(collapseSpace(sb.String()))
	out = strings.ReplaceAll(out, " "+mdHardBreak+" ", mdHardBreak)
	out = strings.ReplaceAll(out, mdHardBreak+" ", mdHardBreak)
	out = strings.ReplaceAll(out, " "+mdHardBreak, mdHardBreak)
	return strings.ReplaceAll(out, mdHardBreak, "  \n")
}

// mdHardBreak marks a <br> through whitespace collapsing.
const mdHardBreak = "\x01"

func (c *mdConverter) inlineNode(n *html.Node) string {
	switch n.Type {
	case html.TextNode:
		return escapeMDText(collapseSpace(n.Data))
	case html.ElementNode:
	default:
		return ""
	}
	if mdSkip[n.DataAtom] || hidden(n) {
		return ""
	}
	wrap := func(mark string) string {
		inner := c.inline(n)
		if inner == "" {
			return ""
		}
		return mark + inner + mark
	}
	switch n.DataAtom {
	case atom.Strong, atom.B:
		return wrap("**")
	case atom.Em, atom.I, atom.Cite:
		return wrap("*")
	case atom.S, atom.Del:
		return wrap("~~")
	case atom.Code, atom.Kbd:
		t := textContent(n)
		if t == "" {
			return ""
		}
		fence := "`"
		if strings.Contains(t, "`") {
			fence = "``"
		}
		return fence + t + fence
	case atom.A:
		text := c.inline(n)
		href := attr(n, "href")
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return text
		}
		if text == "" {
			if img := findElement(n, atom.Img); img != nil {
				return "[" + c.img(img) + "](" + c.abs(href) + ")"
			}
			return ""
		}
		return "[" + text + "](" + c.abs(href) + ")"
	case atom.Img:
		return c.img(n)
	case atom.Br:
		return mdHardBreak
	}
	// Block elements nested in inline context (e.g. <div> in <a>): flatten.
	return " " + c.inline(n) + " "
}

func (c *mdConverter) img(n *html.Node) string {
	src := attr(n, "src")
	if src == "" || strings.HasPrefix(src, "data:") {
		return ""
	}
	alt := strings.ReplaceAll(attr(n, "alt"), "]", "")
	return "![" + alt + "](" + c.abs(src) + ")"
}

var langClass = regexp.MustCompile(`(?:language|lang)-([A-Za-z0-9_+-]+)`)

func (c *mdConverter) pre(n *html.Node) {
	lang := ""
	code := findElement(n, atom.Code)
	for _, el := range []*html.Node{code, n} {
		if el != nil {
			if m := langClass.FindStringSubmatch(attr(el, "class")); m != nil {
				lang = m[1]
				break
			}
		}
	}
	body := strings.Trim(textContent(n), "\n")
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	c.write("\n\n" + fence + lang + "\n" + body + "\n" + fence + "\n\n")
}

func (c *mdConverter) list(n *html.Node, depth int) string {
	ordered := n.DataAtom == atom.Ol
	idx := 1
	if s, err := strconv.Atoi(attr(n, "start")); err == nil && ordered {
		idx = s
	}
	indent := strings.Repeat("   ", depth)
	var lines []string
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.DataAtom != atom.Li || hidden(li) {
			continue
		}
		marker := "- "
		if ordered {
			marker = strconv.Itoa(idx) + ". "
			idx++
		}
		var text strings.Builder
		var nested []string
		for ch := li.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type == html.ElementNode && (ch.DataAtom == atom.Ul || ch.DataAtom == atom.Ol) {
				nested = append(nested, c.list(ch, depth+1))
				continue
			}
			if ch.Type == html.ElementNode && ch.DataAtom == atom.P {
				text.WriteString(" " + c.inline(ch) + " ")
				continue
			}
			text.WriteString(c.inlineNode(ch))
		}
		lines = append(lines, indent+marker+strings.TrimSpace(collapseSpace(text.String())))
		lines = append(lines, nested...)
	}
	return strings.Join(lines, "\n")
}

func (c *mdConverter) table(n *html.Node) string {
	var rows [][]string
	headerRow := -1
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode {
				continue
			}
			switch ch.DataAtom {
			case atom.Thead, atom.Tbody, atom.Tfoot:
				walk(ch)
			case atom.Tr:
				var cells []string
				allTh := true
				for td := ch.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode || (td.DataAtom != atom.Td && td.DataAtom != atom.Th) {
						continue
					}
					if td.DataAtom != atom.Th {
						allTh = false
					}
					cells = append(cells, strings.ReplaceAll(c.inline(td), "|", `\|`))
				}
				if len(cells) == 0 {
					continue
				}
				if allTh && headerRow < 0 && len(rows) == 0 {
					headerRow = 0
				}
				rows = append(rows, cells)
			}
		}
	}
	walk(n)
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	pad := func(r []string) []string {
		for len(r) < cols {
			r = append(r, "")
		}
		return r
	}
	var sb strings.Builder
	header := pad(rows[0])
	body := rows[1:]
	if headerRow < 0 { // no <th> row: use an empty header so data isn't promoted
		header = pad(nil)
		body = rows
	}
	sb.WriteString("| " + strings.Join(header, " | ") + " |\n")
	sep := make([]string, cols)
	for i := range sep {
		sep[i] = "---"
	}
	sb.WriteString("| " + strings.Join(sep, " | ") + " |")
	for _, r := range body {
		sb.WriteString("\n| " + strings.Join(pad(r), " | ") + " |")
	}
	return sb.String()
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
		}
		if x.Type == html.ElementNode && x.DataAtom == atom.Br {
			sb.WriteString("\n")
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return sb.String()
}

var spaceRun = regexp.MustCompile(`[ \t\r\n\f]+`)

func collapseSpace(s string) string { return spaceRun.ReplaceAllString(s, " ") }

// escapeMDText escapes characters that would otherwise start Markdown syntax
// mid-text. Kept minimal so prose stays readable for agents.
func escapeMDText(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "`", "\\`", "*", `\*`, "[", `\[`, "]", `\]`)
	return r.Replace(s)
}

var (
	trailingSpace = regexp.MustCompile(`[ \t]+\n`)
)

func tidyMarkdown(s string) string {
	// Keep explicit hard breaks ("  \n") but drop other trailing whitespace.
	s = strings.ReplaceAll(s, "  \n", "\x00")
	s = trailingSpace.ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, "\x00", "  \n")
	lines := strings.Split(s, "\n")
	inFence := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "```") {
			inFence = !inFence
			lines[i] = strings.TrimLeft(l, " ")
			continue
		}
		if inFence {
			continue // code is verbatim
		}
		if strings.TrimSpace(l) == "" {
			lines[i] = ""
		} else if !strings.HasPrefix(strings.TrimLeft(l, " "), "-") && !startsWithOrdered(l) && !strings.HasPrefix(l, "    ") {
			lines[i] = strings.TrimLeft(l, " ")
		}
	}
	// Collapse runs of blank lines outside code fences.
	var out []string
	blank, inFence2 := 0, false
	for _, l := range lines {
		if strings.HasPrefix(l, "```") {
			inFence2 = !inFence2
		}
		if l == "" && !inFence2 {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}

var orderedItem = regexp.MustCompile(`^\s*\d+\. `)

func startsWithOrdered(l string) bool { return orderedItem.MatchString(l) }
