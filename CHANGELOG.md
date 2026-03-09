# Changelog

All notable changes to LightCMS are documented here, organized by version.

---

## v2.1.0 — Agentic API Improvements

### Theme Reliability
- **Theme CSS on startup**: `static/css/theme-vars.css` is now regenerated from the database every time the server starts, preventing blank styles after a deploy or container restart

### New Content Endpoints
- **`PUT /api/v1/content/by-path?path=/slug`**: Update content by URL path instead of MongoDB ID — useful when you know the page URL but not its ID
- **`POST /api/v1/content/batch-publish`**: Publish multiple content items in one call; pass an ID list or `publish_all_drafts: true`
- **`GET/POST /api/v1/content/{id}/preview`**: Render a content item's HTML without publishing; accepts optional title/data overrides to preview unsaved edits; returns `rendered_html` and `warnings` (missing required fields, unclosed tags, unresolved placeholders)
- **`GET /api/v1/content/{id}?include_rendered=true`**: Include rendered body HTML and validation warnings alongside regular content data

### New Asset Endpoints
- **`POST /api/v1/assets/from-url`**: Fetch a remote URL and store it as a LightCMS asset (HTTP/HTTPS only, 50 MB cap, MIME validation)

### Theme Version Pinning
- **`POST /api/v1/theme/versions/{version}/pin`** and **`/unpin`**: Lock (or unlock) a theme version to protect it from being auto-pruned
- `ThemeVersion` model gains `locked` field (stored in `theme_versions` collection)

### Scoped Search & Replace
- **`POST /api/v1/search-replace/scoped/preview`** and **`/execute`**: Run search-and-replace filtered by `content_ids`, `folder_path`, `template_name`, and/or `category` — safer than a full site-wide replacement

### New MCP Tools (13 added, 41 → 54 total)
- `update_content_by_path` — update by URL path; merges `data` fields like `update_content`
- `publish_multiple` — batch publish by ID list or all drafts at once
- `preview_content` — render HTML without saving; supports field overrides
- `scoped_search_replace_preview` / `scoped_search_replace_execute` — folder/template/category-scoped S&R
- `upload_asset_from_url` — fetch remote file and store as asset
- `pin_theme_version` / `unpin_theme_version` — protect important theme milestones
- Improved descriptions on `get_content`, `create_content`, `update_content`, `update_theme`, `search_replace_preview`, `search_replace_execute` — all now include workflow guidance and examples

---

## v2.0.1 — Configurable Search Ranking

### Configurable Search Ranking
- All search ranking parameters are now stored in the database (`settings` collection, `type=search_config`) and editable from the admin panel at **Tools → End User Search → Search Ranking**
- Previously hardcoded values (nav boost 0.15, title boost 0.20, concept template boost 0.05, `/videos/` demotion −0.05) are now the defaults that ship with every new install
- Configurable fields: title match boost, nav page boost, boosted template name substrings (one per line), template boost score, demoted path prefixes (one per line), demotion penalty score
- Values are clamped to −1.0…1.0 to prevent accidental misconfiguration
- Changes take effect immediately — in-memory cache is invalidated on save
- Save confirmation banner shown after successful update

### Search API Documentation
- Expanded typeahead suggest API documentation in the admin Integration Guide, including full parameter table, response schema, and a combined search+suggest JavaScript example
- Updated README with full Search API and Suggest API reference sections including JavaScript example

---

## v2.0.0 — Multi-User RBAC & Smart Search

### Multi-User Access Control
- **Role-Based Access Control (RBAC)**: Three roles — admin, editor, viewer — with granular permissions enforced on every admin UI page and REST API endpoint
- **User Management**: Admin-only panel at `/cm/users` for creating users, assigning roles, disabling accounts, and resetting passwords
- **Email-based login**: Authentication migrated from a single shared password to per-user email + password credentials
- **Force password change**: Temporary passwords prompt a mandatory change on first login
- **Automatic migration**: On first startup with an empty users collection, the existing admin password hash is carried over into a new admin user account

### Audit Logging
- **Persistent audit trail**: All mutations logged with acting user, action, resource, and timestamp
- **365-day retention**: Audit logs auto-expire via MongoDB TTL index
- **Filterable UI** at `/cm/audit`: filter by action type, resource, and date range
- **Async logging**: `LogAsync` fire-and-forget pattern to avoid blocking request handlers

### User-Scoped API Keys
- API keys now belong to a specific user and inherit that user's permissions
- Admins can view and manage all keys; non-admins can only manage their own
- Keys created before v2.0 remain functional as system-level keys with full access

### Smart Search Ranking
- **Structural boost**: Nav-linked pages (parsed from header HTML, cached 5 min) surface above other results
- **Template-based ranking**: Concept pages rank above generic body-only content
- **Video deprioritisation**: Pages under `/videos/` rank below all other content types
- **Typeahead suggestions**: Same structural ranking applied to prefix-match suggestions
- Ranking priority: title+nav > title-only > nav-linked > concept pages > body-only > video transcripts

### Bug Fixes
- Fixed `/cm/audit` page crash caused by `subtract`/`add` template functions receiving mismatched integer types
- Made arithmetic template functions (`subtract`, `add`) type-flexible via `interface{}` dispatch

---

## v1.4.0 — End-User Search API

- **Full-text search** (`/api/search?q=...`): regex-based exact matching across all published content
- **Semantic vector search**: Voyage AI embeddings stored in MongoDB Atlas; `$vectorSearch` pipeline for similarity queries
- **Hybrid mode**: Reciprocal rank fusion (RRF, k=60) merges full-text and semantic results into a single ranked list
- **Title boosting**: Results where the query appears in the page title float above body-only matches
- **Graceful degradation**: Works without a Voyage API key (full-text only); automatically enables semantic search when configured
- **Rate limiting**: Per-IP (10 req/min) and global (100 req/min) limits with DDoS protection
- **Embedding pipeline**: Background batch generation with progress tracking in the admin panel
- **Typeahead suggestions**: `/api/search/suggest` endpoint for prefix-matching page titles and extracted keywords
- **WARP proxy**: Voyage API calls routed through Cloudflare WARP on Fly.io to avoid IP-based rate limiting
- Upgraded embedding model from `voyage-3-lite` to `voyage-4-lite`
- Fixed SVG assets not displaying when uploaded with `/assets` path prefix
- Expanded upload allowlist to include CSS, JS, JSON, and other text-based web assets

---

## v1.2.0 — OAuth 2.1 & HTTP MCP Transport

- **OAuth 2.1 authorization server**: Full authorization code flow with PKCE (S256), dynamic client registration (RFC 7591), token rotation, and revocation (RFC 7009)
- **Remote MCP clients**: HTTP streamable MCP endpoint at `/mcp` — connects Claude's Cowork, Claude Desktop, and any MCP-compatible app without embedding credentials
- **Discovery endpoints**: `/.well-known/oauth-authorization-server` (RFC 8414) and `/.well-known/oauth-protected-resource` (RFC 9728) for automatic client setup
- **Dynamic MCP server card**: `/.well-known/mcp/server-card.json` with full tool schemas, served live from the running server
- **Smithery registry support**: `smithery.yaml` and packaging config for registry publication
- **Test suite**: 82% → 86% coverage with CI via GitHub Actions and Codecov integration
- Loading state feedback on OAuth authorize buttons

---

## v1.1.0 — REST API, CLI Tool & API Keys

- **REST API** at `/api/v1/`: full JSON API for all content management operations — content, templates, assets, theme, config, redirects, folders, collections
- **API key authentication**: `lc_`-prefixed keys stored as SHA-256 hashes; created and managed in the admin panel
- **CLI tool** (`cmd/cli`): command-line interface wrapping the REST API for scripting and CI/CD workflows
- **MCP refactor**: MCP tools now use the REST API client (`internal/apiclient`) rather than direct DB access
- Partial update support on all PUT endpoints (send only changed fields)

---

## v1.0.0 — Initial Release

- **Content management**: Create, edit, publish, and delete content using customizable templates
- **Template system**: Define reusable page structures with typed fields (text, textarea, richtext, date, image, select); HTML layout with `{{.field_name}}` placeholders
- **Static page generation**: Published content rendered to `content/generated/` for fast, zero-runtime serving
- **Content versioning**: Automatic version snapshots on every update; revert to any prior version
- **Soft delete**: Deleted content recoverable from the admin panel
- **Content collections**: Auto-generated paginated listing pages filtered by category
- **Folders & URL organization**: Hierarchical path structure for content
- **MCP server** (stdio): 43 tools for managing the entire site through AI agents (Claude Code, Claude Desktop)
- **Theme customization**: Colors, fonts, border radius, custom CSS; header/footer HTML injection
- **Theme versioning**: Full history of theme changes with one-click revert
- **Asset management**: Upload, organize, and serve images, documents, and other files
- **URL redirects**: 301/302 rules managed from the admin panel
- **Rich text editor**: TinyMCE integration for visual editing
- **Search & replace**: Site-wide text replacement with preview before execution
- **Admin branding**: Custom logo and site name in the admin panel
- **Security**: CSRF protection, bcrypt passwords (cost 12), session cookies (SameSite=Strict), file upload validation, login rate limiting
- **Fly.io deployment**: `fly.toml` and `Dockerfile` for one-command production deploy
- Deployed at https://metavert-cms.fly.dev/
