package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DB struct {
	client   *mongo.Client
	database *mongo.Database
}

func Connect(ctx context.Context, uri, dbName string) (*DB, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := &DB{
		client:   client,
		database: client.Database(dbName),
	}

	// Create indexes
	if err := db.createIndexes(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) createIndexes(ctx context.Context) error {
	// Content full_path index (unique) - replaces slug index
	_, err := db.database.Collection("content").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "full_path", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	})
	if err != nil {
		return err
	}

	// Drop old unique slug index if it exists (we're changing it to non-unique)
	db.database.Collection("content").Indexes().DropOne(ctx, "slug_1")

	// Content slug index (non-unique now, since same slug can be in different folders)
	_, err = db.database.Collection("content").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "slug", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Content folder_id index for quick folder lookups
	_, err = db.database.Collection("content").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "folder_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Content category index for collections
	_, err = db.database.Collection("content").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "category", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Template name index
	_, err = db.database.Collection("templates").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Folder path index (unique)
	_, err = db.database.Collection("folders").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "path", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Folder parent_id index
	_, err = db.database.Collection("folders").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "parent_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Custom pages slug index
	_, err = db.database.Collection("custom_pages").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Redirect from_path index (unique)
	_, err = db.database.Collection("redirects").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "from_path", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Contact messages created_at index for sorting
	_, err = db.database.Collection("contact_messages").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: -1}},
	})
	if err != nil {
		return err
	}

	// Contact rate limiting index (IP + created_at)
	_, err = db.database.Collection("contact_messages").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "ip_address", Value: 1}, {Key: "created_at", Value: -1}},
	})
	if err != nil {
		return err
	}

	// Asset full_path index (unique)
	_, err = db.database.Collection("assets").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "full_path", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Asset folder index for filtering
	_, err = db.database.Collection("assets").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "folder", Value: 1}},
	})
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) Disconnect(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

// Template operations
func (db *DB) Templates() *mongo.Collection {
	return db.database.Collection("templates")
}

func (db *DB) Content() *mongo.Collection {
	return db.database.Collection("content")
}

func (db *DB) Collections() *mongo.Collection {
	return db.database.Collection("collections")
}

func (db *DB) CustomPages() *mongo.Collection {
	return db.database.Collection("custom_pages")
}

func (db *DB) Settings() *mongo.Collection {
	return db.database.Collection("settings")
}

// Generic CRUD helpers
func (db *DB) InsertOne(ctx context.Context, collection string, doc interface{}) (primitive.ObjectID, error) {
	result, err := db.database.Collection(collection).InsertOne(ctx, doc)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return result.InsertedID.(primitive.ObjectID), nil
}

func (db *DB) FindOne(ctx context.Context, collection string, filter interface{}, result interface{}) error {
	return db.database.Collection(collection).FindOne(ctx, filter).Decode(result)
}

func (db *DB) FindMany(ctx context.Context, collection string, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	return db.database.Collection(collection).Find(ctx, filter, opts...)
}

func (db *DB) UpdateOne(ctx context.Context, collection string, filter, update interface{}) error {
	_, err := db.database.Collection(collection).UpdateOne(ctx, filter, update)
	return err
}

func (db *DB) DeleteOne(ctx context.Context, collection string, filter interface{}) error {
	_, err := db.database.Collection(collection).DeleteOne(ctx, filter)
	return err
}

func (db *DB) DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error) {
	result, err := db.database.Collection(collection).DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

func (db *DB) Count(ctx context.Context, collection string, filter interface{}) (int64, error) {
	return db.database.Collection(collection).CountDocuments(ctx, filter)
}

// Theme settings
type ThemeSettings struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	PrimaryColor    string             `bson:"primary_color"`
	SecondaryColor  string             `bson:"secondary_color"`
	AccentColor     string             `bson:"accent_color"`
	BackgroundColor string             `bson:"background_color"`
	TextColor       string             `bson:"text_color"`
	FontFamily      string             `bson:"font_family"`
	HeadingFont     string             `bson:"heading_font"`
	BorderRadius    string             `bson:"border_radius"`
	CustomCSS       string             `bson:"custom_css"`
	SiteName        string             `bson:"site_name"`
	SiteTagline     string             `bson:"site_tagline"`
	LogoURL         string             `bson:"logo_url"`
	HeadHTML        string             `bson:"head_html"`
	HeaderHTML      string             `bson:"header_html"`
	FooterHTML      string             `bson:"footer_html"`
	UpdatedAt       time.Time          `bson:"updated_at"`
}

func (db *DB) GetThemeSettings(ctx context.Context) (*ThemeSettings, error) {
	var settings ThemeSettings
	err := db.Settings().FindOne(ctx, bson.M{"type": "theme"}).Decode(&settings)
	if err == mongo.ErrNoDocuments {
		// Return default settings
		return &ThemeSettings{
			PrimaryColor:    "#6366f1",
			SecondaryColor:  "#8b5cf6",
			AccentColor:     "#06b6d4",
			BackgroundColor: "#0f172a",
			TextColor:       "#f1f5f9",
			FontFamily:      "Inter, system-ui, sans-serif",
			HeadingFont:     "Space Grotesk, system-ui, sans-serif",
			BorderRadius:    "12px",
			CustomCSS:       "",
			SiteName:        "LightCMS",
			SiteTagline:     "A lightweight content management system",
			HeadHTML:        "",
			HeaderHTML:      "",
			FooterHTML:      "",
		}, nil
	}
	return &settings, err
}

func (db *DB) SaveThemeSettings(ctx context.Context, settings *ThemeSettings) error {
	settings.UpdatedAt = time.Now()
	_, err := db.Settings().UpdateOne(
		ctx,
		bson.M{"type": "theme"},
		bson.M{"$set": settings, "$setOnInsert": bson.M{"type": "theme"}},
		options.Update().SetUpsert(true),
	)
	return err
}

// AdminSettings stores admin configuration including password hash
type AdminSettings struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	PasswordHash      string             `bson:"password_hash"`
	IsDefaultPassword bool               `bson:"is_default_password"`
	UpdatedAt         time.Time          `bson:"updated_at"`
}

// LoginAttempt tracks failed login attempts for rate limiting
type LoginAttempt struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	IP        string             `bson:"ip"`
	Attempts  int                `bson:"attempts"`
	LastAttempt time.Time        `bson:"last_attempt"`
	LockedUntil *time.Time       `bson:"locked_until,omitempty"`
}

func (db *DB) GetAdminSettings(ctx context.Context) (*AdminSettings, error) {
	var settings AdminSettings
	err := db.Settings().FindOne(ctx, bson.M{"type": "admin"}).Decode(&settings)
	if err == mongo.ErrNoDocuments {
		return nil, nil // No settings yet
	}
	return &settings, err
}

func (db *DB) SaveAdminSettings(ctx context.Context, settings *AdminSettings) error {
	settings.UpdatedAt = time.Now()
	_, err := db.Settings().UpdateOne(
		ctx,
		bson.M{"type": "admin"},
		bson.M{"$set": settings, "$setOnInsert": bson.M{"type": "admin"}},
		options.Update().SetUpsert(true),
	)
	return err
}

// SiteConfig stores site-wide configuration settings
type SiteConfig struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty"`
	TitleTemplate        string             `bson:"title_template"`         // Template when page has title, e.g. "{{title}} | {{site_name}}"
	TitleTemplateNoTitle string             `bson:"title_template_no_title"` // Template when page has no title, e.g. "{{site_name}}"
	UpdatedAt            time.Time          `bson:"updated_at"`
}

// GetSiteConfig retrieves the site configuration
func (db *DB) GetSiteConfig(ctx context.Context) (*SiteConfig, error) {
	var config SiteConfig
	err := db.Settings().FindOne(ctx, bson.M{"type": "site_config"}).Decode(&config)
	if err == mongo.ErrNoDocuments {
		// Return defaults
		return &SiteConfig{
			TitleTemplate:        "{{title}} - {{site_name}}",
			TitleTemplateNoTitle: "{{site_name}}",
		}, nil
	}
	return &config, err
}

// SaveSiteConfig saves the site configuration
func (db *DB) SaveSiteConfig(ctx context.Context, config *SiteConfig) error {
	config.UpdatedAt = time.Now()
	_, err := db.Settings().UpdateOne(
		ctx,
		bson.M{"type": "site_config"},
		bson.M{"$set": config, "$setOnInsert": bson.M{"type": "site_config"}},
		options.Update().SetUpsert(true),
	)
	return err
}

// DatabaseVersion tracks the software version used with the database
type DatabaseVersion struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Type      string             `bson:"type"`
	Version   string             `bson:"version"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

// GetDatabaseVersion retrieves the current database version
func (db *DB) GetDatabaseVersion(ctx context.Context) (string, error) {
	var ver DatabaseVersion
	err := db.Settings().FindOne(ctx, bson.M{"type": "database_version"}).Decode(&ver)
	if err == mongo.ErrNoDocuments {
		return "", nil // No version set yet
	}
	if err != nil {
		return "", err
	}
	return ver.Version, nil
}

// SetDatabaseVersion updates the database version
func (db *DB) SetDatabaseVersion(ctx context.Context, version string) error {
	_, err := db.Settings().UpdateOne(
		ctx,
		bson.M{"type": "database_version"},
		bson.M{"$set": bson.M{"version": version, "updated_at": time.Now()}, "$setOnInsert": bson.M{"type": "database_version"}},
		options.Update().SetUpsert(true),
	)
	return err
}

// GetLoginAttempts gets the login attempt record for an IP
func (db *DB) GetLoginAttempts(ctx context.Context, ip string) (*LoginAttempt, error) {
	var attempt LoginAttempt
	err := db.database.Collection("login_attempts").FindOne(ctx, bson.M{"ip": ip}).Decode(&attempt)
	if err == mongo.ErrNoDocuments {
		return &LoginAttempt{IP: ip, Attempts: 0}, nil
	}
	return &attempt, err
}

// RecordFailedLogin increments failed login counter
func (db *DB) RecordFailedLogin(ctx context.Context, ip string) error {
	now := time.Now()
	attempt, _ := db.GetLoginAttempts(ctx, ip)

	// Reset counter if last attempt was more than 15 minutes ago
	if time.Since(attempt.LastAttempt) > 15*time.Minute {
		attempt.Attempts = 0
	}

	attempt.Attempts++
	attempt.LastAttempt = now

	// Lock for increasing duration based on attempts
	// 10 attempts: 1 min, 15 attempts: 5 min, 20+ attempts: 15 min
	if attempt.Attempts >= 20 {
		lockUntil := now.Add(15 * time.Minute)
		attempt.LockedUntil = &lockUntil
	} else if attempt.Attempts >= 15 {
		lockUntil := now.Add(5 * time.Minute)
		attempt.LockedUntil = &lockUntil
	} else if attempt.Attempts >= 10 {
		lockUntil := now.Add(1 * time.Minute)
		attempt.LockedUntil = &lockUntil
	}

	_, err := db.database.Collection("login_attempts").UpdateOne(
		ctx,
		bson.M{"ip": ip},
		bson.M{"$set": attempt},
		options.Update().SetUpsert(true),
	)
	return err
}

// ClearLoginAttempts resets the login attempt counter (on successful login)
func (db *DB) ClearLoginAttempts(ctx context.Context, ip string) error {
	_, err := db.database.Collection("login_attempts").DeleteOne(ctx, bson.M{"ip": ip})
	return err
}

// IsLoginLocked checks if an IP is currently locked out
func (db *DB) IsLoginLocked(ctx context.Context, ip string) (bool, time.Duration) {
	attempt, err := db.GetLoginAttempts(ctx, ip)
	if err != nil || attempt.LockedUntil == nil {
		return false, 0
	}

	if time.Now().Before(*attempt.LockedUntil) {
		return true, time.Until(*attempt.LockedUntil)
	}
	return false, 0
}

// Folders returns the folders collection
func (db *DB) Folders() *mongo.Collection {
	return db.database.Collection("folders")
}

// Redirects returns the redirects collection
func (db *DB) Redirects() *mongo.Collection {
	return db.database.Collection("redirects")
}

// ContactMessages returns the contact_messages collection
func (db *DB) ContactMessages() *mongo.Collection {
	return db.database.Collection("contact_messages")
}

// GetRedirect looks up a redirect by source path
func (db *DB) GetRedirect(ctx context.Context, fromPath string) (*Redirect, error) {
	var redirect Redirect
	err := db.Redirects().FindOne(ctx, bson.M{"from_path": fromPath}).Decode(&redirect)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &redirect, err
}

// Redirect model stored locally for query results
type Redirect struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	FromPath    string             `bson:"from_path"`
	ToPath      string             `bson:"to_path"`
	StatusCode  int                `bson:"status_code"`
	Description string             `bson:"description"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}

// IsContactRateLimited checks if an IP has submitted too many contact forms recently
func (db *DB) IsContactRateLimited(ctx context.Context, ip string) (bool, error) {
	// Allow max 3 submissions per hour per IP
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	count, err := db.ContactMessages().CountDocuments(ctx, bson.M{
		"ip_address": ip,
		"created_at": bson.M{"$gte": oneHourAgo},
	})
	if err != nil {
		return false, err
	}
	return count >= 3, nil
}

// Assets returns the assets collection
func (db *DB) Assets() *mongo.Collection {
	return db.database.Collection("assets")
}

// Asset model stored locally for query results
type Asset struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Filename    string             `bson:"filename"`
	Folder      string             `bson:"folder"`
	FullPath    string             `bson:"full_path"`
	ServePath   string             `bson:"serve_path"` // URL path where file is served (e.g., /favicon.png)
	MimeType    string             `bson:"mime_type"`
	Size        int64              `bson:"size"`
	Data        []byte             `bson:"data"`
	Description string             `bson:"description"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}

// GetAssetByPath retrieves an asset by its full path
func (db *DB) GetAssetByPath(ctx context.Context, fullPath string) (*Asset, error) {
	var asset Asset
	err := db.Assets().FindOne(ctx, bson.M{"full_path": fullPath}).Decode(&asset)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &asset, err
}

// ListAssets returns all assets, optionally filtered by folder
func (db *DB) ListAssets(ctx context.Context, folder string) ([]*Asset, error) {
	filter := bson.M{}
	if folder != "" {
		filter["folder"] = folder
	}

	opts := options.Find().SetSort(bson.D{{Key: "folder", Value: 1}, {Key: "filename", Value: 1}})
	cursor, err := db.Assets().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var assets []*Asset
	if err := cursor.All(ctx, &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

// SaveAsset creates or updates an asset
func (db *DB) SaveAsset(ctx context.Context, asset *Asset) error {
	now := time.Now()
	if asset.ID.IsZero() {
		asset.CreatedAt = now
	}
	asset.UpdatedAt = now

	_, err := db.Assets().UpdateOne(
		ctx,
		bson.M{"full_path": asset.FullPath},
		bson.M{"$set": asset},
		options.Update().SetUpsert(true),
	)
	return err
}

// MigrateAssetServePaths updates legacy assets to have serve_path set
func (db *DB) MigrateAssetServePaths(ctx context.Context) error {
	// Find all assets without serve_path or with empty serve_path
	cursor, err := db.Assets().Find(ctx, bson.M{
		"$or": []bson.M{
			{"serve_path": ""},
			{"serve_path": bson.M{"$exists": false}},
		},
	})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var asset Asset
		if err := cursor.Decode(&asset); err != nil {
			continue
		}

		// Set serve_path to /assets/ + full_path for legacy assets
		servePath := "/assets" + asset.FullPath
		_, err := db.Assets().UpdateOne(ctx,
			bson.M{"_id": asset.ID},
			bson.M{"$set": bson.M{"serve_path": servePath}},
		)
		if err != nil {
			fmt.Printf("Warning: Failed to migrate asset %s: %v\n", asset.Filename, err)
		} else {
			fmt.Printf("Migrated asset %s to serve_path %s\n", asset.Filename, servePath)
		}
	}
	return nil
}

// GetAsset retrieves an asset by ID
func (db *DB) GetAsset(ctx context.Context, id primitive.ObjectID) (*Asset, error) {
	var asset Asset
	err := db.Assets().FindOne(ctx, bson.M{"_id": id}).Decode(&asset)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &asset, err
}

func (db *DB) DeleteAsset(ctx context.Context, id primitive.ObjectID) error {
	_, err := db.Assets().DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// GetAssetFolders returns a list of unique folder paths
func (db *DB) GetAssetFolders(ctx context.Context) ([]string, error) {
	results, err := db.Assets().Distinct(ctx, "folder", bson.M{})
	if err != nil {
		return nil, err
	}

	folders := make([]string, 0, len(results))
	for _, r := range results {
		if s, ok := r.(string); ok {
			folders = append(folders, s)
		}
	}
	return folders, nil
}
