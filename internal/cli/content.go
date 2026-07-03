package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jonradoff/lightcms/v6/internal/apiclient"
)

func (a *App) runContent(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("content subcommand required: list, get, create, update, delete, publish, unpublish, restore, versions, revert")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("content list", flag.ExitOnError)
		includeDeleted := fs.Bool("deleted", false, "Include soft-deleted content")
		category := fs.String("category", "", "Filter by category")
		folderID := fs.String("folder", "", "Filter by folder ID")
		fs.Parse(args[1:])

		contents, err := a.client.ListContent(ctx, *includeDeleted, *category, *folderID)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(contents)
		}

		w := table()
		printRow(w, "ID", "TITLE", "PATH", "PUBLISHED", "UPDATED")
		for _, c := range contents {
			pub := "draft"
			if c.Deleted {
				pub = "deleted"
			} else if c.Published {
				pub = "published"
			}
			printRow(w, c.ID, c.Title, c.FullPath, pub, c.UpdatedAt.Format("2006-01-02 15:04"))
		}
		return w.Flush()

	case "get":
		fs := flag.NewFlagSet("content get", flag.ExitOnError)
		id := fs.String("id", "", "Content ID")
		path := fs.String("path", "", "Content path")
		fs.Parse(args[1:])

		// Also accept positional arg as ID
		if *id == "" && *path == "" && fs.NArg() > 0 {
			*id = fs.Arg(0)
		}

		var content *apiclient.Content
		var err error
		if *id != "" {
			content, err = a.client.GetContent(ctx, *id, false)
		} else if *path != "" {
			content, err = a.client.GetContentByPath(ctx, *path, false)
		} else {
			return fmt.Errorf("--id or --path required")
		}
		if err != nil {
			return err
		}

		return printJSON(content)

	case "create":
		fs := flag.NewFlagSet("content create", flag.ExitOnError)
		templateID := fs.String("template", "", "Template ID (required)")
		title := fs.String("title", "", "Content title (required)")
		slug := fs.String("slug", "", "URL slug (required)")
		folderPath := fs.String("folder", "", "Folder path")
		category := fs.String("category", "", "Category")
		published := fs.Bool("publish", false, "Publish immediately")
		useHeader := fs.Bool("header", false, "Include site header")
		useFooter := fs.Bool("footer", false, "Include site footer")
		useTheme := fs.Bool("theme", false, "Apply site theme")
		rawMode := fs.Bool("raw", false, "Raw HTML mode")
		dataStr := fs.String("data", "{}", "Field data as JSON")
		comment := fs.String("comment", "", "Version comment")
		fs.Parse(args[1:])

		if *templateID == "" || *title == "" || *slug == "" {
			return fmt.Errorf("--template, --title, and --slug are required")
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(*dataStr), &data); err != nil {
			return fmt.Errorf("invalid --data JSON: %w", err)
		}

		req := apiclient.CreateContentRequest{
			TemplateID:     *templateID,
			Title:          *title,
			Slug:           *slug,
			FolderPath:     *folderPath,
			Category:       *category,
			Data:           data,
			Published:      *published,
			UseHeader:      *useHeader,
			UseFooter:      *useFooter,
			UseTheme:       *useTheme,
			RawMode:        *rawMode,
			VersionComment: *comment,
		}

		content, err := a.client.CreateContent(ctx, req)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(content)
		}
		fmt.Printf("Created content '%s' (ID: %s, path: %s)\n", content.Title, content.ID, content.FullPath)
		return nil

	case "update":
		fs := flag.NewFlagSet("content update", flag.ExitOnError)
		id := fs.String("id", "", "Content ID (required)")
		title := fs.String("title", "", "New title")
		slug := fs.String("slug", "", "New slug")
		templateID := fs.String("template", "", "New template ID")
		folderPath := fs.String("folder", "", "New folder path")
		category := fs.String("category", "", "New category")
		dataStr := fs.String("data", "", "Field data as JSON (merged with existing)")
		comment := fs.String("comment", "", "Version comment")
		fs.Parse(args[1:])

		if *id == "" && fs.NArg() > 0 {
			*id = fs.Arg(0)
		}
		if *id == "" {
			return fmt.Errorf("--id required")
		}

		updates := make(apiclient.UpdateContentRequest)
		if *title != "" {
			updates["title"] = *title
		}
		if *slug != "" {
			updates["slug"] = *slug
		}
		if *templateID != "" {
			updates["template_id"] = *templateID
		}
		if *folderPath != "" {
			updates["folder_path"] = *folderPath
		}
		if *category != "" {
			updates["category"] = *category
		}
		if *comment != "" {
			updates["version_comment"] = *comment
		}
		if *dataStr != "" {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(*dataStr), &data); err != nil {
				return fmt.Errorf("invalid --data JSON: %w", err)
			}
			// Merge with existing data
			existing, err := a.client.GetContent(ctx, *id, false)
			if err != nil {
				return err
			}
			merged := existing.Data
			if merged == nil {
				merged = make(map[string]interface{})
			}
			for k, v := range data {
				merged[k] = v
			}
			updates["data"] = merged
		}

		content, err := a.client.UpdateContent(ctx, *id, updates)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(content)
		}
		fmt.Printf("Updated content '%s' (ID: %s)\n", content.Title, content.ID)
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("content ID required")
		}
		if err := a.client.DeleteContent(ctx, id); err != nil {
			return err
		}
		fmt.Println("Content deleted successfully")
		return nil

	case "publish":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("content ID required")
		}
		if err := a.client.PublishContent(ctx, id); err != nil {
			return err
		}
		fmt.Println("Content published successfully")
		return nil

	case "unpublish":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("content ID required")
		}
		if err := a.client.UnpublishContent(ctx, id); err != nil {
			return err
		}
		fmt.Println("Content unpublished successfully")
		return nil

	case "restore":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("content ID required")
		}
		if err := a.client.RestoreContent(ctx, id); err != nil {
			return err
		}
		fmt.Println("Content restored successfully")
		return nil

	case "versions":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("content ID required")
		}

		versions, err := a.client.GetContentVersions(ctx, id)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(versions)
		}

		w := table()
		printRow(w, "VERSION", "TITLE", "COMMENT", "CREATED")
		for _, v := range versions {
			printRow(w, v.Version, v.Title, v.Comment, v.CreatedAt.Format("2006-01-02 15:04"))
		}
		return w.Flush()

	case "revert":
		fs := flag.NewFlagSet("content revert", flag.ExitOnError)
		id := fs.String("id", "", "Content ID (required)")
		version := fs.Int("version", 0, "Version to revert to (required)")
		comment := fs.String("comment", "", "Revert comment")
		fs.Parse(args[1:])

		if *id == "" || *version == 0 {
			return fmt.Errorf("--id and --version required")
		}

		if err := a.client.RevertContentVersion(ctx, *id, *version, *comment); err != nil {
			return err
		}
		fmt.Printf("Content reverted to version %d\n", *version)
		return nil

	default:
		return fmt.Errorf("unknown content subcommand: %s", args[0])
	}
}

// getIDArg extracts an ID from remaining args (first positional arg)
func getIDArg(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// readStdinOrFile reads content from stdin if "-", otherwise from a file path
func readStdinOrFile(path string) ([]byte, error) {
	if path == "-" {
		return os.ReadFile("/dev/stdin")
	}
	return os.ReadFile(path)
}
