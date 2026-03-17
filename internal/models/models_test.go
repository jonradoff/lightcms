package models

import (
	"testing"
)

func TestDefaultTemplates_Count(t *testing.T) {
	if len(DefaultTemplates) != 7 {
		t.Errorf("expected 7 default templates, got %d", len(DefaultTemplates))
	}
}

func TestDefaultTemplates_AllSystem(t *testing.T) {
	for _, tmpl := range DefaultTemplates {
		if !tmpl.IsSystem {
			t.Errorf("expected template %q to be a system template", tmpl.Name)
		}
	}
}

func TestDefaultTemplates_UniqueNames(t *testing.T) {
	seen := map[string]bool{}
	for _, tmpl := range DefaultTemplates {
		if seen[tmpl.Name] {
			t.Errorf("duplicate template name: %q", tmpl.Name)
		}
		seen[tmpl.Name] = true
	}
}

func TestDefaultTemplates_UniqueSlugs(t *testing.T) {
	seen := map[string]bool{}
	for _, tmpl := range DefaultTemplates {
		if seen[tmpl.Slug] {
			t.Errorf("duplicate template slug: %q", tmpl.Slug)
		}
		seen[tmpl.Slug] = true
	}
}

func TestDefaultTemplates_HaveHTMLLayout(t *testing.T) {
	for _, tmpl := range DefaultTemplates {
		if tmpl.HTMLLayout == "" {
			t.Errorf("template %q has empty HTMLLayout", tmpl.Name)
		}
	}
}

func TestDefaultTemplates_HaveFields(t *testing.T) {
	for _, tmpl := range DefaultTemplates {
		if len(tmpl.Fields) == 0 {
			t.Errorf("template %q has no fields", tmpl.Name)
		}
	}
}

func TestDefaultTemplates_RequiredFieldsHaveNames(t *testing.T) {
	for _, tmpl := range DefaultTemplates {
		for _, f := range tmpl.Fields {
			if f.Name == "" {
				t.Errorf("template %q has a field with empty name", tmpl.Name)
			}
			if f.Label == "" {
				t.Errorf("template %q field %q has empty label", tmpl.Name, f.Name)
			}
			validTypes := map[string]bool{
				"text": true, "textarea": true, "richtext": true,
				"date": true, "image": true, "select": true, "markdown": true,
			}
			if !validTypes[f.Type] {
				t.Errorf("template %q field %q has unknown type %q", tmpl.Name, f.Name, f.Type)
			}
		}
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" {
		t.Errorf("expected RoleAdmin='admin', got %q", RoleAdmin)
	}
	if RoleEditor != "editor" {
		t.Errorf("expected RoleEditor='editor', got %q", RoleEditor)
	}
	if RoleViewer != "viewer" {
		t.Errorf("expected RoleViewer='viewer', got %q", RoleViewer)
	}
}

func TestContentStruct_ZeroValue(t *testing.T) {
	var c Content
	if c.Published {
		t.Error("Content should not be published by default")
	}
	if c.Deleted {
		t.Error("Content should not be deleted by default")
	}
}

func TestSnippetStruct_Fields(t *testing.T) {
	s := Snippet{Name: "test", HTML: "<p>hello</p>"}
	if s.Name != "test" {
		t.Errorf("expected 'test', got %q", s.Name)
	}
}

func TestRedirectStatusCodes(t *testing.T) {
	r := Redirect{StatusCode: 301}
	if r.StatusCode != 301 {
		t.Errorf("expected 301, got %d", r.StatusCode)
	}
}
