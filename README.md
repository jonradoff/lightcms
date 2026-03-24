# LightCMS

[![CI](https://github.com/jonradoff/lightcms/actions/workflows/ci.yml/badge.svg)](https://github.com/jonradoff/lightcms/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jonradoff/lightcms/branch/main/graph/badge.svg)](https://codecov.io/gh/jonradoff/lightcms)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonradoff/lightcms)](https://goreportcard.com/report/github.com/jonradoff/lightcms)

A lightweight, AI-native content management system for building and managing websites. Built with Go and MongoDB Atlas.

## Why LightCMS?

**Lightweight**: A clean, focused codebase (~5K lines of Go) that's easy to understand, modify, and extend. No bloated frameworks or complex abstractions.

**AI-Native**: Built from the ground up for the AI era:
- **MCP Integration**: Full Model Context Protocol server with 72 tools for website management. Supports both local stdio and HTTP streamable transports — connect from Claude Code, Claude Desktop, or any MCP-compatible client.
- **OAuth 2.1 for Remote Agents**: Sandboxed desktop apps like Claude's Cowork can securely connect over HTTP using OAuth 2.1 with PKCE. No embedded passwords — just authorize once and the agent manages your site.
- **Fork-Friendly**: Designed to be forked and customized by Claude Code. Ask Claude to add new content types, modify templates, or build custom features — the codebase is structured for AI-assisted development.
- **Natural Language Website Management**: Skip the admin UI entirely. Create pages, manage assets, customize themes, and publish content through conversation.

## Features

### Content Management
- **Template System**: Define reusable content structures with custom fields (text, richtext, image, date, select, markdown)
- **Static Page Generation**: Fast page loads from pre-rendered HTML — no runtime templating overhead
- **Content Versioning**: Full version history with diff comparison and one-click revert
- **Soft Delete**: Recover deleted content with undelete functionality
- **Content Tagging**: Tag any content item with one or more freeform labels, then query by tag across your site
- **Snippets**: Named HTML template fragments used as reusable rendering units in dynamic queries
- **`lc:query` Directives**: Embed live content queries directly in template layouts — at publish time they expand into rendered lists of matching pages
- **Content Collections**: Auto-generated paginated listing pages filtered by category
- **Folders & URL Organization**: Hierarchical content organization with clean URL paths
- **Rich Text Editor**: TinyMCE integration for visual content editing
- **Regex Search & Replace**: Site-wide or scoped search-and-replace with RE2 regex support, capture groups, and mandatory preview step
- **Bulk Operations**: Update or apply field operations across up to 100 pages in a single API call; export/transform/re-import pipelines

### Content Forks (v4.0+)
- **Fork Workspaces**: Create named staging workspaces where sets of page edits can be authored, previewed, and reviewed before going live
- **Sparse Model**: Only edited pages live in a fork — unmodified pages fall through to live content automatically
- **Fork Preview Mode**: Activate via a floating bar injected into the live site; a cookie routes all page requests through the fork so you see exactly how the site will look after merge
- **Merge with Conflict Detection**: Admins merge forks into live content; if a live page was changed after the fork was created, the conflict is recorded (fork wins). New pages created in the fork are inserted into live on merge
- **Full MCP Toolset**: 8 dedicated fork tools — `list_forks`, `create_fork`, `get_fork`, `fork_page`, `remove_fork_page`, `merge_fork`, `archive_fork`, `delete_fork`

### AI Chat Widget (v4.2+)
- **Embeddable Widget**: Add a floating AI chat bubble to any page with a single `<script>` tag — self-contained, no build step required
- **Two-Phase Pipeline**: Each visitor query runs hybrid semantic+fulltext search to retrieve relevant content excerpts, then streams those excerpts through Claude Haiku for a conversational synthesized answer
- **SSE Streaming**: Haiku's token-by-token output is forwarded live to the browser via Server-Sent Events; falls back to plain JSON for non-SSE clients
- **AI-Optional**: Without an Anthropic API key, the widget works as a search-in-chat experience returning ranked excerpts — no API cost
- **Fully Configurable**: Admin "Chat Widget" page controls title, welcome message, placeholder text, primary color, position (bottom-left/right), max results, and editable system/user prompt templates
- **Dedicated Rate Limiting**: Separate per-IP (5/min) and global (30/min) limiters independent from the search and API limiters
- **Source Attribution**: Every response includes the content pages whose excerpts were used, with titles and paths

### Multi-User Access Control (v2.0+)
- **Role-Based Access Control (RBAC)**: Three roles — admin, editor, viewer — with granular permission enforcement on all admin UI pages and REST API endpoints
- **User Management**: Admin panel for creating users, assigning roles, disabling accounts, and resetting passwords
- **Audit Log**: Persistent, searchable log of all mutations (who did what, when) with 365-day retention
- **User-Scoped API Keys**: API keys inherit the permissions of their owning user
- **Force Password Change**: Temporary passwords trigger a mandatory change on first login

### Smart End-User Search
- **Hybrid Search**: Combines full-text exact matching with semantic vector search (Voyage AI embeddings), merged via reciprocal rank fusion
- **Configurable Ranking**: All ranking weights editable in the admin panel — nav boost, title boost, boosted templates, demoted path prefixes, and penalty scores
- **Intelligent Defaults**: Nav-linked pages surface first, concept-template pages rank above generic content, video transcripts are deprioritised
- **Title Boost**: Pages where the query appears in the title always rank above body-only matches
- **Typeahead Suggestions**: Fast prefix-matching suggestions — pages for direct navigation, keywords for full search — with the same structural ranking
- **Works Without Embeddings**: Falls back to full-text search if no Voyage API key is configured
- **Rate Limiting**: Per-IP and global rate limiting for DDoS protection

### Developer & Integration
- **REST API**: Full `/api/v1/` JSON API with API key and OAuth token authentication, RBAC-enforced
- **MCP Server**: 72 tools for agentic website management (stdio + HTTP streamable)
- **OAuth 2.1**: Authorization code flow with PKCE for remote MCP clients — no embedded passwords
- **CLI Tool**: Command-line interface for all content management operations
- **URL Redirects**: 301/302 redirect rules managed from the admin panel

### Site Customization
- **Theme Customization**: Colors, fonts, border radius, custom CSS — all editable in the admin panel with version history
- **Header/Footer HTML**: Full HTML control over site chrome injected around all pages
- **Asset Management**: Upload and manage images, documents, and other files with path-based serving

## Prerequisites

- Go 1.24 or later
- MongoDB Atlas account (free tier works great)

## Quick Start

1. Clone the repository
2. Copy `config.dev.json.example` to `config.dev.json`
3. Edit `config.dev.json` with your MongoDB connection string
4. Run `go run cmd/server/main.go`
5. Visit http://localhost:8082/cm and log in with your email and password
6. On first run, an admin account is created — set `LIGHTCMS_ADMIN_EMAIL` to use your email, or it defaults to `admin@localhost`

## MongoDB Atlas Setup

### Step 1: Create an Atlas Account

1. Go to [MongoDB Atlas](https://www.mongodb.com/cloud/atlas/register)
2. Sign up for a free account (no credit card required)

### Step 2: Create a Cluster

1. Click **"Build a Database"**
2. Select **"M0 FREE"** (Shared) tier
3. Choose your preferred cloud provider and region (closest to you)
4. Click **"Create Deployment"**

### Step 3: Set Up Database Access

1. Create a database user:
   - Username: `lightcms` (or your choice)
   - Password: Generate a secure password (save this!)
   - Click **"Create User"**

2. Add your IP address:
   - Click **"Add My Current IP Address"**
   - Or add `0.0.0.0/0` to allow access from anywhere (less secure, but convenient for development)
   - Click **"Finish and Close"**

### Step 4: Get Your Connection String

1. Click **"Connect"** on your cluster
2. Select **"Drivers"**
3. Copy the connection string, it looks like:
   ```
   mongodb+srv://lightcms:<password>@cluster0.xxxxx.mongodb.net/?retryWrites=true&w=majority
   ```
4. Replace `<password>` with your actual password

### Step 5: Create Your Config File

For development, copy the example and fill in your values:

```bash
cp config.dev.json.example config.dev.json
```

Edit `config.dev.json`:
```json
{
  "port": "8082",
  "mongo_uri": "mongodb+srv://lightcms:YOUR_PASSWORD@cluster0.xxxxx.mongodb.net/lightcms",
  "env": "development",
  "session_secret": "any-random-string-for-dev"
}
```

For production, use `config.prod.json`:
```bash
cp config.prod.json.example config.prod.json
```

Edit with production values (use `openssl rand -hex 32` for session_secret).

## Installation

```bash
# Clone or navigate to the project
cd lightcms

# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go
```

Or use the run script:
```bash
./run.sh
```

## Configuration

LightCMS uses JSON config files. Create either:
- `config.dev.json` - for development
- `config.prod.json` - for production (takes precedence if both exist)

| Field | Description |
|-------|-------------|
| `port` | Server port (e.g., "8082" for dev, "80" for prod) |
| `mongo_uri` | MongoDB Atlas connection string |
| `env` | Environment: "development" or "production" |
| `session_secret` | Random string for session encryption |

**Note:** Config files contain secrets and are excluded from git via `.gitignore`.

## Usage

### Accessing the Site

- **Public site**: http://localhost:8082
- **Admin panel**: http://localhost:8082/cm

### First Login

Log in at `/cm/login` with your email and password. On first startup with an empty database, LightCMS creates an admin account from the `LIGHTCMS_ADMIN_EMAIL` environment variable (defaults to `admin@localhost` with password `admin123`). Change your password immediately after logging in.

To reset a password from the command line:
```bash
go run cmd/resetpw/main.go user@example.com
```

### Creating Content

1. Log in to the admin panel at `/cm`
2. Go to **Content** → **New Content**
3. Select a template (Blog Post, Press Release, Explanatory Page, etc.)
4. Fill in the fields
5. Check "Published" and save

### Managing Users (Admin Only)

1. Go to **Users** in the left sidebar (visible to admins only)
2. Create users with email, display name, and role (admin / editor / viewer)
3. Users receive a temporary password and are prompted to change it on first login
4. Disable accounts or reset passwords from the edit page
5. View a full audit trail of all user actions at **Audit Log**

### Creating Custom Templates

1. Go to **Templates** → **New Template**
2. Define your fields (text, textarea, richtext, date, image, select)
3. Create an HTML layout using `{{.field_name}}` placeholders
4. Save the template

**Available placeholders:**
- `{{.title}}` - Content title
- `{{.slug}}` - URL slug
- `{{.published_at}}` - Publication date
- `{{.your_field_name}}` - Any custom field you define

### Dynamic Index Pages: Tags, Snippets, and `lc:query`

LightCMS includes a system for building dynamic index pages that automatically update as you publish content. Three features work together: **tags** label individual pages, **snippets** define how each result is rendered, and **`lc:query` directives** embed live queries directly inside template layouts.

---

#### Tags

Tags are freeform string labels you attach to any content item. A page can have zero or many tags. They're the primary way to group content for querying.

**Setting tags in the admin UI:**
1. Open any content item in the editor
2. Find the **Tags** field (below the main fields)
3. Type a tag name and press Enter — repeat for multiple tags
4. Save the content item

**Setting tags via the API:**
```bash
curl -X PUT http://localhost:8082/api/v1/content/{id} \
  -H "Authorization: Bearer lc_your_key" \
  -H "Content-Type: application/json" \
  -d '{"tags": ["AI & Machine Intelligence", "Featured"]}'
```

Tags are exact-match strings. Capitalization and spaces are preserved — `"AI & Machine Intelligence"` and `"ai & machine intelligence"` are treated as different tags.

---

#### Snippets

A snippet is a named HTML template fragment stored in the CMS. When `lc:query` runs, it renders each matching content item through a snippet and concatenates the results.

**Creating a snippet:**
1. Go to **Settings → Snippets** in the admin panel
2. Click **New Snippet**, give it a name (e.g. `glossary-pill`)
3. Write HTML using Go template variables:

```html
<a href="{{.FullPath}}">{{.Title}}</a>
```

**Available variables inside a snippet:**

| Variable | Description |
|----------|-------------|
| `{{.Title}}` | The content item's title |
| `{{.FullPath}}` | The public URL path (e.g. `/my-page`) |
| `{{.Slug}}` | URL slug only (e.g. `my-page`) |
| `{{.MetaDescription}}` | Meta description field |
| `{{.PublishedAt}}` | Publication timestamp |

**Example snippets:**

A pill-style link (for glossary / tag cloud layouts):
```html
<a href="{{.FullPath}}" class="pill">{{.Title}}</a>
```

A card with description:
```html
<div class="card">
  <h3><a href="{{.FullPath}}">{{.Title}}</a></h3>
  <p>{{.MetaDescription}}</p>
</div>
```

A simple list item:
```html
<li><a href="{{.FullPath}}">{{.Title}}</a></li>
```

---

#### `lc:query` Directives

An `lc:query` directive is an HTML comment you embed in a **template layout**. At page generation time — before the page is rendered — the CMS finds all matching content items, renders each one through the named snippet, and replaces the comment with the combined HTML.

**Syntax:**
```html
<!-- lc:query filter="tag:TAGNAME" sort="title:asc" snippet="snippet-name" -->
```

**Attributes:**

| Attribute | Required | Description |
|-----------|----------|-------------|
| `filter` | Yes | Filter expression. Currently supports `tag:TAGNAME` to match content tagged with `TAGNAME`. |
| `sort` | No | Sort field and direction: `title:asc`, `title:desc`, `created_at:asc`, `created_at:desc`. Defaults to `title:asc`. |
| `snippet` | Yes | Name of the snippet to render each result through. |

**Example in a template layout:**
```html
<h2>AI & Machine Intelligence</h2>
<div class="links">
<!-- lc:query filter="tag:AI & Machine Intelligence" sort="title:asc" snippet="glossary-pill" -->
</div>
```

After the page is published, the directive is replaced with the rendered output of every published page tagged `AI & Machine Intelligence`, each passed through the `glossary-pill` snippet:

```html
<h2>AI & Machine Intelligence</h2>
<div class="links">
<a href="/artificial-intelligence" class="pill">Artificial Intelligence</a>
<a href="/machine-learning" class="pill">Machine Learning</a>
<a href="/neural-networks" class="pill">Neural Networks</a>
</div>
```

**Important:** `lc:query` directives must be placed in the template's **HTML layout** field, not inside content data fields. The CMS processes them during page generation before Go's template engine runs (which would otherwise strip HTML comments).

---

#### Automatic Regeneration

Index pages that use `lc:query` are automatically regenerated whenever:

- A tagged content item is **published** or **updated**
- The **template layout** is changed
- The **snippet** is updated
- **Regenerate All** is triggered manually from the admin panel

This means you never need to manually rebuild your index pages — publish a new concept page tagged `"Games & Interactive Experiences"` and it appears in every index that queries for that tag within seconds.

---

#### Complete Walkthrough: Building a Tagging-Powered Index

Here's how to build a concepts glossary that automatically stays up to date.

**Step 1: Tag your concept pages**

For each concept page, add the appropriate tag in the content editor. You can use as many tags as you like, and the same content item can appear in multiple index sections.

**Step 2: Create a snippet**

In **Settings → Snippets**, create a snippet named `glossary-pill`:
```html
<a href="{{.FullPath}}">{{.Title}}</a>
```

**Step 3: Create a template with `lc:query` sections**

Create a new template (e.g. "Concepts Index") with this HTML layout:

```html
<article class="index-page">
  <h1>{{.title}}</h1>
  <div class="page-content">

    <h2>AI &amp; Machine Intelligence</h2>
    <div class="concept-links">
<!-- lc:query filter="tag:AI & Machine Intelligence" sort="title:asc" snippet="glossary-pill" -->
    </div>

    <h2>Games &amp; Interactive Experiences</h2>
    <div class="concept-links">
<!-- lc:query filter="tag:Games & Interactive Experiences" sort="title:asc" snippet="glossary-pill" -->
    </div>

    <h2>3D Graphics &amp; Rendering</h2>
    <div class="concept-links">
<!-- lc:query filter="tag:3D Graphics & Rendering" sort="title:asc" snippet="glossary-pill" -->
    </div>

  </div>
</article>
```

Note that `{{.title}}` is the Go template variable for the content item's title. Template variables use Go's `{{.field}}` syntax and are resolved after `lc:query` directives are expanded.

**Step 4: Create and publish an index page**

Create a new content item using your "Concepts Index" template. Give it a title and slug (e.g. `/glossary`). Publish it — the static page is generated with all the current tagged content already in place.

**Step 5: Keep publishing**

From now on, every time you create and publish a new concept page with a matching tag, all index pages that query for that tag are automatically regenerated and updated.

---

#### Template Variables

Beyond content data fields, templates have access to a few built-in variables:

| Variable | Description |
|----------|-------------|
| `{{.title}}` | The content item's title |
| `{{.slug}}` | URL slug |
| `{{.published_at}}` | Publication timestamp |
| `{{.your_field}}` | Any custom field defined in the template (richtext fields render as HTML) |

Custom fields defined on your template are available directly by key. If you define a field with key `intro`, it's available as `{{.intro}}` in the layout. Richtext fields are automatically marked safe — their HTML is rendered as-is without escaping.

---

---

### Content Authoring: Pre-Processing & Markup Features

LightCMS processes content field values before rendering them into your template. Authors can use the following markup features in any text or richtext data field.

---

#### Wikilinks

Link between pages using double-bracket syntax. Wikilinks are resolved at publish time and automatically kept up to date when a page's title or path changes.

| Syntax | Result |
|--------|--------|
| `[[Page Title]]` | Link to a page matched by title (case-insensitive) |
| `[[Page Title\|display text]]` | Same, with custom link text |
| `[[/full/path]]` | Link to a page by its exact URL path |
| `[[/full/path\|display text]]` | Path link with custom link text |

Broken links (no matching page found) render as `<span class="broken-link">Page Title</span>` so they are easy to identify and fix.

---

#### Snippet Includes

Embed a named snippet inline inside any content field:

```
[[include:snippet-name]]
```

The `snippet-name` must exactly match the **Name** field of a snippet in **Settings → Snippets**. Snippet includes are useful for reusable content blocks such as callouts, disclaimers, and calls to action that appear on many pages.

---

#### Table of Contents

Place `{{.lc_toc}}` anywhere in your **template's HTML layout** to inject an auto-generated table of contents at that position:

```html
<nav class="sidebar">
  {{.lc_toc}}
</nav>
<article>
  {{.body}}
</article>
```

At page generation time, LightCMS scans all headings in the final rendered HTML and outputs a `<nav class="lc-toc">` block with anchor links to each one. Headings automatically receive `id=` attributes derived from their text content (see **Heading IDs** below), so the TOC links work without any extra setup.

---

#### Heading IDs

All headings (`<h1>` through `<h6>`) in rendered page output automatically receive `id=` attributes derived from their text. This enables deep-linking to specific sections.

Example:
```html
<!-- In your content field -->
<h2>Getting Started</h2>

<!-- Rendered output -->
<h2 id="getting-started">Getting Started</h2>
```

The `id` is generated by lowercasing the text and replacing spaces and punctuation with hyphens. If two headings produce the same id, a numeric suffix is appended (`getting-started-2`, etc.).

---

#### Markdown Field Type

Template fields can be given the type `markdown` instead of `text` or `richtext`. Markdown fields support **GitHub Flavored Markdown (GFM)** including tables, strikethrough, task lists, and autolinks. The field value is converted to HTML at page generation time.

Markdown fields are a good choice for structured content that benefits from simple markup without a WYSIWYG editor — documentation pages, changelogs, FAQs, and similar content.

To create a Markdown field, set the field type to `markdown` when defining the template:

```json
{ "name": "body", "label": "Body", "type": "markdown", "required": true }
```

---

#### Inline Tag Detection

Mention `#tagname` anywhere in a content field to automatically tag the page with that label. The tag is added to the page's tag list and participates in `lc:query` index pages just like manually applied tags.

Tag rules:
- Must start with a letter
- May contain letters, numbers, underscores, or hyphens
- Example: `This article covers #machine-learning and #ai` adds both tags

This is a convenient alternative to editing the Tags field separately — useful when writing content in Markdown fields or richtext where you want to tag inline.

---

### Creating Collections

Collections display grouped content (like a blog listing page).

1. Go to **Collections** → **New Collection**
2. Set the category filter to match your content's category
3. Define item and page templates
4. The collection will be available at `/collection-slug`

### Customizing the Theme

1. Go to **Theme** in the admin panel
2. Adjust colors, fonts, and border radius
3. Add custom CSS if needed
4. Save to apply changes site-wide

### End-User Search

LightCMS exposes a public search API at `/api/search` that your site's frontend can call.

#### Search API

```
GET /api/search?q=QUERY&mode=hybrid&limit=10
```

| Parameter | Description |
|-----------|-------------|
| `q` | Search query (required) |
| `mode` | `hybrid` (default), `fulltext`, or `semantic` |
| `limit` | Max results 1–50 (default 10) |

```json
{
  "query": "game design",
  "mode": "hybrid",
  "total": 3,
  "results": [
    { "id": "...", "title": "Game Design", "full_path": "/concepts/game-design",
      "snippet": "...matching context...", "score": 0.97, "match_type": "both" }
  ]
}
```

`match_type` is `exact`, `semantic`, or `both`.

#### Typeahead Suggest API

```
GET /api/search/suggest?q=PREFIX&limit=8
```

Returns two lists for building a live typeahead dropdown:

```json
{
  "keywords": ["game design", "game mechanics"],
  "pages":    [{"title": "About Jon Radoff", "path": "/about"}]
}
```

- **keywords** — extracted from published content; clicking one triggers a full search
- **pages** — direct-navigation results, ranked by: nav-linked → boosted-template → title-starts-with → title-contains → demoted paths

#### JavaScript Example

```html
<input type="text" id="q" placeholder="Search..." autocomplete="off">
<ul id="suggest"></ul>
<div id="results"></div>

<script>
const input = document.getElementById('q');
const suggest = document.getElementById('suggest');
const results = document.getElementById('results');
let timer;

// Typeahead while typing
input.addEventListener('input', () => {
  clearTimeout(timer);
  const q = input.value.trim();
  if (q.length < 2) { suggest.innerHTML = ''; return; }
  timer = setTimeout(async () => {
    const r = await fetch('/api/search/suggest?q=' + encodeURIComponent(q) + '&limit=8');
    const d = await r.json();
    suggest.innerHTML = [
      ...(d.pages    || []).map(p => `<li><a href="${p.path}">📄 ${p.title}</a></li>`),
      ...(d.keywords || []).map(k => `<li><a onclick="doSearch('${k}')">🔍 ${k}</a></li>`),
    ].join('');
  }, 200);
});

// Full search on Enter
input.addEventListener('keydown', e => { if (e.key === 'Enter') doSearch(input.value); });

async function doSearch(q) {
  suggest.innerHTML = '';
  const r = await fetch('/api/search?q=' + encodeURIComponent(q) + '&mode=hybrid&limit=10');
  const d = await r.json();
  results.innerHTML = (d.results || [])
    .map(r => `<div><a href="${r.full_path}"><strong>${r.title}</strong></a><p>${r.snippet}</p></div>`)
    .join('') || '<p>No results.</p>';
}
</script>
```

#### Ranking Configuration

Ranking weights are configurable in the admin panel under **Tools → End User Search → Search Ranking**. Defaults: title-match boost 0.20, nav-page boost 0.15, concept-template boost 0.05, video-path penalty −0.05. Configure your Voyage AI key under **Configuration** to enable semantic search.

## Project Structure

```
lightcms/
├── cmd/
│   ├── server/main.go        # HTTP server entry point
│   ├── mcp/main.go           # MCP server entry point
│   ├── cli/main.go           # CLI tool entry point
│   └── resetpw/main.go       # Password reset utility
├── config/
│   └── config.go             # Configuration loading
├── internal/
│   ├── apiclient/            # Reusable HTTP client for REST API
│   ├── auth/                 # Authentication, RBAC permissions, session management
│   ├── cli/                  # CLI subcommands and output formatting
│   ├── database/             # MongoDB connection & operations
│   ├── handlers/             # HTTP handlers (admin UI + REST API)
│   ├── mcp/                  # MCP server and tool definitions
│   ├── middleware/            # API auth middleware (API keys + OAuth)
│   ├── models/               # Data models & default templates
│   ├── oauth/                # OAuth 2.1 authorization server
│   └── services/             # Business logic (content, search, users, audit, etc.)
├── static/                   # CSS, JS, and uploaded files
├── content/                  # Custom pages and generated HTML
└── .goreleaser.yaml          # Release configuration
```

## Default Templates

### Blog Post
Fields: title, excerpt, featured_image, content, author, tags

### Press Release
Fields: headline, subheadline, dateline, release_date, body, boilerplate, contact_info

### Explanatory Page
Fields: title, subtitle, hero_image, intro, main_content, sidebar, cta_text, cta_link

### Concept Page
Fields: title, definition, topic_links — ideal for wiki-style knowledge base entries

### Standard Page, Blank Page, Homepage
General-purpose layouts for flexible content.

## Multi-User Access Control

LightCMS v2.0+ supports multiple users with role-based permissions.

### Roles

| Role | Capabilities |
|------|-------------|
| **admin** | Full access: manage users, templates, theme, settings, audit log, all API keys |
| **editor** | Create/edit/delete/publish content; upload and delete assets; manage own API keys |
| **viewer** | Read-only access to content, templates, assets, and settings |

### Audit Log

Every mutation (content create/update/delete/publish, user management, settings changes, logins) is logged with the acting user's email, timestamp, and relevant details. Logs are retained for 365 days and accessible at `/cm/audit`.

### API Key Permissions

API keys created by a user inherit that user's role. A key created by an editor cannot perform admin-only operations even if its token is shared. Admins can manage all keys; non-admins can only manage their own.

### First-Time Migration

On first startup with an empty `users` collection, LightCMS automatically creates an admin user from the existing password hash in the database. Set the `LIGHTCMS_ADMIN_EMAIL` environment variable to specify which email address to use (defaults to `admin@localhost`).

## API Keys

API keys are required for the REST API, MCP server, and CLI tool. Create them from the admin panel.

1. Log in at `/cm`
2. Go to **Settings** → **API Keys**
3. Click **Create New Key**, give it a name and description
4. Copy the key immediately — it's only shown once

Keys use the format `lc_` followed by 32 hex characters. They're stored as SHA-256 hashes and inherit the permissions of the creating user.

## OAuth 2.1 Authorization

LightCMS implements OAuth 2.1 so that remote MCP clients (like Claude's Cowork) can securely connect without embedding passwords or API keys. This follows the standard authorization code flow with PKCE.

### Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /oauth/register` | Dynamic client registration (RFC 7591) |
| `GET /oauth/authorize` | Authorization page (admin login + consent) |
| `POST /oauth/token` | Token exchange and refresh |
| `POST /oauth/revoke` | Token revocation (RFC 7009) |
| `GET /oauth/jwks` | JWKS endpoint (opaque tokens, returns empty) |

### Security

- **PKCE (S256)** required for all authorization requests
- **Token rotation**: refresh tokens are single-use; a new pair is issued each time
- **Short-lived access tokens**: 1-hour TTL
- **Refresh tokens**: 30-day TTL, revocable
- **Rate limiting**: failed login attempts trigger progressive lockouts (1 min → 5 min → 15 min)
- **All tokens stored as SHA-256 hashes** in the database

### How Clients Connect

1. Client fetches `/.well-known/oauth-authorization-server` to discover endpoints
2. Client calls `POST /oauth/register` with its name and redirect URI
3. Client redirects admin to `/oauth/authorize` with PKCE challenge
4. Admin enters password and approves access
5. Client exchanges the authorization code for access + refresh tokens
6. Client uses the access token as a Bearer token on `/mcp` or `/api/v1/` endpoints

This is all handled automatically by MCP-compatible clients — you just provide your LightCMS URL and approve the connection.

## REST API

LightCMS provides a full REST API at `/api/v1/` authenticated with API keys or OAuth tokens. All endpoints enforce RBAC — the permissions of the authenticated user (or key owner) determine what's allowed.

### Authentication

Include an API key or OAuth access token in the `Authorization` header:

```bash
# With API key
curl -H "Authorization: Bearer lc_your_key_here" http://localhost:8082/api/v1/content

# With OAuth token
curl -H "Authorization: Bearer <oauth_access_token>" http://localhost:8082/api/v1/content
```

### Endpoints

| Resource | Endpoints |
|----------|-----------|
| Content | `GET/POST /content`, `GET/PUT/DELETE /content/{id}`, `POST .../publish`, `.../unpublish`, `.../restore`, `GET .../versions`, `POST .../versions/{v}/revert`, `GET /content/by-path?path=...` |
| Templates | `GET/POST /templates`, `GET/PUT/DELETE /templates/{id}` |
| Snippets | `GET/POST /snippets`, `GET/PUT/DELETE /snippets/{id}` |
| Assets | `GET/POST /assets`, `GET/DELETE /assets/{id}`, `GET /assets/folders`, `GET /assets/by-path?path=...` |
| Theme | `GET/PUT /theme`, `GET /theme/versions`, `POST /theme/versions/{v}/revert` |
| Config | `GET/PUT /config` |
| Redirects | `GET/POST /redirects`, `GET/PUT/DELETE /redirects/{id}` |
| Folders | `GET/POST /folders`, `GET/DELETE /folders/{id}` |
| Collections | `GET/POST /collections`, `GET/PUT/DELETE /collections/{id}` |
| Search | `GET /search?q=...`, `POST /search-replace/preview`, `POST /search-replace/execute` |
| API Keys | `GET/POST /api-keys`, `DELETE /api-keys/{id}` |
| Utility | `POST /regenerate` |

All endpoints return JSON. PUT endpoints support partial updates (only include fields you want to change).

## CLI Tool

The `lightcms` CLI provides command-line access to all content management operations.

### Installation

```bash
# Build from source
go build -o bin/lightcms ./cmd/cli

# Or download a release binary from GitHub
```

### Configuration

```bash
export LIGHTCMS_URL=http://localhost:8082
export LIGHTCMS_API_KEY=lc_your_key_here
```

Or use flags: `--url` and `--api-key`.

### Commands

```bash
lightcms content list                    # List all content
lightcms content get <id>                # Get content by ID
lightcms content create --template <id> --title "My Post" --slug my-post --data '{"body":"Hello"}'
lightcms content publish <id>            # Publish content
lightcms content versions <id>           # Show version history

lightcms template list                   # List templates
lightcms asset upload --file logo.png --path /images/logo.png
lightcms theme update --primary-color "#1a1a2e"
lightcms search "search terms"           # Search content
lightcms api-key create --name "CI/CD"   # Create API key

lightcms --json content list             # JSON output
```

Run `lightcms --help` for full usage.

## MCP Server (AI-Powered Content Management)

[![lightcms MCP server](https://glama.ai/mcp/servers/jonradoff/lightcms/badges/card.svg)](https://glama.ai/mcp/servers/jonradoff/lightcms)

LightCMS includes a full MCP (Model Context Protocol) server with 72 tools for managing your entire website through AI agents. It supports two transport modes:

- **Stdio** — for local tools like Claude Code
- **HTTP Streamable** — for remote/sandboxed clients like Claude's Cowork, Claude Desktop, or any MCP-compatible app

### Option A: Local Setup (Claude Code via Stdio)

Best for developers using Claude Code directly on the same machine.

1. Create an API key in the admin panel at `/cm` → Settings → API Keys
2. Run the setup script:

```bash
export LIGHTCMS_API_KEY=lc_your_key_here
./setup-mcp.sh
```

Or register manually:
```bash
go build -o bin/lightcms-mcp ./cmd/mcp

claude mcp add --transport stdio lightcms-mcp \
  -e LIGHTCMS_URL="http://localhost:8082" \
  -e LIGHTCMS_API_KEY="lc_your_key_here" \
  -- /path/to/lightcms/bin/lightcms-mcp
```

Restart Claude Code and run `/mcp` to verify.

### Option B: Remote Setup (Cowork / Claude Desktop via HTTP + OAuth)

Best for sandboxed desktop apps that can't run local binaries. The HTTP MCP endpoint at `/mcp` supports OAuth 2.1 authorization — no API keys or passwords need to be embedded in the client.

**How it works:**

1. The client discovers your LightCMS instance via well-known endpoints
2. It registers as an OAuth client (one-time, automatic)
3. You authorize the client by entering your admin password in the browser
4. The client receives short-lived access tokens and refreshes them automatically

**To connect from a remote MCP client**, just provide your LightCMS URL (e.g., `https://yoursite.example.com`). The client handles the rest using standard OAuth 2.1 discovery.

**Discovery endpoints:**

| Endpoint | Purpose |
|----------|---------|
| `/.well-known/oauth-authorization-server` | OAuth server metadata (RFC 8414) |
| `/.well-known/oauth-protected-resource` | Protected resource metadata (RFC 9728) |
| `/.well-known/mcp/server-card.json` | MCP server card with tool schemas |

### Authentication

The MCP HTTP endpoint accepts both authentication methods:

- **API keys** (`lc_` prefix) — long-lived, created in admin panel
- **OAuth 2.1 tokens** — short-lived, obtained through the authorization flow

Both methods enforce RBAC based on the authenticated user's role.

### Available Tools (72 total)

- **Content** (20): create, read, update, delete, publish, unpublish, restore, versioning, revert, preview, bulk update, bulk field operation, export, backlinks, update by path, publish multiple
- **Templates** (5): create, read, update, delete, list
- **Snippets** (5): create, read, update, delete, list
- **Assets** (6): upload, upload from URL, read, delete, list files and folders
- **Search** (7): full-text search, end-user search, search-and-replace (global + scoped, preview + execute), reindex embeddings
- **Settings** (23): theme CRUD + versioning + pinning, site config, redirects, folders, collections, regenerate all content
- **Forks** (8): list, create, get, fork page, remove page, merge, archive, delete

For detailed API documentation, see [MCP.md](MCP.md).

### Environment Variables (Stdio Mode)

- `LIGHTCMS_URL` — Server URL (default: `http://localhost:8082`)
- `LIGHTCMS_API_KEY` — API key (required for stdio mode)

## MCP Examples

These examples show how the MCP tools work together to manage a website through natural language. Each example lists the user prompt and the exact MCP tool calls that execute behind the scenes.

### Example 1: Create and Publish a Blog Post

**Prompt:** "Create a blog post about AI agents and publish it"

**Tool calls:**
1. `list_templates` — finds the Blog Post template and its ID
2. `create_content` — creates the post with template ID, title, slug, and field data:
   ```json
   {
     "template_id": "6971098ad0761968133b8e43",
     "title": "The Rise of AI Agents",
     "slug": "rise-of-ai-agents",
     "data": {
       "excerpt": "How autonomous AI agents are reshaping software development.",
       "content": "<p>AI agents represent a fundamental shift...</p>",
       "author": "Editorial Team"
     }
   }
   ```
3. `publish_content` — makes it live; a static HTML page is generated at `/rise-of-ai-agents`

### Example 2: Update the Site Theme

**Prompt:** "Change the site colors to a dark theme with blue accents"

**Tool calls:**
1. `get_theme` — reads current theme settings (colors, fonts, header/footer HTML)
2. `update_theme` — applies the new palette:
   ```json
   {
     "primary_color": "#1a1a2e",
     "secondary_color": "#16213e",
     "accent_color": "#0f3460",
     "background_color": "#0a0a0a",
     "text_color": "#e0e0e0"
   }
   ```
   All published pages are automatically regenerated with the new theme.

### Example 3: Search and Replace Across the Entire Site

**Prompt:** "Replace 'Acme Corp' with 'Acme Industries' everywhere on the site"

**Tool calls:**
1. `search_replace_preview` — shows affected pages without making changes:
   ```json
   { "search": "Acme Corp", "replace": "Acme Industries" }
   ```
   Returns a list of content items, matched fields, and match counts.
2. `search_replace_execute` — applies the replacement after user confirmation. Each affected content item gets a new version for rollback capability.

### Example 4: Create a Custom Template

**Prompt:** "Create a template for team member profiles with name, role, bio, and photo"

**Tool calls:**
1. `create_template` — defines the structure and HTML layout:
   ```json
   {
     "name": "Team Member",
     "slug": "team-member",
     "fields": [
       { "name": "role", "label": "Role", "type": "text", "required": true },
       { "name": "photo", "label": "Photo", "type": "image", "required": false },
       { "name": "bio", "label": "Biography", "type": "richtext", "required": true }
     ],
     "html_layout": "<div class=\"team-member\"><img src=\"{{.photo}}\" alt=\"{{.title}}\"><h2>{{.title}}</h2><h3>{{.role}}</h3><div>{{.bio}}</div></div>"
   }
   ```
   The template is immediately available for creating content.

### Example 5: Upload an Image and Use It in Content

**Prompt:** "Upload this logo and add it to the about page"

**Tool calls:**
1. `upload_asset` — uploads the file as base64 with a serve path:
   ```json
   {
     "filename": "logo.png",
     "serve_path": "/images/logo.png",
     "data_base64": "iVBORw0KGgoAAAANSUhEUg..."
   }
   ```
   The image is now accessible at `/images/logo.png`.
2. `get_content` — retrieves the about page by path to get its current data
3. `update_content` — adds the image reference to the page's content field

### Example 6: Set Up URL Redirects

**Prompt:** "The old blog moved from /news to /blog — set up redirects for the old URLs"

**Tool calls:**
1. `create_redirect` — creates a permanent redirect:
   ```json
   {
     "from_path": "/news",
     "to_path": "/blog",
     "status_code": 301,
     "description": "Blog section moved from /news to /blog"
   }
   ```
   Visitors to `/news` are automatically redirected to `/blog`. Multiple redirects can be created in sequence for different paths.

### Example 7: Content Versioning and Rollback

**Prompt:** "Show me the version history for the homepage, then revert to version 2"

**Tool calls:**
1. `get_content` — retrieves the homepage by path (`/`) to get its ID
2. `get_content_versions` — returns the full version history with timestamps and comments:
   ```
   Version 5 — "Updated hero section" — 2026-02-25
   Version 4 — "Added testimonials" — 2026-02-20
   Version 3 — "Redesigned layout" — 2026-02-15
   Version 2 — "Initial content" — 2026-02-10
   Version 1 — "Created page" — 2026-02-08
   ```
3. `revert_to_version` — rolls back to version 2, creating a new version 6 with the old data. The published page is regenerated immediately.

### Example 8: Organize Content with Folders

**Prompt:** "Create a /docs section and move the API reference page into it"

**Tool calls:**
1. `create_folder` — creates the URL path segment:
   ```json
   { "name": "Documentation", "slug": "docs" }
   ```
2. `get_content` — retrieves the API reference page to get its ID
3. `update_content` — moves it into the new folder:
   ```json
   { "id": "...", "folder_path": "/docs" }
   ```
   The page is now accessible at `/docs/api-reference` instead of `/api-reference`.

### Example 9: Build a Content Collection (Blog Index)

**Prompt:** "Create a blog listing page that shows all blog posts sorted by newest first"

**Tool calls:**
1. `create_collection` — defines the collection with category filter, sorting, and display templates:
   ```json
   {
     "name": "Blog",
     "slug": "blog",
     "category": "blog",
     "sort_field": "created_at",
     "sort_order": "desc",
     "items_per_page": 10,
     "item_template": "<article><h2><a href=\"{{.Path}}\">{{.Title}}</a></h2><p>{{.excerpt}}</p><time>{{.PublishedAt}}</time></article>",
     "page_template": "<div class=\"blog-index\"><h1>Blog</h1>{{.Items}}{{.Pagination}}</div>"
   }
   ```
   A paginated blog listing is now live at `/blog`, automatically including any content with category "blog".

### Example 10: Build a Dynamic Tagged Index Page

**Prompt:** "Create a glossary index that automatically lists all my concept pages grouped by category, and keep it updated as I add new pages"

**Tool calls:**
1. `create_snippet` — creates a reusable rendering template for each result:
   ```json
   {
     "name": "glossary-pill",
     "html": "<a href=\"{{.FullPath}}\">{{.Title}}</a>"
   }
   ```

2. `create_template` — creates the index page template with `lc:query` directives embedded:
   ```json
   {
     "name": "Concepts Index",
     "slug": "concepts-index",
     "fields": [
       { "name": "intro", "label": "Introduction", "type": "textarea" }
     ],
     "html_layout": "<article class=\"index-page\">\n<h1>{{.title}}</h1>\n{{if .intro}}<p>{{.intro}}</p>{{end}}\n\n<h2>AI &amp; Machine Intelligence</h2>\n<div class=\"links\">\n<!-- lc:query filter=\"tag:AI & Machine Intelligence\" sort=\"title:asc\" snippet=\"glossary-pill\" -->\n</div>\n\n<h2>Games &amp; Interactive Experiences</h2>\n<div class=\"links\">\n<!-- lc:query filter=\"tag:Games & Interactive Experiences\" sort=\"title:asc\" snippet=\"glossary-pill\" -->\n</div>\n</article>"
   }
   ```

3. `create_content` — creates the index page using the new template:
   ```json
   {
     "template_id": "<concepts-index-template-id>",
     "title": "Concepts Glossary",
     "slug": "glossary",
     "data": {
       "intro": "An index of all concepts, grouped by category."
     }
   }
   ```

4. `update_content` — tags several existing concept pages (each call):
   ```json
   { "tags": ["AI & Machine Intelligence"] }
   ```

5. `publish_content` — publishes the index page; the `lc:query` directives are resolved at this moment and the page is generated with all currently-tagged content already populated.

From now on, every time a new concept page is published with a matching tag, the index page at `/glossary` is automatically regenerated — no further action needed.

### Example 11: Full-Text Search and Content Audit

**Prompt:** "Find all pages that mention 'pricing' and show me which ones are still in draft"

**Tool calls:**
1. `search_content` — performs a full-text search across all content fields:
   ```json
   { "query": "pricing", "search_type": "fulltext" }
   ```
   Returns matching content items with their publish status, paths, and which fields matched:
   ```
   Found 4 results for 'pricing':
   - "Pricing Plans" at /pricing — published — matched in: content
   - "Enterprise FAQ" at /enterprise-faq — published — matched in: content, sidebar
   - "New Pricing Draft" at /new-pricing — draft — matched in: title, content
   - "Q1 Press Release" at /press/q1-update — draft — matched in: body
   ```
   The two draft items can then be reviewed, edited, and published as needed.

### Working with Snippets

```
# List all snippets
list_snippets

# Create a callout snippet
create_snippet {
  "name": "callout-warning",
  "description": "Warning callout box",
  "html": "<div class=\"callout callout-warning\"><strong>⚠️ {{.Title}}</strong><p>{{.Body}}</p></div>"
}

# Use a snippet inline in content
update_content {
  "id": "...",
  "data": {
    "body": "Here is important information:\n\n[[include:callout-warning]]\n\nContinued text..."
  }
}
```

### Content Tagging & Index Pages

```
# Tag a page at creation time
create_content {
  "template_id": "...",
  "title": "Introduction to AI",
  "slug": "intro-to-ai",
  "tags": ["AI & Machine Intelligence", "Getting Started"],
  "data": { "body": "..." }
}

# Inline tagging via content body
update_content {
  "id": "...",
  "data": {
    "body": "This page covers #machine-learning and #neural-networks."
  }
}

# Query directive in a template layout (for index pages)
<!-- lc:query filter="tag:AI & Machine Intelligence" sort="title:asc" snippet="concept-card" -->
```

### Bulk Operations

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

# Clear a field across all pages of a template
bulk_field_operation {
  "operation": "clear",
  "field": "old_badge",
  "template_name": "Concept Page",
  "version_comment": "Cleared deprecated field"
}

# Prepend a disclaimer to all blog posts
bulk_field_operation {
  "operation": "prepend",
  "field": "body",
  "value": "<div class=\"disclaimer\">Views are my own.</div>",
  "template_name": "Blog Post",
  "version_comment": "Added disclaimer to all posts"
}
```

### Regex Search & Replace

```
# Preview: find all pages with old badge HTML pattern
scoped_search_replace_preview {
  "search": "<div class=\"badge-v1\".*?</div>",
  "replace": "",
  "regex": true,
  "template_name": "Concept Page"
}

# Execute after confirming preview
scoped_search_replace_execute {
  "search": "<div class=\"badge-v1\".*?</div>",
  "replace": "",
  "regex": true,
  "template_name": "Concept Page",
  "version_comment": "Removed old v1 badge HTML"
}
```

### Wikilinks

```
# Link to another page by title
update_content {
  "id": "...",
  "data": {
    "body": "See also: [[Machine Learning]] and [[AI Ethics|ethics considerations]]."
  }
}

# Find what pages link to a given path
get_backlinks { "path": "/concepts/machine-learning" }
```

### Content Versioning

```
# See version history for a page
get_content_versions { "content_id": "abc123" }

# Restore a previous version
revert_to_version {
  "content_id": "abc123",
  "version": 3,
  "version_comment": "Reverted to pre-redesign version"
}
```

## Development

```bash
# Run with hot reload (using air)
go install github.com/cosmtrek/air@latest
air

# Build all binaries
go build -o bin/lightcms-server ./cmd/server
go build -o bin/lightcms-mcp ./cmd/mcp
go build -o bin/lightcms ./cmd/cli

# Run the server
./bin/lightcms-server
```

## Security Notes

For production:
1. Use a strong `session_secret` — **minimum 32 characters** (generate with `openssl rand -hex 32`). The server hard-fails on startup if this requirement isn't met in production.
2. Set `LIGHTCMS_ADMIN_EMAIL` so the initial admin account uses your real email
3. Change the default admin password immediately after first login
4. Use HTTPS (put behind a reverse proxy like nginx or caddy)
5. Restrict MongoDB Atlas IP whitelist to your server IPs
6. API keys inherit the permissions of their owning user — keep admin keys secure
7. Review the audit log regularly at `/cm/audit`
8. Regularly backup your MongoDB database
9. Configure `max_upload_bytes` in site settings to cap file upload size for your use case

Security features built in:
- CSRF protection on all `/cm` routes
- RBAC permission checks on all admin handlers and REST API endpoints
- Session cookies: SameSite=Strict, 24-hour expiry, Secure in production
- File uploads: extension whitelist + MIME validation + configurable size cap
- API request body cap: 10 MiB enforced on all `/api/v1/` endpoints
- Login rate limiting: escalating lockouts (1 min → 5 min → 15 min)
- Per-endpoint rate limiters: regenerate (2/min), search-replace execute (10/min), bulk-update (5/min), export (5/min), reindex (1/min)
- Passwords: bcrypt with cost=12
- Audit logging on all mutations with 365-day retention
- Fly.io deployments: `Fly-Client-IP` header used for real client IP (unspoofable, unlike `X-Forwarded-For`)
- Chat widget prompt injection defense: user input wrapped in XML delimiters, `</` sequences escaped

## Privacy Policy

LightCMS is self-hosted software — you control your database, your hosting, and your data. The MCP server and CLI tool connect to your LightCMS instance via the REST API — no data is transmitted to Metavert LLC or any third party.

For the full privacy policy, see: https://www.metavert.io/lightcms-privacy-policy

## License

MIT
