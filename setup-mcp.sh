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

# Build the MCP server
echo "Building MCP server..."
cd "$SCRIPT_DIR"
go build -o bin/lightcms-mcp ./cmd/mcp
echo "  Built: bin/lightcms-mcp"

# Create the wrapper script
echo "Creating wrapper script..."
cat > "$SCRIPT_DIR/lightcms-mcp-wrapper.sh" << 'WRAPPER'
#!/bin/bash
# LightCMS MCP Wrapper
# Sets up the environment and runs the MCP server

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Set config directory so the MCP server can find config files
export LIGHTCMS_CONFIG_DIR="$SCRIPT_DIR"

# Run the MCP server
exec "$SCRIPT_DIR/bin/lightcms-mcp"
WRAPPER
chmod +x "$SCRIPT_DIR/lightcms-mcp-wrapper.sh"
echo "  Created: lightcms-mcp-wrapper.sh"

# Check if config exists
if [ ! -f "$SCRIPT_DIR/config.dev.json" ] && [ ! -f "$SCRIPT_DIR/config.prod.json" ]; then
    echo ""
    echo "Warning: No config file found!"
    echo "  Copy config.dev.json.example to config.dev.json and add your MongoDB URI."
    echo ""
fi

# Register with Claude Code
echo ""
echo "Registering MCP server with Claude Code..."
claude mcp add --transport stdio lightcms-mcp -- "$SCRIPT_DIR/lightcms-mcp-wrapper.sh" 2>/dev/null || true
echo "  Registered: lightcms-mcp"

echo ""
echo "Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Restart Claude Code (close and reopen VSCode or terminal)"
echo "  2. Run /mcp in Claude Code to verify the connection"
echo "  3. Start managing your site with natural language!"
echo ""
echo "Example commands:"
echo "  - 'List all my content'"
echo "  - 'Create a blog post about AI'"
echo "  - 'Update the site theme to use blue colors'"
