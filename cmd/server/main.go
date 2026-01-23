package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lightcms/config"
	"lightcms/internal/auth"
	"lightcms/internal/build"
	"lightcms/internal/database"
	"lightcms/internal/handlers"
	"lightcms/internal/models"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func main() {
	// Load configuration from JSON config file
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting LightCMS in %s mode", cfg.Env)

	// Validate required config
	if cfg.MongoURI == "" {
		log.Fatal("mongo_uri is required in config file")
	}
	if cfg.SessionSecret == "" {
		log.Fatal("session_secret is required in config file")
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.MongoURI, "lightcms")
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer db.Disconnect(context.Background())

	log.Println("Connected to MongoDB successfully")

	// Initialize session store
	sessionStore := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   cfg.SecureCookies, // true in production (requires HTTPS)
	}

	// Initialize auth manager with database connection
	authManager := auth.NewManager(sessionStore, db)

	// Initialize password hash in database
	if err := authManager.InitializePassword(context.Background()); err != nil {
		log.Printf("Warning: Failed to initialize password: %v", err)
	}

	// Initialize handlers with config
	h := handlers.New(db, authManager, cfg.BaseURL, cfg.Env)

	// Seed default data if needed
	if err := h.SeedDefaults(context.Background()); err != nil {
		log.Printf("Warning: Failed to seed defaults: %v", err)
	}

	// Check for version migration
	if err := checkVersionMigration(db); err != nil {
		log.Printf("Warning: Failed to check version migration: %v", err)
	}

	// Setup router
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("static/uploads"))))

	// Admin routes (under /cm)
	admin := r.PathPrefix("/cm").Subrouter()
	admin.HandleFunc("/login", h.LoginPage).Methods("GET")
	admin.HandleFunc("/login", h.LoginHandler).Methods("POST")
	admin.HandleFunc("/logout", h.LogoutHandler).Methods("GET")
	admin.HandleFunc("", h.AdminDashboard).Methods("GET")
	admin.HandleFunc("/", h.AdminDashboard).Methods("GET")
	admin.HandleFunc("/templates", h.ListTemplates).Methods("GET")
	admin.HandleFunc("/templates/new", h.NewTemplate).Methods("GET")
	admin.HandleFunc("/templates/new", h.CreateTemplate).Methods("POST")
	admin.HandleFunc("/templates/{id}", h.EditTemplate).Methods("GET")
	admin.HandleFunc("/templates/{id}", h.UpdateTemplate).Methods("POST")
	admin.HandleFunc("/templates/{id}/delete", h.DeleteTemplate).Methods("POST")
	admin.HandleFunc("/content", h.ListContent).Methods("GET")
	admin.HandleFunc("/content/new", h.NewContent).Methods("GET")
	admin.HandleFunc("/content/new/{templateID}", h.NewContentWithTemplate).Methods("GET")
	admin.HandleFunc("/content/create", h.CreateContent).Methods("POST")
	admin.HandleFunc("/content/{id}", h.EditContent).Methods("GET")
	admin.HandleFunc("/content/{id}", h.UpdateContent).Methods("POST")
	admin.HandleFunc("/content/{id}/delete", h.DeleteContent).Methods("POST")
	admin.HandleFunc("/content/{id}/undelete", h.UndeleteContent).Methods("POST")
	admin.HandleFunc("/content/{id}/regenerate", h.RegenerateContent).Methods("POST")
	admin.HandleFunc("/content/{id}/versions", h.ListContentVersions).Methods("GET")
	admin.HandleFunc("/content/{id}/versions/{version}/view", h.ViewContentVersion).Methods("GET")
	admin.HandleFunc("/content/{id}/versions/{version}/diff", h.DiffContentVersion).Methods("GET")
	admin.HandleFunc("/content/{id}/versions/{version}/revert", h.RevertContentVersion).Methods("POST")
	admin.HandleFunc("/collections", h.ListCollections).Methods("GET")
	admin.HandleFunc("/collections/new", h.NewCollection).Methods("GET")
	admin.HandleFunc("/collections/new", h.CreateCollection).Methods("POST")
	admin.HandleFunc("/collections/{id}", h.EditCollection).Methods("GET")
	admin.HandleFunc("/collections/{id}", h.UpdateCollection).Methods("POST")
	admin.HandleFunc("/collections/{id}/delete", h.DeleteCollection).Methods("POST")
	admin.HandleFunc("/theme", h.ThemeSettings).Methods("GET")
	admin.HandleFunc("/theme", h.UpdateTheme).Methods("POST")
	admin.HandleFunc("/security", h.SecuritySettings).Methods("GET")
	admin.HandleFunc("/security", h.UpdatePassword).Methods("POST")
	admin.HandleFunc("/config", h.SiteConfiguration).Methods("GET")
	admin.HandleFunc("/config", h.UpdateSiteConfiguration).Methods("POST")
	admin.HandleFunc("/upload", h.UploadFile).Methods("POST")
	admin.HandleFunc("/folders", h.ListFolders).Methods("GET")
	admin.HandleFunc("/folders/new", h.NewFolder).Methods("GET")
	admin.HandleFunc("/folders/new", h.CreateFolder).Methods("POST")
	admin.HandleFunc("/folders/{id}", h.EditFolder).Methods("GET")
	admin.HandleFunc("/folders/{id}", h.UpdateFolder).Methods("POST")
	admin.HandleFunc("/folders/{id}/delete", h.DeleteFolder).Methods("POST")
	admin.HandleFunc("/redirects", h.ListRedirects).Methods("GET")
	admin.HandleFunc("/redirects/new", h.NewRedirect).Methods("GET")
	admin.HandleFunc("/redirects/new", h.CreateRedirect).Methods("POST")
	admin.HandleFunc("/redirects/{id}", h.EditRedirect).Methods("GET")
	admin.HandleFunc("/redirects/{id}", h.UpdateRedirect).Methods("POST")
	admin.HandleFunc("/redirects/{id}/delete", h.DeleteRedirect).Methods("POST")
	admin.HandleFunc("/messages", h.ListContactMessages).Methods("GET")
	admin.HandleFunc("/messages/{id}", h.ViewContactMessage).Methods("GET")
	admin.HandleFunc("/messages/{id}/delete", h.DeleteContactMessage).Methods("POST")
	admin.HandleFunc("/assets", h.AssetLibrary).Methods("GET")
	admin.HandleFunc("/assets/upload", h.AssetUploadForm).Methods("GET")
	admin.HandleFunc("/assets/upload", h.AssetUpload).Methods("POST")
	admin.HandleFunc("/assets/{id}/delete", h.DeleteAsset).Methods("POST")

	// API routes for AJAX
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/template/{id}/fields", h.GetTemplateFields).Methods("GET")
	api.HandleFunc("/slugs", h.GetAllSlugs).Methods("GET")
	api.HandleFunc("/folders", h.GetAllFoldersAPI).Methods("GET")
	api.HandleFunc("/contact", h.ContactFormSubmit).Methods("POST")

	// Public asset serving
	r.PathPrefix("/assets/").HandlerFunc(h.ServeAsset).Methods("GET")

	// Sitemap and robots.txt
	r.HandleFunc("/sitemap.xml", h.ServeSitemap).Methods("GET")
	r.HandleFunc("/robots.txt", h.ServeRobotsTxt).Methods("GET")

	// Public content routes - must be last
	r.HandleFunc("/", h.ServePage).Methods("GET")
	r.HandleFunc("/{slug:.*}", h.ServePage).Methods("GET")

	// Create server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("LightCMS starting on http://localhost:%s", cfg.Port)
		log.Printf("Admin panel available at http://localhost:%s/cm", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// checkVersionMigration checks if the software version has changed and performs migration tasks
func checkVersionMigration(db *database.DB) error {
	ctx := context.Background()

	// Get current software version from build config
	softwareVersion := build.GetVersion()

	// Get database version
	dbVersion, err := db.GetDatabaseVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get database version: %w", err)
	}

	// Check if we need to upgrade
	if dbVersion == softwareVersion {
		return nil // Already up to date
	}

	log.Printf("Version change detected: database=%q, software=%q", dbVersion, softwareVersion)

	// Update database version
	if err := db.SetDatabaseVersion(ctx, softwareVersion); err != nil {
		return fmt.Errorf("failed to set database version: %w", err)
	}

	// Create welcome message
	welcomeMsg := models.ContactMessage{
		Name:      "LightCMS",
		Email:     "",
		Subject:   fmt.Sprintf("Welcome to LightCMS v%s", softwareVersion),
		Message:   fmt.Sprintf(`Welcome to LightCMS v%s! This is a <a href="https://metavert.io">Metavert</a> project. I hope you enjoy using it to maintain your website. --Jon`, softwareVersion),
		IsSystem:  true,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if _, err := db.InsertOne(ctx, "contact_messages", welcomeMsg); err != nil {
		return fmt.Errorf("failed to create welcome message: %w", err)
	}

	log.Printf("Database upgraded to version %s, welcome message created", softwareVersion)
	return nil
}
