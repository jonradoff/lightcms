package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"lightcms/internal/apiclient"
)

func (a *App) runTemplate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("template subcommand required: list, get, create, update, delete")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		templates, err := a.client.ListTemplates(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(templates)
		}

		w := table()
		printRow(w, "ID", "NAME", "SLUG", "CATEGORY", "FIELDS", "SYSTEM")
		for _, t := range templates {
			sys := ""
			if t.IsSystem {
				sys = "yes"
			}
			printRow(w, t.ID, t.Name, t.Slug, t.Category, len(t.Fields), sys)
		}
		return w.Flush()

	case "get":
		if len(args) < 2 {
			return fmt.Errorf("template ID or slug required")
		}
		tmpl, err := a.client.GetTemplate(ctx, args[1])
		if err != nil {
			return err
		}
		return printJSON(tmpl)

	case "create":
		fs := flag.NewFlagSet("template create", flag.ExitOnError)
		name := fs.String("name", "", "Template name (required)")
		slug := fs.String("slug", "", "Template slug (required)")
		desc := fs.String("description", "", "Description")
		category := fs.String("category", "", "Category")
		fieldsJSON := fs.String("fields", "[]", "Fields as JSON array")
		layoutFile := fs.String("layout-file", "", "HTML layout file path")
		layout := fs.String("layout", "", "HTML layout string")
		fs.Parse(args[1:])

		if *name == "" || *slug == "" {
			return fmt.Errorf("--name and --slug required")
		}

		var fields []apiclient.TemplateField
		if err := json.Unmarshal([]byte(*fieldsJSON), &fields); err != nil {
			return fmt.Errorf("invalid --fields JSON: %w", err)
		}

		htmlLayout := *layout
		if *layoutFile != "" {
			data, err := readStdinOrFile(*layoutFile)
			if err != nil {
				return fmt.Errorf("failed to read layout file: %w", err)
			}
			htmlLayout = string(data)
		}

		req := apiclient.CreateTemplateRequest{
			Name:        *name,
			Slug:        *slug,
			Description: *desc,
			Category:    *category,
			Fields:      fields,
			HTMLLayout:  htmlLayout,
		}

		tmpl, err := a.client.CreateTemplate(ctx, req)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(tmpl)
		}
		fmt.Printf("Created template '%s' (ID: %s)\n", tmpl.Name, tmpl.ID)
		return nil

	case "update":
		fs := flag.NewFlagSet("template update", flag.ExitOnError)
		id := fs.String("id", "", "Template ID (required)")
		name := fs.String("name", "", "New name")
		slug := fs.String("slug", "", "New slug")
		desc := fs.String("description", "", "New description")
		category := fs.String("category", "", "New category")
		fieldsJSON := fs.String("fields", "", "Fields as JSON array")
		layoutFile := fs.String("layout-file", "", "HTML layout file path")
		layout := fs.String("layout", "", "HTML layout string")
		fs.Parse(args[1:])

		if *id == "" && fs.NArg() > 0 {
			*id = fs.Arg(0)
		}
		if *id == "" {
			return fmt.Errorf("--id required")
		}

		updates := make(apiclient.UpdateTemplateRequest)
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
		if *fieldsJSON != "" {
			var fields []apiclient.TemplateField
			if err := json.Unmarshal([]byte(*fieldsJSON), &fields); err != nil {
				return fmt.Errorf("invalid --fields JSON: %w", err)
			}
			updates["fields"] = fields
		}
		if *layoutFile != "" {
			data, err := readStdinOrFile(*layoutFile)
			if err != nil {
				return fmt.Errorf("failed to read layout file: %w", err)
			}
			updates["html_layout"] = string(data)
		} else if *layout != "" {
			updates["html_layout"] = *layout
		}

		tmpl, err := a.client.UpdateTemplate(ctx, *id, updates)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(tmpl)
		}
		fmt.Printf("Updated template '%s' (ID: %s)\n", tmpl.Name, tmpl.ID)
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("template ID required")
		}
		if err := a.client.DeleteTemplate(ctx, id); err != nil {
			return err
		}
		fmt.Println("Template deleted successfully")
		return nil

	default:
		return fmt.Errorf("unknown template subcommand: %s", args[0])
	}
}
