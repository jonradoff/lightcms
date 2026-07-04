Read VIBECTL.md for current project status, deployment details, and issue context before starting work.

# LightCMS - Claude Code Memory

## Project Overview

LightCMS is a lightweight, self-hosted content management system built in Go. It uses MongoDB for data storage and generates static HTML pages for public content serving.

**Key URLs:**
- Admin Dashboard: `/cm`
- Public Site: `/`
- MCP Server: `bin/lightcms-mcp` (stdio transport)

## MCP Server Integration

**IMPORTANT:** All website content operations MUST go through the MCP server. Do NOT:
- Write scripts to directly modify the database
- Use Go code to create/edit/delete content
- Bypass the MCP server for any content management tasks

If the MCP server is not available or not working, ASK the user for permission before attempting any content changes through other means.

### Starting the MCP Server

If the user requests a content operation and the MCP server is not yet running, start it using the convenience script:

```bash
./bin/lightcms-mcp-wrapper.sh
```

This wrapper script sets up the required environment variables (LIGHTCMS_CONFIG_DIR) and launches the MCP server. The MCP server uses stdio transport, so it will be connected automatically once started.

### Content Operations (REQUIRE MCP or explicit user permission):
- Creating, editing, publishing, or deleting content
- Managing templates and their HTML layouts
- Uploading or managing assets (images, CSS, JS, documents)
- Updating theme settings (colors, fonts, header/footer HTML)
- Managing redirects, folders, and collections
- Viewing or reverting content versions
- Site configuration changes

### Code Changes (allowed without MCP):
- Adding new features to LightCMS itself
- Fixing bugs in the application
- Changing application behavior or logic
- Adding new MCP tools
- Modifying database schemas or indexes
- Security improvements

### ⚠️ IMPORTANT: Rebuild MCP binary after adding tools

The Claude Desktop MCP integration uses a **local binary** (`bin/lightcms-mcp`) that connects to the remote server. The binary encodes all tool definitions — if you add or remove MCP tools without rebuilding it, Claude Desktop will see stale/missing tools.

**After any change to `cmd/mcp/` or `internal/mcp/`:**
```bash
go build -o bin/lightcms-mcp ./cmd/mcp
```

Then **restart Claude Desktop** to reload the MCP server. Until it's restarted, Cowork will still use the old binary.

### Setting Up MCP with Claude Code

Run the setup script from the lightcms directory:

```bash
./setup-mcp.sh
```

This builds the MCP server, creates the wrapper script, and registers it with Claude Code.

**Manual setup** (if needed):
```bash
# Build the MCP server
go build -o bin/lightcms-mcp ./cmd/mcp

# Register with Claude Code (use the wrapper script, not the binary directly)
claude mcp add --transport stdio lightcms-mcp -- /path/to/lightcms/lightcms-mcp-wrapper.sh
```

After registering, restart Claude Code. You can verify the server is connected:
- Run `/mcp` in Claude Code to check status
- Run `claude mcp list` in terminal to see registered servers

Once connected, you can ask Claude to manage your content naturally:
- "Create a new blog post about AI"
- "List all my published content"
- "Update the homepage hero image"
- "Delete the /random page"

### MCP Server Location
Binary: `bin/lightcms-mcp`
Config: Uses same `config.dev.json` or environment variables as main server

### Available MCP Tools (115 total):

**Content (23 tools):** list_content, get_content, create_content, update_content, update_content_by_path, publish_content, publish_multiple, unpublish_content, delete_content, restore_content, preview_content, get_content_versions, get_content_version, revert_to_version, bulk_create_content, bulk_update_content, bulk_field_operation, export_content, get_backlinks

**Templates (5 tools):** list_templates, get_template, create_template, update_template, delete_template

**Snippets (5 tools):** list_snippets, get_snippet, create_snippet, update_snippet, delete_snippet

**Assets (6 tools):** list_assets, list_asset_folders, get_asset, upload_asset, upload_asset_from_url, delete_asset

**Search (7 tools):** search_content, search_replace_preview, search_replace_execute, scoped_search_replace_preview, scoped_search_replace_execute, end_user_search, reindex_embeddings

**Settings (18 tools):** get_theme, update_theme, get_theme_versions, get_theme_version, revert_theme_to_version, pin_theme_version, unpin_theme_version, get_site_config, update_site_config, list_redirects, create_redirect, update_redirect, delete_redirect, list_folders, create_folder, get_folder, delete_folder, list_collections, create_collection, get_collection, update_collection, delete_collection, regenerate_all_content

**Forks (8 tools):** list_forks, create_fork, get_fork, fork_page, remove_fork_page, merge_fork, archive_fork, delete_fork

**Comments (3 tools, v6.0+):** list_comments, create_comment, delete_comment

**Agent Sandbox & Governance (8 tools, v6.2+):** start_agent_sandbox, get_agent_sandbox, end_agent_sandbox, get_fork_diff, get_agent_session_changes, rollback_agent_session, get_maintenance_report, run_maintenance_scan

**Approvals (11 tools, v6.0+):** list_approval_workflows, get_approval_workflow, create_approval_workflow, update_approval_workflow, delete_approval_workflow, list_approval_requests, get_approval_request, submit_for_approval, approve_request, reject_request, cancel_approval_request

### ⚠️ CRITICAL: Search & Replace Safety

The `search_replace_execute` tool is **destructive** and modifies content permanently. Before using it, you MUST:

1. **ALWAYS run `search_replace_preview` first** to see exactly what will be changed
2. **Show the user the preview results**, including:
   - Number of pages affected
   - Which pages are published vs drafts
   - Sample excerpts showing what will change
3. **Explicitly ask for user confirmation** before running `search_replace_execute`
4. **Never execute search/replace without user consent**, even if the user asked for a "quick fix"

Example interaction:
```
User: "Update all links from http to https"
Assistant: "Let me preview what would change..."
[Run search_replace_preview]
Assistant: "This would affect 15 pages (12 published, 3 drafts) with 47 total replacements. Here are the affected pages: [list]. Should I proceed with these changes?"
User: "Yes, go ahead"
[Run search_replace_execute]
```

## Content Authoring: Wiki-Like Markup

When creating or updating content via MCP or admin UI, the following markup features are processed at page generation time:

### Wikilinks
- `[[Page Title]]` — links to a page by its title (case-insensitive lookup)
- `[[Page Title|display text]]` — link with custom label
- `[[/full/path]]` — links to a page by its URL path
- `[[/full/path|display text]]` — path link with custom label
- Broken links render as `<span class="broken-link">text</span>`
- Links auto-update when a page's title or path changes (via `UpdateWikilinksOnRename`)

### Snippet Includes
- `[[include:snippet-name]]` — embeds a named snippet inline
- Snippet name must match the `name` field in the snippets collection
- Recursion depth limit: 3 levels; cycles are detected and dropped

### Table of Contents
- Add `{{.lc_toc}}` in a template's HTML layout where the TOC should appear
- Auto-generates `<nav class="lc-toc"><ul>...</ul></nav>` from page headings
- All headings automatically get `id=` attributes for anchor navigation

### Markdown Fields
- Set a template field type to `markdown` for GFM rendering at publish time
- Supports tables, strikethrough, task lists, autolinks
- Script policy controls whether raw HTML/scripts are allowed (see Script Policy below)

### Inline Tag Detection
- Mention `#tagname` in any content field to automatically tag the page
- Tag names: start with a letter, alphanumeric/underscore/hyphen, max 50 chars
- Tags feed into `lc:query` index pages

### Snippet Best Practices (for MCP agents)
- Use snippets for reusable UI components: callout boxes, CTA sections, disclaimers, badge patterns
- Snippet variables: `{{.Title}}`, `{{.FullPath}}`, `{{.Slug}}`, `{{.MetaDescription}}`, `{{.PublishedAt}}`
- Reference snippets in content with `[[include:snippet-name]]` or in template layouts via `lc:query` directives

## Agentic Safety Features (v6.2+)

- **Agent sandbox sessions**: `start_agent_sandbox` makes all content writes go into a fork (copy-on-write); live content is untouched until a human reviews the diff at `/cm/forks/{id}` and merges. Publishing/deleting/bulk ops are blocked while sandboxed. PREFER sandbox mode for multi-page or risky edits.
- **Sandbox-only API keys**: keys created with `sandbox_only: true` are server-side restricted — content writes must target a fork; publish/delete/settings/search-replace are rejected. Keys can also carry a `scopes` permission allowlist.
- **Session ledger & rollback**: each MCP session has an agent-session ID recorded in the audit log; `get_agent_session_changes` shows everything the session touched, `rollback_agent_session` undoes it as a unit.
- **Provenance**: every content version records actor (human|agent), via (ui|api|copilot), and the agent session.
- **Maintenance reports**: a daily scan surfaces stale pages, missing meta descriptions, and drafts via `get_maintenance_report` — treat it as a work queue.
- **Public read-only MCP**: `/mcp-public` (no auth) exposes search_site/get_page/list_pages/get_site_info to visitors' agents; `/llms.txt` and `/llms-full.txt` serve AI crawlers.
- **Admin copilot**: `/cm/copilot` lets editors drive content operations in natural language (requires ANTHROPIC_API_KEY; model via LIGHTCMS_COPILOT_MODEL).

## Script Policy

Site-wide setting `markdown_script_policy` (configurable via `update_site_config`):
- `"all"` — default; all roles may use raw HTML including `<script>` in markdown fields
- `"admin_only"` — editors' content is sanitized; admin content passes through unchanged
- `"none"` — all content sanitized regardless of author role

The sanitizer (bluemonday) strips: `<script>`, `<iframe>`, `<form>`, `<input>`, event handlers (`onclick=`, etc.), `javascript:` URIs. All other HTML is preserved.

## Build Commands

```bash
# Build main HTTP server
go build -o bin/lightcms ./cmd/server

# Build MCP server
go build -o bin/lightcms-mcp ./cmd/mcp

# Run main server (requires config.dev.json or env vars)
./bin/lightcms

# Run MCP server (stdio transport)
./bin/lightcms-mcp
```

## Project Structure

```
lightcms/
├── cmd/
│   ├── server/main.go      # Main HTTP server entry point
│   └── mcp/main.go         # MCP server entry point
├── config/config.go        # Configuration (env vars or JSON)
├── internal/
│   ├── auth/               # Session-based authentication
│   ├── database/mongo.go   # MongoDB connection & helpers
│   ├── errors/             # Environment-aware error handling
│   ├── handlers/           # HTTP request handlers (~5000 lines)
│   ├── mcp/                # MCP tool implementations
│   ├── middleware/         # Security headers, file validation
│   ├── models/models.go    # Data models & default templates
│   └── services/           # Business logic layer
├── templates/              # Admin UI HTML templates
├── static/                 # CSS, JS, images
└── content/generated/      # Static HTML output
```

## Key Architecture Patterns

### Service Layer
All business logic goes through services in `internal/services/`:
- **ContentService**: CRUD with automatic versioning, static page generation
- **TemplateService**: Template management with content regeneration
- **AssetService**: File uploads with validation
- **SettingsService**: Theme, config, redirects, folders, collections
- **UserService**: User CRUD, password management, credential validation
- **AuditService**: Sync/async audit log writes, filtered listing
- **APIKeyService**: API key management with user ownership

### Content Versioning
Every content update automatically creates a version. Versions are stored in `content_versions` collection. Use `revert_to_version` to restore previous versions.

**IMPORTANT:** When updating content via MCP, ALWAYS include a `version_comment` parameter with a concise description of what changed, even if the user doesn't explicitly provide one. This makes version history useful for tracking changes over time.

Examples:
- "Updated page title"
- "Revised introduction paragraph"
- "Added new section on security"
- "Fixed typo in heading"
- "Replaced hero image"

### Static Page Generation
Published content is rendered to `content/generated/{path}.html` using template HTML + theme header/footer. Regeneration happens automatically on:
- Content publish/update
- Template layout change
- Theme header/footer change

### Database Collections
- `content` - Content items with full_path (unique index)
- `content_versions` - Version history (includes modified_by, modified_by_email)
- `templates` - Content templates with fields + HTML layout
- `folders` - Content organization hierarchy
- `collections` - Content grouping by category
- `assets` - File metadata (binary stored on filesystem)
- `redirects` - URL redirect rules
- `settings` - Theme, config settings
- `users` - User accounts with email/password/role (unique email index)
- `audit_logs` - Audit trail (who did what, when; 365-day TTL)
- `api_keys` - API keys with optional user_id ownership
- `contact_messages` - Form submissions
- `login_attempts` - Rate limiting data

## Code Conventions

### Naming
- Services: `ContentService`, `TemplateService`
- Handlers: `CreateContent`, `UpdateTemplate`
- DB helpers: `FindOne`, `UpdateOne`, `InsertOne`
- Receivers: `h` (Handler), `s` (Service), `db` (DB)

### Error Handling
```go
return fmt.Errorf("context description: %w", err)
```

### MongoDB Patterns
```go
// Filters use bson.M
filter := bson.M{"_id": id, "deleted": bson.M{"$ne": true}}

// Updates use bson.M with operators
update := bson.M{"$set": bson.M{"title": title, "updated_at": time.Now()}}

// Sorting uses bson.D for order
opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
```

### Context
Always pass context as first parameter for DB/service methods.

## Multiuser RBAC System (v2.0+)

### Roles
- **admin**: Full access — manage users, templates, settings, audit logs, all API keys
- **editor**: Create/edit/delete/publish content, upload/delete assets, manage own API keys
- **viewer**: Read-only access to content, templates, assets, settings

### Auth Flow
- Login with email + password (migrated from single-admin password)
- Session stores: user_id, user_email, user_role
- Force password change on first login with temporary password
- API keys carry the permissions of their owning user

### Migration
On first startup with empty `users` collection, creates admin user from existing `settings.admin` password hash. Set `LIGHTCMS_ADMIN_EMAIL` env var (default: `admin@localhost`).

### Password Reset
```bash
go run cmd/resetpw/main.go [email]  # Reset specific user, or first admin if no email given
```

## Security Notes

- CSRF protection on all `/cm` routes (Gorilla CSRF)
- Session cookies: SameSite=Strict, 24-hour expiry, Secure in production
- File uploads: Extension whitelist + MIME validation
- Path traversal protection on all file operations
- Login rate limiting: Escalating lockout (10→1min, 15→5min, 20+→15min)
- Passwords: bcrypt with cost=12
- RBAC permission checks on all admin handlers and REST API endpoints
- Audit logging on all mutations (async, 365-day TTL auto-cleanup)

## Configuration

**Environment Variables (Production):**
- `MONGO_URI` - MongoDB connection string
- `SESSION_SECRET` - 32+ char secret
- `BASE_URL` - Public URL (e.g., https://example.com)
- `PORT` - Server port (default: 80)
- `ENV` - "production" or "development"
- `SECURE_COOKIES` - "true" for HTTPS

**JSON Config (Development):**
`config.dev.json`:
```json
{
  "port": "8082",
  "mongo_uri": "mongodb+srv://...",
  "env": "development",
  "session_secret": "dev-secret-change-in-prod",
  "base_url": "http://localhost:8082",
  "secure_cookies": false
}
```

## Default Templates

7 built-in system templates:
1. Blog Post
2. Press Release
3. Explanatory Page
4. Blank Page
5. Homepage
6. Concept Page
7. Standard Page

Template fields support types: text, textarea, richtext, date, image, select

## Bulk Operations & Programmatic SEO

LightCMS supports large-scale content operations (2,000+ pages) via optimized bulk APIs. These guidelines apply to programmatic SEO, mass content migration, site-wide link fixing, and similar large-scale tasks.

### Bulk Content Creation
- Use `bulk_create_content` (up to 100 items/call) instead of calling `create_content` in a loop
- Uses MongoDB `InsertMany` with unordered mode — one failure doesn't abort the batch
- Published items get parallel HTML generation (10 concurrent)
- Set `upsert: true` to update existing pages instead of failing on duplicates
- Example: creating 2,000 pages = 20 batch calls of 100

### Content Upsert (Idempotent Creates)
- `create_content` accepts `upsert: true` — if a page exists at the same path, it updates instead of failing
- Eliminates the most common failure mode in retry scenarios
- `bulk_create_content` also supports `upsert: true` for batch idempotency

### Bulk Search & Replace
- `search_replace_preview` and `search_replace_execute` accept a `pairs` array for multi-pair mode
- Each page is scanned once with all pairs applied in order — O(pages) instead of O(pairs × pages)
- Critical for operations like fixing hundreds of broken links in a single pass
- Returns `pages_scanned`, `pages_modified`, `total_replacements` counts
- Pairs are applied in array order — replacement from pair 1 may create text that pair 2 matches

### Conditional Republish
- Static HTML generation uses content hashing (SHA-256) — unchanged pages are skipped automatically
- `RegenerateAllContent` (triggered by theme/template changes) clears all hashes first to force full regen
- For search/replace with `auto_republish: true`, only modified pages get republished

### Content List Pagination
- `list_content` supports `limit` and `offset` parameters for paginated results
- Returns `{items, total, limit, offset, has_more}` envelope when `limit` is set
- Default (no limit) returns all items — backward compatible
- Max limit: 500 per request
- For sites with 2,000+ pages, use pagination to avoid large JSON responses

### Rate Limits
- API: 300 requests/minute per bearer token (sliding window)
- Burst: 20 requests/second per bearer token (prevents runaway scripts)
- Bulk endpoints: additional per-endpoint limits (regenerate, search/replace, bulk update)
- When rate limited, response includes `Retry-After` header

### Best Practices for Large-Scale Operations
1. **Use batch APIs** — `bulk_create_content` and `bulk_update_content` over individual calls
2. **Use multi-pair search/replace** — preview first, then execute with all pairs in one call
3. **Set `auto_republish: true`** on search/replace to avoid a separate publish step
4. **Include `version_comment`** on all bulk operations for readable version history
5. **Use `upsert: true`** for retry-safe content creation (idempotent)
6. **Paginate list_content** with `limit: 100` to avoid loading 2,000+ items at once
7. **Don't exceed 100 items per bulk call** — this is enforced server-side

## Deployment

Deployed to Fly.io (`metavert-cms` app, machine `d890122a371528`). Uses environment variables for configuration. Health check at `/health`.

**Deploy procedure — always use the deploy script:**
```bash
./deploy.sh
```

The script builds the image via `fly deploy --detach`, extracts the image ref, destroys any stuck new machines, then updates the actual running machine directly via `fly machines update`. This is necessary because the running machine (`d890122a371528`) is a legacy non-Launch machine — `fly deploy` doesn't see it as a managed machine and creates new stuck orphan machines instead of updating it. The machine ID is hardcoded in `deploy.sh`.

## Testing Locally

1. Create `config.dev.json` with MongoDB Atlas connection
2. `go build -o bin/lightcms ./cmd/server`
3. `./bin/lightcms`
4. Access admin at http://localhost:8082/cm (default: admin@localhost / admin123)
