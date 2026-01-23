#!/bin/bash

# LightCMS Runner Script

# Check for config file
if [ -f config.prod.json ]; then
    echo "Using production config (config.prod.json)"
elif [ -f config.dev.json ]; then
    echo "Using development config (config.dev.json)"
else
    echo "❌ No config file found!"
    echo ""
    echo "Please create a config file:"
    echo ""
    echo "  For development: config.dev.json"
    echo "  For production:  config.prod.json"
    echo ""
    echo "See config.prod.json.example for the format."
    exit 1
fi

echo ""
echo "🚀 Starting LightCMS..."
echo ""

go run cmd/server/main.go
