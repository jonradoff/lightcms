#!/bin/bash
# LightCMS MCP Wrapper
# Sets up the environment and runs the MCP server

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Set config directory so the MCP server can find config files
export LIGHTCMS_CONFIG_DIR="$SCRIPT_DIR"

# Change to the script directory so relative paths work correctly
cd "$SCRIPT_DIR"

# Run the MCP server
exec "$SCRIPT_DIR/bin/lightcms-mcp"
