# LightCMS

A lightweight, AI-native content management system for building and managing websites. Built with Go and MongoDB Atlas.

## Why LightCMS?

**Lightweight**: A clean, focused codebase (~5K lines of Go) that's easy to understand, modify, and extend. No bloated frameworks or complex abstractions.

**AI-Native**: Built from the ground up for the AI era:
- **MCP Integration**: Full Model Context Protocol server with 41 tools for website management. Control your entire site through Claude Code or other agentic workflows.
- **Fork-Friendly**: Designed to be forked and customized by Claude Code. Ask Claude to add new content types, modify templates, or build custom features—the codebase is structured for AI-assisted development.
- **Natural Language Website Management**: Skip the admin UI entirely. Create pages, manage assets, customize themes, and publish content through conversation.

## Features

- **AI-Powered Website Management**: MCP server for agentic control of your site
- **REST API**: Full `/api/v1/` JSON API with API key authentication
- **CLI Tool**: Command-line interface for all content management operations
- **API Key System**: Create and manage API keys from the admin panel
- **Template System**: Define reusable content structures with custom fields
- **Static Page Generation**: Fast page loads from pre-rendered HTML
- **Content Collections**: Group and display content by category
- **Custom Pages**: Full HTML control for special pages
- **Theme Customization**: Modern, sleek design with customizable colors, fonts, and styles
- **Rich Text Editor**: TinyMCE integration for visual content editing
- **Admin Panel**: Secure content management at `/cm`
- **Content Versioning**: Full version history with diff comparison and revert capability
- **Soft Delete**: Recover deleted content with undelete functionality

## Prerequisites

- Go 1.21 or later
- MongoDB Atlas account (free tier works great)

## Quick Start

1. Clone the repository
2. Copy `config.dev.json.example` to `config.dev.json`
3. Edit `config.dev.json` with your MongoDB connection string
4. Run `go run cmd/server/main.go`
5. Visit http://localhost:8082/cm and login with `admin123`
6. Change your password immediately via Security settings

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

### Default Admin Password

The default password is `admin123`. On first login, you'll be prompted to change it.

The admin password is stored securely in MongoDB using bcrypt hashing.

### Creating Content

1. Log in to the admin panel at `/cm`
2. Go to **Content** → **New Content**
3. Select a template (Blog Post, Press Release, or Explanatory Page)
4. Fill in the fields
5. Check "Published" and save

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

## Project Structure

```
lightcms/
├── cmd/
│   ├── server/main.go        # HTTP server entry point
│   ├── mcp/main.go           # MCP server entry point
│   └── cli/main.go           # CLI tool entry point
├── config/
│   └── config.go             # Configuration loading
├── internal/
│   ├── apiclient/            # Reusable HTTP client for REST API
│   ├── auth/                 # Authentication logic
│   ├── cli/                  # CLI subcommands and output formatting
│   ├── database/             # MongoDB connection & operations
│   ├── handlers/             # HTTP handlers (admin UI + REST API)
│   ├── mcp/                  # MCP server and tool definitions
│   ├── middleware/            # API auth middleware
│   ├── models/               # Data models & default templates
│   └── services/             # Business logic services
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

## API Keys

API keys are required for the REST API, MCP server, and CLI tool. Create them from the admin panel.

1. Log in at `/cm`
2. Go to **Settings** → **API Keys**
3. Click **Create New Key**, give it a name and description
4. Copy the key immediately — it's only shown once

Keys use the format `lc_` followed by 32 hex characters. They're stored as SHA-256 hashes.

## REST API

LightCMS provides a full REST API at `/api/v1/` authenticated with API keys.

### Authentication

Include your API key in the `Authorization` header:

```bash
curl -H "Authorization: Bearer lc_your_key_here" http://localhost:8082/api/v1/content
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

## Using with Claude Code (AI-Powered Content Management)

LightCMS includes an MCP (Model Context Protocol) server that allows you to manage your website content using Claude Code. Instead of navigating the admin UI, you can simply ask Claude to create pages, update content, manage assets, and more.

The MCP server connects to LightCMS via the REST API using an API key — no direct database access needed. This means MCP clients can run remotely.

### Quick Setup

1. Create an API key in the admin panel at `/cm` → Settings → API Keys
2. Run the setup script:

```bash
export LIGHTCMS_API_KEY=lc_your_key_here
./setup-mcp.sh
```

This will:
1. Build the MCP server and CLI binaries
2. Create the wrapper script
3. Register the MCP server with Claude Code

Then restart Claude Code and run `/mcp` to verify the connection.

### Manual Setup

1. Build the MCP server:
   ```bash
   go build -o bin/lightcms-mcp ./cmd/mcp
   ```

2. Register with Claude Code:
   ```bash
   claude mcp add --transport stdio lightcms-mcp \
     -e LIGHTCMS_URL="http://localhost:8082" \
     -e LIGHTCMS_API_KEY="lc_your_key_here" \
     -- /path/to/lightcms/bin/lightcms-mcp
   ```

3. Restart Claude Code and verify with `/mcp`

### Environment Variables

The MCP server uses these environment variables:
- `LIGHTCMS_URL` - Server URL (default: `http://localhost:8082`)
- `LIGHTCMS_API_KEY` - API key (required)

### Available Tools

The MCP server provides 41 tools for complete content management:

- **Content**: Create, read, update, delete, publish, unpublish, versioning
- **Templates**: Manage content templates and their fields
- **Assets**: Upload and manage images, documents, and other files
- **Settings**: Theme customization, redirects, folders, collections

For detailed API documentation, see [MCP.md](MCP.md).

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
2. Change the default admin password immediately after first login
3. Use HTTPS (put behind a reverse proxy like nginx or caddy)
4. Restrict MongoDB Atlas IP whitelist to your server IPs
5. Keep API keys secure — they grant full admin-level access to the REST API
6. Regularly backup your MongoDB database

## Privacy Policy

LightCMS is self-hosted software — you control your database, your hosting, and your data. The MCP server and CLI tool connect to your LightCMS instance via the REST API — no data is transmitted to Metavert LLC or any third party.

For the full privacy policy, see: https://www.metavert.io/lightcms-privacy-policy

## License

MIT
