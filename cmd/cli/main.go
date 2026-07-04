// Command lightcms-cli is the command-line client for administering a LightCMS instance over its REST API.
package main

import (
	"fmt"
	"os"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"
	"github.com/jonradoff/lightcms/v7/internal/cli"
)

func main() {
	// Parse global flags before subcommand
	url := ""
	apiKey := ""
	jsonOutput := false

	// Scan for global flags and strip them from args
	args := os.Args[1:]
	var cleanArgs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url":
			if i+1 < len(args) {
				url = args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			cleanArgs = append(cleanArgs, args[i])
		}
	}

	// Fall back to environment variables
	if url == "" {
		url = os.Getenv("LIGHTCMS_URL")
	}
	if apiKey == "" {
		apiKey = os.Getenv("LIGHTCMS_API_KEY")
	}
	if url == "" {
		url = "http://localhost:8082"
	}

	// Handle version command (no API key needed)
	if len(cleanArgs) > 0 && cleanArgs[0] == "version" {
		fmt.Println("lightcms CLI v1.0.0")
		os.Exit(0)
	}

	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: API key is required\n")
		fmt.Fprintf(os.Stderr, "Set LIGHTCMS_API_KEY or use --api-key flag\n")
		os.Exit(1)
	}

	client := apiclient.New(url, apiKey)
	app := cli.New(client, jsonOutput)

	if err := app.Run(cleanArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`lightcms - LightCMS command-line tool

Usage:
  lightcms [flags] <command> <subcommand> [args]

Global Flags:
  --url <url>        Server URL (default: $LIGHTCMS_URL or http://localhost:8082)
  --api-key <key>    API key (default: $LIGHTCMS_API_KEY)
  --json             Output as JSON

Commands:
  content     Manage content items
  template    Manage templates
  asset       Manage assets
  theme       Manage theme settings
  config      Manage site configuration
  redirect    Manage URL redirects
  folder      Manage content folders
  collection  Manage content collections
  search      Search content
  search-replace  Search and replace across content
  api-key     Manage API keys
  regenerate  Regenerate all published content
  version     Show version information

Use "lightcms <command> --help" for more information about a command.`)
}
