#!/bin/bash
# Wrapper script for lightcms-mcp that sets the config directory
# This allows the MCP server to find config files regardless of working directory

# Get the directory where this script is located (lightcms root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Export the config directory so the MCP server can find config files
export LIGHTCMS_CONFIG_DIR="$SCRIPT_DIR"

# Run the MCP server from bin/
exec "$SCRIPT_DIR/bin/lightcms-mcp" "$@"
