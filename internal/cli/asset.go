package cli

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"
)

func (a *App) runAsset(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("asset subcommand required: list, get, upload, delete, folders")
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("asset list", flag.ExitOnError)
		folder := fs.String("folder", "", "Filter by folder path")
		fs.Parse(args[1:])

		assets, err := a.client.ListAssets(ctx, *folder)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(assets)
		}

		w := table()
		printRow(w, "ID", "FILENAME", "SERVE_PATH", "MIME_TYPE", "SIZE")
		for _, a := range assets {
			printRow(w, a.ID, a.Filename, a.ServePath, a.MimeType, a.Size)
		}
		return w.Flush()

	case "get":
		fs := flag.NewFlagSet("asset get", flag.ExitOnError)
		id := fs.String("id", "", "Asset ID")
		path := fs.String("path", "", "Asset serve path")
		fs.Parse(args[1:])

		if *id == "" && *path == "" && fs.NArg() > 0 {
			*id = fs.Arg(0)
		}

		var asset *apiclient.AssetSummary
		var err error
		if *id != "" {
			asset, err = a.client.GetAsset(ctx, *id)
		} else if *path != "" {
			asset, err = a.client.GetAssetByPath(ctx, *path)
		} else {
			return fmt.Errorf("--id or --path required")
		}
		if err != nil {
			return err
		}

		return printJSON(asset)

	case "upload":
		fs := flag.NewFlagSet("asset upload", flag.ExitOnError)
		file := fs.String("file", "", "Local file to upload (required)")
		servePath := fs.String("path", "", "Serve path (e.g., /images/logo.png) (required)")
		desc := fs.String("description", "", "Description")
		fs.Parse(args[1:])

		if *file == "" || *servePath == "" {
			return fmt.Errorf("--file and --path required")
		}

		data, err := os.ReadFile(*file)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		req := apiclient.UploadAssetRequest{
			Filename:    filepath.Base(*file),
			ServePath:   *servePath,
			DataBase64:  base64.StdEncoding.EncodeToString(data),
			Description: *desc,
		}

		asset, err := a.client.UploadAsset(ctx, req)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(asset)
		}
		fmt.Printf("Uploaded '%s' → %s (%s, %d bytes)\n", asset.Filename, asset.ServePath, asset.MimeType, asset.Size)
		return nil

	case "delete":
		id := getIDArg(args[1:])
		if id == "" {
			return fmt.Errorf("asset ID required")
		}
		if err := a.client.DeleteAsset(ctx, id); err != nil {
			return err
		}
		fmt.Println("Asset deleted successfully")
		return nil

	case "folders":
		folders, err := a.client.ListAssetFolders(ctx)
		if err != nil {
			return err
		}

		if a.json {
			return printJSON(folders)
		}

		for _, f := range folders {
			fmt.Println(f)
		}
		return nil

	default:
		return fmt.Errorf("unknown asset subcommand: %s", args[0])
	}
}
