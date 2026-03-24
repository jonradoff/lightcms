package importer

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// MarkdownPage is one parsed Markdown file ready for import
type MarkdownPage struct {
	Filename    string
	Frontmatter map[string]string // key -> value (all as strings)
	Body        string            // Markdown body (after frontmatter)
}

// ParseMarkdownFile parses a single Markdown file with optional YAML frontmatter.
// Frontmatter is delimited by --- on its own line.
func ParseMarkdownFile(filename, content string) MarkdownPage {
	page := MarkdownPage{
		Filename:    filename,
		Frontmatter: make(map[string]string),
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		end := strings.Index(rest, "\n---\n")
		if end == -1 {
			// Try end of string
			if strings.HasSuffix(rest, "\n---") {
				end = len(rest) - 4
			}
		}
		if end >= 0 {
			fmBlock := rest[:end]
			page.Body = strings.TrimPrefix(rest[end+5:], "\n")
			for _, line := range strings.Split(fmBlock, "\n") {
				idx := strings.Index(line, ":")
				if idx < 0 {
					continue
				}
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])
				// Strip surrounding quotes
				if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
					val = val[1 : len(val)-1]
				}
				if key != "" {
					page.Frontmatter[key] = val
				}
			}
		} else {
			page.Body = content
		}
	} else {
		page.Body = content
	}

	return page
}

// ParseMarkdownZip extracts and parses all .md files from a zip archive.
func ParseMarkdownZip(zipData []byte) ([]MarkdownPage, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	var pages []MarkdownPage
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".md" && ext != ".markdown" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s in zip: %w", f.Name, err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, 5*1024*1024))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("reading %s in zip: %w", f.Name, err)
		}
		page := ParseMarkdownFile(f.Name, string(data))
		pages = append(pages, page)
	}
	return pages, nil
}

// FrontmatterGet is a helper to read a frontmatter key case-insensitively
func FrontmatterGet(fm map[string]string, key string) string {
	if v, ok := fm[key]; ok {
		return v
	}
	lower := strings.ToLower(key)
	for k, v := range fm {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}
