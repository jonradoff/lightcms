package cli

import (
	"fmt"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"
)

// App is the CLI application
type App struct {
	client *apiclient.Client
	json   bool
}

// New creates a new CLI app
func New(client *apiclient.Client, jsonOutput bool) *App {
	return &App{
		client: client,
		json:   jsonOutput,
	}
}

// Run dispatches to the appropriate subcommand
func (a *App) Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified. Run 'lightcms --help' for usage")
	}

	command := args[0]
	subArgs := args[1:]

	switch command {
	case "content":
		return a.runContent(subArgs)
	case "template":
		return a.runTemplate(subArgs)
	case "asset":
		return a.runAsset(subArgs)
	case "theme":
		return a.runTheme(subArgs)
	case "config":
		return a.runConfig(subArgs)
	case "redirect":
		return a.runRedirect(subArgs)
	case "folder":
		return a.runFolder(subArgs)
	case "collection":
		return a.runCollection(subArgs)
	case "search":
		return a.runSearch(subArgs)
	case "search-replace":
		return a.runSearchReplace(subArgs)
	case "api-key":
		return a.runAPIKey(subArgs)
	case "regenerate":
		return a.runRegenerate(subArgs)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
