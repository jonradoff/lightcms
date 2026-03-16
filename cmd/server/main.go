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
	lightmcp "lightcms/internal/mcp"
	"lightcms/internal/middleware"
	"lightcms/internal/models"
	"lightcms/internal/oauth"
	"lightcms/internal/services"

	"github.com/gorilla/csrf"
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
	// Enforce minimum SESSION_SECRET entropy: 16 bytes minimum, 32 recommended.
	// Fail hard in production; log a warning in development so local dev still works.
	if len(cfg.SessionSecret) < 16 {
		msg := "session_secret is too short (minimum 16 characters; 32+ recommended)"
		if cfg.Env == "production" {
			log.Fatal(msg)
		} else {
			log.Printf("WARNING: %s", msg)
		}
	} else if len(cfg.SessionSecret) < 32 {
		log.Printf("WARNING: session_secret should be at least 32 characters for production use")
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

	// Initialize session store with secure settings
	sessionStore := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400, // 24 hours (reduced from 7 days for security)
		HttpOnly: true,
		Secure:   cfg.SecureCookies,  // true in production (requires HTTPS)
		SameSite: http.SameSiteStrictMode, // Prevent CSRF via cookies
	}

	// Initialize user and audit services
	userService := services.NewUserService(db)
	auditService := services.NewAuditService(db)

	// Initialize auth manager with database connection
	authManager := auth.NewManager(sessionStore, db, userService)

	// Migrate to multi-user system (creates first admin user from legacy password if needed)
	if err := authManager.MigrateToMultiUser(context.Background()); err != nil {
		log.Printf("Warning: Failed to migrate to multi-user: %%v", err)
	}

	// Initialize snippet service (needed by both handlers and API handler)
	snippetService := services.NewSnippetService(db)

	// Initialize analytics service (DAU/MAU tracking)
	analyticsService := services.NewAnalyticsService(context.Background(), db)

	// Initialize handlers with config
	h := handlers.New(db, authManager, cfg.BaseURL, cfg.Env, userService, auditService, snippetService)
	h.SetAnalyticsService(analyticsService)

	// Initialize trusted proxy config for rate limiting
	proxyConfig := middleware.DefaultCloudConfig()

	// Seed default data if needed
	if err := h.SeedDefaults(context.Background()); err != nil {
		log.Printf("Warning: Failed to seed defaults: %v", err)
	}

	// Check for version migration
	if err := checkVersionMigration(db); err != nil {
		log.Printf("Warning: Failed to check version migration: %v", err)
	}

	// Migrate legacy assets to have serve_path
	if err := db.MigrateAssetServePaths(context.Background()); err != nil {
		log.Printf("Warning: Failed to migrate asset serve paths: %v", err)
	}

	// Ensure theme version 1 exists (save current theme as first version if no versions exist)
	settingsService := services.NewSettingsService(db, services.NewContentService(db))
	if err := settingsService.EnsureThemeVersion1(context.Background()); err != nil {
		log.Printf("Warning: Failed to ensure theme version 1: %v", err)
	}

	// Regenerate theme CSS on startup — ensures static/css/theme-vars.css exists after deploy/restart
	if err := settingsService.EnsureThemeCSS(context.Background()); err != nil {
		log.Printf("Warning: Failed to regenerate theme CSS on startup: %v", err)
	}

	// Initialize services for API and change watcher
	contentService := services.NewContentService(db)
	h.SetContentService(contentService)
	templateService := services.NewTemplateService(db, contentService)
	assetService := services.NewAssetService(db)

	// Initialize search service (always available; semantic search requires Voyage API key)
	searchService := services.NewSearchService(db, cfg.VoyageAPIKey)
	contentService.SetSearchService(searchService) // Always set — needed for keyword cache rebuild on content changes
	if cfg.VoyageAPIKey != "" {
		log.Println("End-user search enabled (fulltext + semantic via Voyage AI)")
	} else {
		log.Println("End-user search enabled (fulltext only — set VOYAGE_API_KEY for semantic search)")
	}

	// Build search keyword cache on startup
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := searchService.RebuildKeywords(ctx); err != nil {
			log.Printf("Warning: failed to build search keywords: %v", err)
		}
	}()

	// Start content change watcher for real-time sync with database changes
	// This enables automatic static page regeneration when content is modified via MCP
	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	go contentService.WatchForChanges(watchCtx)

	// Wire search service into handlers
	h.SetSearchService(searchService)
	h.SetProxyConfig(proxyConfig)

	// Setup router
	r := mux.NewRouter()

	// Apply security headers middleware to all routes
	r.Use(middleware.SecurityHeaders)

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("static/uploads"))))

	// CSRF protection for admin routes
	// Generate a 32-byte key from session secret for CSRF
	csrfKey := []byte(cfg.SessionSecret)
	if len(csrfKey) > 32 {
		csrfKey = csrfKey[:32]
	} else if len(csrfKey) < 32 {
		// Pad with zeros if too short (shouldn't happen with proper secrets)
		padded := make([]byte, 32)
		copy(padded, csrfKey)
		csrfKey = padded
	}

	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(cfg.SecureCookies),
		csrf.Path("/cm"),
		csrf.SameSite(csrf.SameSiteStrictMode),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("CSRF validation failed for %s %s", r.Method, r.URL.Path)
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
		})),
	)

	// Admin routes (under /cm)
	admin := r.PathPrefix("/cm").Subrouter()
	admin.Use(csrfMiddleware)
	admin.HandleFunc("/login", h.LoginPage).Methods("GET")
	admin.HandleFunc("/login", h.LoginHandler).Methods("POST")
	admin.HandleFunc("/logout", h.LogoutHandler).Methods("POST") // Changed to POST for security
	admin.HandleFunc("/change-password", h.ForceChangePasswordPage).Methods("GET")
	admin.HandleFunc("/change-password", h.ForceChangePasswordHandler).Methods("POST")
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
	admin.HandleFunc("/content/{id}/change-template/{template_id}", h.ChangeTemplatePreview).Methods("GET")
	admin.HandleFunc("/content/{id}/change-template/{template_id}/confirm", h.ConfirmChangeTemplate).Methods("POST")
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
	admin.HandleFunc("/theme/versions", h.ThemeVersions).Methods("GET")
	admin.HandleFunc("/theme/versions/{version}", h.ThemeVersionDiff).Methods("GET")
	admin.HandleFunc("/theme/versions/{version}/revert", h.RevertThemeVersion).Methods("POST")
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
	admin.HandleFunc("/api-keys", h.APIKeysPage).Methods("GET")
	admin.HandleFunc("/api-keys/new", h.NewAPIKeyPage).Methods("GET")
	admin.HandleFunc("/api-keys/new", h.CreateAPIKey).Methods("POST")
	admin.HandleFunc("/api-keys/{id}/delete", h.DeleteAPIKey).Methods("POST")

	// User management routes (admin only)
	admin.HandleFunc("/users", h.UsersPage).Methods("GET")
	admin.HandleFunc("/users/new", h.NewUserPage).Methods("GET")
	admin.HandleFunc("/users/new", h.CreateUser).Methods("POST")
	admin.HandleFunc("/users/{id}", h.EditUserPage).Methods("GET")
	admin.HandleFunc("/users/{id}", h.UpdateUser).Methods("POST")
	admin.HandleFunc("/users/{id}/toggle-disabled", h.ToggleUserDisabled).Methods("POST")
	admin.HandleFunc("/users/{id}/reset-password", h.ResetUserPassword).Methods("POST")

	// Audit log (admin only)
	admin.HandleFunc("/audit", h.AuditLogPage).Methods("GET")

	// Snippets
	admin.HandleFunc("/snippets", h.ListSnippets).Methods("GET")
	admin.HandleFunc("/snippets/new", h.NewSnippet).Methods("GET")
	admin.HandleFunc("/snippets/new", h.CreateSnippet).Methods("POST")
	admin.HandleFunc("/snippets/{id}", h.EditSnippet).Methods("GET")
	admin.HandleFunc("/snippets/{id}", h.UpdateSnippet).Methods("POST")
	admin.HandleFunc("/snippets/{id}/delete", h.DeleteSnippet).Methods("POST")

	// Tools routes
	admin.HandleFunc("/tools/broken-links", h.BrokenLinkFinder).Methods("GET")
	admin.HandleFunc("/tools/search", h.SearchToolPage).Methods("GET")
	admin.HandleFunc("/tools/search/test", h.SearchToolTest).Methods("GET")
	admin.HandleFunc("/tools/search/reindex", h.SearchToolReindex).Methods("POST")
	admin.HandleFunc("/tools/search/config", h.SearchToolSaveConfig).Methods("POST")

	// REST API v1 routes (API key authenticated, JSON only)
	apiKeyService := services.NewAPIKeyService(db)
	apiHandler := handlers.NewAPIHandler(contentService, templateService, assetService, settingsService, apiKeyService, auditService, snippetService)
	apiHandler.SetSearchService(searchService)
	apiAuthMiddleware := middleware.NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		apiKey, err := apiKeyService.ValidateAPIKey(ctx, rawKey)
		if err != nil {
			return nil, err
		}
		// Resolve the user who owns this API key
		if apiKey.UserID != nil {
			user, err := userService.GetByID(ctx, *apiKey.UserID)
			if err == nil && user != nil {
				go analyticsService.RecordActivity(context.Background(), user.ID.Hex())
				return &auth.SessionUser{
					ID:    user.ID.Hex(),
					Email: user.Email,
					Role:  user.Role,
				}, nil
			}
		}
		// System/legacy key without user — return nil (no permission enforcement)
		return nil, nil
	})

	// OAuth 2.1 setup for MCP Cowork connector support
	oauthService := services.NewOAuthService(db)
	systemAPIKey, err := services.EnsureSystemAPIKey(context.Background(), db, apiKeyService)
	if err != nil {
		log.Fatalf("Failed to create system API key: %v", err)
	}
	resourceMetadataURL := cfg.BaseURL + "/.well-known/oauth-protected-resource"
	apiAuthMiddleware.SetOAuth(
		func(ctx context.Context, rawToken string) (interface{}, error) {
			_, err := oauthService.ValidateAccessToken(ctx, rawToken)
			if err != nil {
				return nil, err
			}
			// OAuth tokens go through the system API key path — no specific user context
			// (the system key will be resolved on the subsequent internal request)
			return nil, nil
		},
		systemAPIKey,
		resourceMetadataURL,
	)
	oauthHandler := oauth.NewHandler(oauthService, authManager, cfg.BaseURL, cfg.SessionSecret)

	apiv1 := r.PathPrefix("/api/v1").Subrouter()
	apiv1.Use(apiAuthMiddleware.Middleware)
	apiv1.Use(middleware.APIRateLimit) // per-token sliding-window rate limit (300 req/min)

	// Content
	apiv1.HandleFunc("/content", apiHandler.APIListContent).Methods("GET")
	apiv1.HandleFunc("/content", apiHandler.APICreateContent).Methods("POST")
	apiv1.HandleFunc("/content/by-path", apiHandler.APIGetContentByPath).Methods("GET")
	apiv1.HandleFunc("/content/by-path", apiHandler.APIUpdateContentByPath).Methods("PUT")
	apiv1.HandleFunc("/content/backlinks", apiHandler.APIGetBacklinks).Methods("GET")
	apiv1.HandleFunc("/content/batch-publish", apiHandler.APIBatchPublishContent).Methods("POST")
	apiv1.HandleFunc("/content/bulk-update", apiHandler.APIBulkUpdateContent).Methods("POST")
	apiv1.HandleFunc("/content/bulk-field-op", apiHandler.APIBulkFieldOperation).Methods("POST")
	apiv1.HandleFunc("/content/export", apiHandler.APIExportContent).Methods("POST")
	apiv1.HandleFunc("/content/{id}", apiHandler.APIGetContent).Methods("GET")
	apiv1.HandleFunc("/content/{id}", apiHandler.APIUpdateContent).Methods("PUT")
	apiv1.HandleFunc("/content/{id}", apiHandler.APIDeleteContent).Methods("DELETE")
	apiv1.HandleFunc("/content/{id}/restore", apiHandler.APIRestoreContent).Methods("POST")
	apiv1.HandleFunc("/content/{id}/publish", apiHandler.APIPublishContent).Methods("POST")
	apiv1.HandleFunc("/content/{id}/unpublish", apiHandler.APIUnpublishContent).Methods("POST")
	apiv1.HandleFunc("/content/{id}/preview", apiHandler.APIPreviewContent).Methods("GET", "POST")
	apiv1.HandleFunc("/content/{id}/versions", apiHandler.APIListContentVersions).Methods("GET")
	apiv1.HandleFunc("/content/{id}/versions/{version}", apiHandler.APIGetContentVersion).Methods("GET")
	apiv1.HandleFunc("/content/{id}/versions/{version}/revert", apiHandler.APIRevertContentVersion).Methods("POST")

	// Templates
	apiv1.HandleFunc("/templates", apiHandler.APIListTemplates).Methods("GET")
	apiv1.HandleFunc("/templates", apiHandler.APICreateTemplate).Methods("POST")
	apiv1.HandleFunc("/templates/{id}", apiHandler.APIGetTemplate).Methods("GET")
	apiv1.HandleFunc("/templates/{id}", apiHandler.APIUpdateTemplate).Methods("PUT")
	apiv1.HandleFunc("/templates/{id}", apiHandler.APIDeleteTemplate).Methods("DELETE")

	// Assets
	apiv1.HandleFunc("/assets", apiHandler.APIListAssets).Methods("GET")
	apiv1.HandleFunc("/assets", apiHandler.APIUploadAsset).Methods("POST")
	apiv1.HandleFunc("/assets/from-url", apiHandler.APIUploadAssetFromURL).Methods("POST")
	apiv1.HandleFunc("/assets/folders", apiHandler.APIListAssetFolders).Methods("GET")
	apiv1.HandleFunc("/assets/by-path", apiHandler.APIGetAssetByPath).Methods("GET")
	apiv1.HandleFunc("/assets/{id}", apiHandler.APIGetAsset).Methods("GET")
	apiv1.HandleFunc("/assets/{id}", apiHandler.APIDeleteAsset).Methods("DELETE")

	// Theme
	apiv1.HandleFunc("/theme", apiHandler.APIGetTheme).Methods("GET")
	apiv1.HandleFunc("/theme", apiHandler.APIUpdateTheme).Methods("PUT")
	apiv1.HandleFunc("/theme/versions", apiHandler.APIListThemeVersions).Methods("GET")
	apiv1.HandleFunc("/theme/versions/{version}", apiHandler.APIGetThemeVersion).Methods("GET")
	apiv1.HandleFunc("/theme/versions/{version}/revert", apiHandler.APIRevertThemeVersion).Methods("POST")
	apiv1.HandleFunc("/theme/versions/{version}/pin", apiHandler.APIPinThemeVersion).Methods("POST")
	apiv1.HandleFunc("/theme/versions/{version}/unpin", apiHandler.APIPinThemeVersion).Methods("POST")

	// Site Config
	apiv1.HandleFunc("/config", apiHandler.APIGetSiteConfig).Methods("GET")
	apiv1.HandleFunc("/config", apiHandler.APIUpdateSiteConfig).Methods("PUT")

	// Redirects
	apiv1.HandleFunc("/redirects", apiHandler.APIListRedirects).Methods("GET")
	apiv1.HandleFunc("/redirects", apiHandler.APICreateRedirect).Methods("POST")
	apiv1.HandleFunc("/redirects/{id}", apiHandler.APIGetRedirect).Methods("GET")
	apiv1.HandleFunc("/redirects/{id}", apiHandler.APIUpdateRedirect).Methods("PUT")
	apiv1.HandleFunc("/redirects/{id}", apiHandler.APIDeleteRedirect).Methods("DELETE")

	// Folders
	apiv1.HandleFunc("/folders", apiHandler.APIListFolders).Methods("GET")
	apiv1.HandleFunc("/folders", apiHandler.APICreateFolder).Methods("POST")
	apiv1.HandleFunc("/folders/{id}", apiHandler.APIGetFolder).Methods("GET")
	apiv1.HandleFunc("/folders/{id}", apiHandler.APIDeleteFolder).Methods("DELETE")

	// Collections
	apiv1.HandleFunc("/collections", apiHandler.APIListCollections).Methods("GET")
	apiv1.HandleFunc("/collections", apiHandler.APICreateCollection).Methods("POST")
	apiv1.HandleFunc("/collections/{id}", apiHandler.APIGetCollection).Methods("GET")
	apiv1.HandleFunc("/collections/{id}", apiHandler.APIUpdateCollection).Methods("PUT")
	apiv1.HandleFunc("/collections/{id}", apiHandler.APIDeleteCollection).Methods("DELETE")

	// Search
	apiv1.HandleFunc("/search", apiHandler.APISearchContent).Methods("GET")
	apiv1.HandleFunc("/search-replace/preview", apiHandler.APISearchReplacePreview).Methods("POST")
	apiv1.HandleFunc("/search-replace/execute", apiHandler.APISearchReplaceExecute).Methods("POST")
	apiv1.HandleFunc("/search-replace/scoped/preview", apiHandler.APIScopedSearchReplacePreview).Methods("POST")
	apiv1.HandleFunc("/search-replace/scoped/execute", apiHandler.APIScopedSearchReplaceExecute).Methods("POST")

	// End-user search (authenticated API)
	apiv1.HandleFunc("/end-user-search", apiHandler.APIEndUserSearch).Methods("GET")
	apiv1.HandleFunc("/end-user-search/suggest", apiHandler.APIEndUserSearchSuggest).Methods("GET")
	apiv1.HandleFunc("/reindex-embeddings", apiHandler.APIReindexEmbeddings).Methods("POST")

	// API Keys
	apiv1.HandleFunc("/api-keys", apiHandler.APIListAPIKeys).Methods("GET")
	apiv1.HandleFunc("/api-keys", apiHandler.APICreateAPIKey).Methods("POST")
	apiv1.HandleFunc("/api-keys/{id}", apiHandler.APIDeleteAPIKey).Methods("DELETE")

	// Snippets
	apiv1.HandleFunc("/snippets", apiHandler.APIListSnippets).Methods("GET")
	apiv1.HandleFunc("/snippets", apiHandler.APICreateSnippet).Methods("POST")
	apiv1.HandleFunc("/snippets/{id}", apiHandler.APIGetSnippet).Methods("GET")
	apiv1.HandleFunc("/snippets/{id}", apiHandler.APIUpdateSnippet).Methods("PUT")
	apiv1.HandleFunc("/snippets/{id}", apiHandler.APIDeleteSnippet).Methods("DELETE")

	// Regenerate
	apiv1.HandleFunc("/regenerate", apiHandler.APIRegenerateAllContent).Methods("POST")

	// API routes for AJAX (admin panel internal use)
	// Note: Most API routes require authentication (checked in handlers)
	// The /api/contact route is public for contact form submissions
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/template/{id}/fields", h.GetTemplateFields).Methods("GET")    // Auth checked in handler
	api.HandleFunc("/slugs", h.GetAllSlugs).Methods("GET")                         // Auth checked in handler
	api.HandleFunc("/folders", h.GetAllFoldersAPI).Methods("GET")                  // Auth checked in handler
	api.HandleFunc("/contact", h.ContactFormSubmitWithConfig(proxyConfig)).Methods("POST") // Public, uses trusted proxy config
	api.HandleFunc("/content/search", h.SearchContent).Methods("GET")              // Auth checked in handler
	api.HandleFunc("/content/check-slug", h.CheckSlug).Methods("GET")              // Auth checked in handler
	api.HandleFunc("/content/replace-preview", h.ReplacePreview).Methods("GET")    // Auth checked in handler
	api.HandleFunc("/content/replace-execute", h.ReplaceExecute).Methods("POST")   // Auth checked in handler
	api.HandleFunc("/tools/broken-links/scan", h.BrokenLinkScan).Methods("GET")    // Auth checked in handler
	api.HandleFunc("/tools/fix-link", h.FixBrokenLink).Methods("POST")             // Auth checked in handler
	api.HandleFunc("/search", h.EndUserSearch).Methods("GET")                       // Public end-user search
	api.HandleFunc("/search/suggest", h.EndUserSearchSuggest).Methods("GET")       // Public typeahead suggestions

	// OAuth 2.1 endpoints (no auth middleware — these implement their own auth)
	r.HandleFunc("/oauth/register", oauthHandler.Register).Methods("POST")
	r.HandleFunc("/oauth/authorize", oauthHandler.Authorize).Methods("GET")
	r.HandleFunc("/oauth/authorize", oauthHandler.AuthorizeSubmit).Methods("POST")
	r.HandleFunc("/oauth/token", oauthHandler.Token).Methods("POST")
	r.HandleFunc("/oauth/revoke", oauthHandler.TokenRevocation).Methods("POST")

	// MCP Streamable HTTP endpoint (API key + OAuth authenticated)
	mcpHandler := lightmcp.NewHTTPHandler(cfg.Port)
	mcpSubrouter := r.PathPrefix("/mcp").Subrouter()
	mcpSubrouter.Use(apiAuthMiddleware.Middleware)
	mcpSubrouter.Handle("", mcpHandler)

	// Public asset serving
	r.PathPrefix("/assets/").HandlerFunc(h.ServeAsset).Methods("GET")

	// Simple health check for Fly.io TCP probes
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Structured healthz endpoint (vibectl VibeCtl Health Check Protocol)
	r.HandleFunc("/healthz", h.Healthz).Methods("GET")

	// OAuth well-known metadata endpoints (RFC 9728 + RFC 8414)
	r.HandleFunc("/.well-known/oauth-protected-resource", oauth.ProtectedResourceMetadataHandler(cfg.BaseURL)).Methods("GET", "OPTIONS")
	r.HandleFunc("/.well-known/oauth-authorization-server", oauth.AuthorizationServerMetadataHandler(cfg.BaseURL)).Methods("GET", "OPTIONS")
	r.HandleFunc("/oauth/jwks", oauth.JWKSHandler()).Methods("GET")

	// MCP well-known server card (dynamic, includes full tool schemas)
	serverCardJSON, err := lightmcp.ServerCard(cfg.BaseURL)
	if err != nil {
		log.Printf("Warning: could not generate MCP server card: %v", err)
	}
	r.HandleFunc("/.well-known/mcp/server-card.json", func(w http.ResponseWriter, rr *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if serverCardJSON != nil {
			w.Write(serverCardJSON)
		} else {
			http.ServeFile(w, rr, ".well-known/mcp/server-card.json")
		}
	}).Methods("GET")

	// Sitemap and robots.txt
	r.HandleFunc("/sitemap.xml", h.ServeSitemap).Methods("GET")
	r.HandleFunc("/robots.txt", h.ServeRobotsTxt).Methods("GET")

	// Public content routes - must be last
	r.HandleFunc("/", h.ServePage).Methods("GET")
	r.HandleFunc("/{slug:.*}", h.ServePage).Methods("GET")

	// Create server
	// Note: WriteTimeout is set high (5 min) to support SSE streaming endpoints like broken link scanner
	// which can take several minutes to complete scanning all pages
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  5 * time.Minute,
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
