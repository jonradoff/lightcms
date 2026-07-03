package cli

import (
	"context"
	"flag"
	"fmt"
)

func (a *App) runTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("theme subcommand required: get, update, versions, revert")
	}

	ctx := context.Background()

	switch args[0] {
	case "get":
		theme, err := a.client.GetTheme(ctx)
		if err != nil {
			return err
		}
		return printJSON(theme)

	case "update":
		fs := flag.NewFlagSet("theme update", flag.ExitOnError)
		siteName := fs.String("site-name", "", "Site name")
		siteTagline := fs.String("tagline", "", "Site tagline")
		logoURL := fs.String("logo", "", "Logo URL")
		primaryColor := fs.String("primary-color", "", "Primary color (hex)")
		secondaryColor := fs.String("secondary-color", "", "Secondary color (hex)")
		accentColor := fs.String("accent-color", "", "Accent color (hex)")
		bgColor := fs.String("bg-color", "", "Background color (hex)")
		textColor := fs.String("text-color", "", "Text color (hex)")
		fontFamily := fs.String("font", "", "Body font family")
		headingFont := fs.String("heading-font", "", "Heading font family")
		borderRadius := fs.String("border-radius", "", "Border radius")
		customCSS := fs.String("css", "", "Custom CSS")
		cssFile := fs.String("css-file", "", "Custom CSS file")
		headHTML := fs.String("head-html", "", "Head HTML")
		headFile := fs.String("head-file", "", "Head HTML file")
		headerHTML := fs.String("header-html", "", "Header HTML")
		headerFile := fs.String("header-file", "", "Header HTML file")
		footerHTML := fs.String("footer-html", "", "Footer HTML")
		footerFile := fs.String("footer-file", "", "Footer HTML file")
		fs.Parse(args[1:])

		updates := make(map[string]interface{})
		if *siteName != "" {
			updates["site_name"] = *siteName
		}
		if *siteTagline != "" {
			updates["site_tagline"] = *siteTagline
		}
		if *logoURL != "" {
			updates["logo_url"] = *logoURL
		}
		if *primaryColor != "" {
			updates["primary_color"] = *primaryColor
		}
		if *secondaryColor != "" {
			updates["secondary_color"] = *secondaryColor
		}
		if *accentColor != "" {
			updates["accent_color"] = *accentColor
		}
		if *bgColor != "" {
			updates["background_color"] = *bgColor
		}
		if *textColor != "" {
			updates["text_color"] = *textColor
		}
		if *fontFamily != "" {
			updates["font_family"] = *fontFamily
		}
		if *headingFont != "" {
			updates["heading_font"] = *headingFont
		}
		if *borderRadius != "" {
			updates["border_radius"] = *borderRadius
		}

		// Handle CSS - file takes priority
		if *cssFile != "" {
			data, err := readStdinOrFile(*cssFile)
			if err != nil {
				return fmt.Errorf("failed to read CSS file: %w", err)
			}
			updates["custom_css"] = string(data)
		} else if *customCSS != "" {
			updates["custom_css"] = *customCSS
		}

		// Handle head HTML
		if *headFile != "" {
			data, err := readStdinOrFile(*headFile)
			if err != nil {
				return fmt.Errorf("failed to read head HTML file: %w", err)
			}
			updates["head_html"] = string(data)
		} else if *headHTML != "" {
			updates["head_html"] = *headHTML
		}

		// Handle header HTML
		if *headerFile != "" {
			data, err := readStdinOrFile(*headerFile)
			if err != nil {
				return fmt.Errorf("failed to read header HTML file: %w", err)
			}
			updates["header_html"] = string(data)
		} else if *headerHTML != "" {
			updates["header_html"] = *headerHTML
		}

		// Handle footer HTML
		if *footerFile != "" {
			data, err := readStdinOrFile(*footerFile)
			if err != nil {
				return fmt.Errorf("failed to read footer HTML file: %w", err)
			}
			updates["footer_html"] = string(data)
		} else if *footerHTML != "" {
			updates["footer_html"] = *footerHTML
		}

		if len(updates) == 0 {
			return fmt.Errorf("no fields to update")
		}

		theme, err := a.client.UpdateTheme(ctx, updates)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(theme)
		}
		fmt.Println("Theme updated successfully")
		return nil

	case "versions":
		versions, err := a.client.ListThemeVersions(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(versions)
		}

		w := table()
		printRow(w, "VERSION", "SITE_NAME", "COMMENT", "CREATED")
		for _, v := range versions {
			printRow(w, v.Version, v.SiteName, v.Comment, v.CreatedAt.Format("2006-01-02 15:04"))
		}
		return w.Flush()

	case "revert":
		fs := flag.NewFlagSet("theme revert", flag.ExitOnError)
		version := fs.Int("version", 0, "Version to revert to (required)")
		comment := fs.String("comment", "", "Revert comment")
		fs.Parse(args[1:])

		if *version == 0 {
			return fmt.Errorf("--version required")
		}

		if err := a.client.RevertThemeVersion(ctx, *version, *comment); err != nil {
			return err
		}
		fmt.Printf("Theme reverted to version %d\n", *version)
		return nil

	default:
		return fmt.Errorf("unknown theme subcommand: %s", args[0])
	}
}
