package cli

import (
	"context"
	"flag"
	"fmt"
)

func (a *App) runAPIKey(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("api-key subcommand required: list, create, delete")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		keys, err := a.client.ListAPIKeys(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(keys)
		}

		w := table()
		printRow(w, "ID", "NAME", "PREFIX", "DESCRIPTION", "LAST USED", "CREATED")
		for _, k := range keys {
			lastUsed := "never"
			if k.LastUsedAt != nil {
				lastUsed = k.LastUsedAt.Format("2006-01-02 15:04")
			}
			printRow(w, k.ID, k.Name, k.Prefix, k.Description, lastUsed, k.CreatedAt.Format("2006-01-02 15:04"))
		}
		return w.Flush()

	case "create":
		fs := flag.NewFlagSet("api-key create", flag.ExitOnError)
		name := fs.String("name", "", "Key name (required)")
		desc := fs.String("description", "", "Key description")
		fs.Parse(args[1:])

		if *name == "" {
			return fmt.Errorf("--name required")
		}

		result, err := a.client.CreateAPIKey(ctx, *name, *desc)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(result)
		}

		fmt.Printf("API key created successfully\n\n")
		fmt.Printf("  Name:   %s\n", result.Name)
		fmt.Printf("  Key:    %s\n", result.Key)
		fmt.Printf("  Prefix: %s\n", result.Prefix)
		fmt.Printf("\nSave this key now — it cannot be shown again.\n")
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("API key ID required")
		}
		if err := a.client.DeleteAPIKey(ctx, id); err != nil {
			return err
		}
		fmt.Println("API key deleted successfully")
		return nil

	default:
		return fmt.Errorf("unknown api-key subcommand: %s", args[0])
	}
}
