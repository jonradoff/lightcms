# Changelog

All notable changes to LightCMS are documented here, organized by version.

---

## [7.0.6] - 2026-07-04

### Added
- Copilot can answer analytics questions: new get_analytics tool exposes most popular pages, top referrers, and a traffic summary (DAU/MAU/uptime), defaulting to human traffic.
- Copilot chat shows an animated typing indicator with progressive status messages during long operations instead of static text.

### Fixed
- Analytics queries from the copilot include the current hour (the hour-bucket upper bound is exclusive).

---

## [7.0.5] - 2026-07-04

### Fixed
- Agent-session rollback could "revert" a page to the session's own version when the version write landed before the session's audit entry (timing race, caught by CI). Rollback now selects targets by version provenance — a session's own versions are never rollback targets — with timestamp comparison only as a fallback for pre-provenance versions.

---

## [7.0.4] - 2026-07-04

### Fixed
- Copilot chat requests failed CSRF validation ("Invalid or missing CSRF token"): the page template pre-quoted the token with printf %q inside a script block, but html/template already quotes values in JS context, so the browser sent a token wrapped in literal quote characters. Regression test renders through the real CSRF middleware.

---

## [7.0.3] - 2026-07-04

### Fixed
- WebSite JSON-LD now also reaches raw-HTML homepages (Blank Page templates with the theme disabled), injected into the authored <head>. The 7.0.1 change only covered themed homepages.

---

## [7.0.2] - 2026-07-04

### Added
- Admin sidebar shows the running software version under the logo (read from build.json, which is now kept in sync with releases).

---

## [7.0.1] - 2026-07-04

### Added
- Homepage now emits schema.org WebSite JSON-LD (name, tagline, URL) with a SearchAction advertising the public search endpoint — previously the homepage was the one page without structured data.

---

## [7.0.0] - 2026-07-04

### Added — the agentic release
- **llms.txt & llms-full.txt**: auto-generated Markdown site index and full plain-text content for AI crawlers (llmstxt.org proposal), plus schema.org JSON-LD (BlogPosting/NewsArticle/WebPage) on all published pages.
- **Fork-sandboxed agent sessions ("PRs for content")**: MCP tools `start_agent_sandbox` / `get_agent_sandbox` / `end_agent_sandbox`. While active, all agent content writes copy-on-write into a fork; publishing, deleting, and bulk operations are blocked; humans review the per-field diff (`GET /api/v1/forks/{id}/diff`, `get_fork_diff`) and merge.
- **Agent governance**: API keys accept a `scopes` permission allowlist and a `sandbox_only` flag (server-enforced fork-only writes). Every MCP session gets an agent-session ID; `GET /api/v1/agent-sessions/{id}/changes` lists everything a session changed and `POST .../rollback` undoes it as a unit.
- **Change provenance**: content versions record actor (human|agent), via (ui|api|copilot), and agent session. API edits now carry version attribution.
- **Admin copilot** (`/cm/copilot`): natural-language content editing via an Anthropic tool-use loop with per-tool RBAC and audit logging.
- **Self-hosted embeddings**: `LIGHTCMS_EMBEDDINGS_PROVIDER=ollama` generates semantic-search embeddings locally (default model nomic-embed-text) — no hosted API required.
- **Public read-only MCP endpoint** (`/mcp-public`): visitors' agents can search_site / get_page / list_pages / get_site_info with no authentication; drafts and forks are structurally excluded.
- **Maintenance scans**: daily site-health reports (stale pages, missing meta descriptions, drafts, optional broken-link job) via API and MCP tools.

### Fixed
- Editing an unpublished fork copy no longer deletes the live page's static file; fork content can no longer overwrite live static files or pollute the embedding index.

---

## [6.1.1] - 2026-03-31

### Security
- **Search/replace input validation**: Pairs array capped at 100 per request. Search and replace text capped at 100K characters. Regex patterns capped at 1,000 characters to prevent ReDoS.
- **Legacy API keys deprecated**: API keys without an associated user now return 403 instead of bypassing all permission checks. Keys must be re-created with a user owner.
- **Script policy enforced at write time**: Bulk create and bulk update operations now sanitize content data fields (bluemonday) when the script policy is `admin_only` or `none`, as defense-in-depth alongside the existing render-time enforcement.
- **Bulk field operation field validation**: System fields (`_id`, `published`, `template_id`, `full_path`, etc.) are blocked from modification via the bulk field operation endpoint.
- **Per-page locking in concurrent operations**: Search/replace, bulk update, and bulk field operations now use per-page mutual exclusion to prevent race conditions when multiple workers target the same content item.
- **Error message sanitization**: Bulk operation error responses no longer expose MongoDB internals, field names, or collection structure. Detailed errors are logged server-side only.
- **Enhanced audit logging**: Bulk create operations now include a sample of created content IDs (up to 10) in the audit trail.

---

## [6.1.0] - 2026-03-31

### Added
- **Bulk Create Content**: New `bulk_create_content` MCP tool and `POST /api/v1/content/bulk-create` endpoint. Creates up to 100 content items in a single call using MongoDB `InsertMany` (unordered — partial failures don't abort the batch). Published items get parallel HTML generation (10 concurrent goroutines). Returns per-item `{id, full_path, success, error}` results.
- **Content Upsert**: `create_content` and `bulk_create_content` now accept `upsert: true`. If a page already exists at the same path, it updates instead of returning a duplicate key error. Returns `{action: "created"}` or `{action: "updated"}`. Eliminates the most common failure mode in retry scenarios.
- **Multi-Pair Search & Replace**: `search_replace_preview` and `search_replace_execute` (and scoped variants) now accept a `pairs` array of `{search, replace, regex}` objects. All pairs are applied in a single pass per page — O(pages) instead of O(pairs x pages). Critical for operations like fixing hundreds of broken links simultaneously.
- **Content List Pagination**: `list_content` now supports `limit` (1-500) and `offset` query parameters. When `limit` is set, returns `{items, total, limit, offset, has_more}` envelope. Default behavior (no limit) unchanged for backward compatibility.
- **Burst Rate Limiting**: New per-token 20 requests/second burst limiter on all `/api/v1/` endpoints. Returns HTTP 429 with `Retry-After: 1` header. Complements the existing 300 req/min sliding window. Prevents runaway scripts from overwhelming the server.
- **Site Analytics Dashboard**: New analytics section at `/cm/analytics` with hourly visitor chart (HTML bar chart with hover tooltips), uptime monitoring strip, top pages by views, top referrers (with Non-bot/Bots/All tabs), browser/device donut chart. Per-page analytics at `/cm/analytics/page` with referrer breakdown. Referrer drill-down report at `/cm/analytics/referrer` showing top pages from each source.
- **Analytics Tab on Content Editor**: New "Analytics" tab in the content edit page bottom panel showing 7-day and 30-day view counts and top referrers for that specific page.
- **Page View Tracking**: Every public page view records page path, referrer source (direct/internal/external domain), and browser category. Data buffered in memory and flushed to MongoDB every 30 seconds for minimal performance impact.

### Changed
- **Search/Replace now returns counts**: Both preview and execute responses include `pages_scanned`, `pages_modified` (execute), `total_replacements`, and per-pair summaries in multi-pair mode.
- **Conditional Republish**: Static HTML generation now uses SHA-256 content hashing. Pages whose rendered output hasn't changed are skipped during regeneration. `RegenerateAllContent` clears all hashes first to force full regeneration when theme/template/snippet changes affect output globally.
- **Deploy script hardened**: Orphan machine cleanup now catches machines in any state (created, starting, stopped), not just created.

### Fixed
- **Analytics data not recording**: The `site_stats` collection couldn't be created due to MongoDB Atlas's 500-collection limit. Hourly stats now stored in the existing `user_activity` collection using a sentinel document pattern.
- **Analytics chart empty**: JavaScript time format mismatch (Go's ISO omits zero milliseconds, JS `toISOString()` always includes them) caused the hour-keyed lookup to never match. Fixed by using epoch-millisecond keys.
- **Analytics write buffer failing silently**: The per-page referrer key used `\x00` (null byte) as a separator, which BSON rejects in field names. Changed to `||` separator. This bug caused ALL buffered writes (page views, referrers, user agents) to silently fail since they shared a single `$inc` update.
- **Uptime strip current hour**: The most recent hour in the uptime strip now always shows green (the server is by definition online if you're viewing the page).

---

## [6.0.2] - 2026-03-24

### Added
- **`GET /api/v1/users` endpoint**: Returns a minimal user list (id, email, display_name, role) for UI features like @mention autocomplete and workflow approver selection. Requires `content.view` permission.
- **Workflow creation UI**: Admin approvals page now includes an inline "New Workflow" form. Search users by email to add approvers, choose trigger type (all_contributor / folder_path / template_id / tag), set mode (sequential / concurrent), and create the workflow without leaving the page.
- **API attribution in audit log**: Audit log entries created via API key now have `via_api: true` stored in the database. The audit log display shows "(api)" next to the username for API-originated actions. Session-based (browser) actions are unaffected.

### Fixed
- **Admin UI calls to `/api/v1/` now authenticated via session**: The API auth middleware now accepts session cookies as a fallback when no `Authorization: Bearer` header is present. This fixes all admin UI interactions that call `/api/v1/` endpoints directly from browser JavaScript (posting comments, approving/rejecting requests, deleting workflows) — they were returning "Missing authorization header" for session-authenticated users.
- **Contributor role missing from user dropdowns**: The Create User and Edit User forms now include the Contributor role option.
- **Approvals moved to Content nav section**: The "Approvals" link in the admin sidebar is now under the Content section (alongside Content, Templates, Forks, Imports) instead of the Inbox section.
- **Broken `Bearer ` token in mention user search**: The @mention user autocomplete was sending an empty Bearer token, causing a 401. It now relies on session cookie fallback.

---

## [6.0.1] - 2026-03-24

### Security
- **CSRF key hardening**: CSRF protection key is now derived via SHA-256 hash of the session secret instead of zero-padding, eliminating low-entropy padding.
- **APIGetSiteConfig auth**: GET `/api/v1/config` now requires `settings.view` permission. The Cloudflare API token is redacted (`***`) in the response — it is write-only via the admin UI.
- **OAuth system key user context**: The system API key used for OAuth-authenticated MCP sessions is now associated with an admin user. OAuth sessions previously bypassed all permission checks via a nil-user path; they now properly resolve to admin-level RBAC.
- **Self-approval blocked**: Users can no longer approve their own content submissions.
- **Sequential workflow order enforced**: Only the approver assigned to the current step in a sequential workflow may advance it.
- **Workflow approver validation**: Approver user IDs are validated against real users when creating or updating approval workflows.
- **Bulk export gated**: `APIExportContent` now requires `content.edit` permission (was unauthenticated).
- **20+ read-only API endpoints now require login**: All `/api/v1/` read endpoints (content list/get/versions/search/preview/backlinks, template list/get, asset list/get/folders, snippet list/get, redirect list/get, folder list/get, collection list/get, theme get/versions, config get, comment list) now require at minimum viewer-level credentials. Public end-user endpoints (`/api/search`, `/api/chat`, `/api/contact`) are unchanged.
- **Comment rate limiting**: `POST /api/v1/content/{id}/comments` is now rate-limited to 20 requests/minute per API token.
- **Comment length limit**: Comment text is capped at 10,000 characters.
- **Comment XSS fix**: `user_display_name` and `user_email` are now HTML-escaped before injection into dynamically created comment DOM nodes. Mention dropdown also escaped.
- **`APIForceUnlockContent` RBAC**: Replaced hardcoded `role == "admin"` string check with `requirePermission(PermUserManage)`.
- **Sidebar approval badge efficiency**: `renderAdmin` now calls `CountPending()` (single count query) instead of `ListPending()` (full result set) for the badge count on every admin page render.

---

## [6.0.0] - 2026-03-24

### Added
- **Content Approvals**: Configurable approval workflows for content publishing. Contributors can save drafts and submit for approval; editors and admins approve or reject from the new `/cm/approvals` dashboard. Rejection comments are auto-posted to the content's discussion thread.
- **Contributor role**: New RBAC role between Viewer and Editor. Contributors can create content and upload assets (held in a pending queue), post discussion comments, and submit content for approval — but cannot publish directly or manage templates, snippets, theme, or settings.
- **Approval workflows**: Configurable trigger-based workflows (by contributor, folder path, template ID, or tag). Supports sequential mode (approvers in order) and concurrent mode (any N-of-M). Default behavior (no workflow) allows any editor or admin to approve.
- **Content Discussion tab**: Inline threaded comment section at the bottom of every content edit page. Supports @mention autocomplete. Admins can delete any comment. Live badge count on the tab.
- **Tabbed bottom section on content edit page**: Discussion, Version History, and Forks are now organized into tabs instead of stacked plain sections.
- **Approvals Dashboard** (`/cm/approvals`): Shows "My Queue" (requests where the current user is an approver) and "Other Pending". Approve/reject directly from the page with a required rejection comment. Approval badge count shown in the sidebar nav.
- **Dashboard: Requiring Approvals section**: When pending approval requests exist, a summary appears on the admin dashboard.
- **Dashboard: Recent Comments section**: Five most recent discussion comments with links to the relevant content pages.
- **Webhook events**: Three new event types — `comment.created`, `content.pending_approval`, `asset.pending_review`. Subscribable from the webhook form and documented in the reference table.
- **MCP tools** (14 new tools): `list_comments`, `create_comment`, `delete_comment`, `list_approval_workflows`, `get_approval_workflow`, `create_approval_workflow`, `update_approval_workflow`, `delete_approval_workflow`, `list_approval_requests`, `get_approval_request`, `submit_for_approval`, `approve_request`, `reject_request`, `cancel_approval_request`.
- **Messages mark-all-read**: "Mark All as Read" button on the contact messages list page.

### Changed
- MCP server version updated to 6.0.0.

---

## [5.0.0] - 2026-03-24

### Added
- **Import Pipeline**: Three new import types with a unified job dashboard at `/cm/imports`:
  - **RSS/Atom Import**: Configure recurring feed sources with schedule (hourly/daily/weekly), template mapping, folder targeting, and auto-publish. Full run history and per-job logs.
  - **Markdown Import**: Upload single `.md` files or `.zip` archives of Markdown. YAML frontmatter controls title, slug, folder, template, tags, and scheduling. Supports Notion exports, Obsidian vaults, Hugo/Jekyll migrations, and AI-generated content.
  - **CSV Import**: Upload CSV files and map columns to content fields. Specify the title column; all other columns become fields automatically.
- **Real-time import status**: SSE-powered live log stream at `/cm/imports/{jobID}`. Watch imports happen line-by-line or review full history after the fact.
- **MCP import tools** (10 new tools): `list_import_sources`, `create_import_source`, `update_import_source`, `delete_import_source`, `trigger_import_source`, `import_markdown`, `import_csv`, `list_import_jobs`, `get_import_job`, `cancel_import_job`.
- **Agentic bulk content creation**: `import_markdown` is specifically designed for AI agents to generate and import large batches of content in a single call. See `MCP.md` for workflow examples.
- **MCP.md**: New developer guide for MCP/agentic workflows with bulk import examples.
- **Deduplication**: Imports match by `full_path` — re-importing the same slug updates rather than duplicating.
- **Import job TTL**: Log entries auto-expire after 90 days.

---

## [4.5.0] - 2026-03-24

### Added
- **Webhooks**: HMAC-SHA256 signed webhook events for content lifecycle (publish, unpublish, delete, create, update). Configurable per-webhook with retry logic and delivery history. Admin UI at `/cm/webhooks`.
- **Scheduled Publishing**: Set a future `publish_at` timestamp on any content item. Background scheduler publishes automatically.
- **Content Locking**: Advisory lock when editing content (30-minute expiry). Warning banner if another user is already editing. Force unlock for admins.
- **Incremental Static Regeneration**: Template changes now regenerate content in batches of 20 instead of all-at-once, preventing server overload on large sites.
- **Edge Caching Headers**: ETag, Cache-Control, Last-Modified, and Vary headers on all public pages. Conditional request support (304 Not Modified).
- **Cloudflare Integration**: Configure Zone ID and API Token to automatically purge Cloudflare cache when content is published or unpublished.
- **Structured JSON Logging**: All server logs now emit structured JSON with timestamp, level, message, and context fields.
- **Rate Limit Dashboard**: New tab on the audit log page showing locked IPs, attempt counts, and a clear button.
- **MCP Prompt Resources**: Three new MCP resources — `lightcms://site/structure`, `lightcms://content/recent`, `lightcms://theme/config` — exposing live site data for AI context.

---

## v4.2.0 — AI Chat Widget, Security Hardening, Fork MCP Tools & Performance Overhaul

### AI Chat Widget

An embeddable AI-powered chat widget that lets site visitors ask questions in natural language and receive answers synthesized from the site's own content.

- **Two-phase query pipeline**: Every question first runs a hybrid semantic + full-text search (reusing the same `SearchService` used by the MCP `end_user_search` tool) to retrieve the most relevant content excerpts. Those excerpts are then passed to Claude Haiku for streaming synthesis into a conversational answer.
- **Server-sent events (SSE) streaming**: The `/api/chat/query` endpoint responds as a live SSE stream, forwarding Haiku's token-by-token output to the browser in real time. Falls back to plain JSON for clients that don't support SSE.
- **Graceful AI-optional mode**: When no `ANTHROPIC_API_KEY` is configured, the widget still works — it returns ranked search results with excerpts, skipping the synthesis step. Sites without an API key get a useful search-in-chat experience.
- **Fully configurable from the admin UI**: A new "Chat Widget" admin page exposes all settings — enabled/disabled toggle, widget title, welcome message, placeholder text, primary color, position (bottom-left / bottom-right), max results, and editable system/user prompt templates with `{siteName}`, `{excerpts}`, and `{question}` placeholders.
- **Embeddable JS widget** (`/static/js/chat-widget.js`): A self-contained floating chat bubble that fetches its config from `/api/chat/config` and posts queries to `/api/chat/query`. Add to any page with a single `<script>` tag.
- **Dedicated rate limiting**: Chat endpoints use separate per-IP and global rate limits (default: 5/min per IP, 30/min global), independent from the search and API rate limiters. Anthropic API calls are metered separately from ordinary search traffic.
- **Source attribution**: Every response includes a `sources` SSE event listing the content pages whose excerpts were used, with titles and paths.

### Security

- **CORS lockdown**: Chat widget endpoints (`/api/chat/*`) now restrict `Access-Control-Allow-Origin` to the site's configured `BASE_URL` instead of `*`. Falls back to `*` only when `BASE_URL` is unset.
- **Prompt injection defense**: User query text in the chat widget is wrapped in `<user_question>...</user_question>` XML delimiters before being interpolated into the Anthropic prompt. The `</` sequence is escaped to prevent tag injection.
- **Prompt template validation**: Saved chat widget system/user prompts are validated to only allow known placeholders (`{siteName}`, `{question}`, `{excerpts}`). Unknown placeholders are rejected at save time.
- **Configurable upload size limit**: `MaxUploadBytes` added to `SiteConfig`, settable via the admin Configuration page and the `update_site_config` API. Defaults to 1 MiB (2× the largest asset stored at time of release). Enforced via `http.MaxBytesReader` on both the file upload endpoint and the asset upload endpoint.
- **API body size limit**: All `/api/v1/` endpoints now enforce a 10 MiB request body cap via `APIBodySizeLimit` middleware, preventing memory exhaustion from oversized payloads.
- **Fly.io IP spoofing fix**: `TrustedProxyConfig` adds a `TrustFlyProxy` mode that reads `Fly-Client-IP` (set exclusively by Fly.io's edge proxy) instead of `X-Forwarded-For`, which can be set by anyone. `DefaultCloudConfig()` now uses this mode, preventing rate-limit and audit-log bypass.
- **Session secret entropy**: Production deployments now hard-fail on startup if `SESSION_SECRET` is shorter than 32 characters (previously 16).
- **Per-endpoint rate limiters**: New limiters protect expensive endpoints beyond the global 300 req/min cap — regenerate (2/min), search-replace execute (10/min), asset-from-url (10/min), bulk-update (5/min), export (5/min), reindex-embeddings (1/min).
- **Rate limiter map pruning**: A background goroutine prunes stale token entries from all rate limiter maps every 5 minutes, preventing unbounded memory growth over long-running deployments.

### Fork MCP Tools (8 new tools)

Full MCP and REST API coverage for the fork workspaces system introduced in v4.0.0:

- **`list_forks`** — List all forks with status and page count.
- **`create_fork`** — Create a named fork workspace.
- **`get_fork`** — Retrieve fork details including merge conflicts from last merge.
- **`fork_page`** — Add a page to a fork (accepts content ID or path); returns fork page ID for use with `update_content`.
- **`remove_fork_page`** — Remove a page from a fork (reverts to live content on preview).
- **`merge_fork`** — Merge all fork changes into live content (admin only).
- **`archive_fork`** — Archive a fork without merging.
- **`delete_fork`** — Permanently delete a fork and all its page copies.

### Performance

- **Admin template caching**: Admin HTML templates are compiled once at startup (via `sync.Once`) and cached. Previously each admin page request re-parsed the full template from source — eliminated entirely.
- **Content list pagination**: The admin Content page now loads 100 items at a time with Previous/Next controls and a total count. Previously it loaded the entire `content` collection into memory, which OOM'd the server on large sites.
- **Content indexes**: Added `{updated_at: -1}` and compound `{fork_id, deleted, updated_at}` indexes to fix `Sort exceeded memory limit` errors on the MongoDB Atlas free tier when the collection grew past ~32 MiB.
- **Search/replace streaming**: All four search/replace handlers (global preview, global execute, scoped preview, scoped execute) now stream documents one at a time via cursor iteration instead of loading the entire collection into a Go slice. Memory is bounded regardless of collection size. Coverage is complete — every document is still checked, nothing is truncated.
- **New `StreamContent` / `StreamContentScoped`** service methods return raw `*mongo.Cursor` for callers that need bounded-memory processing.
- **`QueryContentForDirective` cap**: `lc:query` template directives now apply a 10,000-item `SetLimit` cap, preventing a single index page from OOM-ing the server on a large site.
- **Wikilink index cache**: `buildWikilinkIndex` caches results for 60 seconds (TTL), invalidated immediately on any title, path, or publish-status change. Previously a full `content` collection scan ran for every page publish — on bulk operations this was O(n) scans for n pages.
- **`UpdateWikilinksOnRename` streaming**: Changed from `FindAll` (full load into slice) to streaming cursor iteration, bounding memory when many pages reference a renamed item.
- **Export scope push-down**: `APIExportContent` now uses `ListContentScoped` to push all filters (template, category, folder, content IDs) to MongoDB, instead of loading all content then filtering in Go.

### Database (earlier v4.2.0 work)

- **New indexes**: Added `settings.type`, `login_attempts.ip`, `theme_versions.version`, and `content.plain_text` (text index) to eliminate collection scans on hot query paths.
- **`ListAssets` projection**: Excludes the binary `data` field from asset listing queries, dramatically reducing wire transfer for asset list operations.
- **Atomic login rate limiting**: `RecordFailedLogin` replaced a read-then-write pattern with `FindOneAndUpdate` + `$inc`, eliminating a race condition under concurrent login attempts.

### Search (earlier v4.2.0 work)

- **Parallel hybrid search**: `SearchHybrid` now runs `SearchFullText` and `SearchSemantic` concurrently via goroutines, halving latency when both sources are available.
- **`sort.Slice` everywhere**: All ranking insertion sorts in `SearchFullText`, `Suggest`, and `RebuildKeywords` replaced with `sort.Slice` / `sort.SliceStable` for O(n log n) behaviour on larger result sets.
- **Pre-normalized search config**: `getSearchConfig` lowercases and trims all path/template lists once on cache load, removing redundant per-query `strings.ToLower`/`strings.TrimSpace` calls.

### Content Regeneration (earlier v4.2.0 work)

- **Parallel `RegenerateAllContent`**: Sequential per-page loop replaced with a semaphore-bounded goroutine pool (6 workers), enabling concurrent static page generation.
- **Single wikilink index per bulk regen**: `buildWikilinkIndex` is called once before the worker pool starts and shared across all workers.
- **Targeted `UpdateWikilinksOnRename`**: `$regex` pre-filter limits the scan to documents that likely reference the old title/path.

---

## v4.1.0 — Bug Fixes

### Bug Fixes

- **Redirect deletion now takes effect immediately** — Redirects were served with `Cache-Control: max-age=3600`, causing browsers to cache redirect responses for up to an hour. Deleting a redirect had no effect until the browser cache expired. Changed to `Cache-Control: no-store` so browsers never cache redirects and always re-check with the server.

---

## v4.0.0 — Content Forks

This release adds fork workspaces for safe page experimentation, plus bulk operation resilience improvements and content search refinements.

### Content Forks

- **Fork workspaces** — Editors and admins can create isolated staging workspaces ("forks") where sets of page changes can be authored, previewed, and reviewed before going live. Only touched pages live in a fork (sparse model — unmodified pages fall through to live).
- **Fork preview** — Activate fork preview via a floating purple bar injected into the live site. A `lc_fork_preview` cookie routes page requests through the fork, showing exactly how the site will look after merge.
- **Merge with conflict detection** — Admins merge forks into live content. If a live page was edited after the fork was created, the conflict is recorded in the merge result (fork wins). Newly created fork pages are inserted as live content on merge.
- **Permission gating** — Creating and editing forks requires at least `editor` role; merging requires `admin`.
- **"Fork to workspace" button** — Available on any content edit page; opens the forks list with a one-click "Add to this fork" action.
- **Fork exclusion from content list** — Fork copies are invisible in the main admin content list and search results.

### Performance & Resilience

- **Coalescing background workers** — `RegenerateIndexPages` and `RebuildKeywords` now use debounce channels: a bulk update of 25 items triggers one regeneration pass instead of 25 concurrent goroutines. Eliminates the goroutine explosion that caused server timeouts during large bulk operations.

### Content Search

- **Slug search mode** — New search option alongside Title Only and Full Text. Searches slug and full_path fields, sorted alphabetically by slug.
- **Homepage always first** — The site index page (`/`) is always pinned to position 0 in the content list and all search modes. For slug search, the homepage is explicitly fetched and prepended if not already in results.
- **"/" query fix** — Searching `/` in Title or Full Text mode now correctly includes the homepage (stored internally with empty slug/path).

### Dashboard

- Removed Collections KPI stat card.

### Bug Fixes

- **Fork page unique index** — Replaced the single-field `full_path` unique index with a compound `{full_path, fork_id}` unique index, allowing fork copies to share paths with their live counterparts without constraint violations.

---

## v3.3.0 — Bulk Operation Performance

This release significantly improves the performance of all bulk and batch operations, with particular gains at scale (hundreds of content items).

### Performance improvements

- **Scope filters pushed to MongoDB** — `bulk_field_operation`, `scoped_search_replace_execute`, and `export_content` now push `template_name`, `category`, `folder_path`, and `content_ids` filters down to MongoDB instead of loading all content and filtering in Go. For a 600-item site running a scoped operation on 100 pages, this eliminates ~500 unnecessary document transfers.
- **Goroutine worker pool (10 concurrent)** — All four bulk/search-replace execute handlers now process items concurrently with a pool of 10 workers instead of sequentially. On a 100-item batch this yields up to 8–10× faster wall time.
- **Batch content fetch in `bulk_update_content`** — Per-item `GetContent` calls replaced with a single `$in` query that fetches all requested IDs at once. 100-item bulk update: 100 DB round-trips → 1 for the initial fetch.
- **`content_versions` index on `content_id`** — Previously missing; every `UpdateContent` call triggered a full collection scan on `content_versions` for the version count. This index makes versioning O(log n) per item.
- **`template_name` index on `content`** — Enables the new MongoDB-level template scope filter to use an index scan instead of a full collection scan.
- **`InsertMany` helper** — Added `db.InsertMany` for future bulk version insert use.

### New service methods

- `ContentService.ListContentScoped(ctx, ContentScope)` — MongoDB-native scoped list with support for template_name, category, folder_path, and content_ids filters.
- `ContentService.GetContentByIDs(ctx, []ObjectID)` — Batch fetch multiple content items in one `$in` query, returning a map keyed by ID.

---

## v3.2.0 — Healthz Endpoint & DAU/MAU Analytics

This release adds a structured health check endpoint following the vibectl VibeCtl Health Check Protocol, plus user activity tracking for DAU/MAU metrics surfaced in both the health endpoint and the admin dashboard.

### `/healthz` Endpoint

- New `GET /healthz` endpoint — unauthenticated, returns JSON following the [vibectl VibeCtl Health Check Protocol](https://github.com/jonradoff/vibectl):
  - `status`: `healthy` / `degraded` / `unhealthy` derived from dependency checks
  - `name`: `"LightCMS"` — software identifier
  - `version`: current build version (e.g. `"3.2.0"`)
  - `uptime`: seconds since process start
  - `dependencies`: live MongoDB ping check
  - `kpis`: DAU, MAU, and content pages created today
- Returns HTTP 503 when status is `unhealthy`
- The existing `/health` endpoint (plain-text `OK` for Fly.io TCP probes) is unchanged

### Analytics & KPIs

- New `AnalyticsService` backed by `user_activity` MongoDB collection
- Activity recorded automatically on admin login and API key authentication (fire-and-forget goroutine, no request latency impact)
- Storage: upsert on `{user_id, date}` — at most one document per user per calendar day, O(1) writes
- **DAU** (Daily Active Users): distinct users active today (UTC)
- **MAU** (Monthly Active Users): distinct users active in the last 30 calendar days (aggregation pipeline)
- **Content created today**: content items with `created_at ≥ midnight UTC` (no additional tracking needed — uses existing field)

### Admin Dashboard

- Three new stat cards added to the dashboard: Daily Active Users, Monthly Active Users, Pages Created Today

---

## v3.1.0 — Bulk Operations, Wiki-Like Markup & Security Hardening

This release adds first-class support for bulk content operations (eliminating N×1 API call patterns), a full wiki-like markup system for content authoring, and comprehensive security fixes.

### Bulk Content Operations

- **`bulk_update_content`**: Update up to 100 content items in a single MCP/API call. Each item uses merge semantics — only supplied fields change. Supports `clear_fields`, `dry_run` validation, and returns per-item success/error details.
- **`bulk_field_operation`**: Apply a single operation (`clear`, `set`, `prepend`, `append`, `wrap`) to a field across all matching pages in one call. Scoped by template, folder, category, or content IDs. Supports `dry_run`.
- **`export_content`**: Export content items with full field data as a structured JSON array. Designed for the export → transform → `bulk_update_content` pipeline. Scope filters supported.
- **`list_content` with field data**: New `include_data: true` and `include_fields: ["field1"]` parameters return full template field values in list results, eliminating per-item `get_content` calls for bulk read workflows.
- **`clear_fields` on `update_content`**: Explicitly set fields to empty string — removes ambiguity about merge semantics.
- **`dry_run` on `update_content` and `bulk_update_content`**: Validate payloads without committing changes.
- **Concurrency guidance**: Tool descriptions document that up to 20 concurrent `update_content` calls are safe; larger batches should use `bulk_update_content`.

### Regex Search & Replace

- All four search/replace tools (`search_replace_preview`, `search_replace_execute`, `scoped_search_replace_preview`, `scoped_search_replace_execute`) now support `"regex": true`.
- When enabled, `search` is treated as a Go RE2 regular expression. Use `$1`, `$2` for capture group references in `replace`.
- Input validation: patterns capped at 500 characters, max 20 capture groups, 10× expansion guard on replacement.

### Wiki-Like Markup System

- **Wikilinks**: `[[Page Title]]`, `[[Page Title|display text]]`, `[[/path]]`, `[[/path|display text]]` syntax in any content field. Resolves to `<a>` at page generation time; broken links render as `<span class="broken-link">`. Links auto-update when a page's title or path changes.
- **Snippet includes**: `[[include:snippet-name]]` embeds a named snippet inline in any content field. Recursion depth limit (3) and cycle detection prevent infinite expansion.
- **Table of contents**: Add `{{.lc_toc}}` anywhere in a template's HTML layout to auto-generate a `<nav class="lc-toc">` from the page's headings.
- **Heading IDs**: All `<h1>`–`<h6>` tags automatically get `id=` attributes derived from their text content, enabling deep-linking.
- **Markdown field type**: Template fields can be set to type `markdown`. Field values are converted through goldmark (GitHub Flavored Markdown) at page generation time. Supports tables, strikethrough, task lists, autolinks.
- **Inline `#tag` detection**: Mentioning `#tagname` in any content field automatically adds that tag to the page's tag list, which feeds into `lc:query` index pages.
- **`get_backlinks` MCP tool**: Returns all published pages that link to a given path via wikilinks or `<a>` tags. Useful for impact assessment before renaming/deleting pages.
- **Version history user attribution**: The "By" column in version history now shows the email of the editor who made each change.
- **Admin search by slug/path**: The admin content search now matches on slug and full URL path in addition to title and content.

### Configurable Script Policy

- New site config setting `markdown_script_policy` controls who may use raw `<script>` tags and unsafe HTML in content fields:
  - `"all"` (default) — all users may use scripts; backward-compatible with existing content
  - `"admin_only"` — admin-authored content passes through unchanged; editor-authored content is sanitized via bluemonday (strips `<script>`, `<iframe>`, event handlers, `javascript:` URIs; allows all other HTML)
  - `"none"` — all content sanitized regardless of author role
- Configurable via the admin Settings page or via the `update_site_config` MCP tool.

### Security Fixes

- **Export authorization**: `export_content` now requires authentication (previously unauthenticated)
- **Regex input validation**: Pattern length cap (500 chars), capture group limit (20), and expansion guard prevent regex-based DoS
- **Snippet recursion guard**: Depth limit of 3 and cycle detection prevent infinite expansion chains
- **Version history permissions**: `get_content_versions` and `get_content_version` now enforce `PermContentView`
- **Folder path scope bug**: Scoped search/replace with `folder_path: "/blog"` no longer accidentally matches `/blog-old`, `/blog-archive`, etc.
- **Bulk field op limit**: `bulk_field_operation` returns 400 if the operation would affect more than 500 pages (prevents unbounded database writes per request)
- **Wikilink href safety**: Resolved links that don't start with `/` are rendered as broken-link spans rather than potentially-unsafe hrefs
- **TOC HTML escaping**: Heading IDs in TOC anchor `href` attributes are now properly HTML-escaped
- **Rate limiter cleanup**: Background goroutine prunes stale entries every 10 minutes to prevent unbounded memory growth

---

## v3.0.0 — Dynamic Index Pages: Tags, Snippets & `lc:query`

This release adds a first-class system for building dynamic, automatically-updated index pages — the most significant content modelling capability added since v1.0.

### Content Tagging

- **Tags field on all content items**: any content item can now carry zero or more freeform string labels
- Tags are set in the admin editor (below the main fields) or via `{"tags": [...]}` in the REST API
- Tags are exact-match strings; capitalization and spacing are preserved
- Full REST API support: `GET /api/v1/content?tag=TAGNAME` filters by tag; `PUT /api/v1/content/{id}` accepts a `tags` array
- Tags are indexed in MongoDB for efficient filtering

### Snippets

- **New `snippets` collection**: named HTML template fragments with Go template variable support
- **Admin UI** at `/cm/snippets`: create, edit, and delete snippets with a live editor
- **REST API**: `GET/POST /api/v1/snippets`, `GET/PUT/DELETE /api/v1/snippets/{id}`
- **Available variables in snippets**: `{{.Title}}`, `{{.FullPath}}`, `{{.Slug}}`, `{{.MetaDescription}}`, `{{.PublishedAt}}`
- Snippets are rendered through Go's `html/template`, so struct fields are HTML-escaped by default — safe from XSS

### `lc:query` Directives

- **Embed live content queries in template layouts** using HTML comment syntax:
  ```html
  <!-- lc:query filter="tag:TAGNAME" sort="title:asc" snippet="snippet-name" -->
  ```
- Directives are processed at page generation time before Go's template engine runs, then replaced with the rendered output of all matching published content items
- Supports `filter="tag:X"`, `filter="category:X"`, `filter="template:X"`, `filter="folder:X"` (and shorthand `tag="X"` etc.)
- Sort options: `title:asc`, `title:desc`, `created_at:asc`, `created_at:desc`
- Multiple directives per template — each is an independent query
- Falls back to a plain `<a href>` link if no snippet is specified

### Automatic Cascade Regeneration

- When a tagged content item is published or updated, all index pages whose templates contain `lc:query` directives are automatically regenerated — no manual rebuild needed
- Regeneration also triggers when a template layout or snippet is updated
- `POST /api/v1/regenerate` triggers a full-site regeneration (unchanged behaviour, now also covers `lc:query` pages)

### MCP Tools (5 new, 54 → 59 total)

- `list_snippets` — list all snippets
- `get_snippet` — retrieve a snippet by ID or name
- `create_snippet` — create a new named HTML snippet
- `update_snippet` — update a snippet's name or HTML
- `delete_snippet` — delete a snippet

### Security Fix

- **XSS in `lc:query` default fallback**: when `lc:query` had no `snippet` attribute, `item.Title` and `item.FullPath` were concatenated directly into HTML without escaping. Fixed to use `template.HTMLEscapeString()` on both values. The snippet path (which is the recommended usage) was already safe via `html/template` auto-escaping.

### Documentation

- README expanded with full documentation of Tags, Snippets, `lc:query`, and the complete walkthrough for building a tagging-powered index page
- New MCP example (Example 10): building a dynamic glossary index end-to-end
- Snippets endpoint added to REST API reference table
- Security analysis section in changelog (this entry)

---

## v2.6.0 — MCP Tool Improvements

### Bug Fixes
- **`get_theme` returned empty strings** — `ThemeSettings` struct was missing `json` struct tags; all theme fields now serialize correctly via the REST API and MCP
- **`search_replace_execute` response missing search/replace fields** — both `search_replace_execute` and `scoped_search_replace_execute` now echo back the `search` and `replace` strings used, alongside `total_replacements`, `pages_updated`, and `updated_pages`

### New MCP Feature: Rendered HTML in `get_content`
- `get_content` now accepts an `include_rendered` boolean parameter
- When `true`, the response includes a `rendered_html` field containing the fully rendered page HTML (template fields interpolated + theme header/footer applied)
- Works for both published and draft content — lets agents inspect exactly what visitors see without any publishing step
- Complements `preview_content`, which is better for previewing unsaved field overrides

---

## v2.5.0 — Security Hardening

### SSRF Prevention (Critical)
- **`upload_asset_from_url`** now uses a custom HTTP dialer that resolves the target hostname at connect time and blocks all private/reserved IP ranges before making any network request, preventing Server-Side Request Forgery (SSRF) and DNS-rebinding attacks
- Blocked ranges: loopback (127.0.0.0/8, ::1/128), RFC1918 private (10/8, 172.16/12, 192.168/16), link-local/AWS metadata (169.254.0.0/16), CGNAT (100.64.0.0/10), IPv6 ULA (fc00::/7) and link-local (fe80::/10)
- Error messages from this endpoint no longer echo internal network details back to callers

### ReDoS Prevention (Critical)
- **`SearchFullText`** now runs under a 5-second context deadline, preventing unanchored regex scans from pinning CPU on large corpora or pathological queries

### Permission Checks (High)
- **`POST /api/v1/search-replace/preview`**: now requires `PermSearchReplace` (admin-only); previously accessible to any authenticated API key
- **`POST /api/v1/search-replace/scoped/preview`**: same fix — matched the execute endpoint's permission requirement
- **`POST /api/v1/reindex-embeddings`**: now requires `PermSettingsEdit` (admin-only); previously accessible to any authenticated API key, enabling potential DoS via expensive embedding operations

### API Rate Limiting (High)
- New per-bearer-token sliding-window rate limiter applied to the entire `/api/v1/` subrouter: 300 requests per token per minute. Returns 429 with `Retry-After: 60` on violation.

### Asset serve_path Whitelist (Medium)
- `upload_asset` and `upload_asset_from_url` now enforce that `serve_path` begins with `/assets/`, `/images/`, `/docs/`, `/media/`, or `/files/`

### Trusted Proxy IP Extraction (Medium)
- Public search rate limiter now uses `middleware.GetClientIP` with the configured trusted proxy settings, preventing IP spoofing via forged `X-Forwarded-For` headers to bypass per-IP limits

### Database Indexes (Medium)
- Added compound index `{published: 1, deleted: 1}` on the `content` collection, covering the most common list query pattern and improving performance under load

### Audit Log TTL Index Idempotency (Low)
- Audit log TTL index creation now tolerates "already exists" errors, ensuring the 365-day TTL is always present even on databases created before this index was added

### Session Secret Validation (Low)
- Server now checks `SESSION_SECRET` length at startup: warns if < 32 characters, fails hard if < 16 characters in production mode

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
