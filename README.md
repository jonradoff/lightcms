# LightCMS

A lightweight, modern content management system built with Go and MongoDB Atlas.

## Features

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
│   └── server/
│       └── main.go           # Application entry point
├── config/
│   └── config.go             # Configuration loading
├── internal/
│   ├── auth/
│   │   └── auth.go           # Authentication logic
│   ├── database/
│   │   └── mongo.go          # MongoDB connection & operations
│   ├── handlers/
│   │   ├── handlers.go       # HTTP handlers
│   │   └── admin_templates.go # Admin UI templates
│   └── models/
│       └── models.go         # Data models & default templates
├── static/
│   ├── css/
│   │   ├── main.css          # Public site styles
│   │   └── theme-vars.css    # Theme CSS variables
│   └── uploads/              # Uploaded files
├── content/
│   ├── pages/                # Custom page files
│   └── generated/            # Generated static HTML
├── config.dev.json.example   # Development config template
├── config.prod.json.example  # Production config template
└── README.md
```

## Default Templates

### Blog Post
Fields: title, excerpt, featured_image, content, author, tags

### Press Release
Fields: headline, subheadline, dateline, release_date, body, boilerplate, contact_info

### Explanatory Page
Fields: title, subtitle, hero_image, intro, main_content, sidebar, cta_text, cta_link

## Using with Claude Code (AI-Powered Content Management)

LightCMS includes an MCP (Model Context Protocol) server that allows you to manage your website content using Claude Code. Instead of navigating the admin UI, you can simply ask Claude to create pages, update content, manage assets, and more.

### Setup

1. Build the MCP server:
   ```bash
   go build -o bin/lightcms-mcp ./cmd/mcp
   ```

2. Register the MCP server with Claude Code using the wrapper script:
   ```bash
   claude mcp add --transport stdio lightcms-mcp -- /path/to/lightcms/lightcms-mcp-wrapper.sh
   ```

   The wrapper script automatically sets `LIGHTCMS_CONFIG_DIR` so the MCP server can find your config files regardless of where Claude Code runs from.

3. Restart Claude Code (or start a new session)

4. Verify the connection by running `/mcp` in Claude Code

### Alternative: Environment Variables

Instead of using config files, you can configure via environment variables:

```bash
claude mcp add --transport stdio lightcms-mcp \
  -e MONGO_URI="mongodb+srv://..." \
  -e SESSION_SECRET="your-secret" \
  -- /path/to/lightcms/bin/lightcms-mcp
```

### Configuration Options

The MCP server supports these environment variables:
- `LIGHTCMS_CONFIG_DIR` - Directory containing config.dev.json or config.prod.json
- `MONGO_URI` - MongoDB connection string (bypasses config files)
- `SESSION_SECRET` - Session encryption key (required with MONGO_URI)
- `BASE_URL` - Public site URL (optional)

### Example Commands

Once connected, you can manage your site with natural language:

- "Create a new blog post about machine learning"
- "List all my draft content"
- "Update the homepage title to 'Welcome to My Site'"
- "Delete the /old-page content"
- "Show me all templates"
- "Upload an image for my latest blog post"
- "Change the site's primary color to blue"

### Available Tools

The MCP server provides 38 tools for complete content management:

- **Content**: Create, read, update, delete, publish, unpublish, versioning
- **Templates**: Manage content templates and their fields
- **Assets**: Upload and manage images, documents, and other files
- **Settings**: Theme customization, redirects, folders, collections

## Development

```bash
# Run with hot reload (using air)
go install github.com/cosmtrek/air@latest
air

# Build for production
go build -o lightcms cmd/server/main.go

# Run production build
./lightcms
```

## Security Notes

For production:
1. Use a strong `session_secret` (generate with `openssl rand -hex 32`)
2. Change the default admin password immediately after first login
3. Use HTTPS (put behind a reverse proxy like nginx or caddy)
4. Restrict MongoDB Atlas IP whitelist to your server IPs
5. Regularly backup your MongoDB database

## License

MIT
