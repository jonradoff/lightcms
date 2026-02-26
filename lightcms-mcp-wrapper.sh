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
