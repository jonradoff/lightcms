# LightCMS - Claude Code Memory

## Project Overview

LightCMS is a lightweight, self-hosted content management system built in Go. It uses MongoDB for data storage and generates static HTML pages for public content serving.

**Key URLs:**
- Admin Dashboard: `/cm`
- Public Site: `/`
- MCP Server: `bin/lightcms-mcp` (stdio transport)

## MCP Server Integration

**IMPORTANT:** When the user asks about changing website content, templates, themes, assets, or site configuration, prefer using the LightCMS MCP server tools instead of making code changes. The MCP server provides 38 tools for content management operations.

### When to Use MCP Server:
- Creating, editing, publishing, or deleting content
- Managing templates and their HTML layouts
- Uploading or managing assets (images, CSS, JS, documents)
- Updating theme settings (colors, fonts, header/footer HTML)
- Managing redirects, folders, and collections
- Viewing or reverting content versions
- Site configuration changes

### When to Make Code Changes:
- Adding new features to LightCMS itself
- Fixing bugs in the application
- Changing application behavior or logic
- Adding new MCP tools
- Modifying database schemas or indexes
- Security improvements

### MCP Server Location
Binary: `bin/lightcms-mcp`
Config: Uses same `config.dev.json` or environment variables as main server

### Available MCP Tools (38 total):

**Content (12 tools):** list_content, get_content, create_content, update_content, publish_content, unpublish_content, delete_content, restore_content, get_content_versions, get_content_version, revert_to_version

**Templates (5 tools):** list_templates, get_template, create_template, update_template, delete_template

**Assets (5 tools):** list_assets, list_asset_folders, get_asset, upload_asset, delete_asset

**Settings (16 tools):** get_theme, update_theme, get_site_config, update_site_config, list_redirects, create_redirect, update_redirect, delete_redirect, list_folders, create_folder, get_folder, delete_folder, list_collections, create_collection, get_collection, update_collection, delete_collection, regenerate_all_content

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

### Content Versioning
Every content update automatically creates a version. Versions are stored in `content_versions` collection. Use `revert_to_version` to restore previous versions.

### Static Page Generation
Published content is rendered to `content/generated/{path}.html` using template HTML + theme header/footer. Regeneration happens automatically on:
- Content publish/update
- Template layout change
- Theme header/footer change

### Database Collections
- `content` - Content items with full_path (unique index)
- `content_versions` - Version history
- `templates` - Content templates with fields + HTML layout
- `folders` - Content organization hierarchy
- `collections` - Content grouping by category
- `assets` - File metadata (binary stored on filesystem)
- `redirects` - URL redirect rules
- `settings` - Theme, config, admin settings
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

## Security Notes

- CSRF protection on all `/cm` routes (Gorilla CSRF)
- Session cookies: SameSite=Strict, 24-hour expiry, Secure in production
- File uploads: Extension whitelist + MIME validation
- Path traversal protection on all file operations
- Login rate limiting: Escalating lockout (10→1min, 15→5min, 20+→15min)
- Passwords: bcrypt with cost=12

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

## Deployment

Deployed to Fly.io. Uses environment variables for configuration. Health check at `/health`.

## Testing Locally

1. Create `config.dev.json` with MongoDB Atlas connection
2. `go build -o bin/lightcms ./cmd/server`
3. `./bin/lightcms`
4. Access admin at http://localhost:8082/cm (default password: admin123)
