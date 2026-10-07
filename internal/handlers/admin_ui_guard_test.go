package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Guards for three admin UI bugs fixed in 7.4.1: a form nested inside
// another form, a Go value pre-quoted inside inline JS, and server text
// reaching a styled dialog as HTML.

var formTagRe = regexp.MustCompile(`(?i)<form\b|</form\s*>`)

// nestedFormProblems walks the <form> and </form> tags of a template source
// in order and reports every form opened inside another one, and any
// unbalanced tag. Forms built in JS strings count too: they are written
// balanced, so they do not disturb the depth.
func nestedFormProblems(src string) []string {
	var problems []string
	depth := 0
	for _, loc := range formTagRe.FindAllStringIndex(src, -1) {
		line := 1 + strings.Count(src[:loc[0]], "\n")
		if strings.HasPrefix(src[loc[0]:loc[1]], "</") {
			if depth == 0 {
				problems = append(problems, "line "+strconv.Itoa(line)+": </form> with no open form")
				continue
			}
			depth--
			continue
		}
		depth++
		if depth > 1 {
			problems = append(problems, "line "+strconv.Itoa(line)+": <form> opened inside another form")
		}
	}
	if depth != 0 {
		problems = append(problems, "unclosed <form>")
	}
	return problems
}

// templateBody strips the shared layout so line numbers and counts are the
// template's own.
func templateBody(tpl string) string {
	return strings.TrimSuffix(strings.TrimPrefix(tpl, adminLayoutStart), adminLayoutEnd)
}

func sortedAdminTemplateNames() []string {
	names := make([]string, 0, len(adminTemplates))
	for name := range adminTemplates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HTML forms cannot nest: the parser drops the inner <form>, its controls
// join the outer form, and its </form> closes the outer one. "Delete Page"
// in the content editor submitted the Update form this way, and the webhook
// "Regenerate Secret" button saved the webhook. A second form goes outside,
// with the button pointing at it through form="id".
func TestAdminTemplates_NoNestedForms(t *testing.T) {
	// The checker itself
	nested := `<form method="POST" class="form-card"><input name="title">
		<form method="POST" action="/x/delete"><button>Delete</button></form>
		<button>Update</button></form>`
	if got := nestedFormProblems(nested); len(got) != 1 {
		t.Fatalf("checker found %d problems in the nested fixture, want 1: %v", len(got), got)
	}
	flat := `<form id="a"><button form="b">Delete</button></form><FORM id="b"></FORM>
		<script>html += '<form method="POST">'; html += '</form>';</script><div class="form-card"></div>`
	if got := nestedFormProblems(flat); len(got) != 0 {
		t.Fatalf("checker flagged sibling forms: %v", got)
	}
	if got := nestedFormProblems(`<form>`); len(got) != 1 {
		t.Fatalf("checker missed an unclosed form: %v", got)
	}

	forms := 0
	for _, name := range sortedAdminTemplateNames() {
		forms += strings.Count(adminTemplates[name], "<form")
		for _, problem := range nestedFormProblems(templateBody(adminTemplates[name])) {
			t.Errorf("admin template %q: %s; put the second form outside and use form=\"id\" on its button", name, problem)
		}
	}
	for layout, src := range map[string]string{"adminLayoutStart": adminLayoutStart, "adminLayoutEnd": adminLayoutEnd} {
		for _, problem := range nestedFormProblems(src) {
			t.Errorf("%s: %s", layout, problem)
		}
	}
	if forms < 50 {
		t.Fatalf("scan saw only %d forms; the admin templates have far more", forms)
	}

	// Every other Go file that serves HTML (OAuth consent page, SEO tool, ...)
	for _, path := range servedSources(t) {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "admin_templates.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, _ := filepath.Rel(repoRoot, path)
		for _, problem := range nestedFormProblems(string(data)) {
			t.Errorf("%s: %s", filepath.ToSlash(rel), problem)
		}
	}
}

// --- parsed-HTML helpers for the rendered editor page ---

func htmlAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findNodes(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func hasAncestor(n, ancestor *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

// submitButton returns the one submit button with the given label.
func submitButton(t *testing.T, doc *html.Node, label string) *html.Node {
	t.Helper()
	buttons := findNodes(doc, func(n *html.Node) bool {
		return n.Data == "button" && htmlAttr(n, "type") == "submit" && nodeText(n) == label
	})
	if len(buttons) != 1 {
		t.Fatalf("found %d %q submit buttons, want 1", len(buttons), label)
	}
	return buttons[0]
}

// The rendered content editor, parsed the way a browser parses it: Update
// belongs to the edit form, Delete Page to its own form (with the CSRF token
// and the styled confirmation), and the role reaches inline JS as a plain
// string.
func TestEditContent_DeleteFormAndRoleRendering(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Guard Page", "guard-page", "/guard-page")
	rr := csrfAuthGet(t, h, "/cm/content/{id}", "/cm/content/"+contentID.Hex(), h.EditContent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse editor page: %v", err)
	}

	editAction := "/cm/content/" + contentID.Hex()
	editForms := findNodes(doc, func(n *html.Node) bool { return n.Data == "form" && htmlAttr(n, "action") == editAction })
	deleteForms := findNodes(doc, func(n *html.Node) bool { return n.Data == "form" && htmlAttr(n, "action") == editAction+"/delete" })
	if len(editForms) != 1 || len(deleteForms) != 1 {
		t.Fatalf("parsed page has %d edit forms and %d delete forms, want 1 and 1 (a nested form is dropped by the parser)", len(editForms), len(deleteForms))
	}
	editForm, deleteForm := editForms[0], deleteForms[0]
	if hasAncestor(deleteForm, editForm) || hasAncestor(editForm, deleteForm) {
		t.Fatal("the delete form and the edit form are nested")
	}

	update := submitButton(t, doc, "Update")
	if !hasAncestor(update, editForm) || htmlAttr(update, "form") != "" {
		t.Error("Update is not a plain submit button inside the edit form")
	}
	del := submitButton(t, doc, "Delete Page")
	if id := htmlAttr(deleteForm, "id"); id == "" || htmlAttr(del, "form") != id {
		t.Errorf("Delete Page has form=%q, the delete form has id=%q; they must match", htmlAttr(del, "form"), id)
	}
	// The button stays in the editor's action row, beside Update
	if !hasAncestor(del, update.Parent.Parent) {
		t.Error("Delete Page moved out of the form-actions row")
	}
	// Everything the edit form posts is still inside it
	for _, name := range []string{"title", "slug", "published", "hold", "meta_description"} {
		controls := findNodes(doc, func(n *html.Node) bool { return htmlAttr(n, "name") == name })
		if len(controls) == 0 {
			t.Errorf("no %q control on the editor page", name)
		}
		for _, c := range controls {
			if !hasAncestor(c, editForm) {
				t.Errorf("%q control is outside the edit form", name)
			}
		}
	}

	if tokens := findNodes(deleteForm, func(n *html.Node) bool {
		return n.Data == "input" && htmlAttr(n, "name") == "gorilla.csrf.Token" && htmlAttr(n, "value") != ""
	}); len(tokens) != 1 {
		t.Errorf("delete form carries %d CSRF tokens, want 1", len(tokens))
	}
	if onsubmit := htmlAttr(deleteForm, "onsubmit"); !strings.Contains(onsubmit, "confirmDelete(this,") && htmlAttr(deleteForm, "data-confirm") == "" {
		t.Errorf("delete form has no styled confirmation (onsubmit=%q)", onsubmit)
	}

	// The role is compared with === 'admin' when a comment is posted
	if !strings.Contains(body, `const currentUserRole = "admin";`) {
		i := strings.Index(body, "const currentUserRole")
		got := "(missing)"
		if i >= 0 {
			got = strings.SplitN(body[i:], "\n", 2)[0]
		}
		t.Errorf("role is not rendered as a plain JS string: %s", got)
	}
}

// html/template quotes and escapes values in <script> context itself, so
// printf "%q" wraps the value in a second pair of literal quotes (this broke
// copilot CSRF, and the comment Delete button). No served template may use it.
func TestServedTemplates_NoPrintfQuote(t *testing.T) {
	re := regexp.MustCompile(`\{\{-?\s*[^{}]*\bprintf\s+["` + "`" + `][^"` + "`" + `]*%q`)
	if !re.MatchString(`const role = {{printf "%q" .CurrentUserRole}};`) || !re.MatchString(`{{- .X | printf "id=%q" }}`) {
		t.Fatal("checker misses the pre-quoting printf pattern")
	}
	if re.MatchString(`const role = {{.CurrentUserRole}};`) || re.MatchString(`{{printf "%d" .N}}`) {
		t.Fatal("checker flags a bare value")
	}
	for _, path := range servedSources(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, _ := filepath.Rel(repoRoot, path)
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				t.Errorf(`%s:%d pre-quotes a template value with printf "%%q"; write the bare {{.Value}}`, filepath.ToSlash(rel), i+1)
			}
		}
	}
}

var dialogCallRe = regexp.MustCompile(`\b(showAlert|showConfirm)\(`)

// dialogMessageArgs returns the first argument (the message) of every
// showAlert/showConfirm call in src, skipping the function definitions.
func dialogMessageArgs(src string) []string {
	var args []string
	for _, loc := range dialogCallRe.FindAllStringIndex(src, -1) {
		if strings.HasSuffix(src[:loc[0]], "function ") {
			continue
		}
		depth, quote := 0, byte(0)
		i := loc[1]
	scan:
		for ; i < len(src); i++ {
			c := src[i]
			switch {
			case quote != 0:
				if c == '\\' {
					i++
				} else if c == quote {
					quote = 0
				}
			case c == '\'' || c == '"' || c == '`':
				quote = c
			case c == '(' || c == '[' || c == '{':
				depth++
			case c == ')' || c == ']' || c == '}':
				if depth == 0 {
					break scan
				}
				depth--
			case c == ',' && depth == 0:
				break scan
			}
		}
		args = append(args, strings.TrimSpace(src[loc[1]:i]))
	}
	return args
}

var (
	fixedStringRe = regexp.MustCompile(`^'(?:[^'\\]|\\.)*'$|^"(?:[^"\\]|\\.)*"$`)
	// Messages that are deliberately markup. Each mixes fixed HTML with a
	// value that cannot carry markup; anything new must go through dialogText.
	dialogMarkupAllowed = map[string]bool{
		// count is an array length
		`'Are you sure you want to replace text in ' + count + ' page(s)?<br><br>This action will save a version of each page before making changes.'`: true,
	}
)

func dialogMessageIsSafe(arg string) bool {
	if fixedStringRe.MatchString(arg) || dialogMarkupAllowed[arg] {
		return true
	}
	// dialogText(...) wrapping the whole argument
	if !strings.HasPrefix(arg, "dialogText(") || !strings.HasSuffix(arg, ")") {
		return false
	}
	inner := dialogMessageArgs("showAlert(" + arg + ")")
	return len(inner) == 1 && inner[0] == arg
}

// showAlert and showConfirm render their message as HTML. A message is
// either one fixed string or wrapped whole in dialogText(), which escapes it:
// the search-and-replace dialogs used to concatenate data.error and
// err.message straight in.
func TestAdminTemplates_DialogMessagesAreEscaped(t *testing.T) {
	// The checker itself: the pre-fix calls fail, the fixed ones pass
	for _, bad := range []string{
		`showAlert('Error: ' + data.error, 'Replace Failed');`,
		`showAlert('Failed to execute replace: ' + err.message, 'Replace Failed');`,
		`showAlert(e.error||'Failed', 'Approve Failed');`,
		`showAlert(dialogText('Error: ') + data.error, 'Replace Failed');`,
		`if (!(await showConfirm('Delete ' + name + '?', 'Delete'))) return;`,
	} {
		args := dialogMessageArgs(bad)
		if len(args) != 1 || dialogMessageIsSafe(args[0]) {
			t.Fatalf("checker accepts %s (args %q)", bad, args)
		}
	}
	for _, good := range []string{
		`showAlert(dialogText('Error: ' + data.error), 'Replace Failed');`,
		`showAlert(dialogText(err.error || 'Failed, sorry (really)'), 'Comment Failed');`,
		`showAlert('URL copied to clipboard!', 'Copied');`,
		`if (!(await showConfirm('Delete this comment?', 'Delete Comment'))) return;`,
		`showConfirm(dialogText(message), form.getAttribute('data-confirm-title') || 'Confirm').then(function(confirmed) {`,
	} {
		args := dialogMessageArgs(good)
		if len(args) != 1 || !dialogMessageIsSafe(args[0]) {
			t.Fatalf("checker rejects %s (args %q)", good, args)
		}
	}
	if got := dialogMessageArgs(`function showAlert(message, title, callback) {`); len(got) != 0 {
		t.Fatalf("checker treats the definition as a call: %q", got)
	}

	calls := 0
	for _, path := range servedSources(t) {
		rel, _ := filepath.Rel(repoRoot, path)
		rel = filepath.ToSlash(rel)
		if rel == vendoredQuillJS {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, arg := range dialogMessageArgs(string(data)) {
			calls++
			if !dialogMessageIsSafe(arg) {
				t.Errorf("%s: dialog message %s is not a fixed string; wrap the whole message in dialogText()", rel, arg)
			}
		}
	}
	if calls < 20 {
		t.Fatalf("scan saw only %d showAlert/showConfirm calls", calls)
	}

	// The search-and-replace paths by name, and the helper they rely on
	list := adminTemplates["content_list"]
	for _, want := range []string{
		`showAlert(dialogText('Error: ' + data.error), 'Replace Failed');`,
		`showAlert(dialogText('Failed to execute replace: ' + err.message), 'Replace Failed');`,
	} {
		if !strings.Contains(list, want) {
			t.Errorf("content_list is missing %s", want)
		}
	}
	if !strings.Contains(adminLayoutStart, "el.textContent = text == null ? '' : String(text);\n        return el.innerHTML;") {
		t.Error("dialogText no longer escapes through textContent")
	}
}
