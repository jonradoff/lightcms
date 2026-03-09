# LightCMS

[![CI](https://github.com/jonradoff/lightcms/actions/workflows/ci.yml/badge.svg)](https://github.com/jonradoff/lightcms/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jonradoff/lightcms/branch/main/graph/badge.svg)](https://codecov.io/gh/jonradoff/lightcms)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonradoff/lightcms)](https://goreportcard.com/report/github.com/jonradoff/lightcms)

A lightweight, AI-native content management system for building and managing websites. Built with Go and MongoDB Atlas.

## Why LightCMS?

**Lightweight**: A clean, focused codebase (~5K lines of Go) that's easy to understand, modify, and extend. No bloated frameworks or complex abstractions.

**AI-Native**: Built from the ground up for the AI era:
- **MCP Integration**: Full Model Context Protocol server with 43 tools for website management. Supports both local stdio and HTTP streamable transports — connect from Claude Code, Claude Desktop, or any MCP-compatible client.
- **OAuth 2.1 for Remote Agents**: Sandboxed desktop apps like Claude's Cowork can securely connect over HTTP using OAuth 2.1 with PKCE. No embedded passwords — just authorize once and the agent manages your site.
- **Fork-Friendly**: Designed to be forked and customized by Claude Code. Ask Claude to add new content types, modify templates, or build custom features — the codebase is structured for AI-assisted development.
- **Natural Language Website Management**: Skip the admin UI entirely. Create pages, manage assets, customize themes, and publish content through conversation.

## Features

### Content Management
- **Template System**: Define reusable content structures with custom fields (text, richtext, image, date, select)
- **Static Page Generation**: Fast page loads from pre-rendered HTML — no runtime templating overhead
- **Content Versioning**: Full version history with diff comparison and one-click revert
- **Soft Delete**: Recover deleted content with undelete functionality
- **Content Collections**: Auto-generated paginated listing pages filtered by category
- **Folders & URL Organization**: Hierarchical content organization with clean URL paths
- **Rich Text Editor**: TinyMCE integration for visual content editing
- **Search & Replace**: Site-wide search and replace with preview before execution

### Multi-User Access Control (v2.0+)
- **Role-Based Access Control (RBAC)**: Three roles — admin, editor, viewer — with granular permission enforcement on all admin UI pages and REST API endpoints
- **User Management**: Admin panel for creating users, assigning roles, disabling accounts, and resetting passwords
- **Audit Log**: Persistent, searchable log of all mutations (who did what, when) with 365-day retention
- **User-Scoped API Keys**: API keys inherit the permissions of their owning user
- **Force Password Change**: Temporary passwords trigger a mandatory change on first login

### Smart End-User Search
- **Hybrid Search**: Combines full-text exact matching with semantic vector search (Voyage AI embeddings), merged via reciprocal rank fusion
- **Intelligent Ranking**: Results ranked by structural importance — nav-linked pages surface first, followed by concept pages, then general content; video transcripts are deprioritised
- **Title Boost**: Pages where the query appears in the title always rank above body-only matches
- **Typeahead Suggestions**: Fast prefix-matching page suggestions with the same structural ranking
- **Works Without Embeddings**: Falls back to full-text search if no Voyage API key is configured
- **Rate Limiting**: Per-IP and global rate limiting for DDoS protection

### Developer & Integration
- **REST API**: Full `/api/v1/` JSON API with API key and OAuth token authentication, RBAC-enforced
- **MCP Server**: 43 tools for agentic website management (stdio + HTTP streamable)
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

LightCMS exposes a public search API at `/api/search` that your site's frontend can call. It supports three modes:

- `mode=fulltext` — exact regex match across all content
- `mode=semantic` — vector similarity search (requires Voyage AI key)
- `mode=hybrid` — merges both using reciprocal rank fusion (recommended)

Results are ranked by: title match → nav-linked pages → concept pages → general body content → video transcripts. Configure your Voyage AI key in the admin panel under **Configuration** to enable semantic search.

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

LightCMS includes a full MCP (Model Context Protocol) server with 43 tools for managing your entire website through AI agents. It supports two transport modes:

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

### Available Tools

- **Content** (12): create, read, update, delete, publish, unpublish, versioning, search
- **Templates** (5): create, read, update, delete, list
- **Assets** (5): upload, read, delete, list files and folders
- **Settings** (16): theme, site config, redirects, folders, collections
- **Search** (3): full-text search, search-and-replace with preview
- **Utility** (2): regenerate all content, server card

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

### Example 10: Full-Text Search and Content Audit

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
1. Use a strong `session_secret` (generate with `openssl rand -hex 32`)
2. Set `LIGHTCMS_ADMIN_EMAIL` so the initial admin account uses your real email
3. Change the default admin password immediately after first login
4. Use HTTPS (put behind a reverse proxy like nginx or caddy)
5. Restrict MongoDB Atlas IP whitelist to your server IPs
6. API keys inherit the permissions of their owning user — keep admin keys secure
7. Review the audit log regularly at `/cm/audit`
8. Regularly backup your MongoDB database

Security features built in:
- CSRF protection on all `/cm` routes
- RBAC permission checks on all admin handlers and REST API endpoints
- Session cookies: SameSite=Strict, 24-hour expiry, Secure in production
- File uploads: extension whitelist + MIME validation
- Login rate limiting: escalating lockouts (1 min → 5 min → 15 min)
- Passwords: bcrypt with cost=12
- Audit logging on all mutations with 365-day retention

## Privacy Policy

LightCMS is self-hosted software — you control your database, your hosting, and your data. The MCP server and CLI tool connect to your LightCMS instance via the REST API — no data is transmitted to Metavert LLC or any third party.

For the full privacy policy, see: https://www.metavert.io/lightcms-privacy-policy

## License

MIT
