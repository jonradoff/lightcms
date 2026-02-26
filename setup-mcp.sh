#!/bin/bash

# LightCMS MCP Setup Script
# This script builds the MCP server and registers it with Claude Code

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "LightCMS MCP Setup"
echo "=================="
echo ""

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Check for Claude Code CLI
if ! command -v claude &> /dev/null; then
    echo "Error: Claude Code CLI not found. Please install Claude Code first."
    echo "Visit: https://claude.ai/download"
    exit 1
fi

# Create bin directory if it doesn't exist
mkdir -p "$SCRIPT_DIR/bin"

# Build the MCP server and CLI
echo "Building MCP server..."
cd "$SCRIPT_DIR"
go build -o bin/lightcms-mcp ./cmd/mcp
echo "  Built: bin/lightcms-mcp"

echo "Building CLI tool..."
go build -o bin/lightcms ./cmd/cli
echo "  Built: bin/lightcms"

# Create the wrapper script
echo "Creating wrapper script..."
cat > "$SCRIPT_DIR/lightcms-mcp-wrapper.sh" << 'WRAPPER'
#!/bin/bash
# LightCMS MCP Wrapper
# Sets up the environment and runs the MCP server

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load API key from .env file if it exists
if [ -f "$SCRIPT_DIR/.env" ]; then
    export $(grep -v '^#' "$SCRIPT_DIR/.env" | xargs)
fi

# Run the MCP server (requires LIGHTCMS_URL and LIGHTCMS_API_KEY env vars)
exec "$SCRIPT_DIR/bin/lightcms-mcp"
WRAPPER
chmod +x "$SCRIPT_DIR/lightcms-mcp-wrapper.sh"
echo "  Created: lightcms-mcp-wrapper.sh"

# Check for API key configuration
if [ -z "$LIGHTCMS_API_KEY" ] && [ ! -f "$SCRIPT_DIR/.env" ]; then
    echo ""
    echo "Next: Create an API key for MCP access."
    echo ""
    echo "  1. Start the LightCMS server:  go run cmd/server/main.go"
    echo "  2. Log in to the admin panel:  http://localhost:8082/cm"
    echo "  3. Go to Settings → API Keys → Create New Key"
    echo "  4. Create a .env file with your key:"
    echo ""
    echo "     echo 'LIGHTCMS_URL=http://localhost:8082' > $SCRIPT_DIR/.env"
    echo "     echo 'LIGHTCMS_API_KEY=lc_your_key_here' >> $SCRIPT_DIR/.env"
    echo ""
fi

# Register with Claude Code
echo ""
echo "Registering MCP server with Claude Code..."

# Register with env vars passed through
if [ -n "$LIGHTCMS_API_KEY" ]; then
    claude mcp add --transport stdio lightcms-mcp \
        -e LIGHTCMS_URL="${LIGHTCMS_URL:-http://localhost:8082}" \
        -e LIGHTCMS_API_KEY="$LIGHTCMS_API_KEY" \
        -- "$SCRIPT_DIR/bin/lightcms-mcp" 2>/dev/null || true
else
    claude mcp add --transport stdio lightcms-mcp -- "$SCRIPT_DIR/lightcms-mcp-wrapper.sh" 2>/dev/null || true
fi
echo "  Registered: lightcms-mcp"

echo ""
echo "Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Restart Claude Code (close and reopen VSCode or terminal)"
echo "  2. Run /mcp in Claude Code to verify the connection"
echo "  3. Start managing your site with natural language!"
echo ""
echo "CLI tool is available at: bin/lightcms"
echo "  Example: LIGHTCMS_API_KEY=lc_xxx bin/lightcms content list"
