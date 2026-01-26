# LightCMS MCP API Reference

This document provides a complete reference for the LightCMS Model Context Protocol (MCP) server. The MCP server enables AI agents like Claude Code to manage your website through natural language.

## Overview

The LightCMS MCP server exposes 41 tools organized into these categories:

| Category | Tools | Description |
|----------|-------|-------------|
| [Content](#content-tools) | 12 | Create, edit, publish, version, and delete content |
| [Templates](#template-tools) | 5 | Define content structures with custom fields |
| [Assets](#asset-tools) | 5 | Upload and manage images, documents, and files |
| [Theme](#theme-tools) | 2 | Customize colors, fonts, header/footer |
| [Site Config](#site-config-tools) | 2 | Configure site-wide settings |
| [Redirects](#redirect-tools) | 4 | Manage URL redirects |
| [Folders](#folder-tools) | 4 | Organize content into URL hierarchies |
| [Collections](#collection-tools) | 5 | Group and display content by category |
| [Search](#search-tools) | 3 | Search content and perform bulk find/replace |
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

**Returns:** Array of content summaries with id, title, slug, full_path, category, published, deleted, updated_at, published_at.

---

### get_content

Get a single content item by ID or path.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | No* | Content ID (MongoDB ObjectID) |
| `path` | string | No* | Content path (e.g., `/about` or `/blog/my-post`) |

*One of `id` or `path` is required.

**Returns:** Complete content object including all field data.

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

Update an existing content item. Creates a new version automatically.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | string | Yes | Content ID (MongoDB ObjectID) |
| `template_id` | string | No | Change template |
| `title` | string | No | Update title |
| `slug` | string | No | Update URL slug |
| `folder_path` | string | No | Move to different folder |
| `category` | string | No | Update category |
| `meta_description` | string | No | Update SEO description |
| `og_image` | string | No | Update Open Graph image |
| `data` | object | No | Update field values |
| `use_header` | boolean | No | Toggle site header |
| `use_footer` | boolean | No | Toggle site footer |
| `use_theme` | boolean | No | Toggle theme |
| `raw_mode` | boolean | No | Toggle raw HTML mode |
| `version_comment` | string | No | Optional comment describing this version change |

Only include fields you want to change.

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

### get_content_versions

Get version history for a content item.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `content_id` | string | Yes | Content ID (MongoDB ObjectID) |

**Returns:** Array of version summaries with version number, title, slug, full_path, published, comment (if any), created_at.

---

### get_content_version

Get a specific version of a content item.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `content_id` | string | Yes | Content ID (MongoDB ObjectID) |
| `version` | integer | Yes | Version number |

**Returns:** Complete content object at that version, including comment if any.

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
  "type": "text|textarea|richtext|date|image|select",
  "required": true,
  "placeholder": "Placeholder text",
  "default": "Default value",
  "options": "option1,option2,option3"
}
```

**Returns:** `{ success, id, message }`

**Example:**
```json
{
  "name": "Product Page",
  "slug": "product-page",
  "description": "Template for product listings",
  "category": "products",
  "fields": [
    {"name": "price", "label": "Price", "type": "text", "required": true},
    {"name": "description", "label": "Description", "type": "richtext", "required": true},
    {"name": "image", "label": "Product Image", "type": "image", "required": false}
  ],
  "html_layout": "<div class=\"product\"><h1>{{.title}}</h1><p class=\"price\">{{.price}}</p><div>{{.description}}</div><img src=\"{{.image}}\"></div>"
}
```

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

**Returns:** Asset metadata (not file content—use serve_path to access the file).

---

### upload_asset

Upload a new asset.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `filename` | string | Yes | Original filename with extension |
| `serve_path` | string | Yes | URL path (e.g., `/images/logo.png`) |
| `data_base64` | string | Yes | Base64-encoded file content |
| `description` | string | No | Asset description |

**Allowed file types:** Images (jpg, png, gif, webp, svg, ico), Documents (pdf, doc, docx, txt, md), Web (css, js, json, xml, html), Fonts (woff, woff2, ttf, otf, eot)

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

## Site Config Tools

### get_site_config

Get site configuration.

**Parameters:** None

**Returns:** Site config with title_template and title_template_no_title.

---

### update_site_config

Update site configuration.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `title_template` | string | No | Title template with `{{title}}` and `{{site_name}}` |
| `title_template_no_title` | string | No | Title when page has no title |

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

**Returns:** Object with query, search_type, total count, and matches array. Each match includes id, title, full_path, template_name, published status, and matched_in (array of field names where match was found).

**Example:**
```json
{
  "query": "artificial intelligence",
  "search_type": "fulltext"
}
```

---

### search_replace_preview

Preview search and replace results without making changes. **Always use this before `search_replace_execute`** to understand the impact.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `search` | string | Yes | Text to search for |
| `replace` | string | Yes | Text to replace with |

**Returns:** Object with:
- `search`, `replace`: The search/replace terms
- `total_matches`: Total number of text matches found
- `affected_pages`: Number of content pages that will be modified
- `published_pages`: Number of published pages affected
- `draft_pages`: Number of draft pages affected
- `matches`: Array of affected content with details including:
  - `id`, `title`, `full_path`, `published`
  - `match_count`: Number of matches in this page
  - `field_matches`: Object mapping field names to match counts
  - `sample_excerpt`: Preview showing the replacement in context
- `warning`: Reminder this is preview only
- `destructive_warning`: Warning about permanent changes

**Example:**
```json
{
  "search": "http://example.com",
  "replace": "https://example.com"
}
```

---

### search_replace_execute

Execute search and replace across all content. **⚠️ WARNING: This is a destructive operation that modifies content permanently.** Always run `search_replace_preview` first to review what will be changed.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `search` | string | Yes | Text to search for |
| `replace` | string | Yes | Text to replace with |
| `version_comment` | string | No | Comment for version history. Default: "Bulk search and replace: 'X' → 'Y'" |

**Returns:** Object with:
- `success`: Boolean indicating success
- `search`, `replace`: The search/replace terms
- `total_replacements`: Total number of replacements made
- `pages_updated`: Number of pages modified
- `published_pages`: Number of published pages affected
- `version_comment`: The version comment used
- `updated_pages`: Array of modified content with details

**Example:**
```json
{
  "search": "old-domain.com",
  "replace": "new-domain.com",
  "version_comment": "Updated domain links"
}
```

**Safety Notes:**
- Each modified content item gets a new version, so changes can be reverted
- Preview your changes first with `search_replace_preview`
- The operation searches in both titles and all content data fields
- Published pages are automatically regenerated after replacement

---

## Utility Tools

### regenerate_all_content

Regenerate all published static HTML pages. Use after major theme or template changes.

**Parameters:** None

**Returns:** Success message with count of regenerated pages.

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
