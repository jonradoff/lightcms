package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/middleware"
)

// Guards over everything the server sends to a browser: the admin templates
// (Go string constants) and the files under static/.

const repoRoot = "../.."

// vendoredQuillJS is third-party and byte-identical to upstream (see
// TestVendoredQuill_MatchesNotice). Its two prompt() fallbacks are
// unreachable with the snow theme; static/admin/quill/README.md explains.
const vendoredQuillJS = "static/admin/quill/quill.js"

// nativeDialogRe matches a call to alert(), confirm() or prompt(), bare or
// through window/self/globalThis/top/parent. A method on some other object
// (dialog.confirm(...)) or a longer name (showConfirm(...)) does not match.
var nativeDialogRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_$.])(?:(?:window|self|globalThis|top|parent)\s*\.\s*)?(alert|confirm|prompt)\s*\(`)

func TestNativeDialogRe(t *testing.T) {
	for _, s := range []string{
		`alert('x')`, `if (!confirm("Delete?")) return;`, `onclick="return confirm('x')"`,
		`var v = prompt ('name')`, `window.alert(1)`, `x=window . confirm(1)`, `self.prompt()`, `;alert(1)`,
	} {
		if !nativeDialogRe.MatchString(s) {
			t.Errorf("no match for %q", s)
		}
	}
	for _, s := range []string{
		`showAlert('x')`, `await showConfirm('x')`, `dialog.confirm('x')`, `confirmRevert(form, 3)`,
		`data-confirm="Delete?"`, `/change-template/{id}/confirm"`, `class="security-alert"`, `$prompt(1)`,
	} {
		if nativeDialogRe.MatchString(s) {
			t.Errorf("unexpected match for %q", s)
		}
	}
}

// servedSources returns every file whose text can reach a browser: Go files
// holding HTML/JS templates, and the JS/HTML/CSS under static/ (uploads are
// site content, not part of the software).
func servedSources(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"internal/handlers/*.go", "internal/oauth/*.go", "cmd/server/*.go"} {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil || len(matches) == 0 {
			t.Fatalf("no files for %s (err %v)", pattern, err)
		}
		for _, m := range matches {
			if !strings.HasSuffix(m, "_test.go") {
				files = append(files, m)
			}
		}
	}
	staticDir := filepath.Join(repoRoot, "static")
	err := filepath.WalkDir(staticDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == filepath.Join(staticDir, "uploads") {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".js", ".mjs", ".html", ".htm", ".css":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk static: %v", err)
	}
	return files
}

// Project rule: never a native browser dialog. Confirmations go through the
// styled confirm modal (data-confirm forms or showConfirm), messages through
// showAlert.
func TestServedTemplatesAndJS_NoNativeDialogs(t *testing.T) {
	sawTemplates, sawChatWidget := false, false
	for _, path := range servedSources(t) {
		rel, _ := filepath.Rel(repoRoot, path)
		rel = filepath.ToSlash(rel)
		sawTemplates = sawTemplates || rel == "internal/handlers/admin_templates.go"
		sawChatWidget = sawChatWidget || rel == "static/js/chat-widget.js"
		if rel == vendoredQuillJS {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if m := nativeDialogRe.FindStringSubmatch(line); m != nil {
				t.Errorf("%s:%d calls the native %s() dialog; use showAlert/showConfirm or a data-confirm form", rel, i+1, m[1])
			}
		}
	}
	if !sawTemplates || !sawChatWidget {
		t.Fatalf("scan missed expected files (admin templates %v, chat widget %v)", sawTemplates, sawChatWidget)
	}

	// The assembled templates too, in case one is built from pieces
	for name, tpl := range adminTemplates {
		if m := nativeDialogRe.FindStringSubmatch(tpl); m != nil {
			t.Errorf("admin template %q calls the native %s() dialog", name, m[1])
		}
	}

	// Destructive forms keep their confirmation, now through data-confirm
	for name, want := range map[string]int{
		"fork_detail":   3, // merge, archive, remove page
		"theme":         1, // revert to version
		"webhooks_list": 1, // delete
		"webhook_form":  1, // regenerate secret
		"imports":       1, // delete RSS source
	} {
		body := strings.TrimSuffix(strings.TrimPrefix(adminTemplates[name], adminLayoutStart), adminLayoutEnd)
		if n := strings.Count(body, `data-confirm="`); n != want {
			t.Errorf("%s has %d data-confirm forms, want %d", name, n, want)
		}
	}
	for _, helper := range []string{"getAttribute('data-confirm')", "function showConfirm(", "function showAlert(", "function dialogText("} {
		if !strings.Contains(adminLayoutStart, helper) {
			t.Errorf("admin layout is missing %s", helper)
		}
	}
}

// adminCSP returns the parsed Content-Security-Policy the server sends for
// admin pages: directive name → source expressions.
func adminCSP(t *testing.T) map[string][]string {
	t.Helper()
	rr := httptest.NewRecorder()
	middleware.SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cm/content", nil))
	header := rr.Header().Get("Content-Security-Policy")
	if header == "" {
		t.Fatal("no Content-Security-Policy on /cm")
	}
	csp := map[string][]string{}
	for _, directive := range strings.Split(header, ";") {
		if fields := strings.Fields(directive); len(fields) > 0 {
			csp[strings.ToLower(fields[0])] = fields[1:]
		}
	}
	return csp
}

// cspAllows reports whether a cross-origin URL is permitted by a directive
// (falling back to default-src). 'self' and keyword sources never match a
// cross-origin URL.
func cspAllows(csp map[string][]string, directive, rawURL string) bool {
	sources, ok := csp[directive]
	if !ok {
		sources = csp["default-src"]
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "" {
		u.Scheme = "https" // protocol-relative; production is HTTPS
	}
	for _, src := range sources {
		switch {
		case src == "*":
			return true
		case strings.HasPrefix(src, "'"):
			continue
		case strings.HasSuffix(src, ":") && !strings.Contains(src, "/"): // scheme source, e.g. https:
			if strings.TrimSuffix(src, ":") == u.Scheme {
				return true
			}
		default: // host source, optionally with scheme and *. wildcard
			scheme, host := "", src
			if i := strings.Index(src, "://"); i >= 0 {
				scheme, host = src[:i], src[i+3:]
			}
			host = strings.TrimSuffix(strings.SplitN(host, "/", 2)[0], ":*")
			if scheme != "" && scheme != u.Scheme {
				continue
			}
			if host == u.Host || (strings.HasPrefix(host, "*.") && strings.HasSuffix(u.Host, host[1:])) {
				return true
			}
		}
	}
	return false
}

var (
	scriptTagRe = regexp.MustCompile(`(?is)<script\b[^>]*>`)
	linkTagRe   = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	srcAttrRe   = regexp.MustCompile(`(?is)\ssrc\s*=\s*["']?([^"'\s>]+)`)
	hrefAttrRe  = regexp.MustCompile(`(?is)\shref\s*=\s*["']?([^"'\s>]+)`)
	relSheetRe  = regexp.MustCompile(`(?is)\srel\s*=\s*["']?[^"'>]*\bstylesheet\b`)
)

type assetRef struct {
	directive string // CSP directive that governs it
	url       string
}

// externalAssetRefs lists the scripts and stylesheets a template loads by URL.
func externalAssetRefs(tpl string) []assetRef {
	var refs []assetRef
	for _, tag := range scriptTagRe.FindAllString(tpl, -1) {
		if m := srcAttrRe.FindStringSubmatch(tag); m != nil {
			refs = append(refs, assetRef{"script-src", m[1]})
		}
	}
	for _, tag := range linkTagRe.FindAllString(tpl, -1) {
		if !relSheetRe.MatchString(tag) {
			continue
		}
		if m := hrefAttrRe.FindStringSubmatch(tag); m != nil {
			refs = append(refs, assetRef{"style-src", m[1]})
		}
	}
	return refs
}

func isCrossOrigin(u string) bool {
	l := strings.ToLower(u)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "//")
}

// checkTemplateAssets returns one problem per script/stylesheet the admin
// CSP would block, or that points at a missing local file.
func checkTemplateAssets(csp map[string][]string, name, tpl string) []string {
	var problems []string
	for _, ref := range externalAssetRefs(tpl) {
		if isCrossOrigin(ref.url) {
			if !cspAllows(csp, ref.directive, ref.url) {
				problems = append(problems, fmt.Sprintf("admin template %q loads %s, which the admin CSP %s (%s) blocks; vendor it under static/admin/",
					name, ref.url, ref.directive, strings.Join(csp[ref.directive], " ")))
			}
			continue
		}
		if strings.HasPrefix(ref.url, "/static/") && !strings.Contains(ref.url, "{{") {
			file := filepath.Join(repoRoot, filepath.FromSlash(strings.SplitN(ref.url, "?", 2)[0]))
			if _, err := os.Stat(file); err != nil {
				problems = append(problems, fmt.Sprintf("admin template %q references %s, which is not in the repo", name, ref.url))
			}
		}
	}
	return problems
}

// The admin CSP (script-src 'self' 'unsafe-inline') silently blocks any
// script or stylesheet loaded from another origin — the rich-text editor was
// dead from v1.1 to v7.4.0 because Quill came from a CDN. Every such
// reference in an admin template must be allowed by the policy the server
// actually sends.
func TestAdminTemplates_ExternalAssetsAllowedByCSP(t *testing.T) {
	csp := adminCSP(t)

	// The checker itself: the old CDN tags fail, the allowed font host passes
	old := `<link href="https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.snow.css" rel="stylesheet">
		<script src="https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js"></script>
		<script src='//cdn.example.com/x.js' defer></script>
		<link rel="stylesheet" href="/static/admin/does-not-exist.css">`
	if got := checkTemplateAssets(csp, "old", old); len(got) != 4 {
		t.Fatalf("checker found %d problems in the CDN fixture, want 4: %v", len(got), got)
	}
	ok := `<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
		<link href="https://fonts.googleapis.com/css2?family=Inter" rel="stylesheet">
		<script>var inline = 1;</script><a href="https://example.com">x</a>`
	if got := checkTemplateAssets(csp, "ok", ok); len(got) != 0 {
		t.Fatalf("checker flagged allowed references: %v", got)
	}

	names := make([]string, 0, len(adminTemplates))
	for name := range adminTemplates {
		names = append(names, name)
	}
	sort.Strings(names)
	scripts := 0
	for _, name := range names {
		for _, ref := range externalAssetRefs(adminTemplates[name]) {
			if ref.directive == "script-src" {
				scripts++
			}
		}
		for _, problem := range checkTemplateAssets(csp, name, adminTemplates[name]) {
			t.Error(problem)
		}
	}
	if scripts == 0 {
		t.Error("found no <script src> in any admin template; the editor's Quill tag should be there")
	}
	for _, name := range []string{"content_form", "theme"} {
		if !strings.Contains(adminTemplates[name], `src="/static/admin/quill/quill.js`) {
			t.Errorf("%s does not load the vendored Quill", name)
		}
	}
}

// The vendored Quill files must be the exact upstream bytes recorded in the
// notice next to them.
func TestVendoredQuill_MatchesNotice(t *testing.T) {
	dir := filepath.Join(repoRoot, "static", "admin", "quill")
	notice, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read notice: %v", err)
	}
	for _, name := range []string{"quill.js", "quill.snow.css", "quill.js.LICENSE.txt", "LICENSE"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("vendored file missing: %v", err)
			continue
		}
		sum := sha256.Sum256(data)
		row := "| `" + name + "` |"
		line := ""
		for _, l := range strings.Split(string(notice), "\n") {
			if strings.HasPrefix(l, row) {
				line = l
			}
		}
		if !strings.Contains(line, "`"+hex.EncodeToString(sum[:])+"`") {
			t.Errorf("%s: SHA-256 %s is not the one recorded in README.md (%q)", name, hex.EncodeToString(sum[:]), line)
		}
	}
	if !strings.Contains(string(notice), "| Version | 2.0.3 |") || !strings.Contains(adminTemplates["content_form"], "quill.js?v=2.0.3") {
		t.Error("README.md version and the ?v= query on the template tags disagree")
	}
}
