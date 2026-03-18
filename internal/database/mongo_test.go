package database

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// testDB returns a shared test database connection, skipping if MONGODB_URI
// is not set. Uses the testutil-equivalent setup but lives inside the database
// package to avoid an import cycle.
func testDB(t *testing.T) *DB {
	t.Helper()
	sharedTestOnce.Do(func() {
		loadTestEnv(t)
		uri := lookupEnv("MONGODB_URI")
		if uri == "" {
			return
		}
		dbName := lookupEnv("DATABASE_NAME")
		if dbName == "" {
			dbName = "lightcms-test"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		wc := writeconcern.New(writeconcern.WMajority())
		wcOpts := options.Client().SetWriteConcern(wc)
		db, err := Connect(ctx, uri, dbName, wcOpts)
		if err != nil {
			sharedTestErr = err
			return
		}
		sharedTestDB = db
	})
	if sharedTestErr != nil {
		t.Fatalf("testDB: %v", sharedTestErr)
	}
	if sharedTestDB == nil {
		t.Skip("skipping: MONGODB_URI not set")
	}
	// Wipe collections used by this package's tests.
	ctx := context.Background()
	for _, c := range []string{"settings", "theme_versions", "login_attempts",
		"contact_messages", "assets", "redirects"} {
		sharedTestDB.database.Collection(c).DeleteMany(ctx, bson.M{})
	}
	return sharedTestDB
}

// --- Theme settings ---

func TestGetThemeSettings_Default(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	s, err := db.GetThemeSettings(ctx)
	if err != nil {
		t.Fatalf("GetThemeSettings: %v", err)
	}
	if s.PrimaryColor != "#6366f1" {
		t.Errorf("expected default primary_color #6366f1, got %s", s.PrimaryColor)
	}
	if s.SiteName != "LightCMS" {
		t.Errorf("expected default site_name LightCMS, got %s", s.SiteName)
	}
}

func TestSaveAndGetThemeSettings(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	ts := &ThemeSettings{
		PrimaryColor: "#ff0000",
		SiteName:     "TestSite",
		FontFamily:   "Helvetica",
	}
	if err := db.SaveThemeSettings(ctx, ts); err != nil {
		t.Fatalf("SaveThemeSettings: %v", err)
	}

	got, err := db.GetThemeSettings(ctx)
	if err != nil {
		t.Fatalf("GetThemeSettings after save: %v", err)
	}
	if got.PrimaryColor != "#ff0000" {
		t.Errorf("expected #ff0000, got %s", got.PrimaryColor)
	}
	if got.SiteName != "TestSite" {
		t.Errorf("expected TestSite, got %s", got.SiteName)
	}

	// Upsert — update existing
	ts.SiteName = "Updated"
	if err := db.SaveThemeSettings(ctx, ts); err != nil {
		t.Fatalf("SaveThemeSettings (update): %v", err)
	}
	got2, _ := db.GetThemeSettings(ctx)
	if got2.SiteName != "Updated" {
		t.Errorf("expected Updated, got %s", got2.SiteName)
	}
}

// --- Theme versions ---

func TestThemeVersionCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	v := &ThemeVersion{Version: 1, PrimaryColor: "#aaa", Comment: "initial"}
	if err := db.SaveThemeVersion(ctx, v); err != nil {
		t.Fatalf("SaveThemeVersion: %v", err)
	}
	if v.ID.IsZero() {
		t.Error("expected non-zero ID after save")
	}

	v2 := &ThemeVersion{Version: 2, PrimaryColor: "#bbb"}
	db.SaveThemeVersion(ctx, v2)

	versions, err := db.GetThemeVersions(ctx)
	if err != nil {
		t.Fatalf("GetThemeVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	// Should be sorted descending
	if versions[0].Version != 2 {
		t.Errorf("expected version 2 first, got %d", versions[0].Version)
	}

	got, err := db.GetThemeVersion(ctx, 1)
	if err != nil {
		t.Fatalf("GetThemeVersion(1): %v", err)
	}
	if got.PrimaryColor != "#aaa" {
		t.Errorf("expected #aaa, got %s", got.PrimaryColor)
	}

	count, err := db.GetThemeVersionCount(ctx)
	if err != nil {
		t.Fatalf("GetThemeVersionCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestSetThemeVersionLocked(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	v := &ThemeVersion{Version: 1, PrimaryColor: "#ccc"}
	db.SaveThemeVersion(ctx, v)

	if err := db.SetThemeVersionLocked(ctx, 1, true); err != nil {
		t.Fatalf("SetThemeVersionLocked: %v", err)
	}

	got, _ := db.GetThemeVersion(ctx, 1)
	if !got.Locked {
		t.Error("expected version to be locked")
	}

	db.SetThemeVersionLocked(ctx, 1, false)
	got2, _ := db.GetThemeVersion(ctx, 1)
	if got2.Locked {
		t.Error("expected version to be unlocked")
	}
}

// --- Admin settings ---

func TestGetAdminSettings_None(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	s, err := db.GetAdminSettings(ctx)
	if err != nil {
		t.Fatalf("GetAdminSettings: %v", err)
	}
	if s != nil {
		t.Error("expected nil when no admin settings exist")
	}
}

func TestSaveAndGetAdminSettings(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	as := &AdminSettings{PasswordHash: "hash123", IsDefaultPassword: true}
	if err := db.SaveAdminSettings(ctx, as); err != nil {
		t.Fatalf("SaveAdminSettings: %v", err)
	}

	got, err := db.GetAdminSettings(ctx)
	if err != nil {
		t.Fatalf("GetAdminSettings: %v", err)
	}
	if got.PasswordHash != "hash123" {
		t.Errorf("expected hash123, got %s", got.PasswordHash)
	}
	if !got.IsDefaultPassword {
		t.Error("expected is_default_password true")
	}
}

// --- Site config ---

func TestGetSiteConfig_Default(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	cfg, err := db.GetSiteConfig(ctx)
	if err != nil {
		t.Fatalf("GetSiteConfig: %v", err)
	}
	if cfg.MarkdownScriptPolicy != "all" {
		t.Errorf("expected policy 'all', got %s", cfg.MarkdownScriptPolicy)
	}
	if cfg.TitleTemplate != "{{title}} - {{site_name}}" {
		t.Errorf("unexpected title template: %s", cfg.TitleTemplate)
	}
}

func TestSaveAndGetSiteConfig(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	cfg := &SiteConfig{
		TitleTemplate:        "{{title}} | MySite",
		TitleTemplateNoTitle: "MySite",
		MarkdownScriptPolicy: "none",
	}
	if err := db.SaveSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveSiteConfig: %v", err)
	}

	got, _ := db.GetSiteConfig(ctx)
	if got.MarkdownScriptPolicy != "none" {
		t.Errorf("expected 'none', got %s", got.MarkdownScriptPolicy)
	}
}

// --- Search config ---

func TestGetSearchConfig_Default(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	cfg, err := db.GetSearchConfig(ctx)
	if err != nil {
		t.Fatalf("GetSearchConfig: %v", err)
	}
	if cfg.NavBoost != 0.15 {
		t.Errorf("expected NavBoost 0.15, got %f", cfg.NavBoost)
	}
	if cfg.TitleBoost != 0.20 {
		t.Errorf("expected TitleBoost 0.20, got %f", cfg.TitleBoost)
	}
}

func TestSaveAndGetSearchConfig(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	cfg := &SearchConfig{NavBoost: 0.5, TitleBoost: 0.3, BoostTemplates: []string{"blog"}, BoostTemplateScore: 0.1, DemotePathPrefixes: []string{"/old/"}, DemoteScore: -0.1}
	if err := db.SaveSearchConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveSearchConfig: %v", err)
	}

	got, _ := db.GetSearchConfig(ctx)
	if got.NavBoost != 0.5 {
		t.Errorf("expected 0.5, got %f", got.NavBoost)
	}
}

func TestGetSearchConfig_ZeroValues(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Save a search config with all zero values
	cfg := &SearchConfig{}
	db.SaveSearchConfig(ctx, cfg)

	got, _ := db.GetSearchConfig(ctx)
	// Should return defaults when all values are zero
	if got.NavBoost != 0.15 {
		t.Errorf("expected default NavBoost 0.15 for zero-value record, got %f", got.NavBoost)
	}
}

// --- Database version ---

func TestDatabaseVersion(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	v, err := db.GetDatabaseVersion(ctx)
	if err != nil {
		t.Fatalf("GetDatabaseVersion: %v", err)
	}
	if v != "" {
		t.Errorf("expected empty version, got %s", v)
	}

	if err := db.SetDatabaseVersion(ctx, "3.0.0"); err != nil {
		t.Fatalf("SetDatabaseVersion: %v", err)
	}

	v2, _ := db.GetDatabaseVersion(ctx)
	if v2 != "3.0.0" {
		t.Errorf("expected 3.0.0, got %s", v2)
	}

	// Update
	db.SetDatabaseVersion(ctx, "3.1.0")
	v3, _ := db.GetDatabaseVersion(ctx)
	if v3 != "3.1.0" {
		t.Errorf("expected 3.1.0, got %s", v3)
	}
}

// --- Login rate limiting ---

func TestGetLoginAttempts_NoRecord(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	a, err := db.GetLoginAttempts(ctx, "10.0.0.1")
	if err != nil {
		t.Fatalf("GetLoginAttempts: %v", err)
	}
	if a.Attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", a.Attempts)
	}
}

func TestRecordFailedLogin_Escalation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	ip := "192.168.1.100"

	// Record 9 failures — should NOT be locked
	for i := 0; i < 9; i++ {
		if err := db.RecordFailedLogin(ctx, ip); err != nil {
			t.Fatalf("RecordFailedLogin %d: %v", i, err)
		}
	}
	locked, _ := db.IsLoginLocked(ctx, ip)
	if locked {
		t.Error("should not be locked at 9 attempts")
	}

	// 10th → 1 min lock
	db.RecordFailedLogin(ctx, ip)
	locked, dur := db.IsLoginLocked(ctx, ip)
	if !locked {
		t.Error("should be locked at 10 attempts")
	}
	if dur < 50*time.Second || dur > 2*time.Minute {
		t.Errorf("expected ~1min lock, got %v", dur)
	}
}

func TestRecordFailedLogin_15and20(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	ip := "192.168.1.200"

	// Record 15 failures → 5 min lock
	for i := 0; i < 15; i++ {
		db.RecordFailedLogin(ctx, ip)
	}
	locked, dur := db.IsLoginLocked(ctx, ip)
	if !locked {
		t.Error("should be locked at 15 attempts")
	}
	if dur < 4*time.Minute || dur > 6*time.Minute {
		t.Errorf("expected ~5min lock, got %v", dur)
	}
}

func TestRecordFailedLogin_20Plus(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	ip := "192.168.1.201"

	// Record 20 failures → 15 min lock
	for i := 0; i < 20; i++ {
		db.RecordFailedLogin(ctx, ip)
	}
	locked, dur := db.IsLoginLocked(ctx, ip)
	if !locked {
		t.Error("should be locked at 20 attempts")
	}
	if dur < 14*time.Minute || dur > 16*time.Minute {
		t.Errorf("expected ~15min lock, got %v", dur)
	}
}

func TestClearLoginAttempts(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	ip := "192.168.1.101"

	for i := 0; i < 10; i++ {
		db.RecordFailedLogin(ctx, ip)
	}

	if err := db.ClearLoginAttempts(ctx, ip); err != nil {
		t.Fatalf("ClearLoginAttempts: %v", err)
	}

	locked, _ := db.IsLoginLocked(ctx, ip)
	if locked {
		t.Error("should not be locked after clear")
	}
	a, _ := db.GetLoginAttempts(ctx, ip)
	if a.Attempts != 0 {
		t.Errorf("expected 0 attempts after clear, got %d", a.Attempts)
	}
}

func TestIsLoginLocked_NotLocked(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	locked, dur := db.IsLoginLocked(ctx, "never-seen-ip")
	if locked {
		t.Error("should not be locked for unknown IP")
	}
	if dur != 0 {
		t.Errorf("expected 0 duration, got %v", dur)
	}
}

// --- Redirect ---

func TestGetRedirect_NotFound(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	r, err := db.GetRedirect(ctx, "/nonexistent")
	if err != nil {
		t.Fatalf("GetRedirect: %v", err)
	}
	if r != nil {
		t.Error("expected nil for non-existent redirect")
	}
}

func TestGetRedirect_Found(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now()
	_, err := db.Redirects().InsertOne(ctx, Redirect{
		FromPath:   "/old-page",
		ToPath:     "/new-page",
		StatusCode: 301,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("InsertOne: %v", err)
	}

	r, err := db.GetRedirect(ctx, "/old-page")
	if err != nil {
		t.Fatalf("GetRedirect: %v", err)
	}
	if r == nil {
		t.Fatal("expected redirect, got nil")
	}
	if r.ToPath != "/new-page" {
		t.Errorf("expected /new-page, got %s", r.ToPath)
	}
	if r.StatusCode != 301 {
		t.Errorf("expected 301, got %d", r.StatusCode)
	}
}

// --- Contact rate limiting ---

func TestIsContactRateLimited_Under(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	limited, err := db.IsContactRateLimited(ctx, "10.0.0.5")
	if err != nil {
		t.Fatalf("IsContactRateLimited: %v", err)
	}
	if limited {
		t.Error("should not be limited with 0 submissions")
	}
}

func TestIsContactRateLimited_AtLimit(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	ip := "10.0.0.6"

	// Insert 3 messages within the last hour
	for i := 0; i < 3; i++ {
		db.ContactMessages().InsertOne(ctx, bson.M{
			"ip_address": ip,
			"created_at": time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}

	limited, err := db.IsContactRateLimited(ctx, ip)
	if err != nil {
		t.Fatalf("IsContactRateLimited: %v", err)
	}
	if !limited {
		t.Error("should be limited at 3 submissions")
	}
}

// --- Asset operations ---

func TestSaveAndGetAsset(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	a := &Asset{
		Filename: "test.png",
		Folder:   "/images",
		FullPath: "/images/test.png",
		MimeType: "image/png",
		Size:     1024,
		Data:     []byte("fakepng"),
	}
	if err := db.SaveAsset(ctx, a); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}

	got, err := db.GetAssetByPath(ctx, "/images/test.png")
	if err != nil {
		t.Fatalf("GetAssetByPath: %v", err)
	}
	if got == nil {
		t.Fatal("expected asset, got nil")
	}
	if got.Filename != "test.png" {
		t.Errorf("expected test.png, got %s", got.Filename)
	}
	if got.Size != 1024 {
		t.Errorf("expected size 1024, got %d", got.Size)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// Get by ID
	gotByID, err := db.GetAsset(ctx, got.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if gotByID.Filename != "test.png" {
		t.Error("GetAsset returned wrong asset")
	}
}

func TestGetAssetByPath_NotFound(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	a, err := db.GetAssetByPath(ctx, "/nope.png")
	if err != nil {
		t.Fatalf("GetAssetByPath: %v", err)
	}
	if a != nil {
		t.Error("expected nil for non-existent asset")
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	a, err := db.GetAsset(ctx, primitive.NewObjectID())
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if a != nil {
		t.Error("expected nil for non-existent asset ID")
	}
}

func TestListAssets(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.SaveAsset(ctx, &Asset{Filename: "a.png", Folder: "/img", FullPath: "/img/a.png", MimeType: "image/png"})
	db.SaveAsset(ctx, &Asset{Filename: "b.css", Folder: "/css", FullPath: "/css/b.css", MimeType: "text/css"})
	db.SaveAsset(ctx, &Asset{Filename: "c.png", Folder: "/img", FullPath: "/img/c.png", MimeType: "image/png"})

	// List all
	all, err := db.ListAssets(ctx, "")
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 assets, got %d", len(all))
	}

	// Filter by folder
	imgs, err := db.ListAssets(ctx, "/img")
	if err != nil {
		t.Fatalf("ListAssets /img: %v", err)
	}
	if len(imgs) != 2 {
		t.Errorf("expected 2 /img assets, got %d", len(imgs))
	}
}

func TestDeleteAsset(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	a := &Asset{Filename: "del.png", Folder: "/", FullPath: "/del.png", MimeType: "image/png"}
	db.SaveAsset(ctx, a)

	got, _ := db.GetAssetByPath(ctx, "/del.png")
	if got == nil {
		t.Fatal("expected asset to exist")
	}

	if err := db.DeleteAsset(ctx, got.ID); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}

	deleted, _ := db.GetAsset(ctx, got.ID)
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestGetAssetFolders(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.SaveAsset(ctx, &Asset{Filename: "a.png", Folder: "/images", FullPath: "/images/a.png"})
	db.SaveAsset(ctx, &Asset{Filename: "b.css", Folder: "/css", FullPath: "/css/b.css"})
	db.SaveAsset(ctx, &Asset{Filename: "c.png", Folder: "/images", FullPath: "/images/c.png"})

	folders, err := db.GetAssetFolders(ctx)
	if err != nil {
		t.Fatalf("GetAssetFolders: %v", err)
	}
	if len(folders) != 2 {
		t.Errorf("expected 2 unique folders, got %d: %v", len(folders), folders)
	}
}

func TestSaveAsset_Upsert(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	a := &Asset{Filename: "upsert.png", Folder: "/", FullPath: "/upsert.png", MimeType: "image/png", Size: 100}
	db.SaveAsset(ctx, a)

	// Update with same full_path
	a2 := &Asset{Filename: "upsert.png", Folder: "/", FullPath: "/upsert.png", MimeType: "image/png", Size: 200}
	db.SaveAsset(ctx, a2)

	got, _ := db.GetAssetByPath(ctx, "/upsert.png")
	if got.Size != 200 {
		t.Errorf("expected size 200 after upsert, got %d", got.Size)
	}

	// Should still be only one asset with that path
	all, _ := db.ListAssets(ctx, "/")
	count := 0
	for _, asset := range all {
		if asset.FullPath == "/upsert.png" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 asset with path /upsert.png, got %d", count)
	}
}

// --- Migrate asset serve paths ---

func TestMigrateAssetServePaths(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Insert assets without serve_path
	db.Assets().InsertOne(ctx, bson.M{
		"filename":  "old.png",
		"folder":    "/img",
		"full_path": "/img/old.png",
		"mime_type": "image/png",
	})
	db.Assets().InsertOne(ctx, bson.M{
		"filename":   "new.png",
		"folder":     "/img",
		"full_path":  "/img/new.png",
		"serve_path": "/assets/img/new.png",
		"mime_type":  "image/png",
	})

	if err := db.MigrateAssetServePaths(ctx); err != nil {
		t.Fatalf("MigrateAssetServePaths: %v", err)
	}

	old, _ := db.GetAssetByPath(ctx, "/img/old.png")
	if old == nil {
		t.Fatal("expected old asset")
	}
	if old.ServePath != "/assets/img/old.png" {
		t.Errorf("expected /assets/img/old.png, got %s", old.ServePath)
	}

	// Already-migrated asset should be unchanged
	newA, _ := db.GetAssetByPath(ctx, "/img/new.png")
	if newA.ServePath != "/assets/img/new.png" {
		t.Errorf("expected /assets/img/new.png unchanged, got %s", newA.ServePath)
	}
}

// --- Generic CRUD helpers ---

func TestInsertMany_Empty(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Should be a no-op
	if err := db.InsertMany(ctx, "settings", nil); err != nil {
		t.Fatalf("InsertMany empty: %v", err)
	}
}

func TestDeleteMany(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "1.2.3.4"})
	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "1.2.3.4"})
	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "5.6.7.8"})

	deleted, err := db.DeleteMany(ctx, "contact_messages", bson.M{"ip": "1.2.3.4"})
	if err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}
}

func TestCount(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "a"})
	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "b"})

	c, err := db.Count(ctx, "contact_messages", bson.M{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if c != 2 {
		t.Errorf("expected 2, got %d", c)
	}
}

func TestFindAll(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "x"})
	db.InsertOne(ctx, "contact_messages", bson.M{"ip": "y"})

	var results []bson.M
	if err := db.FindAll(ctx, "contact_messages", bson.M{}, &results); err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

// --- Collection accessors ---

func TestCollectionAccessors(t *testing.T) {
	db := testDB(t)
	accessors := map[string]func() interface{}{
		"templates":        func() interface{} { return db.Templates() },
		"content":          func() interface{} { return db.Content() },
		"collections":      func() interface{} { return db.Collections() },
		"custom_pages":     func() interface{} { return db.CustomPages() },
		"settings":         func() interface{} { return db.Settings() },
		"theme_versions":   func() interface{} { return db.ThemeVersions() },
		"folders":          func() interface{} { return db.Folders() },
		"redirects":        func() interface{} { return db.Redirects() },
		"contact_messages": func() interface{} { return db.ContactMessages() },
		"assets":           func() interface{} { return db.Assets() },
		"api_keys":         func() interface{} { return db.APIKeys() },
		"users":            func() interface{} { return db.Users() },
		"audit_logs":       func() interface{} { return db.AuditLogs() },
	}
	for name, fn := range accessors {
		if fn() == nil {
			t.Errorf("Collection accessor %s returned nil", name)
		}
	}
}
