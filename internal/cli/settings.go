package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"
)

// Config

func (a *App) runConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("config subcommand required: get, update")
	}

	ctx := context.Background()

	switch args[0] {
	case "get":
		config, err := a.client.GetSiteConfig(ctx)
		if err != nil {
			return err
		}
		return printJSON(config)

	case "update":
		fs := flag.NewFlagSet("config update", flag.ExitOnError)
		titleTemplate := fs.String("title-template", "", "Page title template")
		titleTemplateNoTitle := fs.String("title-template-no-title", "", "Title template when no title")
		fs.Parse(args[1:])

		updates := make(map[string]interface{})
		if *titleTemplate != "" {
			updates["title_template"] = *titleTemplate
		}
		if *titleTemplateNoTitle != "" {
			updates["title_template_no_title"] = *titleTemplateNoTitle
		}

		config, err := a.client.UpdateSiteConfig(ctx, updates)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(config)
		}
		fmt.Println("Site config updated successfully")
		return nil

	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

// Redirects

func (a *App) runRedirect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("redirect subcommand required: list, create, update, delete")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		redirects, err := a.client.ListRedirects(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(redirects)
		}

		w := table()
		printRow(w, "ID", "FROM", "TO", "STATUS", "DESCRIPTION")
		for _, r := range redirects {
			printRow(w, r.ID, r.FromPath, r.ToPath, r.StatusCode, r.Description)
		}
		return w.Flush()

	case "create":
		fs := flag.NewFlagSet("redirect create", flag.ExitOnError)
		from := fs.String("from", "", "Source path (required)")
		to := fs.String("to", "", "Destination path (required)")
		status := fs.Int("status", 301, "Status code (301 or 302)")
		desc := fs.String("description", "", "Description")
		fs.Parse(args[1:])

		if *from == "" || *to == "" {
			return fmt.Errorf("--from and --to required")
		}

		req := apiclient.CreateRedirectRequest{
			FromPath:    *from,
			ToPath:      *to,
			StatusCode:  *status,
			Description: *desc,
		}

		redirect, err := a.client.CreateRedirect(ctx, req)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(redirect)
		}
		fmt.Printf("Created redirect %s → %s (ID: %s)\n", redirect.FromPath, redirect.ToPath, redirect.ID)
		return nil

	case "update":
		fs := flag.NewFlagSet("redirect update", flag.ExitOnError)
		id := fs.String("id", "", "Redirect ID (required)")
		from := fs.String("from", "", "Source path")
		to := fs.String("to", "", "Destination path")
		status := fs.Int("status", 0, "Status code")
		desc := fs.String("description", "", "Description")
		fs.Parse(args[1:])

		if *id == "" && fs.NArg() > 0 {
			*id = fs.Arg(0)
		}
		if *id == "" {
			return fmt.Errorf("--id required")
		}

		updates := make(map[string]interface{})
		if *from != "" {
			updates["from_path"] = *from
		}
		if *to != "" {
			updates["to_path"] = *to
		}
		if *status != 0 {
			updates["status_code"] = *status
		}
		if *desc != "" {
			updates["description"] = *desc
		}

		if _, err := a.client.UpdateRedirect(ctx, *id, updates); err != nil {
			return err
		}
		fmt.Println("Redirect updated successfully")
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("redirect ID required")
		}
		if err := a.client.DeleteRedirect(ctx, id); err != nil {
			return err
		}
		fmt.Println("Redirect deleted successfully")
		return nil

	default:
		return fmt.Errorf("unknown redirect subcommand: %s", args[0])
	}
}

// Folders

func (a *App) runFolder(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("folder subcommand required: list, create, delete")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		folders, err := a.client.ListFolders(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(folders)
		}

		w := table()
		printRow(w, "ID", "NAME", "SLUG", "PATH")
		for _, f := range folders {
			printRow(w, f.ID, f.Name, f.Slug, f.Path)
		}
		return w.Flush()

	case "create":
		fs := flag.NewFlagSet("folder create", flag.ExitOnError)
		name := fs.String("name", "", "Folder name (required)")
		slug := fs.String("slug", "", "Folder slug (required)")
		parentID := fs.String("parent", "", "Parent folder ID")
		fs.Parse(args[1:])

		if *name == "" || *slug == "" {
			return fmt.Errorf("--name and --slug required")
		}

		req := apiclient.CreateFolderRequest{
			Name:     *name,
			Slug:     *slug,
			ParentID: *parentID,
		}

		folder, err := a.client.CreateFolder(ctx, req)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(folder)
		}
		fmt.Printf("Created folder '%s' at %s (ID: %s)\n", folder.Name, folder.Path, folder.ID)
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("folder ID required")
		}
		if err := a.client.DeleteFolder(ctx, id); err != nil {
			return err
		}
		fmt.Println("Folder deleted successfully")
		return nil

	default:
		return fmt.Errorf("unknown folder subcommand: %s", args[0])
	}
}

// Collections

func (a *App) runCollection(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("collection subcommand required: list, get, create, update, delete")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		collections, err := a.client.ListCollections(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(collections)
		}

		w := table()
		printRow(w, "ID", "NAME", "SLUG", "CATEGORY")
		for _, c := range collections {
			printRow(w, c.ID, c.Name, c.Slug, c.Category)
		}
		return w.Flush()

	case "get":
		if len(args) < 2 {
			return fmt.Errorf("collection ID required")
		}
		collection, err := a.client.GetCollection(ctx, args[1])
		if err != nil {
			return err
		}
		return printJSON(collection)

	case "create":
		fs := flag.NewFlagSet("collection create", flag.ExitOnError)
		name := fs.String("name", "", "Collection name (required)")
		slug := fs.String("slug", "", "Collection slug (required)")
		desc := fs.String("description", "", "Description")
		category := fs.String("category", "", "Content category")
		sortField := fs.String("sort-field", "", "Sort field")
		sortOrder := fs.String("sort-order", "", "Sort order (asc/desc)")
		itemsPerPage := fs.Int("per-page", 0, "Items per page")
		fs.Parse(args[1:])

		if *name == "" || *slug == "" {
			return fmt.Errorf("--name and --slug required")
		}

		req := map[string]interface{}{
			"name": *name,
			"slug": *slug,
		}
		if *desc != "" {
			req["description"] = *desc
		}
		if *category != "" {
			req["category"] = *category
		}
		if *sortField != "" {
			req["sort_field"] = *sortField
		}
		if *sortOrder != "" {
			req["sort_order"] = *sortOrder
		}
		if *itemsPerPage != 0 {
			req["items_per_page"] = *itemsPerPage
		}

		collection, err := a.client.CreateCollection(ctx, req)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(collection)
		}
		fmt.Printf("Created collection '%s' (ID: %s)\n", collection.Name, collection.ID)
		return nil

	case "update":
		fs := flag.NewFlagSet("collection update", flag.ExitOnError)
		id := fs.String("id", "", "Collection ID (required)")
		name := fs.String("name", "", "New name")
		slug := fs.String("slug", "", "New slug")
		desc := fs.String("description", "", "New description")
		category := fs.String("category", "", "New category")
		sortField := fs.String("sort-field", "", "Sort field")
		sortOrder := fs.String("sort-order", "", "Sort order")
		itemsPerPage := fs.Int("per-page", 0, "Items per page")
		fs.Parse(args[1:])

		if *id == "" && fs.NArg() > 0 {
			*id = fs.Arg(0)
		}
		if *id == "" {
			return fmt.Errorf("--id required")
		}

		updates := make(map[string]interface{})
		if *name != "" {
			updates["name"] = *name
		}
		if *slug != "" {
			updates["slug"] = *slug
		}
		if *desc != "" {
			updates["description"] = *desc
		}
		if *category != "" {
			updates["category"] = *category
		}
		if *sortField != "" {
			updates["sort_field"] = *sortField
		}
		if *sortOrder != "" {
			updates["sort_order"] = *sortOrder
		}
		if *itemsPerPage != 0 {
			updates["items_per_page"] = *itemsPerPage
		}

		if _, err := a.client.UpdateCollection(ctx, *id, updates); err != nil {
			return err
		}
		fmt.Println("Collection updated successfully")
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("collection ID required")
		}
		if err := a.client.DeleteCollection(ctx, id); err != nil {
			return err
		}
		fmt.Println("Collection deleted successfully")
		return nil

	default:
		return fmt.Errorf("unknown collection subcommand: %s", args[0])
	}
}

// Regenerate

func (a *App) runRegenerate(args []string) error {
	ctx := context.Background()
	if err := a.client.RegenerateAllContent(ctx); err != nil {
		return err
	}
	fmt.Println("All published content regenerated successfully")
	return nil
}
