package cli

import (
	"context"
	"flag"
	"fmt"
)

func (a *App) runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	searchType := fs.String("type", "fulltext", "Search type: fulltext or name")
	includeDeleted := fs.Bool("deleted", false, "Include deleted content")
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("search query required")
	}
	query := fs.Arg(0)

	ctx := context.Background()
	result, err := a.client.SearchContent(ctx, query, *searchType, *includeDeleted)
	if err != nil {
		return err
	}

	if a.json {
		return printJSON(result)
	}

	fmt.Printf("Found %d results for '%s'\n\n", result.Total, result.Query)
	w := table()
	printRow(w, "ID", "TITLE", "PATH", "PUBLISHED", "MATCHED IN")
	for _, m := range result.Matches {
		pub := "draft"
		if m.Published {
			pub = "published"
		}
		matched := ""
		for i, f := range m.MatchedIn {
			if i > 0 {
				matched += ", "
			}
			matched += f
		}
		printRow(w, m.ID, m.Title, m.FullPath, pub, matched)
	}
	return w.Flush()
}

func (a *App) runSearchReplace(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("search-replace subcommand required: preview, execute")
	}

	ctx := context.Background()

	switch args[0] {
	case "preview":
		fs := flag.NewFlagSet("search-replace preview", flag.ExitOnError)
		search := fs.String("search", "", "Text to search for (required)")
		replace := fs.String("replace", "", "Text to replace with (required)")
		fs.Parse(args[1:])

		if *search == "" || *replace == "" {
			return fmt.Errorf("--search and --replace required")
		}

		result, err := a.client.SearchReplacePreview(ctx, *search, *replace)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(result)
		}

		fmt.Printf("Preview: '%s' → '%s'\n", *search, *replace)
		fmt.Printf("Total matches: %d across %d pages\n\n", result.TotalMatches, result.AffectedPages)

		if len(result.Matches) > 0 {
			w := table()
			printRow(w, "ID", "TITLE", "PATH", "MATCHES")
			for _, m := range result.Matches {
				printRow(w, m.ID, m.Title, m.FullPath, m.MatchCount)
			}
			w.Flush()
		}

		fmt.Println("\nUse 'search-replace execute' to apply changes.")
		return nil

	case "execute":
		fs := flag.NewFlagSet("search-replace execute", flag.ExitOnError)
		search := fs.String("search", "", "Text to search for (required)")
		replace := fs.String("replace", "", "Text to replace with (required)")
		comment := fs.String("comment", "", "Version comment")
		fs.Parse(args[1:])

		if *search == "" || *replace == "" {
			return fmt.Errorf("--search and --replace required")
		}

		result, err := a.client.SearchReplaceExecute(ctx, *search, *replace, *comment)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(result)
		}

		fmt.Printf("Replaced '%s' → '%s'\n", *search, *replace)
		fmt.Printf("Total replacements: %d across %d pages\n", result.TotalReplacements, result.PagesUpdated)
		return nil

	default:
		return fmt.Errorf("unknown search-replace subcommand: %s", args[0])
	}
}
