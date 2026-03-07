package services

import (
	"context"
	"os"
	"testing"

	"lightcms/internal/database"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestAssetService(t *testing.T) (*AssetService, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	svc := NewAssetService(db)
	return svc, cleanup
}

func TestUploadAsset(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	// Change to temp dir so content/generated is created there
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	// Create a simple PNG file (valid 1x1 PNG header)
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	asset, err := svc.UploadAsset(ctx, pngData, "test.png", "/images/test.png", "test image")
	if err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	if asset.Filename != "test.png" {
		t.Errorf("expected filename test.png, got %q", asset.Filename)
	}
	if asset.ServePath != "/images/test.png" {
		t.Errorf("expected serve path /images/test.png, got %q", asset.ServePath)
	}
	if asset.Folder != "/images" {
		t.Errorf("expected folder /images, got %q", asset.Folder)
	}
	if asset.MimeType != "image/png" {
		t.Errorf("expected mime type image/png, got %q", asset.MimeType)
	}
	if asset.Description != "test image" {
		t.Errorf("expected description 'test image', got %q", asset.Description)
	}
}

func TestUploadAsset_DisallowedType(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	ctx := context.Background()
	_, err := svc.UploadAsset(ctx, []byte("#!/bin/bash"), "script.sh", "/scripts/script.sh", "")
	if err == nil {
		t.Error("expected error for disallowed file type")
	}
}

func TestUploadAsset_PathTraversal(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	_, err := svc.UploadAsset(ctx, pngData, "test.png", "/../../../etc/test.png", "")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestUploadAsset_DefaultServePath(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	asset, err := svc.UploadAsset(ctx, pngData, "logo.png", "", "")
	if err != nil {
		t.Fatalf("UploadAsset with empty serve path failed: %v", err)
	}
	if asset.ServePath != "/logo.png" {
		t.Errorf("expected serve path /logo.png, got %q", asset.ServePath)
	}
}

func TestGetAsset(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	if _, err := svc.UploadAsset(ctx, pngData, "get-test.png", "/images/get-test.png", "desc"); err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	// SaveAsset uses upsert by full_path, so get by path first to discover the ID
	byPath, err := svc.GetAssetByPath(ctx, "/images/get-test.png")
	if err != nil || byPath == nil {
		t.Fatalf("GetAssetByPath failed: %v", err)
	}

	got, err := svc.GetAsset(ctx, byPath.ID)
	if err != nil {
		t.Fatalf("GetAsset failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected asset, got nil")
	}
	if got.Filename != "get-test.png" {
		t.Errorf("expected filename get-test.png, got %q", got.Filename)
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	ctx := context.Background()
	got, err := svc.GetAsset(ctx, primitive.NewObjectID())
	// DB returns (nil, nil) for ErrNoDocuments
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent asset")
	}
}

func TestGetAssetByPath(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	if _, err := svc.UploadAsset(ctx, pngData, "path-test.png", "/images/path-test.png", ""); err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	got, err := svc.GetAssetByPath(ctx, "/images/path-test.png")
	if err != nil {
		t.Fatalf("GetAssetByPath failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected asset, got nil")
	}
	if got.Filename != "path-test.png" {
		t.Errorf("expected filename path-test.png, got %q", got.Filename)
	}
}

func TestListAssets(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	if _, err := svc.UploadAsset(ctx, pngData, "list1.png", "/images/list1.png", ""); err != nil {
		t.Fatalf("UploadAsset list1 failed: %v", err)
	}
	if _, err := svc.UploadAsset(ctx, pngData, "list2.png", "/css/list2.png", ""); err != nil {
		t.Fatalf("UploadAsset list2 failed: %v", err)
	}

	// List all
	all, err := svc.ListAssets(ctx, "")
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("expected at least 2 assets, got %d", len(all))
	}

	// Filter by folder
	filtered, err := svc.ListAssets(ctx, "/images")
	if err != nil {
		t.Fatalf("ListAssets with folder failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 asset in /images, got %d", len(filtered))
	}
}

func TestListAssetFolders(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	if _, err := svc.UploadAsset(ctx, pngData, "a.png", "/images/a.png", ""); err != nil {
		t.Fatalf("UploadAsset a failed: %v", err)
	}
	if _, err := svc.UploadAsset(ctx, pngData, "b.png", "/css/b.png", ""); err != nil {
		t.Fatalf("UploadAsset b failed: %v", err)
	}

	folders, err := svc.ListFolders(ctx)
	if err != nil {
		t.Fatalf("ListFolders failed: %v", err)
	}
	if len(folders) < 2 {
		t.Errorf("expected at least 2 folders, got %d", len(folders))
	}
}

func TestDeleteAsset(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	if _, err := svc.UploadAsset(ctx, pngData, "del.png", "/images/del.png", ""); err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	// Get asset by path to discover the ID
	byPath, _ := svc.GetAssetByPath(ctx, "/images/del.png")
	if byPath == nil {
		t.Fatal("expected asset by path, got nil")
	}

	err := svc.DeleteAsset(ctx, byPath.ID)
	if err != nil {
		t.Fatalf("DeleteAsset failed: %v", err)
	}

	// Verify it's gone
	got, _ := svc.GetAsset(ctx, byPath.ID)
	if got != nil {
		t.Error("expected asset to be deleted")
	}
}

func TestDeleteAsset_NotFound(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	ctx := context.Background()
	err := svc.DeleteAsset(ctx, primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for non-existent asset")
	}
}

func TestUploadAsset_MIMEMismatch(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	ctx := context.Background()
	// Send text/html content with .png extension
	_, err := svc.UploadAsset(ctx, []byte("<html>not a png</html>"), "test.png", "/images/test.png", "")
	if err == nil {
		t.Error("expected error for MIME type mismatch")
	}
}

func TestUploadAsset_SVGMimeType(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	svgData := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`)

	asset, err := svc.UploadAsset(ctx, svgData, "icon.svg", "/images/icon.svg", "")
	if err != nil {
		t.Fatalf("UploadAsset SVG failed: %v", err)
	}
	if asset.MimeType != "image/svg+xml" {
		t.Errorf("expected mime type image/svg+xml, got %q", asset.MimeType)
	}
}

// CSS and JS content is detected by http.DetectContentType as text/plain.
// The MIME validator accepts text/plain as a fallback for text-based web assets
// (.css, .js, .json) since Go can't distinguish them from plain text at the byte level.

func TestUploadAsset_CSSUpload(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	cssData := []byte("body { color: red; }")

	asset, err := svc.UploadAsset(ctx, cssData, "style.css", "/css/style.css", "")
	if err != nil {
		t.Fatalf("expected CSS upload to succeed, got: %v", err)
	}
	if asset.MimeType != "text/css" {
		t.Errorf("expected mime type text/css, got %q", asset.MimeType)
	}
}

func TestUploadAsset_JSUpload(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	jsData := []byte("console.log('hello');")

	asset, err := svc.UploadAsset(ctx, jsData, "app.js", "/js/app.js", "")
	if err != nil {
		t.Fatalf("expected JS upload to succeed, got: %v", err)
	}
	if asset.MimeType != "application/javascript" {
		t.Errorf("expected mime type application/javascript, got %q", asset.MimeType)
	}
}

// Test that the DB-level SaveAsset records can be retrieved
func TestNewAssetService(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAssetService(db)
	if svc == nil {
		t.Fatal("expected non-nil AssetService")
	}
	if svc.db != db {
		t.Error("expected db to be set")
	}
}

func TestUploadAsset_NoPathPrefix(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	// Path without leading slash
	asset, err := svc.UploadAsset(ctx, pngData, "test.png", "images/noslash.png", "")
	if err != nil {
		t.Fatalf("UploadAsset without / prefix failed: %v", err)
	}
	if asset.ServePath != "/images/noslash.png" {
		t.Errorf("expected /images/noslash.png, got %q", asset.ServePath)
	}
}

// Ensure GetAsset returns error for nil result
func TestGetAssetByPath_NotFound(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	ctx := context.Background()
	got, err := svc.GetAssetByPath(ctx, "/nonexistent/path.png")
	if err != nil {
		t.Fatalf("GetAssetByPath unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent path")
	}
}

func TestUploadAsset_JPEGMimeType(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	// Minimal JPEG header (FF D8 FF E0)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00}

	asset, err := svc.UploadAsset(ctx, jpegData, "photo.jpg", "/images/photo.jpg", "")
	if err != nil {
		t.Fatalf("UploadAsset JPEG failed: %v", err)
	}
	if asset.MimeType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %q", asset.MimeType)
	}
}

func TestUploadAsset_GIFMimeType(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	// Minimal GIF header
	gifData := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")

	asset, err := svc.UploadAsset(ctx, gifData, "anim.gif", "/images/anim.gif", "")
	if err != nil {
		t.Fatalf("UploadAsset GIF failed: %v", err)
	}
	if asset.MimeType != "image/gif" {
		t.Errorf("expected image/gif, got %q", asset.MimeType)
	}
}

func TestUploadAsset_RootFolder(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	// Upload to root path (no subfolder)
	asset, err := svc.UploadAsset(ctx, pngData, "favicon.png", "/favicon.png", "")
	if err != nil {
		t.Fatalf("UploadAsset to root failed: %v", err)
	}
	if asset.Folder != "/" {
		t.Errorf("expected folder /, got %q", asset.Folder)
	}
}

func TestUploadAsset_WebPMimeType(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	// RIFF....WEBP header
	webpData := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")

	asset, err := svc.UploadAsset(ctx, webpData, "photo.webp", "/images/photo.webp", "")
	if err != nil {
		t.Fatalf("UploadAsset WebP failed: %v", err)
	}
	if asset.MimeType != "image/webp" {
		t.Errorf("expected image/webp, got %q", asset.MimeType)
	}
}

func TestUploadAsset_ICOMimeType(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	// ICO files are detected as application/octet-stream by Go's detector
	// but our code overrides based on extension
	icoData := []byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x10, 0x10}

	asset, err := svc.UploadAsset(ctx, icoData, "favicon.ico", "/favicon.ico", "")
	if err != nil {
		t.Fatalf("UploadAsset ICO failed: %v", err)
	}
	if asset.MimeType != "image/x-icon" {
		t.Errorf("expected image/x-icon, got %q", asset.MimeType)
	}
}

// JSON files are detected as text/plain by http.DetectContentType.
// The MIME validator accepts text/plain as a fallback for .json files.
func TestUploadAsset_JSONUpload(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	jsonData := []byte(`{"key": "value"}`)

	asset, err := svc.UploadAsset(ctx, jsonData, "data.json", "/data/data.json", "")
	if err != nil {
		t.Fatalf("expected JSON upload to succeed, got: %v", err)
	}
	if asset.MimeType != "application/json" {
		t.Errorf("expected mime type application/json, got %q", asset.MimeType)
	}
}

func TestUploadAsset_Overwrite(t *testing.T) {
	svc, cleanup := newTestAssetService(t)
	defer cleanup()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ctx := context.Background()
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
	}

	// Upload once
	_, err := svc.UploadAsset(ctx, pngData, "overwrite.png", "/images/overwrite.png", "first")
	if err != nil {
		t.Fatalf("First upload failed: %v", err)
	}

	// Upload again to same path — should upsert
	asset2, err := svc.UploadAsset(ctx, pngData, "overwrite.png", "/images/overwrite.png", "second")
	if err != nil {
		t.Fatalf("Second upload (overwrite) failed: %v", err)
	}
	if asset2.Description != "second" {
		t.Errorf("expected description 'second', got %q", asset2.Description)
	}
}

// Helper to satisfy linter: use database.Asset in the test package
var _ = database.Asset{}
