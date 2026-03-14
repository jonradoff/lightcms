# LightCMS MCP API Reference

This document provides a complete reference for the LightCMS Model Context Protocol (MCP) server. The MCP server enables AI agents like Claude Code to manage your website through natural language.

## Overview

The LightCMS MCP server exposes 63 tools organized into these categories:

| Category | Tools | Description |
|----------|-------|-------------|
| [Content](#content-tools) | 22 | Create, edit, publish, bulk-update, version, and delete content |
| [Templates](#template-tools) | 5 | Define content structures with custom fields |
| [Snippets](#snippet-tools) | 5 | Reusable HTML fragments for content composition |
| [Assets](#asset-tools) | 6 | Upload and manage images, documents, and files |
| [Theme](#theme-tools) | 7 | Customize colors, fonts, header/footer; version history |
| [Site Config](#site-config-tools) | 2 | Configure site-wide settings |
| [Redirects](#redirect-tools) | 4 | Manage URL redirects |
| [Folders](#folder-tools) | 4 | Organize content into URL hierarchies |
| [Collections](#collection-tools) | 5 | Group and display content by category |
| [Search](#search-tools) | 7 | Search content, bulk find/replace, end-user search |
| [Utility](#utility-tools) | 1 | Regenerate all static pages |

---

## Content Tools

### list_content

List all content items with optional filters.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `folder_id` | string | No | Filter by folder ID (MongoDB ObjectID) |
| `category` | string | No | Filter by content category |
| `include_deleted` | boolean | No | Include soft-deleted content |
| `include_data` | boolean | No | Include full template field values in results |
| `include_fields` | array of strings | No | Return only specific template fields (e.g., `["body", "layer_badge"]`). Implies `include_data: true`. |
| `tag` | string | No | Filter by tag |
| `template_name` | string | No | Filter by template name |

**Returns:** Array of content summaries with id, title, slug, full_path, category, published, deleted, updated_at, published_at. When `include_data` or `include_fields` is set, each item also contains a `data` object with the requested field values.

---

### get_content

Get a single content item by ID or path.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | No* | Content ID (MongoDB ObjectID) |
| `path` | string | No* | Content path (e.g., `/about` or `/blog/my-post`) |
| `include_rendered` | boolean | No | Include fully rendered HTML in response |

*One of `id` or `path` is required.

**Returns:** Complete content object including all field data. When `include_rendered` is true, also includes `rendered_html` containing the fully rendered page HTML (template fields interpolated + theme header/footer applied).

---

### create_content

Create a new content item.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `template_id` | string | Yes | Template ID (MongoDB ObjectID) |
| `title` | string | Yes | Content title |
| `slug` | string | Yes | URL slug for the content |
| `data` | object | Yes | Template field values |
| `folder_path` | string | No | Folder path (e.g., `/blog`) |
| `category` | string | No | Content category for collections |
| `tags` | array of strings | No | Tags for this content item |
| `meta_description` | string | No | SEO meta description |
| `og_image` | string | No | Open Graph image URL |
| `use_header` | boolean | No | Include site header (default: true) |
| `use_footer` | boolean | No | Include site footer (default: true) |
| `use_theme` | boolean | No | Apply site theme/layout (default: true) |
| `raw_mode` | boolean | No | Use raw HTML mode |
| `published` | boolean | No | Publish immediately |
| `version_comment` | string | No | Optional comment describing this version |

**Returns:** `{ success, id, full_path, message }`

**Example:**
```json
{
  "template_id": "6971098ad0761968133b8e43",
  "title": "My First Post",
  "slug": "my-first-post",
  "folder_path": "/blog",
  "category": "blog",
  "tags": ["AI & Machine Intelligence", "Getting Started"],
  "data": {
    "excerpt": "A brief introduction...",
    "content": "<p>Full article content here...</p>",
    "author": "Jane Doe"
  },
  "published": true,
  "version_comment": "Initial creation"
}
```

---

### update_content

Update an existing content item. Creates a new version automatically. Uses merge semantics — only supplied fields change.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |
| `template_id` | string | No | Change template |
| `title` | string | No | Update title |
| `slug` | string | No | Update URL slug |
| `folder_path` | string | No | Move to different folder |
| `category` | string | No | Update category |
| `tags` | array of strings | No | Replace tags array |
| `meta_description` | string | No | Update SEO description |
| `og_image` | string | No | Update Open Graph image |
| `data` | object | No | Update field values (merged with existing) |
| `clear_fields` | array of strings | No | Field names to explicitly clear to empty string |
| `use_header` | boolean | No | Toggle site header |
| `use_footer` | boolean | No | Toggle site footer |
| `use_theme` | boolean | No | Toggle theme |
| `raw_mode` | boolean | No | Toggle raw HTML mode |
| `version_comment` | string | No | Optional comment describing this version change |
| `dry_run` | boolean | No | Validate payload without committing changes |

Only include fields you want to change. Use `clear_fields` to set a field to empty string explicitly (plain `data` omits preserve existing values).

**Returns:** `{ success, id, full_path, message }`. When `dry_run: true`, returns `{ dry_run: true, valid: true, message }` without writing.

---

### update_content_by_path

Update content by URL path instead of ID. Useful when you know the page URL but not its MongoDB ID.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `path` | string | Yes | URL path of the content (e.g., `/blog/my-post`) |
| `title` | string | No | Update title |
| `slug` | string | No | Update URL slug |
| `folder_path` | string | No | Move to different folder |
| `category` | string | No | Update category |
| `tags` | array of strings | No | Replace tags array |
| `meta_description` | string | No | Update SEO description |
| `og_image` | string | No | Update Open Graph image |
| `data` | object | No | Update field values (merged with existing) |
| `clear_fields` | array of strings | No | Field names to explicitly clear |
| `version_comment` | string | No | Comment describing this change |

**Returns:** `{ success, id, full_path, message }`

---

### publish_content

Publish a content item, making it visible on the public site.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |

**Returns:** Success message.

---

### publish_multiple

Batch publish multiple content items in one call.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `ids` | array of strings | No* | Array of content IDs to publish |
| `publish_all_drafts` | boolean | No* | Publish all draft content items |

*One of `ids` or `publish_all_drafts` is required.

**Returns:** `{ success, published_count, message }`

---

### unpublish_content

Remove content from the public site (keeps it in the system).

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |

**Returns:** Success message.

---

### delete_content

Soft-delete a content item. Can be restored later.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |

**Returns:** Success message.

---

### restore_content

Restore a soft-deleted content item.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |

**Returns:** Success message.

---

### preview_content

Render a content item's HTML without publishing. Accepts optional field overrides to preview unsaved edits.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |
| `title` | string | No | Override title for preview |
| `data` | object | No | Override field values for preview |

**Returns:** `{ rendered_html, warnings }`. Warnings include missing required fields, unclosed tags, and unresolved placeholders.

---

### get_content_versions

Get version history for a content item. Requires `PermContentView`.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `content_id` | string | Yes | Content ID (MongoDB ObjectID) |

**Returns:** Array of version summaries with version number, title, slug, full_path, published, comment (if any), created_at, and the email of the editor who made the change.

---

### get_content_version

Get a specific version of a content item. Requires `PermContentView`.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `content_id` | string | Yes | Content ID (MongoDB ObjectID) |
| `version` | integer | Yes | Version number |

**Returns:** Complete content object at that version, including comment and editor email if any.

---

### revert_to_version

Revert content to a previous version. Creates a new version with the old data.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `content_id` | string | Yes | Content ID (MongoDB ObjectID) |
| `version` | integer | Yes | Version number to revert to |
| `version_comment` | string | No | Optional comment for the revert (e.g., "Reverted to v3") |

**Returns:** Success message.

---

### bulk_update_content

Update up to 100 content items in a single call. Each item uses merge semantics — only supplied fields change. Safer and more efficient than N concurrent `update_content` calls.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `updates` | array | Yes | Array of update objects (max 100) |
| `version_comment` | string | No | Version comment applied to all updated items |
| `dry_run` | boolean | No | Validate all payloads without committing |

**Update object:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |
| `title` | string | No | Update title |
| `data` | object | No | Update field values (merged) |
| `clear_fields` | array of strings | No | Field names to clear |
| `tags` | array of strings | No | Replace tags array |
| `category` | string | No | Update category |
| `meta_description` | string | No | Update SEO description |

**Returns:** `{ success, updated_count, errors: [{ id, error }] }`. Individual item errors do not abort the batch — successes and failures are reported separately. When `dry_run: true`, returns validation results without writing.

**Concurrency guidance:** Up to 20 concurrent `update_content` calls are safe; larger batches should use `bulk_update_content`.

**Example:**
```json
{
  "updates": [
    { "id": "abc123", "data": { "layer_badge": "<span class=\"badge new\">New</span>" } },
    { "id": "def456", "data": { "layer_badge": "<span class=\"badge new\">New</span>" } }
  ],
  "version_comment": "Updated layer badges to v2 design"
}
```

---

### bulk_field_operation

Apply a single operation to a field across all matching content items. Useful for mass content transformations.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `operation` | string | Yes | One of: `clear`, `set`, `prepend`, `append`, `wrap` |
| `field` | string | Yes | Template field name to operate on |
| `value` | string | No | Value for `set`, `prepend`, `append` operations |
| `wrap_before` | string | No | HTML prepended before existing value (`wrap` only) |
| `wrap_after` | string | No | HTML appended after existing value (`wrap` only) |
| `template_name` | string | No | Scope to content using this template |
| `folder_path` | string | No | Scope to content in this folder path |
| `category` | string | No | Scope to content with this category |
| `content_ids` | array of strings | No | Scope to specific content IDs |
| `version_comment` | string | No | Version comment applied to all updated items |
| `dry_run` | boolean | No | Preview affected pages without writing |

**Operations:**
- `clear` — sets field to empty string
- `set` — replaces field with `value`
- `prepend` — inserts `value` before existing content
- `append` — inserts `value` after existing content
- `wrap` — surrounds existing content with `wrap_before` + existing + `wrap_after`

**Limits:** Returns 400 if the operation would affect more than 500 pages.

**Returns:** `{ success, affected_count, pages: [...] }`. When `dry_run: true`, returns the list of pages that would be modified.

**Example:**
```json
{
  "operation": "prepend",
  "field": "body",
  "value": "<div class=\"disclaimer\">Views are my own.</div>",
  "template_name": "Blog Post",
  "version_comment": "Added disclaimer to all posts"
}
```

---

### export_content

Export content items with full field data as a structured JSON array. Designed for the export → transform → `bulk_update_content` pipeline. Requires authentication.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `template_name` | string | No | Scope to content using this template |
| `folder_path` | string | No | Scope to content in this folder path |
| `category` | string | No | Scope to content with this category |
| `content_ids` | array of strings | No | Export specific content IDs |
| `fields` | array of strings | No | Return only specific template fields |
| `include_unpublished` | boolean | No | Include draft/unpublished content |

**Returns:** Array of content objects, each with `id`, `title`, `full_path`, `published`, `tags`, and `data` containing the requested field values.

**Typical workflow:**
1. `export_content` with `template_name` and `fields`
2. Transform/enrich the data externally
3. `bulk_update_content` with the transformed data

---

### get_backlinks

Return all published pages that link to a given path via wikilinks or `<a>` tags. Useful for impact assessment before renaming or deleting a page.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `path` | string | Yes | URL path to check for backlinks (e.g., `/concepts/machine-learning`) |

**Returns:** Array of content summaries with id, title, full_path, and published status for each page that contains a link to the specified path.

---

## Template Tools

### list_templates

List all available templates.

**Parameters:** None

**Returns:** Array of template summaries with id, name, slug, description, category, is_system, field_count.

---

### get_template

Get a template by ID or slug.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | No* | Template ID (MongoDB ObjectID) |
| `slug` | string | No* | Template slug |

*One of `id` or `slug` is required.

**Returns:** Complete template including fields array and html_layout.

---

### create_template

Create a new template.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Template name |
| `slug` | string | Yes | Template slug for URLs |
| `fields` | array | Yes | Field definitions (see below) |
| `html_layout` | string | Yes | HTML with `{{.FieldName}}` placeholders |
| `description` | string | No | Template description |
| `category` | string | No | Template category |

**Field Definition:**
```json
{
  "name": "field_name",
  "label": "Display Label",
  "type": "text|textarea|richtext|date|image|select|markdown",
  "required": true,
  "placeholder": "Placeholder text",
  "default": "Default value",
  "options": "option1,option2,option3"
}
```

The `markdown` field type renders GFM (GitHub Flavored Markdown) at page generation time. Use `{{.lc_toc}}` in the HTML layout to auto-generate a table of contents from page headings.

**Returns:** `{ success, id, message }`

---

### update_template

Update an existing template. Changing html_layout regenerates all content using this template.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Template ID (MongoDB ObjectID) |
| `name` | string | No | Update name |
| `slug` | string | No | Update slug |
| `description` | string | No | Update description |
| `category` | string | No | Update category |
| `fields` | array | No | Update field definitions |
| `html_layout` | string | No | Update HTML layout |

**Returns:** `{ success, id, message }`

---

### delete_template

Delete a template. Cannot delete system templates or templates with existing content.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Template ID (MongoDB ObjectID) |

**Returns:** Success message.

---

## Snippet Tools

Snippets are named HTML template fragments with Go template variable support. They are used in two ways:
1. **Inline in content**: via `[[include:snippet-name]]` wikilink syntax
2. **In template layouts**: via `lc:query` directives that render matched content items through a snippet

### list_snippets

List all snippets.

**Parameters:** None

**Returns:** Array of snippets with id, name, description, created_at, updated_at.

---

### get_snippet

Get a snippet by ID or name.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | No* | Snippet ID (MongoDB ObjectID) |
| `name` | string | No* | Snippet name |

*One of `id` or `name` is required.

**Returns:** Complete snippet including html content.

---

### create_snippet

Create a new named HTML snippet.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Unique snippet name (used in `[[include:name]]` and `lc:query`) |
| `html` | string | Yes | HTML content with optional Go template variables |
| `description` | string | No | Human-readable description |

**Available template variables:** `{{.Title}}`, `{{.FullPath}}`, `{{.Slug}}`, `{{.MetaDescription}}`, `{{.PublishedAt}}`

Snippets render through Go's `html/template`, so struct fields are HTML-escaped by default — safe from XSS.

**Returns:** `{ success, id, message }`

---

### update_snippet

Update a snippet's name or HTML.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Snippet ID (MongoDB ObjectID) |
| `name` | string | No | Update name |
| `html` | string | No | Update HTML content |
| `description` | string | No | Update description |

**Returns:** Success message.

---

### delete_snippet

Delete a snippet.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Snippet ID (MongoDB ObjectID) |

**Returns:** Success message.

---

## Asset Tools

### list_assets

List all assets in the library.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `folder` | string | No | Filter by folder path (e.g., `/images`) |

**Returns:** Array of asset summaries with id, filename, folder, serve_path, mime_type, size, description, created_at.

---

### list_asset_folders

List all unique folder paths in the asset library.

**Parameters:** None

**Returns:** Array of folder path strings.

---

### get_asset

Get asset metadata by ID or path.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | No* | Asset ID (MongoDB ObjectID) |
| `path` | string | No* | Asset serve path (e.g., `/images/logo.png`) |

*One of `id` or `path` is required.

**Returns:** Asset metadata (not file content — use serve_path to access the file).

---

### upload_asset

Upload a new asset.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `filename` | string | Yes | Original filename with extension |
| `serve_path` | string | Yes | URL path (must begin with `/assets/`, `/images/`, `/docs/`, `/media/`, or `/files/`) |
| `data_base64` | string | Yes | Base64-encoded file content |
| `description` | string | No | Asset description |

**Allowed file types:** Images (jpg, png, gif, webp, svg, ico), Documents (pdf, doc, docx, txt, md), Web (css, js, json, xml, html), Fonts (woff, woff2, ttf, otf, eot)

**Returns:** `{ success, id, serve_path, mime_type, size, message }`

---

### upload_asset_from_url

Fetch a remote URL and store it as a LightCMS asset. SSRF-protected: private/reserved IP ranges are blocked.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `url` | string | Yes | HTTP/HTTPS URL to fetch (max 50 MB) |
| `serve_path` | string | Yes | URL path where the asset will be served |
| `description` | string | No | Asset description |

**Returns:** `{ success, id, serve_path, mime_type, size, message }`

---

### delete_asset

Delete an asset.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Asset ID (MongoDB ObjectID) |

**Returns:** Success message.

---

## Theme Tools

### get_theme

Get current theme settings.

**Parameters:** None

**Returns:** Theme object with all settings:
- Colors: `primary_color`, `secondary_color`, `accent_color`, `background_color`, `text_color`
- Typography: `font_family`, `heading_font`, `border_radius`
- Branding: `site_name`, `site_tagline`, `logo_url`
- Custom: `custom_css`, `head_html`, `header_html`, `footer_html`

---

### update_theme

Update theme settings. Changing header_html or footer_html regenerates all published content.

**Parameters:** (all optional)
| Name | Type | Description |
|------|------|-------------|
| `primary_color` | string | Hex color (e.g., `#6366f1`) |
| `secondary_color` | string | Hex color |
| `accent_color` | string | Hex color |
| `background_color` | string | Hex color |
| `text_color` | string | Hex color |
| `font_family` | string | CSS font (e.g., `Inter, sans-serif`) |
| `heading_font` | string | CSS font for headings |
| `border_radius` | string | CSS value (e.g., `12px`) |
| `site_name` | string | Site name |
| `site_tagline` | string | Site tagline |
| `logo_url` | string | Logo image URL |
| `custom_css` | string | Additional CSS |
| `head_html` | string | Custom `<head>` HTML |
| `header_html` | string | Custom header HTML |
| `footer_html` | string | Custom footer HTML |

**Returns:** Success message.

---

### get_theme_versions

Get version history for the theme.

**Parameters:** None

**Returns:** Array of theme version summaries with version number, created_at, and locked status.

---

### get_theme_version

Get a specific theme version.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `version` | integer | Yes | Theme version number |

**Returns:** Complete theme settings at that version.

---

### revert_theme_to_version

Revert the theme to a previous version.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `version` | integer | Yes | Theme version number to revert to |

**Returns:** Success message.

---

### pin_theme_version

Lock a theme version to protect it from auto-pruning.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `version` | integer | Yes | Theme version number to pin |

**Returns:** Success message.

---

### unpin_theme_version

Unlock a pinned theme version.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `version` | integer | Yes | Theme version number to unpin |

**Returns:** Success message.

---

## Site Config Tools

### get_site_config

Get site configuration.

**Parameters:** None

**Returns:** Site config with `title_template`, `title_template_no_title`, and `markdown_script_policy`.

---

### update_site_config

Update site configuration.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `title_template` | string | No | Title template with `{{title}}` and `{{site_name}}` |
| `title_template_no_title` | string | No | Title when page has no title |
| `markdown_script_policy` | string | No | Script policy: `"all"`, `"admin_only"`, or `"none"` |

**Script policy values:**
- `"all"` (default) — all users may use raw HTML including `<script>` in markdown fields
- `"admin_only"` — editors' content is sanitized; admin content passes through unchanged
- `"none"` — all content sanitized regardless of author role

**Example:** `"{{title}} | {{site_name}}"`

**Returns:** Success message.

---

## Redirect Tools

### list_redirects

List all URL redirects.

**Parameters:** None

**Returns:** Array of redirects with id, from_path, to_path, status_code, description.

---

### create_redirect

Create a new redirect.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `from_path` | string | Yes | Source path (e.g., `/old-page`) |
| `to_path` | string | Yes | Destination path or URL |
| `status_code` | integer | No | 301 (permanent) or 302 (temporary). Default: 301 |
| `description` | string | No | Description/note |

**Returns:** `{ success, id, message }`

---

### update_redirect

Update an existing redirect.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Redirect ID (MongoDB ObjectID) |
| `from_path` | string | No | Update source path |
| `to_path` | string | No | Update destination |
| `status_code` | integer | No | Update status code |
| `description` | string | No | Update description |

**Returns:** Success message.

---

### delete_redirect

Delete a redirect.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Redirect ID (MongoDB ObjectID) |

**Returns:** Success message.

---

## Folder Tools

### list_folders

List all content folders.

**Parameters:** None

**Returns:** Array of folders with id, name, slug, path, parent_id.

---

### create_folder

Create a new folder.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Display name |
| `slug` | string | Yes | URL segment |
| `parent_id` | string | No | Parent folder ID for nesting |

**Returns:** `{ success, id, path, message }`

---

### get_folder

Get a folder by ID.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Folder ID (MongoDB ObjectID) |

**Returns:** Complete folder object.

---

### delete_folder

Delete an empty folder.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Folder ID (MongoDB ObjectID) |

Cannot delete folders containing content or subfolders.

**Returns:** Success message.

---

## Collection Tools

### list_collections

List all content collections.

**Parameters:** None

**Returns:** Array of collections with id, name, slug, description, category, sort_field, sort_order, items_per_page.

---

### create_collection

Create a new collection.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Collection name |
| `slug` | string | Yes | URL slug |
| `description` | string | No | Description |
| `category` | string | No | Content category to include |
| `sort_field` | string | No | Field to sort by |
| `sort_order` | string | No | `asc` or `desc` |
| `item_template` | string | No | HTML template for each item |
| `page_template` | string | No | HTML template for collection page |
| `items_per_page` | integer | No | Pagination size |

**Returns:** `{ success, id, message }`

---

### get_collection

Get a collection by ID.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Collection ID (MongoDB ObjectID) |

**Returns:** Complete collection object.

---

### update_collection

Update a collection.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Collection ID (MongoDB ObjectID) |
| `name` | string | No | Update name |
| `slug` | string | No | Update slug |
| `description` | string | No | Update description |
| `category` | string | No | Update category filter |
| `sort_field` | string | No | Update sort field |
| `sort_order` | string | No | Update sort order |
| `item_template` | string | No | Update item template |
| `page_template` | string | No | Update page template |
| `items_per_page` | integer | No | Update pagination |

**Returns:** Success message.

---

### delete_collection

Delete a collection. Does not delete the content in the collection.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Collection ID (MongoDB ObjectID) |

**Returns:** Success message.

---

## Search Tools

### search_content

Search across all content items by title or full text.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | Yes | Search query string |
| `search_type` | string | No | `name` (title only) or `fulltext` (all fields). Default: `fulltext` |
| `include_deleted` | boolean | No | Include soft-deleted content |

**Returns:** Object with query, search_type, total count, and matches array. Each match includes id, title, full_path, template_name, published status, and matched_in (array of field names where match was found). Admin search also matches on slug and full URL path.

---

### search_replace_preview

Preview search and replace results without making changes. **Always use this before `search_replace_execute`** to understand the impact.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `search` | string | Yes | Text to search for (or RE2 regex when `regex: true`) |
| `replace` | string | Yes | Text to replace with (use `$1`, `$2` for capture groups) |
| `regex` | boolean | No | Treat `search` as a Go RE2 regular expression |

**Returns:** Object with:
- `search`, `replace`: The search/replace terms
- `total_matches`: Total number of text matches found
- `affected_pages`: Number of content pages that will be modified
- `published_pages`: Number of published pages affected
- `draft_pages`: Number of draft pages affected
- `matches`: Array of affected content with details

---

### search_replace_execute

Execute search and replace across all content. **Warning: This is a destructive operation.** Always run `search_replace_preview` first.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `search` | string | Yes | Text to search for (or RE2 regex when `regex: true`) |
| `replace` | string | Yes | Text to replace with (use `$1`, `$2` for capture groups) |
| `regex` | boolean | No | Treat `search` as a Go RE2 regular expression |
| `version_comment` | string | No | Comment for version history |

**Regex limits:** Pattern max 500 chars, max 20 capture groups, replacement expansion capped at 10×.

**Returns:** `{ success, search, replace, total_replacements, pages_updated, published_pages, version_comment, updated_pages }`

---

### scoped_search_replace_preview

Preview search and replace scoped to specific content. Safer than site-wide replacement.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `search` | string | Yes | Text to search for (or RE2 regex when `regex: true`) |
| `replace` | string | Yes | Text to replace with |
| `regex` | boolean | No | Treat `search` as a Go RE2 regular expression |
| `content_ids` | array of strings | No | Scope to specific content IDs |
| `folder_path` | string | No | Scope to folder (exact prefix match, e.g. `/blog` won't match `/blog-old`) |
| `template_name` | string | No | Scope to content using this template |
| `category` | string | No | Scope to content with this category |

**Returns:** Same as `search_replace_preview`.

---

### scoped_search_replace_execute

Execute scoped search and replace. **Warning: Destructive operation.** Always preview first.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `search` | string | Yes | Text to search for (or RE2 regex when `regex: true`) |
| `replace` | string | Yes | Text to replace with |
| `regex` | boolean | No | Treat `search` as a Go RE2 regular expression |
| `content_ids` | array of strings | No | Scope to specific content IDs |
| `folder_path` | string | No | Scope to folder (exact prefix match) |
| `template_name` | string | No | Scope to content using this template |
| `category` | string | No | Scope to content with this category |
| `version_comment` | string | No | Comment for version history |

**Returns:** `{ success, search, replace, total_replacements, pages_updated, published_pages, version_comment, updated_pages }`

---

### end_user_search

Search published content using the end-user search engine (full-text + optional semantic vector search).

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | Yes | Search query |
| `mode` | string | No | `fulltext`, `semantic`, or `hybrid` (default). `hybrid` uses reciprocal rank fusion. |
| `limit` | integer | No | Max results to return |

**Returns:** Ranked array of search results with id, title, full_path, excerpt, and score.

---

### reindex_embeddings

Trigger background regeneration of semantic search embeddings for all published content. Requires admin permissions.

**Parameters:** None

**Returns:** Success message with embedding count.

---

## Utility Tools

### regenerate_all_content

Regenerate all published static HTML pages. Use after major theme or template changes.

**Parameters:** None

**Returns:** Success message with count of regenerated pages.

---

## Content Authoring Markup

The following markup features are processed at page generation time:

### Wikilinks

Link between pages using double-bracket syntax. Resolved at publish time and auto-updated when a page's title or path changes.

| Syntax | Description |
|--------|-------------|
| `[[Page Title]]` | Link to a page by title (case-insensitive) |
| `[[Page Title\|display text]]` | Link with custom label |
| `[[/full/path]]` | Link by URL path |
| `[[/full/path\|display text]]` | Path link with custom label |

- Broken links render as `<span class="broken-link">text</span>`
- Links that resolve to a non-`/`-prefixed href are treated as broken (security)
- Use `get_backlinks` to find all pages linking to a given path

### Snippet Includes

```
[[include:snippet-name]]
```

Embeds a named snippet inline. Recursion depth limit: 3 levels. Cycles are detected and dropped.

### Table of Contents

Add `{{.lc_toc}}` in a template's HTML layout where the TOC should appear:

```html
<nav class="sidebar">
  {{.lc_toc}}
</nav>
<article>{{.body}}</article>
```

Renders as `<nav class="lc-toc"><ul>...</ul></nav>` based on heading elements. All headings automatically get `id=` attributes for anchor navigation.

### Markdown Fields

Set a template field type to `markdown` to enable GFM rendering at publish time:
- Tables, strikethrough, task lists, autolinks
- Script policy controls whether raw HTML/scripts are passed through or sanitized
- Raw `<script>` and event handlers stripped when policy is `"admin_only"` (for editors) or `"none"` (for everyone)

### Inline Tag Detection

Mentioning `#tagname` in any content field automatically adds that tag to the page:

- Tag names must start with a letter, then alphanumeric/underscore/hyphen, max 50 chars
- Detected tags are merged with explicitly-set tags
- Tags feed into `lc:query` index pages

---

## Bulk Operations

### Export → Transform → Import Workflow

```
# Step 1: Export all Concept Pages with specific fields
export_content {
  "template_name": "Concept Page",
  "fields": ["definition", "layer_badge"]
}

# Step 2: Transform the data externally, then bulk update
bulk_update_content {
  "updates": [
    { "id": "abc123", "data": { "layer_badge": "<new html>" } },
    { "id": "def456", "data": { "layer_badge": "<new html>" } }
  ],
  "version_comment": "Updated layer badges"
}
```

### Clear a Field Across a Template

```
bulk_field_operation {
  "operation": "clear",
  "field": "old_badge",
  "template_name": "Concept Page",
  "version_comment": "Cleared deprecated field"
}
```

### Prepend a Disclaimer to All Blog Posts

```
bulk_field_operation {
  "operation": "prepend",
  "field": "body",
  "value": "<div class=\"disclaimer\">Views are my own.</div>",
  "template_name": "Blog Post",
  "version_comment": "Added disclaimer to all posts"
}
```

### Dry-Run Before Committing

```
bulk_field_operation {
  "operation": "set",
  "field": "layer_badge",
  "value": "<span class=\"badge\">Updated</span>",
  "template_name": "Concept Page",
  "dry_run": true
}
```

### Concurrency Guidance

- Up to 20 concurrent `update_content` calls are safe for small batches
- Use `bulk_update_content` (max 100 items per call) for larger batches
- Use `bulk_field_operation` when applying the same change across many pages
- `bulk_field_operation` limit: 500 pages per call; use `content_ids` scoping for larger datasets

---

## Common Workflows

### Create a Blog

1. Create a folder: `create_folder` with name "Blog", slug "blog"
2. Create posts: `create_content` with folder_path "/blog", category "blog"
3. Create collection: `create_collection` with category "blog" to list all posts

### Update Site Branding

1. Get current theme: `get_theme`
2. Update colors/fonts: `update_theme` with new values
3. Update header/footer HTML if needed (triggers regeneration)

### Manage Content Versions

1. List versions: `get_content_versions`
2. View old version: `get_content_version`
3. Restore if needed: `revert_to_version`

### Set Up Redirects

1. List existing: `list_redirects`
2. Create redirect: `create_redirect` with from_path, to_path
3. Use 301 for permanent moves, 302 for temporary

### Bulk Update Links or Text

1. Preview changes: `search_replace_preview` with search and replace terms
2. Review affected pages and match counts carefully
3. Execute changes: `search_replace_execute` (creates versions for rollback)
4. If needed, revert individual pages: `revert_to_version`

### Build a Dynamic Tagged Index Page

1. Create a snippet for the index card: `create_snippet`
2. Create an index page template with `<!-- lc:query filter="tag:TAGNAME" snippet="my-snippet" -->` in the layout
3. Tag content items at creation or via `update_content` with `tags: [...]`
4. Publish the index page — it auto-regenerates whenever tagged content changes
