package services

import (
	"strings"
	"testing"
)

func TestHTMLToMarkdown(t *testing.T) {
	src := `<html><head><title>x</title><style>.a{}</style></head><body>
<nav><a href="/">Home</a></nav>
<h1>Hello   World</h1>
<p>Some <strong>bold</strong> and <em>italic</em> text with a <a href="/about">link</a>
and <code>inline_code</code>.</p>
<script>alert(1)</script>
<div style="display:none">hidden text</div>
<ul><li>One</li><li>Two<ul><li>Nested</li></ul></li></ul>
<ol start="3"><li>Third</li><li>Fourth</li></ol>
<pre><code class="language-go">func main() {
    fmt.Println("hi")
}</code></pre>
<blockquote><p>Quoted line</p></blockquote>
<table><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody><tr><td>a|b</td><td>1</td></tr></tbody></table>
<img src="/img/x.png" alt="An image">
<details><summary>Question?</summary><p>Answer.</p></details>
<hr>
<p>Line one<br>Line two</p>
</body></html>`
	md := HTMLToMarkdown(src, "https://example.net")

	for _, want := range []string{
		"# Hello World",
		"Some **bold** and *italic* text with a [link](https://example.net/about) and `inline_code`.",
		"- One\n- Two\n   - Nested",
		"3. Third\n4. Fourth",
		"```go\nfunc main() {\n    fmt.Println(\"hi\")\n}\n```",
		"> Quoted line",
		"| Name | Value |\n| --- | --- |\n| a\\|b | 1 |",
		"![An image](https://example.net/img/x.png)",
		"**Question?**\n\nAnswer.",
		"---",
		"Line one  \nLine two",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	for _, bad := range []string{"alert(1)", "hidden text", "Home", ".a{}", "\n\n\n"} {
		if strings.Contains(md, bad) {
			t.Errorf("unexpected %q in:\n%s", bad, md)
		}
	}
}

func TestHTMLToMarkdownInlineSiblings(t *testing.T) {
	md := HTMLToMarkdown(`<div>
<a href="/a">One</a>
<a href="/b">Two</a>
</div>`, "https://x.y")
	if !strings.Contains(md, "[One](https://x.y/a) [Two](https://x.y/b)") {
		t.Errorf("adjacent links ran together: %q", md)
	}
}

func TestHTMLToMarkdownFragmentAndEscaping(t *testing.T) {
	md := HTMLToMarkdown(`<div><p>Price is 5 * 3 [approx]</p><a href="javascript:x()">js</a><a href="#top">top</a></div>`, "https://example.net/")
	if !strings.Contains(md, `Price is 5 \* 3 \[approx\]`) {
		t.Errorf("escaping: %s", md)
	}
	if strings.Contains(md, "javascript:") {
		t.Errorf("javascript link kept: %s", md)
	}
	if !strings.Contains(md, "[top](#top)") {
		t.Errorf("fragment link: %s", md)
	}
	if HTMLToMarkdown("", "https://x.y") != "\n" && strings.TrimSpace(HTMLToMarkdown("", "https://x.y")) != "" {
		t.Error("empty input should give empty output")
	}
}
