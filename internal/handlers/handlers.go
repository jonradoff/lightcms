package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/build"
	"lightcms/internal/database"
	"lightcms/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Handler struct {
	db      *database.DB
	auth    *auth.Manager
	baseURL string
}

func New(db *database.DB, authManager *auth.Manager, baseURL string) *Handler {
	return &Handler{db: db, auth: authManager, baseURL: baseURL}
}

// SeedDefaults creates default templates and hello world page if they don't exist
func (h *Handler) SeedDefaults(ctx context.Context) error {
	// Seed default templates
	for _, tmpl := range models.DefaultTemplates {
		count, err := h.db.Count(ctx, "templates", bson.M{"slug": tmpl.Slug})
		if err != nil {
			return err
		}
		if count == 0 {
			tmpl.CreatedAt = time.Now()
			tmpl.UpdatedAt = time.Now()
			if _, err := h.db.InsertOne(ctx, "templates", tmpl); err != nil {
				return err
			}
		}
	}

	// Create Hello World page if no content exists
	count, err := h.db.Count(ctx, "content", bson.M{})
	if err != nil {
		return err
	}
	if count == 0 {
		// Get the explanatory page template
		var tmpl models.Template
		err := h.db.FindOne(ctx, "templates", bson.M{"slug": "explanatory-page"}, &tmpl)
		if err != nil {
			return err
		}

		now := time.Now()
		helloWorld := models.Content{
			TemplateID:   tmpl.ID,
			TemplateName: tmpl.Name,
			Title:        "Welcome to LightCMS",
			Slug:         "",
			Category:     "pages",
			Published:    true,
			PublishedAt:  &now,
			UseHeader:    true,
			UseFooter:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
			Data: map[string]interface{}{
				"title":    "Welcome to LightCMS",
				"subtitle": "A lightweight, modern content management system",
				"intro":    "<p>Your new website is ready to be customized. LightCMS makes it easy to create beautiful, fast websites with a powerful template system.</p>",
				"main_content": `<h2>Getting Started</h2>
<p>Welcome to your new LightCMS installation! Here's how to get started:</p>
<ol>
<li><strong>Access the Admin Panel</strong> - Visit <a href="/cm">/cm</a> to log in and manage your content.</li>
<li><strong>Create Content</strong> - Use templates to create blog posts, press releases, or custom pages.</li>
<li><strong>Customize Your Theme</strong> - Adjust colors, fonts, and styling to match your brand.</li>
<li><strong>Build Collections</strong> - Group your content into browsable collections.</li>
</ol>
<h2>Features</h2>
<ul>
<li>📝 <strong>Template System</strong> - Define reusable content structures</li>
<li>🎨 <strong>Theme Customization</strong> - Modern, sleek design with full control</li>
<li>⚡ <strong>Static Generation</strong> - Fast page loads from pre-rendered HTML</li>
<li>📱 <strong>Responsive Design</strong> - Beautiful on all devices</li>
<li>🔒 <strong>Secure Admin</strong> - Password-protected content management</li>
</ul>`,
				"cta_text": "Go to Admin Panel",
				"cta_link": "/cm",
			},
		}

		if _, err := h.db.InsertOne(ctx, "content", helloWorld); err != nil {
			return err
		}

		// Generate static page
		if err := h.generateStaticPage(ctx, &helloWorld, &tmpl); err != nil {
			return err
		}
	}

	// Create default blog collection
	collCount, err := h.db.Count(ctx, "collections", bson.M{})
	if err != nil {
		return err
	}
	if collCount == 0 {
		blogCollection := models.Collection{
			Name:        "Blog",
			Slug:        "blog",
			Description: "All blog posts",
			Category:    "blog",
			SortField:   "published_at",
			SortOrder:   "desc",
			ItemsPerPage: 10,
			ItemTemplate: `<article class="collection-item">
	<a href="/{{.slug}}">
		{{if .featured_image}}<img src="{{.featured_image}}" alt="{{.title}}" class="item-image">{{end}}
		<h3>{{.title}}</h3>
		{{if .excerpt}}<p class="excerpt">{{.excerpt}}</p>{{end}}
		<time>{{.published_at}}</time>
	</a>
</article>`,
			PageTemplate: `<div class="collection-page">
	<header class="collection-header">
		<h1>{{.collection_name}}</h1>
		<p>{{.collection_description}}</p>
	</header>
	<div class="collection-items">
		{{.items}}
	</div>
	{{.pagination}}
</div>`,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if _, err := h.db.InsertOne(ctx, "collections", blogCollection); err != nil {
			return err
		}
	}

	// Ensure default pages exist
	h.ensureDefaultPages(ctx)

	// Set default header/footer in theme if not set
	theme, _ := h.db.GetThemeSettings(ctx)
	if theme.FooterHTML == "" {
		theme.FooterHTML = `<p>Created with LightCMS, licensed under <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank">CC BY 4.0</a></p>
<p>&copy; 2026 <a href="https://metavert.io" target="_blank">Metavert LLC</a></p>`
		h.db.SaveThemeSettings(ctx, theme)
	}

	return nil
}

// ensureDefaultPages checks for and creates default pages like 404
func (h *Handler) ensureDefaultPages(ctx context.Context) {
	// Check for 404 page in content
	var existing404 models.Content
	err := h.db.FindOne(ctx, "content", bson.M{"slug": "404"}, &existing404)
	if err != nil {
		// Get the explanatory page template
		var tmpl models.Template
		if err := h.db.FindOne(ctx, "templates", bson.M{"slug": "explanatory-page"}, &tmpl); err != nil {
			return
		}

		now := time.Now()
		page404 := models.Content{
			TemplateID:   tmpl.ID,
			TemplateName: tmpl.Name,
			Title:        "Page Not Found",
			Slug:         "404",
			Category:     "pages",
			Published:    true,
			PublishedAt:  &now,
			UseHeader:    true,
			UseFooter:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
			Data: map[string]interface{}{
				"title":    "404",
				"subtitle": "Page Not Found",
				"intro":    "",
				"main_content": `<div class="error-page">
<p>The page you're looking for doesn't exist or has been moved.</p>
<a href="/" class="cta-button">Go Home</a>
</div>
<style>
.error-page {
	text-align: center;
	padding: 2rem 0;
}
.error-page p {
	color: rgba(241, 245, 249, 0.7);
	margin-bottom: 2rem;
	font-size: 1.2rem;
}
</style>`,
				"cta_text": "",
				"cta_link": "",
			},
		}

		if id, err := h.db.InsertOne(ctx, "content", page404); err == nil {
			page404.ID = id
			h.generateStaticPage(ctx, &page404, &tmpl)
		}
	}
}

// LoginPage renders the login form
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	data := make(map[string]interface{})

	// Check if rate limited
	if locked, duration := h.auth.CheckRateLimit(ctx, r); locked {
		data["Error"] = fmt.Sprintf("Too many login attempts. Please try again in %s.", duration)
		data["RateLimited"] = true
	}

	h.renderAdmin(w, r, "login", data)
}

// LoginHandler processes login
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check rate limit first
	if locked, duration := h.auth.CheckRateLimit(ctx, r); locked {
		h.renderAdmin(w, r, "login", map[string]interface{}{
			"Error":       fmt.Sprintf("Too many login attempts. Please try again in %s.", duration),
			"RateLimited": true,
		})
		return
	}

	password := r.FormValue("password")
	if h.auth.ValidatePassword(ctx, password) {
		h.auth.ClearRateLimit(ctx, r)
		if err := h.auth.Login(w, r); err != nil {
			http.Error(w, "Login failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	// Record failed attempt
	h.auth.RecordFailedLogin(ctx, r)

	// Check if now rate limited after this attempt
	errorMsg := "Invalid password"
	rateLimited := false
	if locked, duration := h.auth.CheckRateLimit(ctx, r); locked {
		errorMsg = fmt.Sprintf("Invalid password. Too many attempts - locked for %s.", duration)
		rateLimited = true
	}

	h.renderAdmin(w, r, "login", map[string]interface{}{
		"Error":       errorMsg,
		"RateLimited": rateLimited,
	})
}

// LogoutHandler logs out the user
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	h.auth.Logout(w, r)
	http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
}

// AdminDashboard shows the main admin page
func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	contentCount, _ := h.db.Count(ctx, "content", bson.M{})
	templateCount, _ := h.db.Count(ctx, "templates", bson.M{})
	collectionCount, _ := h.db.Count(ctx, "collections", bson.M{})

	// Get recent content
	cursor, _ := h.db.FindMany(ctx, "content", bson.M{}, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(5))
	var recentContent []models.Content
	if cursor != nil {
		cursor.All(ctx, &recentContent)
	}

	// Check if using default password
	isDefaultPassword := h.auth.IsDefaultPassword(ctx)

	h.renderAdmin(w, r, "dashboard", map[string]interface{}{
		"ContentCount":      contentCount,
		"TemplateCount":     templateCount,
		"CollectionCount":   collectionCount,
		"RecentContent":     recentContent,
		"IsDefaultPassword": isDefaultPassword,
	})
}

// Template handlers
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "templates", bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var templates []models.Template
	if err := cursor.All(ctx, &templates); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "templates_list", map[string]interface{}{"Templates": templates})
}

func (h *Handler) NewTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	h.renderAdmin(w, r, "template_form", map[string]interface{}{"IsNew": true})
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	tmpl := h.parseTemplateForm(r)
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()

	ctx := r.Context()
	if _, err := h.db.InsertOne(ctx, "templates", tmpl); err != nil {
		h.renderAdmin(w, r, "template_form", map[string]interface{}{"IsNew": true, "Error": err.Error(), "Template": tmpl})
		return
	}

	http.Redirect(w, r, "/cm/templates", http.StatusSeeOther)
}

func (h *Handler) EditTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var tmpl models.Template
	if err := h.db.FindOne(r.Context(), "templates", bson.M{"_id": id}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	h.renderAdmin(w, r, "template_form", map[string]interface{}{"IsNew": false, "Template": tmpl})
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	tmpl := h.parseTemplateForm(r)
	tmpl.UpdatedAt = time.Now()

	ctx := r.Context()
	if err := h.db.UpdateOne(ctx, "templates", bson.M{"_id": id}, bson.M{"$set": tmpl}); err != nil {
		h.renderAdmin(w, r, "template_form", map[string]interface{}{"IsNew": false, "Error": err.Error(), "Template": tmpl})
		return
	}

	// Regenerate all content using this template
	h.regenerateContentByTemplate(ctx, id)

	http.Redirect(w, r, "/cm/templates", http.StatusSeeOther)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Check if it's a system template
	var tmpl models.Template
	if err := h.db.FindOne(r.Context(), "templates", bson.M{"_id": id}, &tmpl); err == nil && tmpl.IsSystem {
		http.Error(w, "Cannot delete system templates", http.StatusForbidden)
		return
	}

	if err := h.db.DeleteOne(r.Context(), "templates", bson.M{"_id": id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/templates", http.StatusSeeOther)
}

func (h *Handler) parseTemplateForm(r *http.Request) models.Template {
	r.ParseForm()

	var fields []models.TemplateField
	fieldNames := r.Form["field_name[]"]
	fieldLabels := r.Form["field_label[]"]
	fieldTypes := r.Form["field_type[]"]
	fieldRequired := r.Form["field_required[]"]
	fieldPlaceholders := r.Form["field_placeholder[]"]
	fieldOptions := r.Form["field_options[]"]

	for i := range fieldNames {
		if fieldNames[i] == "" {
			continue
		}
		required := false
		if i < len(fieldRequired) && fieldRequired[i] == "on" {
			required = true
		}
		placeholder := ""
		if i < len(fieldPlaceholders) {
			placeholder = fieldPlaceholders[i]
		}
		opts := ""
		if i < len(fieldOptions) {
			opts = fieldOptions[i]
		}
		fields = append(fields, models.TemplateField{
			Name:        fieldNames[i],
			Label:       fieldLabels[i],
			Type:        fieldTypes[i],
			Required:    required,
			Placeholder: placeholder,
			Options:     opts,
		})
	}

	return models.Template{
		Name:        r.FormValue("name"),
		Slug:        slugify(r.FormValue("name")),
		Description: r.FormValue("description"),
		Category:    r.FormValue("category"),
		HTMLLayout:  r.FormValue("html_layout"),
		Fields:      fields,
	}
}

// Content handlers
func (h *Handler) ListContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	// Ensure default pages exist
	h.ensureDefaultPages(ctx)

	// Get filter parameters
	showDeleted := r.URL.Query().Get("deleted") == "true"
	folderFilter := r.URL.Query().Get("folder")

	// Build query filter
	filter := bson.M{}
	if showDeleted {
		filter["deleted"] = true
	} else {
		filter["deleted"] = bson.M{"$ne": true}
	}

	if folderFilter != "" && folderFilter != "all" {
		if folderFilter == "root" {
			// Root level: no folder or empty folder path
			filter["$or"] = []bson.M{
				{"folder_path": ""},
				{"folder_path": bson.M{"$exists": false}},
			}
		} else {
			filter["folder_path"] = folderFilter
		}
	}

	cursor, err := h.db.FindMany(ctx, "content", filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var content []models.Content
	if err := cursor.All(ctx, &content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get all folders for the filter dropdown
	folderCursor, err := h.db.FindMany(ctx, "folders", bson.M{}, options.Find().SetSort(bson.D{{Key: "path", Value: 1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var folders []models.Folder
	folderCursor.All(ctx, &folders)

	h.renderAdmin(w, r, "content_list", map[string]interface{}{
		"Content":      content,
		"Folders":      folders,
		"ShowDeleted":  showDeleted,
		"FolderFilter": folderFilter,
	})
}

func (h *Handler) NewContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "templates", bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var templates []models.Template
	cursor.All(ctx, &templates)

	h.renderAdmin(w, r, "content_select_template", map[string]interface{}{"Templates": templates})
}

func (h *Handler) NewContentWithTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	templateID, err := primitive.ObjectIDFromHex(vars["templateID"])
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": templateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	folders := h.getAllFolders(ctx)

	h.renderAdmin(w, r, "content_form", map[string]interface{}{
		"IsNew":    true,
		"Template": tmpl,
		"Folders":  folders,
	})
}

func (h *Handler) CreateContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(32 << 20)

	templateID, err := primitive.ObjectIDFromHex(r.FormValue("template_id"))
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": templateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Parse field data
	data := make(map[string]interface{})
	for _, field := range tmpl.Fields {
		if field.Type == "image" {
			// Handle file upload
			file, header, err := r.FormFile("field_" + field.Name)
			if err == nil {
				defer file.Close()
				filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
				filepath := filepath.Join("static/uploads", filename)
				dst, err := os.Create(filepath)
				if err == nil {
					defer dst.Close()
					io.Copy(dst, file)
					data[field.Name] = "/uploads/" + filename
				}
			}
		} else {
			data[field.Name] = r.FormValue("field_" + field.Name)
		}
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = slugify(title)
	}

	// Handle folder selection
	folderIDStr := r.FormValue("folder_id")
	var folderID *primitive.ObjectID
	folderPath := ""
	fullPath := "/" + slug
	if slug == "" {
		fullPath = "/"
	}

	if folderIDStr != "" && folderIDStr != "root" {
		fid, err := primitive.ObjectIDFromHex(folderIDStr)
		if err == nil {
			folderID = &fid
			var folder models.Folder
			if err := h.db.FindOne(ctx, "folders", bson.M{"_id": fid}, &folder); err == nil {
				folderPath = folder.Path
				if slug == "" {
					fullPath = folderPath
				} else {
					fullPath = folderPath + "/" + slug
				}
			}
		}
	}

	published := r.FormValue("published") == "on"
	var publishedAt *time.Time
	if published {
		now := time.Now()
		publishedAt = &now
	}

	// For blank pages, check if raw mode is enabled
	// Check if checkboxes are checked
	// Note: With hidden+checkbox pattern, we need to check if "on" is in the values
	rawMode := false
	useHeader := false
	useFooter := false
	useTheme := false
	if values, ok := r.Form["raw_mode"]; ok {
		for _, v := range values {
			if v == "on" {
				rawMode = true
				break
			}
		}
	}
	if values, ok := r.Form["use_header"]; ok {
		for _, v := range values {
			if v == "on" {
				useHeader = true
				break
			}
		}
	}
	if values, ok := r.Form["use_footer"]; ok {
		for _, v := range values {
			if v == "on" {
				useFooter = true
				break
			}
		}
	}
	if values, ok := r.Form["use_theme"]; ok {
		for _, v := range values {
			if v == "on" {
				useTheme = true
				break
			}
		}
	}

	// Handle SEO fields
	metaDescription := r.FormValue("meta_description")
	ogImage := ""
	// Handle OG image upload
	ogFile, ogHeader, err := r.FormFile("og_image")
	if err == nil {
		defer ogFile.Close()
		filename := fmt.Sprintf("%d_og_%s", time.Now().UnixNano(), ogHeader.Filename)
		ogFilepath := filepath.Join("static/uploads", filename)
		dst, err := os.Create(ogFilepath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, ogFile)
			ogImage = "/uploads/" + filename
		}
	}

	content := models.Content{
		TemplateID:      templateID,
		TemplateName:    tmpl.Name,
		Title:           title,
		Slug:            slug,
		FolderID:        folderID,
		FolderPath:      folderPath,
		FullPath:        fullPath,
		Category:        tmpl.Category,
		MetaDescription: metaDescription,
		OGImage:         ogImage,
		Data:            data,
		Published:       published,
		PublishedAt:     publishedAt,
		UseHeader:       useHeader,
		UseFooter:       useFooter,
		UseTheme:        useTheme,
		RawMode:         rawMode,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Extract and track internal links
	content.InternalLinks = h.extractInternalLinksFromContent(&content)

	id, err := h.db.InsertOne(ctx, "content", content)
	if err != nil {
		h.renderAdmin(w, r, "content_form", map[string]interface{}{
			"IsNew":    true,
			"Template": tmpl,
			"Content":  content,
			"Error":    err.Error(),
		})
		return
	}

	content.ID = id

	// Save the initial version (v1)
	if err := h.saveContentVersion(ctx, &content); err != nil {
		fmt.Printf("Warning: Failed to save initial content version: %v\n", err)
	}

	// Generate static page
	if published {
		h.generateStaticPage(ctx, &content, &tmpl)
	}

	// Regenerate sitemap after content creation
	go h.RegenerateSitemap(context.Background())

	http.Redirect(w, r, "/cm/content", http.StatusSeeOther)
}

func (h *Handler) EditContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var content models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	folders := h.getAllFolders(ctx)

	// Get version history for this content
	versionCursor, err := h.db.FindMany(ctx, "content_versions", bson.M{"content_id": id}, options.Find().SetSort(bson.D{{Key: "version", Value: -1}}).SetLimit(20))
	var versions []models.ContentVersion
	if err == nil {
		versionCursor.All(ctx, &versions)
	}

	// Get same-slug historical pages (different document IDs but same slug, including deleted)
	var sameSlugPages []models.Content
	if content.Slug != "" {
		sameSlugCursor, err := h.db.FindMany(ctx, "content", bson.M{
			"slug": content.Slug,
			"_id":  bson.M{"$ne": id},
		}, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
		if err == nil {
			sameSlugCursor.All(ctx, &sameSlugPages)
		}
	}

	// Check for error query param (e.g., path conflict on undelete)
	errorMsg := ""
	if r.URL.Query().Get("error") == "path_conflict" {
		errorMsg = "Cannot restore: another page already exists at this URL path. Change the slug first or delete the conflicting page."
	}

	h.renderAdmin(w, r, "content_form", map[string]interface{}{
		"IsNew":         false,
		"Template":      tmpl,
		"Content":       content,
		"Folders":       folders,
		"Versions":      versions,
		"SameSlugPages": sameSlugPages,
		"Error":         errorMsg,
	})
}

func (h *Handler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	r.ParseMultipartForm(32 << 20)

	ctx := r.Context()
	var existingContent models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"_id": id}, &existingContent); err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": existingContent.TemplateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Parse field data
	data := make(map[string]interface{})
	for _, field := range tmpl.Fields {
		if field.Type == "image" {
			file, header, err := r.FormFile("field_" + field.Name)
			if err == nil {
				defer file.Close()
				filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
				filepath := filepath.Join("static/uploads", filename)
				dst, err := os.Create(filepath)
				if err == nil {
					defer dst.Close()
					io.Copy(dst, file)
					data[field.Name] = "/uploads/" + filename
				}
			} else if existingData, ok := existingContent.Data[field.Name]; ok {
				data[field.Name] = existingData
			}
		} else {
			data[field.Name] = r.FormValue("field_" + field.Name)
		}
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")

	// Preserve empty slug for homepage (full_path "/")
	if slug == "" && existingContent.FullPath == "/" {
		// Keep slug empty for homepage
	} else if slug == "" {
		slug = slugify(title)
	}

	// Handle folder selection
	folderIDStr := r.FormValue("folder_id")
	var folderID *primitive.ObjectID
	folderPath := ""
	fullPath := "/" + slug
	if slug == "" {
		fullPath = "/"
	}

	if folderIDStr != "" && folderIDStr != "root" {
		fid, err := primitive.ObjectIDFromHex(folderIDStr)
		if err == nil {
			folderID = &fid
			var folder models.Folder
			if err := h.db.FindOne(ctx, "folders", bson.M{"_id": fid}, &folder); err == nil {
				folderPath = folder.Path
				if slug == "" {
					fullPath = folderPath
				} else {
					fullPath = folderPath + "/" + slug
				}
			}
		}
	}

	published := r.FormValue("published") == "on"
	var publishedAt *time.Time
	if published && existingContent.PublishedAt == nil {
		now := time.Now()
		publishedAt = &now
	} else {
		publishedAt = existingContent.PublishedAt
	}

	// Track old full path for dependency updates and file cleanup
	oldFullPath := existingContent.FullPath
	if oldFullPath == "" {
		// Legacy content without full_path
		oldFullPath = "/" + existingContent.Slug
		if existingContent.Slug == "" {
			oldFullPath = "/"
		}
	}

	// Delete old static file if path changed
	if oldFullPath != fullPath {
		oldStaticPath := h.getStaticFilePath(oldFullPath)
		os.Remove(oldStaticPath)
	}

	// Check if checkboxes are checked
	// Note: With hidden+checkbox pattern, we need to check if "on" is in the values
	rawMode := false
	useHeader := false
	useFooter := false
	useTheme := false
	if values, ok := r.Form["raw_mode"]; ok {
		for _, v := range values {
			if v == "on" {
				rawMode = true
				break
			}
		}
	}
	if values, ok := r.Form["use_header"]; ok {
		for _, v := range values {
			if v == "on" {
				useHeader = true
				break
			}
		}
	}
	if values, ok := r.Form["use_footer"]; ok {
		for _, v := range values {
			if v == "on" {
				useFooter = true
				break
			}
		}
	}
	if values, ok := r.Form["use_theme"]; ok {
		for _, v := range values {
			if v == "on" {
				useTheme = true
				break
			}
		}
	}

	// Handle SEO fields
	metaDescription := r.FormValue("meta_description")
	ogImage := existingContent.OGImage // Keep existing if not uploading new
	// Handle OG image upload
	ogFile, ogHeader, err := r.FormFile("og_image")
	if err == nil {
		defer ogFile.Close()
		filename := fmt.Sprintf("%d_og_%s", time.Now().UnixNano(), ogHeader.Filename)
		ogFilepath := filepath.Join("static/uploads", filename)
		dst, err := os.Create(ogFilepath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, ogFile)
			ogImage = "/uploads/" + filename
		}
	}

	// Extract internal links from updated data
	tempContent := &models.Content{Data: data}
	internalLinks := h.extractInternalLinksFromContent(tempContent)

	update := bson.M{
		"$set": bson.M{
			"title":            title,
			"slug":             slug,
			"folder_id":        folderID,
			"folder_path":      folderPath,
			"full_path":        fullPath,
			"meta_description": metaDescription,
			"og_image":         ogImage,
			"data":             data,
			"published":        published,
			"published_at":     publishedAt,
			"use_header":       useHeader,
			"use_footer":       useFooter,
			"use_theme":        useTheme,
			"raw_mode":         rawMode,
			"internal_links":   internalLinks,
			"updated_at":       time.Now(),
		},
	}

	if err := h.db.UpdateOne(ctx, "content", bson.M{"_id": id}, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If full path changed, update all dependent content links
	if oldFullPath != fullPath {
		if err := h.updateDependentContentByPath(ctx, oldFullPath, fullPath); err != nil {
			// Log but don't fail the request
			fmt.Printf("Warning: Failed to update dependent content: %v\n", err)
		}
	}

	// Update content struct with new values for static generation and version saving
	existingContent.Title = title
	existingContent.Slug = slug
	existingContent.FolderID = folderID
	existingContent.FolderPath = folderPath
	existingContent.FullPath = fullPath
	existingContent.MetaDescription = metaDescription
	existingContent.OGImage = ogImage
	existingContent.Data = data
	existingContent.Published = published
	existingContent.PublishedAt = publishedAt
	existingContent.UseTheme = useTheme
	existingContent.RawMode = rawMode
	existingContent.UseHeader = useHeader
	existingContent.UseFooter = useFooter

	// Save this version after the update succeeds
	if err := h.saveContentVersion(ctx, &existingContent); err != nil {
		fmt.Printf("Warning: Failed to save content version: %v\n", err)
	}

	if published {
		h.generateStaticPage(ctx, &existingContent, &tmpl)
	} else {
		// Remove static file if unpublished
		staticPath := filepath.Join("content/generated", slug+".html")
		os.Remove(staticPath)
	}

	// Regenerate sitemap after content update
	go h.RegenerateSitemap(context.Background())

	http.Redirect(w, r, "/cm/content", http.StatusSeeOther)
}

func (h *Handler) DeleteContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var content models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	// Delete static file using full path
	staticPath := h.getStaticFilePath(content.FullPath)
	if staticPath == "" {
		// Fallback for legacy content
		staticPath = filepath.Join("content/generated", content.Slug+".html")
	}
	os.Remove(staticPath)

	// Soft delete: mark as deleted instead of removing from database
	// Use a unique deleted path to avoid unique index conflicts
	now := time.Now()
	deletedPath := fmt.Sprintf("__deleted__/%s/%d", id.Hex(), now.UnixNano())
	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"deleted_at": now,
			"published":  false,     // Unpublish when deleting
			"full_path":  deletedPath, // Unique path for deleted items
			"updated_at": now,
		},
	}

	if err := h.db.UpdateOne(ctx, "content", bson.M{"_id": id}, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Regenerate sitemap after content deletion
	go h.RegenerateSitemap(context.Background())

	http.Redirect(w, r, "/cm/content", http.StatusSeeOther)
}

func (h *Handler) UndeleteContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var content models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	// Reconstruct full_path
	fullPath := "/" + content.Slug
	if content.Slug == "" {
		fullPath = "/"
	}
	if content.FolderPath != "" {
		if content.Slug == "" {
			fullPath = content.FolderPath
		} else {
			fullPath = content.FolderPath + "/" + content.Slug
		}
	}

	// Check if another content exists at this path
	var existingContent models.Content
	err = h.db.FindOne(ctx, "content", bson.M{
		"full_path": fullPath,
		"deleted":   bson.M{"$ne": true},
		"_id":       bson.M{"$ne": id},
	}, &existingContent)
	if err == nil {
		// Another content exists at this path - redirect with error
		http.Redirect(w, r, "/cm/content/"+id.Hex()+"?error=path_conflict", http.StatusSeeOther)
		return
	}

	// Restore content
	update := bson.M{
		"$set": bson.M{
			"deleted":    false,
			"full_path":  fullPath,
			"updated_at": time.Now(),
		},
		"$unset": bson.M{
			"deleted_at": "",
		},
	}

	if err := h.db.UpdateOne(ctx, "content", bson.M{"_id": id}, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/content/"+id.Hex(), http.StatusSeeOther)
}

func (h *Handler) ListContentVersions(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	cursor, err := h.db.FindMany(ctx, "content_versions", bson.M{"content_id": id}, options.Find().SetSort(bson.D{{Key: "version", Value: -1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var versions []models.ContentVersion
	if err := cursor.All(ctx, &versions); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

func (h *Handler) ViewContentVersion(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	contentID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	var version models.ContentVersion
	versionStr := vars["version"]

	if versionStr == "latest" {
		// Get the latest version
		cursor, err := h.db.FindMany(ctx, "content_versions", bson.M{"content_id": contentID}, options.Find().SetSort(bson.D{{Key: "version", Value: -1}}).SetLimit(1))
		if err != nil {
			http.Error(w, "Error finding versions", http.StatusInternalServerError)
			return
		}
		var versions []models.ContentVersion
		if err := cursor.All(ctx, &versions); err != nil || len(versions) == 0 {
			http.Error(w, "No versions found", http.StatusNotFound)
			return
		}
		version = versions[0]
	} else {
		versionNum, err := strconv.Atoi(versionStr)
		if err != nil {
			http.Error(w, "Invalid version number", http.StatusBadRequest)
			return
		}
		if err := h.db.FindOne(ctx, "content_versions", bson.M{"content_id": contentID, "version": versionNum}, &version); err != nil {
			http.Error(w, "Version not found", http.StatusNotFound)
			return
		}
	}

	// Get the template to render the content
	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": version.TemplateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Create a temporary content object for rendering
	tempContent := models.Content{
		ID:           contentID,
		TemplateID:   version.TemplateID,
		TemplateName: version.TemplateName,
		Title:        version.Title,
		Slug:         version.Slug,
		Data:         version.Data,
		UseHeader:    version.UseHeader,
		UseFooter:    version.UseFooter,
		UseTheme:     version.UseTheme,
	}

	// Render the content using the template
	rendered := h.renderContent(&tempContent, &tmpl)

	// Render with preview banner
	theme, _ := h.db.GetThemeSettings(ctx)

	previewBanner := fmt.Sprintf(`<div style="background: #f59e0b; color: #000; padding: 1rem; text-align: center; position: fixed; top: 0; left: 0; right: 0; z-index: 9999;">
		<strong>Version Preview</strong> - Viewing version %d of "%s" (saved %s)
		<a href="/cm/content/%s" style="margin-left: 1rem; color: #000; text-decoration: underline;">← Back to Editor</a>
	</div><div style="padding-top: 50px;">`, version.Version, version.Title, version.CreatedAt.Format("Jan 2, 2006 3:04 PM"), contentID.Hex())

	h.renderPublicWithOptions(w, r, theme, previewBanner+rendered+"</div>", version.UseHeader, version.UseFooter)
}

func (h *Handler) DiffContentVersion(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	contentID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get the version to compare
	var version models.ContentVersion
	versionStr := vars["version"]

	if versionStr == "latest" {
		cursor, err := h.db.FindMany(ctx, "content_versions", bson.M{"content_id": contentID}, options.Find().SetSort(bson.D{{Key: "version", Value: -1}}).SetLimit(1))
		if err != nil {
			http.Error(w, "Error finding versions", http.StatusInternalServerError)
			return
		}
		var versions []models.ContentVersion
		if err := cursor.All(ctx, &versions); err != nil || len(versions) == 0 {
			http.Error(w, "No versions found", http.StatusNotFound)
			return
		}
		version = versions[0]
	} else {
		versionNum, err := strconv.Atoi(versionStr)
		if err != nil {
			http.Error(w, "Invalid version number", http.StatusBadRequest)
			return
		}
		if err := h.db.FindOne(ctx, "content_versions", bson.M{"content_id": contentID, "version": versionNum}, &version); err != nil {
			http.Error(w, "Version not found", http.StatusNotFound)
			return
		}
	}

	// Get current content
	var currentContent models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"_id": contentID}, &currentContent); err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	// Render the diff page
	h.renderAdmin(w, r, "version_diff", map[string]interface{}{
		"Version": version,
		"Current": currentContent,
	})
}

func (h *Handler) RevertContentVersion(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	contentID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	versionNum, err := strconv.Atoi(vars["version"])
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get the version to revert to
	var version models.ContentVersion
	if err := h.db.FindOne(ctx, "content_versions", bson.M{"content_id": contentID, "version": versionNum}, &version); err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	// Revert to the selected version
	update := bson.M{
		"$set": bson.M{
			"template_id":      version.TemplateID,
			"template_name":    version.TemplateName,
			"title":            version.Title,
			"slug":             version.Slug,
			"folder_id":        version.FolderID,
			"folder_path":      version.FolderPath,
			"full_path":        version.FullPath,
			"category":         version.Category,
			"meta_description": version.MetaDescription,
			"og_image":         version.OGImage,
			"data":             version.Data,
			"use_header":       version.UseHeader,
			"use_footer":       version.UseFooter,
			"use_theme":        version.UseTheme,
			"raw_mode":         version.RawMode,
			"updated_at":       time.Now(),
		},
	}

	if err := h.db.UpdateOne(ctx, "content", bson.M{"_id": contentID}, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the reverted content and save it as a new version
	var revertedContent models.Content
	h.db.FindOne(ctx, "content", bson.M{"_id": contentID}, &revertedContent)
	if err := h.saveContentVersion(ctx, &revertedContent); err != nil {
		fmt.Printf("Warning: Failed to save content version after revert: %v\n", err)
	}

	// Regenerate static file if published
	if revertedContent.Published {
		var tmpl models.Template
		if err := h.db.FindOne(ctx, "templates", bson.M{"_id": revertedContent.TemplateID}, &tmpl); err == nil {
			h.generateStaticPage(ctx, &revertedContent, &tmpl)
		}
	}

	http.Redirect(w, r, "/cm/content/"+contentID.Hex(), http.StatusSeeOther)
}

func (h *Handler) RegenerateContent(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var content models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	if content.Published {
		h.generateStaticPage(ctx, &content, &tmpl)
	}

	http.Redirect(w, r, "/cm/content", http.StatusSeeOther)
}

// Collection handlers
func (h *Handler) ListCollections(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "collections", bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var collections []models.Collection
	cursor.All(ctx, &collections)

	h.renderAdmin(w, r, "collections_list", map[string]interface{}{"Collections": collections})
}

func (h *Handler) NewCollection(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	h.renderAdmin(w, r, "collection_form", map[string]interface{}{"IsNew": true})
}

func (h *Handler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	itemsPerPage := 10
	fmt.Sscanf(r.FormValue("items_per_page"), "%d", &itemsPerPage)

	collection := models.Collection{
		Name:         r.FormValue("name"),
		Slug:         slugify(r.FormValue("name")),
		Description:  r.FormValue("description"),
		Category:     r.FormValue("category"),
		SortField:    r.FormValue("sort_field"),
		SortOrder:    r.FormValue("sort_order"),
		ItemTemplate: r.FormValue("item_template"),
		PageTemplate: r.FormValue("page_template"),
		ItemsPerPage: itemsPerPage,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	ctx := r.Context()
	if _, err := h.db.InsertOne(ctx, "collections", collection); err != nil {
		h.renderAdmin(w, r, "collection_form", map[string]interface{}{
			"IsNew":      true,
			"Collection": collection,
			"Error":      err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/cm/collections", http.StatusSeeOther)
}

func (h *Handler) EditCollection(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var collection models.Collection
	if err := h.db.FindOne(r.Context(), "collections", bson.M{"_id": id}, &collection); err != nil {
		http.Error(w, "Collection not found", http.StatusNotFound)
		return
	}

	h.renderAdmin(w, r, "collection_form", map[string]interface{}{"IsNew": false, "Collection": collection})
}

func (h *Handler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	itemsPerPage := 10
	fmt.Sscanf(r.FormValue("items_per_page"), "%d", &itemsPerPage)

	update := bson.M{
		"$set": bson.M{
			"name":          r.FormValue("name"),
			"slug":          slugify(r.FormValue("name")),
			"description":   r.FormValue("description"),
			"category":      r.FormValue("category"),
			"sort_field":    r.FormValue("sort_field"),
			"sort_order":    r.FormValue("sort_order"),
			"item_template": r.FormValue("item_template"),
			"page_template": r.FormValue("page_template"),
			"items_per_page": itemsPerPage,
			"updated_at":    time.Now(),
		},
	}

	if err := h.db.UpdateOne(r.Context(), "collections", bson.M{"_id": id}, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/collections", http.StatusSeeOther)
}

func (h *Handler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteOne(r.Context(), "collections", bson.M{"_id": id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/collections", http.StatusSeeOther)
}

// Folder handlers
func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "folders", bson.M{}, options.Find().SetSort(bson.D{{Key: "path", Value: 1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var folders []models.Folder
	cursor.All(ctx, &folders)

	// Build folder tree for display
	folderTree := h.buildFolderTree(folders)

	h.renderAdmin(w, r, "folders_list", map[string]interface{}{
		"Folders":    folders,
		"FolderTree": folderTree,
	})
}

func (h *Handler) NewFolder(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	folders := h.getAllFolders(ctx)

	h.renderAdmin(w, r, "folder_form", map[string]interface{}{
		"IsNew":   true,
		"Folders": folders,
	})
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	ctx := r.Context()

	name := r.FormValue("name")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = slugify(name)
	}

	parentIDStr := r.FormValue("parent_id")
	var parentID *primitive.ObjectID
	parentPath := ""

	if parentIDStr != "" && parentIDStr != "root" {
		pid, err := primitive.ObjectIDFromHex(parentIDStr)
		if err == nil {
			parentID = &pid
			// Get parent folder path
			var parentFolder models.Folder
			if err := h.db.FindOne(ctx, "folders", bson.M{"_id": pid}, &parentFolder); err == nil {
				parentPath = parentFolder.Path
			}
		}
	}

	// Build full path
	fullPath := "/" + slug
	if parentPath != "" {
		fullPath = parentPath + "/" + slug
	}

	folder := models.Folder{
		Name:      name,
		Slug:      slug,
		ParentID:  parentID,
		Path:      fullPath,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if _, err := h.db.InsertOne(ctx, "folders", folder); err != nil {
		folders := h.getAllFolders(ctx)
		h.renderAdmin(w, r, "folder_form", map[string]interface{}{
			"IsNew":   true,
			"Folder":  folder,
			"Folders": folders,
			"Error":   err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/cm/folders", http.StatusSeeOther)
}

func (h *Handler) EditFolder(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var folder models.Folder
	if err := h.db.FindOne(ctx, "folders", bson.M{"_id": id}, &folder); err != nil {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	folders := h.getAllFolders(ctx)

	h.renderAdmin(w, r, "folder_form", map[string]interface{}{
		"IsNew":   false,
		"Folder":  folder,
		"Folders": folders,
	})
}

func (h *Handler) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	ctx := r.Context()

	var existingFolder models.Folder
	if err := h.db.FindOne(ctx, "folders", bson.M{"_id": id}, &existingFolder); err != nil {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	oldPath := existingFolder.Path

	name := r.FormValue("name")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = slugify(name)
	}

	parentIDStr := r.FormValue("parent_id")
	var parentID *primitive.ObjectID
	parentPath := ""

	if parentIDStr != "" && parentIDStr != "root" {
		pid, err := primitive.ObjectIDFromHex(parentIDStr)
		if err == nil {
			parentID = &pid
			var parentFolder models.Folder
			if err := h.db.FindOne(ctx, "folders", bson.M{"_id": pid}, &parentFolder); err == nil {
				parentPath = parentFolder.Path
			}
		}
	}

	// Build new full path
	newPath := "/" + slug
	if parentPath != "" {
		newPath = parentPath + "/" + slug
	}

	update := bson.M{
		"$set": bson.M{
			"name":       name,
			"slug":       slug,
			"parent_id":  parentID,
			"path":       newPath,
			"updated_at": time.Now(),
		},
	}

	if err := h.db.UpdateOne(ctx, "folders", bson.M{"_id": id}, update); err != nil {
		folders := h.getAllFolders(ctx)
		h.renderAdmin(w, r, "folder_form", map[string]interface{}{
			"IsNew":   false,
			"Folder":  existingFolder,
			"Folders": folders,
			"Error":   err.Error(),
		})
		return
	}

	// If path changed, update all child folders and content
	if oldPath != newPath {
		h.updateFolderPaths(ctx, oldPath, newPath)
	}

	http.Redirect(w, r, "/cm/folders", http.StatusSeeOther)
}

func (h *Handler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if folder has children
	childCount, _ := h.db.Count(ctx, "folders", bson.M{"parent_id": id})
	if childCount > 0 {
		http.Error(w, "Cannot delete folder with subfolders", http.StatusBadRequest)
		return
	}

	// Check if folder has content
	contentCount, _ := h.db.Count(ctx, "content", bson.M{"folder_id": id})
	if contentCount > 0 {
		http.Error(w, "Cannot delete folder containing content. Move or delete content first.", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteOne(ctx, "folders", bson.M{"_id": id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/folders", http.StatusSeeOther)
}

// GetAllFolders returns all folders as JSON for the content form dropdown
func (h *Handler) GetAllFoldersAPI(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	folders := h.getAllFolders(ctx)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `[{"path":"/","name":"/ (root)"}`)
	for _, f := range folders {
		name := strings.ReplaceAll(f.Name, `"`, `\"`)
		fmt.Fprintf(w, `,{"id":%q,"path":%q,"name":%q}`, f.ID.Hex(), f.Path, f.Path+" ("+name+")")
	}
	fmt.Fprint(w, "]")
}

// Helper to get all folders sorted by path
func (h *Handler) getAllFolders(ctx context.Context) []models.Folder {
	cursor, err := h.db.FindMany(ctx, "folders", bson.M{}, options.Find().SetSort(bson.D{{Key: "path", Value: 1}}))
	if err != nil {
		return nil
	}
	var folders []models.Folder
	cursor.All(ctx, &folders)
	return folders
}

// Build folder tree structure for display
type FolderTreeItem struct {
	Folder   models.Folder
	Children []FolderTreeItem
	Depth    int
}

func (h *Handler) buildFolderTree(folders []models.Folder) []FolderTreeItem {
	// Create map for quick lookup
	folderMap := make(map[primitive.ObjectID]models.Folder)
	for _, f := range folders {
		folderMap[f.ID] = f
	}

	// Build flat list with depth
	var result []FolderTreeItem
	for _, f := range folders {
		depth := strings.Count(f.Path, "/") - 1
		result = append(result, FolderTreeItem{
			Folder: f,
			Depth:  depth,
		})
	}
	return result
}

// Update all folder paths when a parent folder path changes
func (h *Handler) updateFolderPaths(ctx context.Context, oldPath, newPath string) {
	// Find all folders under the old path
	cursor, _ := h.db.FindMany(ctx, "folders", bson.M{
		"path": bson.M{"$regex": "^" + regexp.QuoteMeta(oldPath) + "/"},
	})

	var childFolders []models.Folder
	cursor.All(ctx, &childFolders)

	for _, cf := range childFolders {
		updatedPath := strings.Replace(cf.Path, oldPath, newPath, 1)
		h.db.UpdateOne(ctx, "folders", bson.M{"_id": cf.ID}, bson.M{
			"$set": bson.M{"path": updatedPath, "updated_at": time.Now()},
		})
	}

	// Update content in these folders
	h.updateContentFolderPaths(ctx, oldPath, newPath)
}

// Update content folder paths and full paths when folder path changes
func (h *Handler) updateContentFolderPaths(ctx context.Context, oldFolderPath, newFolderPath string) {
	// Find all content with folder_path starting with old path
	cursor, _ := h.db.FindMany(ctx, "content", bson.M{
		"folder_path": bson.M{"$regex": "^" + regexp.QuoteMeta(oldFolderPath)},
	})

	var contents []models.Content
	cursor.All(ctx, &contents)

	for _, c := range contents {
		updatedFolderPath := strings.Replace(c.FolderPath, oldFolderPath, newFolderPath, 1)
		updatedFullPath := updatedFolderPath
		if c.Slug != "" {
			updatedFullPath = updatedFolderPath + "/" + c.Slug
		}

		// Delete old static file
		oldStaticPath := h.getStaticFilePath(c.FullPath)
		os.Remove(oldStaticPath)

		h.db.UpdateOne(ctx, "content", bson.M{"_id": c.ID}, bson.M{
			"$set": bson.M{
				"folder_path": updatedFolderPath,
				"full_path":   updatedFullPath,
				"updated_at":  time.Now(),
			},
		})

		// Regenerate static file at new location if published
		if c.Published {
			c.FolderPath = updatedFolderPath
			c.FullPath = updatedFullPath
			var tmpl models.Template
			if err := h.db.FindOne(ctx, "templates", bson.M{"_id": c.TemplateID}, &tmpl); err == nil {
				h.generateStaticPage(ctx, &c, &tmpl)
			}
		}
	}
}

// Get the static file path for a content item's full path
func (h *Handler) getStaticFilePath(fullPath string) string {
	if fullPath == "" || fullPath == "/" {
		return "content/generated/index.html"
	}
	// Remove leading slash and add .html
	path := strings.TrimPrefix(fullPath, "/")
	return filepath.Join("content/generated", path+".html")
}

// Theme handlers
func (h *Handler) ThemeSettings(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	settings, err := h.db.GetThemeSettings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "theme", map[string]interface{}{"Settings": settings})
}

func (h *Handler) UpdateTheme(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	r.ParseForm()

	// Get old settings to check if header/footer changed
	oldSettings, _ := h.db.GetThemeSettings(r.Context())
	headerChanged := oldSettings == nil || oldSettings.HeaderHTML != r.FormValue("header_html")
	footerChanged := oldSettings == nil || oldSettings.FooterHTML != r.FormValue("footer_html")

	settings := &database.ThemeSettings{
		PrimaryColor:    r.FormValue("primary_color"),
		SecondaryColor:  r.FormValue("secondary_color"),
		AccentColor:     r.FormValue("accent_color"),
		BackgroundColor: r.FormValue("background_color"),
		TextColor:       r.FormValue("text_color"),
		FontFamily:      r.FormValue("font_family"),
		HeadingFont:     r.FormValue("heading_font"),
		BorderRadius:    r.FormValue("border_radius"),
		CustomCSS:       r.FormValue("custom_css"),
		SiteName:        r.FormValue("site_name"),
		SiteTagline:     r.FormValue("site_tagline"),
		LogoURL:         r.FormValue("logo_url"),
		HeaderHTML:      r.FormValue("header_html"),
		FooterHTML:      r.FormValue("footer_html"),
	}

	if err := h.db.SaveThemeSettings(r.Context(), settings); err != nil {
		h.renderAdmin(w, r, "theme", map[string]interface{}{
			"Settings": settings,
			"Error":    err.Error(),
		})
		return
	}

	// Regenerate CSS file
	h.generateThemeCSS(settings)

	// If header or footer changed, regenerate all published content
	if headerChanged || footerChanged {
		h.regenerateAllContent(r.Context())
	}

	h.renderAdmin(w, r, "theme", map[string]interface{}{
		"Settings": settings,
		"Success":  "Theme updated successfully!",
	})
}

func (h *Handler) generateThemeCSS(settings *database.ThemeSettings) {
	css := fmt.Sprintf(`:root {
    --primary: %s;
    --secondary: %s;
    --accent: %s;
    --background: %s;
    --text: %s;
    --font-family: %s;
    --heading-font: %s;
    --border-radius: %s;
}

%s`, settings.PrimaryColor, settings.SecondaryColor, settings.AccentColor,
		settings.BackgroundColor, settings.TextColor, settings.FontFamily,
		settings.HeadingFont, settings.BorderRadius, settings.CustomCSS)

	os.WriteFile("static/css/theme-vars.css", []byte(css), 0644)
}

// SecuritySettings shows the password change form
func (h *Handler) SecuritySettings(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	isDefaultPassword := h.auth.IsDefaultPassword(ctx)

	h.renderAdmin(w, r, "security", map[string]interface{}{
		"IsDefaultPassword": isDefaultPassword,
	})
}

// UpdatePassword handles password change
func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	ctx := r.Context()
	isDefaultPassword := h.auth.IsDefaultPassword(ctx)

	// Validate confirm password matches
	if newPassword != confirmPassword {
		h.renderAdmin(w, r, "security", map[string]interface{}{
			"Error":             "New passwords do not match",
			"IsDefaultPassword": isDefaultPassword,
		})
		return
	}

	// Attempt to change password
	if err := h.auth.ChangePassword(ctx, currentPassword, newPassword); err != nil {
		h.renderAdmin(w, r, "security", map[string]interface{}{
			"Error":             err.Error(),
			"IsDefaultPassword": isDefaultPassword,
		})
		return
	}

	h.renderAdmin(w, r, "security", map[string]interface{}{
		"Success":           "Password changed successfully!",
		"IsDefaultPassword": false,
	})
}

// SiteConfiguration shows the site configuration page
func (h *Handler) SiteConfiguration(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	config, err := h.db.GetSiteConfig(ctx)
	if err != nil {
		http.Error(w, "Error loading configuration", http.StatusInternalServerError)
		return
	}

	theme, _ := h.db.GetThemeSettings(ctx)
	dbVersion, _ := h.db.GetDatabaseVersion(ctx)

	// Import build package for software version
	softwareVersion := build.GetVersion()

	h.renderAdmin(w, r, "config", map[string]interface{}{
		"Config":          config,
		"SiteName":        theme.SiteName,
		"SoftwareVersion": softwareVersion,
		"DatabaseVersion": dbVersion,
	})
}

// UpdateSiteConfiguration handles site configuration updates
func (h *Handler) UpdateSiteConfiguration(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	ctx := r.Context()

	config, _ := h.db.GetSiteConfig(ctx)
	config.TitleTemplate = r.FormValue("title_template")
	config.TitleTemplateNoTitle = r.FormValue("title_template_no_title")

	if err := h.db.SaveSiteConfig(ctx, config); err != nil {
		theme, _ := h.db.GetThemeSettings(ctx)
		h.renderAdmin(w, r, "config", map[string]interface{}{
			"Config":   config,
			"SiteName": theme.SiteName,
			"Error":    "Failed to save configuration",
		})
		return
	}

	theme, _ := h.db.GetThemeSettings(ctx)
	h.renderAdmin(w, r, "config", map[string]interface{}{
		"Config":   config,
		"SiteName": theme.SiteName,
		"Success":  "Configuration saved successfully!",
	})
}

// File upload handler
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
	filepath := filepath.Join("static/uploads", filename)
	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"location": "/uploads/%s"}`, filename)
}

// API handler for template fields
func (h *Handler) GetTemplateFields(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var tmpl models.Template
	if err := h.db.FindOne(r.Context(), "templates", bson.M{"_id": id}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"fields": %v}`, tmpl.Fields)
}

// GetAllSlugs returns all content slugs for link autocomplete
func (h *Handler) GetAllSlugs(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "content", bson.M{}, options.Find().SetProjection(bson.M{"slug": 1, "full_path": 1, "title": 1}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build JSON response
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "[")
	for i, c := range contents {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		// Use full_path if available, fall back to slug for legacy content
		path := c.FullPath
		if path == "" {
			path = "/" + c.Slug
			if c.Slug == "" {
				path = "/"
			}
		}
		// Escape JSON strings properly
		title := strings.ReplaceAll(c.Title, `"`, `\"`)
		title = strings.ReplaceAll(title, "\n", " ")
		fmt.Fprintf(w, `{"slug":%q,"title":%q}`, path, title)
	}
	fmt.Fprint(w, "]")
}

// ServePage serves public content
func (h *Handler) ServePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug := vars["slug"]

	ctx := r.Context()
	theme, _ := h.db.GetThemeSettings(ctx)

	// Build full path from URL
	fullPath := "/" + slug
	if slug == "" {
		fullPath = "/"
	}

	// Check for redirect first
	redirect, err := h.db.GetRedirect(ctx, fullPath)
	if err == nil && redirect != nil {
		statusCode := redirect.StatusCode
		if statusCode == 0 {
			statusCode = 301
		}
		// Add cache-control to limit browser caching of redirects
		// This helps when redirects are later deleted
		w.Header().Set("Cache-Control", "max-age=3600, must-revalidate")
		http.Redirect(w, r, redirect.ToPath, statusCode)
		return
	}

	// Check for collection first (collections are still top-level)
	var collection models.Collection
	if err := h.db.FindOne(ctx, "collections", bson.M{"slug": slug}, &collection); err == nil {
		h.serveCollection(w, r, &collection, theme)
		return
	}

	// Look up content from database - try full_path first, fall back to slug for legacy
	var content models.Content
	filter := bson.M{"published": true, "full_path": fullPath}
	if err := h.db.FindOne(ctx, "content", filter, &content); err != nil {
		// Fall back to legacy slug lookup
		legacyFilter := bson.M{"published": true, "slug": slug}
		if err := h.db.FindOne(ctx, "content", legacyFilter, &content); err != nil {
			h.serve404(w, r, theme)
			return
		}
	}

	// For blank pages with raw mode and no theme, serve raw HTML directly
	if !content.UseTheme && content.TemplateName == "Blank Page" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if htmlContent, ok := content.Data["content"].(string); ok {
			w.Write([]byte(htmlContent))
		}
		return
	}

	// Try to serve from static file first
	staticPath := h.getStaticFilePath(fullPath)

	if _, err := os.Stat(staticPath); err == nil {
		staticContent, _ := os.ReadFile(staticPath)
		h.renderPublicWithSEO(w, r, theme, string(staticContent), content.UseHeader, content.UseFooter,
			content.Title, content.MetaDescription, content.OGImage, fullPath)
		return
	}

	// Fall back to rendering from database
	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err != nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	rendered := h.renderContent(&content, &tmpl)
	h.renderPublicWithSEO(w, r, theme, rendered, content.UseHeader, content.UseFooter,
		content.Title, content.MetaDescription, content.OGImage, fullPath)
}

func (h *Handler) serveCollection(w http.ResponseWriter, r *http.Request, collection *models.Collection, theme *database.ThemeSettings) {
	ctx := r.Context()

	sortOrder := 1
	if collection.SortOrder == "desc" {
		sortOrder = -1
	}
	sortField := collection.SortField
	if sortField == "" {
		sortField = "created_at"
	}

	opts := options.Find().SetSort(bson.D{{Key: sortField, Value: sortOrder}})
	cursor, err := h.db.FindMany(ctx, "content", bson.M{"category": collection.Category, "published": true}, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []models.Content
	cursor.All(ctx, &items)

	// Render each item using item template
	var itemsHTML strings.Builder
	for _, item := range items {
		itemData := make(map[string]interface{})
		for k, v := range item.Data {
			itemData[k] = v
		}
		itemData["slug"] = item.Slug
		itemData["title"] = item.Title
		if item.PublishedAt != nil {
			itemData["published_at"] = item.PublishedAt.Format("January 2, 2006")
		}

		tmpl, err := template.New("item").Parse(collection.ItemTemplate)
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		tmpl.Execute(&buf, itemData)
		itemsHTML.WriteString(buf.String())
	}

	// Render page template
	pageData := map[string]interface{}{
		"collection_name":        collection.Name,
		"collection_description": collection.Description,
		"items":                  template.HTML(itemsHTML.String()),
		"pagination":             "",
	}

	pageTmpl, _ := template.New("page").Parse(collection.PageTemplate)
	var pageHTML bytes.Buffer
	pageTmpl.Execute(&pageHTML, pageData)

	h.renderPublic(w, r, theme, pageHTML.String())
}

func (h *Handler) serve404(w http.ResponseWriter, r *http.Request, theme *database.ThemeSettings) {
	w.WriteHeader(http.StatusNotFound)
	ctx := r.Context()

	// Try to serve 404 content page
	var content404 models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"slug": "404", "published": true}, &content404); err == nil {
		// Try static file first
		staticPath := "content/generated/404.html"
		if _, err := os.Stat(staticPath); err == nil {
			staticContent, _ := os.ReadFile(staticPath)
			h.renderPublicWithOptions(w, r, theme, string(staticContent), content404.UseHeader, content404.UseFooter)
			return
		}

		// Fall back to rendering from DB
		var tmpl models.Template
		if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content404.TemplateID}, &tmpl); err == nil {
			rendered := h.renderContent(&content404, &tmpl)
			h.renderPublicWithOptions(w, r, theme, rendered, content404.UseHeader, content404.UseFooter)
			return
		}
	}

	// Fallback to simple 404
	h.renderPublicWithOptions(w, r, theme, `<div style="text-align:center;padding:4rem"><h1>404</h1><p>Page not found</p></div>`, true, true)
}

func (h *Handler) renderContent(content *models.Content, tmpl *models.Template) string {
	data := make(map[string]interface{})
	for k, v := range content.Data {
		// Convert string values to template.HTML so they're not escaped
		if str, ok := v.(string); ok {
			data[k] = template.HTML(str)
		} else {
			data[k] = v
		}
	}
	data["title"] = content.Title
	data["slug"] = content.Slug
	if content.PublishedAt != nil {
		data["published_at"] = content.PublishedAt.Format("January 2, 2006")
	}

	t, err := template.New("content").Parse(tmpl.HTMLLayout)
	if err != nil {
		return fmt.Sprintf("<p>Error rendering content: %v</p>", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("<p>Error rendering content: %v</p>", err)
	}

	return buf.String()
}

func (h *Handler) generateStaticPage(ctx context.Context, content *models.Content, tmpl *models.Template) error {
	rendered := h.renderContent(content, tmpl)

	// Use full path for file location, supporting folders
	staticPath := h.getStaticFilePath(content.FullPath)
	if staticPath == "" {
		// Fallback for legacy content without full_path
		filename := content.Slug + ".html"
		if content.Slug == "" {
			filename = "index.html"
		}
		staticPath = filepath.Join("content/generated", filename)
	}

	// Ensure directory exists
	dir := filepath.Dir(staticPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(staticPath, []byte(rendered), 0644)
}

// regenerateAllContent regenerates all published content (used when header/footer/templates change)
func (h *Handler) regenerateAllContent(ctx context.Context) {
	// Regenerate all published content items
	cursor, err := h.db.FindMany(ctx, "content", bson.M{"published": true})
	if err != nil {
		return
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return
	}

	for _, content := range contents {
		var tmpl models.Template
		if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err != nil {
			continue
		}
		h.generateStaticPage(ctx, &content, &tmpl)
	}
}

// regenerateContentByTemplate regenerates all content using a specific template
func (h *Handler) regenerateContentByTemplate(ctx context.Context, templateID primitive.ObjectID) {
	cursor, err := h.db.FindMany(ctx, "content", bson.M{"template_id": templateID, "published": true})
	if err != nil {
		return
	}

	var tmpl models.Template
	if err := h.db.FindOne(ctx, "templates", bson.M{"_id": templateID}, &tmpl); err != nil {
		return
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return
	}

	for _, content := range contents {
		h.generateStaticPage(ctx, &content, &tmpl)
	}
}

func (h *Handler) renderPublic(w http.ResponseWriter, r *http.Request, theme *database.ThemeSettings, content string) {
	h.renderPublicWithOptions(w, r, theme, content, true, true)
}

func (h *Handler) renderPublicWithOptions(w http.ResponseWriter, r *http.Request, theme *database.ThemeSettings, content string, useHeader, useFooter bool) {
	h.renderPublicWithSEO(w, r, theme, content, useHeader, useFooter, "", "", "", "")
}

func (h *Handler) renderPublicWithSEO(w http.ResponseWriter, r *http.Request, theme *database.ThemeSettings, content string, useHeader, useFooter bool, title, metaDescription, ogImage, canonicalURL string) {
	tmpl := template.Must(template.New("layout").Parse(publicLayout))

	// Get site config for title template
	ctx := r.Context()
	config, _ := h.db.GetSiteConfig(ctx)

	// Apply title template
	pageTitle := h.applyTitleTemplate(config, title, theme.SiteName)

	data := map[string]interface{}{
		"Theme":           theme,
		"Content":         template.HTML(content),
		"UseHeader":       useHeader,
		"UseFooter":       useFooter,
		"HeaderHTML":      template.HTML(theme.HeaderHTML),
		"FooterHTML":      template.HTML(theme.FooterHTML),
		"PageTitle":       pageTitle,
		"MetaDescription": metaDescription,
		"OGImage":         ogImage,
		"CanonicalURL":    canonicalURL,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

// applyTitleTemplate applies the configured title template
func (h *Handler) applyTitleTemplate(config *database.SiteConfig, title, siteName string) string {
	var templateStr string
	if title == "" {
		templateStr = config.TitleTemplateNoTitle
		if templateStr == "" {
			templateStr = "{{site_name}}"
		}
	} else {
		templateStr = config.TitleTemplate
		if templateStr == "" {
			templateStr = "{{title}} - {{site_name}}"
		}
	}

	// Replace template variables
	result := strings.ReplaceAll(templateStr, "{{title}}", title)
	result = strings.ReplaceAll(result, "{{site_name}}", siteName)

	return result
}

func (h *Handler) renderAdmin(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["IsAuthenticated"] = h.auth.IsAuthenticated(r)

	ctx := r.Context()
	theme, _ := h.db.GetThemeSettings(ctx)
	data["Theme"] = theme

	// Get unread message count for sidebar badge
	unreadCount, _ := h.db.Count(ctx, "contact_messages", bson.M{"read": false})
	data["UnreadMessageCount"] = unreadCount

	funcMap := template.FuncMap{
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		"multiply": func(a, b int) int {
			return a * b
		},
		"divide": func(a, b int64) int64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	tmpl := template.Must(template.New("admin").Funcs(funcMap).Parse(adminTemplates[name]))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Prevent caching of admin pages
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	// Security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	tmpl.Execute(w, data)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// extractInternalLinks finds all internal links (starting with /) in HTML content
func extractInternalLinks(htmlContent string) []string {
	linkRegex := regexp.MustCompile(`href=["'](/[^"'#?]*)["']`)
	matches := linkRegex.FindAllStringSubmatch(htmlContent, -1)

	linkSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			// Normalize: remove trailing slash, convert to slug
			link = strings.TrimSuffix(link, "/")
			if link == "" {
				link = "/" // homepage
			}
			linkSet[link] = true
		}
	}

	links := make([]string, 0, len(linkSet))
	for link := range linkSet {
		links = append(links, link)
	}
	return links
}

// saveContentVersion saves the current state of content as a new version
func (h *Handler) saveContentVersion(ctx context.Context, content *models.Content) error {
	// Get the next version number
	count, err := h.db.Count(ctx, "content_versions", bson.M{"content_id": content.ID})
	if err != nil {
		return err
	}
	version := int(count) + 1

	contentVersion := models.ContentVersion{
		ContentID:       content.ID,
		Version:         version,
		TemplateID:      content.TemplateID,
		TemplateName:    content.TemplateName,
		Title:           content.Title,
		Slug:            content.Slug,
		FolderID:        content.FolderID,
		FolderPath:      content.FolderPath,
		FullPath:        content.FullPath,
		Category:        content.Category,
		MetaDescription: content.MetaDescription,
		OGImage:         content.OGImage,
		Data:            content.Data,
		Published:       content.Published,
		PublishedAt:     content.PublishedAt,
		UseHeader:       content.UseHeader,
		UseFooter:       content.UseFooter,
		UseTheme:        content.UseTheme,
		RawMode:         content.RawMode,
		CreatedAt:       time.Now(),
	}

	_, err = h.db.InsertOne(ctx, "content_versions", contentVersion)
	return err
}

// extractInternalLinksFromContent extracts all internal links from a content item's data
func (h *Handler) extractInternalLinksFromContent(content *models.Content) []string {
	linkSet := make(map[string]bool)

	for _, value := range content.Data {
		if strVal, ok := value.(string); ok {
			for _, link := range extractInternalLinks(strVal) {
				linkSet[link] = true
			}
		}
	}

	links := make([]string, 0, len(linkSet))
	for link := range linkSet {
		links = append(links, link)
	}
	return links
}

// updateLinksInHTML replaces old slug references with new slug in HTML
func updateLinksInHTML(html, oldSlug, newSlug string) string {
	oldPath := "/" + oldSlug
	newPath := "/" + newSlug
	if oldSlug == "" {
		oldPath = "/"
	}
	if newSlug == "" {
		newPath = "/"
	}

	// Replace href="oldPath" with href="newPath"
	// Handle various quote styles and edge cases
	patterns := []struct {
		old string
		new string
	}{
		{fmt.Sprintf(`href="%s"`, oldPath), fmt.Sprintf(`href="%s"`, newPath)},
		{fmt.Sprintf(`href='%s'`, oldPath), fmt.Sprintf(`href='%s'`, newPath)},
		{fmt.Sprintf(`href="%s#`, oldPath), fmt.Sprintf(`href="%s#`, newPath)},
		{fmt.Sprintf(`href='%s#`, oldPath), fmt.Sprintf(`href='%s#`, newPath)},
		{fmt.Sprintf(`href="%s?`, oldPath), fmt.Sprintf(`href="%s?`, newPath)},
		{fmt.Sprintf(`href='%s?`, oldPath), fmt.Sprintf(`href='%s?`, newPath)},
	}

	result := html
	for _, p := range patterns {
		result = strings.ReplaceAll(result, p.old, p.new)
	}
	return result
}

// updateDependentContent updates all content that links to oldSlug to point to newSlug (legacy)
func (h *Handler) updateDependentContent(ctx context.Context, oldSlug, newSlug string) error {
	oldPath := "/" + oldSlug
	if oldSlug == "" {
		oldPath = "/"
	}
	newPath := "/" + newSlug
	if newSlug == "" {
		newPath = "/"
	}
	return h.updateDependentContentByPath(ctx, oldPath, newPath)
}

// updateDependentContentByPath updates all content that links to oldPath to point to newPath
func (h *Handler) updateDependentContentByPath(ctx context.Context, oldPath, newPath string) error {
	if oldPath == newPath {
		return nil
	}

	// Find all content that has this internal link
	cursor, err := h.db.FindMany(ctx, "content", bson.M{"internal_links": oldPath})
	if err != nil {
		return err
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return err
	}

	for _, content := range contents {
		updated := false
		// Update links in all data fields
		for key, value := range content.Data {
			if strVal, ok := value.(string); ok {
				newVal := updateLinksInHTMLByPath(strVal, oldPath, newPath)
				if newVal != strVal {
					content.Data[key] = newVal
					updated = true
				}
			}
		}

		if updated {
			// Re-extract internal links after update
			content.InternalLinks = h.extractInternalLinksFromContent(&content)

			// Update in database
			if err := h.db.UpdateOne(ctx, "content", bson.M{"_id": content.ID}, bson.M{
				"$set": bson.M{
					"data":           content.Data,
					"internal_links": content.InternalLinks,
					"updated_at":     time.Now(),
				},
			}); err != nil {
				return err
			}

			// Regenerate static page if published
			if content.Published {
				var tmpl models.Template
				if err := h.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err == nil {
					h.generateStaticPage(ctx, &content, &tmpl)
				}
			}
		}
	}

	// Also update header and footer in theme settings
	theme, err := h.db.GetThemeSettings(ctx)
	if err == nil && theme != nil {
		themeUpdated := false

		newHeader := updateLinksInHTMLByPath(theme.HeaderHTML, oldPath, newPath)
		if newHeader != theme.HeaderHTML {
			theme.HeaderHTML = newHeader
			themeUpdated = true
		}

		newFooter := updateLinksInHTMLByPath(theme.FooterHTML, oldPath, newPath)
		if newFooter != theme.FooterHTML {
			theme.FooterHTML = newFooter
			themeUpdated = true
		}

		if themeUpdated {
			h.db.SaveThemeSettings(ctx, theme)
			// Regenerate all content since header/footer changed
			h.regenerateAllContent(ctx)
		}
	}

	return nil
}

// updateLinksInHTMLByPath replaces old path references with new path in HTML
func updateLinksInHTMLByPath(html, oldPath, newPath string) string {
	// Replace href="oldPath" with href="newPath"
	// Handle various quote styles and edge cases
	patterns := []struct {
		old string
		new string
	}{
		{fmt.Sprintf(`href="%s"`, oldPath), fmt.Sprintf(`href="%s"`, newPath)},
		{fmt.Sprintf(`href='%s'`, oldPath), fmt.Sprintf(`href='%s'`, newPath)},
		{fmt.Sprintf(`href="%s#`, oldPath), fmt.Sprintf(`href="%s#`, newPath)},
		{fmt.Sprintf(`href='%s#`, oldPath), fmt.Sprintf(`href='%s#`, newPath)},
		{fmt.Sprintf(`href="%s?`, oldPath), fmt.Sprintf(`href="%s?`, newPath)},
		{fmt.Sprintf(`href='%s?`, oldPath), fmt.Sprintf(`href='%s?`, newPath)},
	}

	result := html
	for _, p := range patterns {
		result = strings.ReplaceAll(result, p.old, p.new)
	}
	return result
}

// Public layout template
const publicLayout = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{if .PageTitle}}{{.PageTitle}}{{else}}{{.Theme.SiteName}}{{end}}</title>
    {{if .MetaDescription}}<meta name="description" content="{{.MetaDescription}}">{{end}}
    {{if .CanonicalURL}}<link rel="canonical" href="{{.CanonicalURL}}">{{end}}
    <!-- Open Graph / Social Media -->
    <meta property="og:type" content="website">
    <meta property="og:title" content="{{if .PageTitle}}{{.PageTitle}}{{else}}{{.Theme.SiteName}}{{end}}">
    {{if .MetaDescription}}<meta property="og:description" content="{{.MetaDescription}}">{{end}}
    {{if .OGImage}}<meta property="og:image" content="{{.OGImage}}">{{end}}
    <meta name="twitter:card" content="summary_large_image">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Space+Grotesk:wght@400;500;600;700&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="/static/css/theme-vars.css">
    <link rel="stylesheet" href="/static/css/main.css">
</head>
<body>
    {{if .UseHeader}}
    <header class="site-header">
        {{if .HeaderHTML}}
        {{.HeaderHTML}}
        {{else}}
        <nav class="main-nav">
            <div class="nav-container">
                <a href="/" class="logo">{{if .Theme.LogoURL}}<img src="{{.Theme.LogoURL}}" alt="{{.Theme.SiteName}}" class="site-logo">{{else}}{{.Theme.SiteName}}{{end}}</a>
                <div class="nav-links">
                    <a href="/">Home</a>
                    <a href="/blog">Blog</a>
                </div>
            </div>
        </nav>
        {{end}}
    </header>
    {{end}}
    <main class="main-content">
        <div class="container">
            {{.Content}}
        </div>
    </main>
    {{if .UseFooter}}
    <footer class="main-footer">
        {{if .FooterHTML}}
        <div class="container">
            {{.FooterHTML}}
        </div>
        {{else}}
        <div class="container">
            <p>{{.Theme.SiteTagline}}</p>
            <p class="copyright">&copy; 2026 {{.Theme.SiteName}}. Powered by LightCMS.</p>
        </div>
        {{end}}
    </footer>
    {{end}}
</body>
</html>`

// ============================================
// Redirect Management Handlers
// ============================================

// ListRedirects shows all redirects
func (h *Handler) ListRedirects(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "redirects", bson.M{}, options.Find().SetSort(bson.D{{Key: "from_path", Value: 1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var redirects []models.Redirect
	cursor.All(ctx, &redirects)

	h.renderAdmin(w, r, "redirects_list", map[string]interface{}{
		"Redirects": redirects,
	})
}

// NewRedirect shows form for new redirect
func (h *Handler) NewRedirect(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	h.renderAdmin(w, r, "redirect_form", map[string]interface{}{
		"IsNew": true,
	})
}

// CreateRedirect creates a new redirect
func (h *Handler) CreateRedirect(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	fromPath := r.FormValue("from_path")
	toPath := r.FormValue("to_path")
	statusCode := 301
	if r.FormValue("status_code") == "302" {
		statusCode = 302
	}
	description := r.FormValue("description")

	// Ensure paths start with /
	if !strings.HasPrefix(fromPath, "/") {
		fromPath = "/" + fromPath
	}
	if !strings.HasPrefix(toPath, "/") && !strings.HasPrefix(toPath, "http") {
		toPath = "/" + toPath
	}

	redirect := models.Redirect{
		FromPath:    fromPath,
		ToPath:      toPath,
		StatusCode:  statusCode,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := r.Context()
	if _, err := h.db.InsertOne(ctx, "redirects", redirect); err != nil {
		h.renderAdmin(w, r, "redirect_form", map[string]interface{}{
			"IsNew":    true,
			"Error":    "Failed to create redirect: " + err.Error(),
			"Redirect": redirect,
		})
		return
	}

	http.Redirect(w, r, "/cm/redirects", http.StatusSeeOther)
}

// EditRedirect shows form to edit a redirect
func (h *Handler) EditRedirect(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var redirect models.Redirect
	ctx := r.Context()
	if err := h.db.FindOne(ctx, "redirects", bson.M{"_id": id}, &redirect); err != nil {
		http.Error(w, "Redirect not found", http.StatusNotFound)
		return
	}

	h.renderAdmin(w, r, "redirect_form", map[string]interface{}{
		"IsNew":    false,
		"Redirect": redirect,
	})
}

// UpdateRedirect updates an existing redirect
func (h *Handler) UpdateRedirect(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	fromPath := r.FormValue("from_path")
	toPath := r.FormValue("to_path")
	statusCode := 301
	if r.FormValue("status_code") == "302" {
		statusCode = 302
	}
	description := r.FormValue("description")

	// Ensure paths start with /
	if !strings.HasPrefix(fromPath, "/") {
		fromPath = "/" + fromPath
	}
	if !strings.HasPrefix(toPath, "/") && !strings.HasPrefix(toPath, "http") {
		toPath = "/" + toPath
	}

	ctx := r.Context()
	update := bson.M{
		"$set": bson.M{
			"from_path":   fromPath,
			"to_path":     toPath,
			"status_code": statusCode,
			"description": description,
			"updated_at":  time.Now(),
		},
	}

	if err := h.db.UpdateOne(ctx, "redirects", bson.M{"_id": id}, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/redirects", http.StatusSeeOther)
}

// DeleteRedirect removes a redirect
func (h *Handler) DeleteRedirect(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.db.DeleteOne(ctx, "redirects", bson.M{"_id": id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/redirects", http.StatusSeeOther)
}

// ============================================
// Contact Form Handlers
// ============================================

// ContactFormSubmit handles public contact form submissions
func (h *Handler) ContactFormSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get IP address for rate limiting
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.Split(forwarded, ",")[0]
	}

	// Check rate limit
	isLimited, err := h.db.IsContactRateLimited(ctx, ip)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if isLimited {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"success":false,"error":"Too many submissions. Please try again later."}`))
		return
	}

	// Parse form
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	message := strings.TrimSpace(r.FormValue("message"))

	// Validate required fields
	if name == "" || email == "" || message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"All fields are required."}`))
		return
	}

	// Sanitize inputs to prevent injection attacks
	// Remove any characters that could be used for mailto: injection or phishing
	email = sanitizeEmail(email)
	name = sanitizeContactInput(name)
	message = sanitizeContactInput(message)

	// Validate email format more strictly
	if !isValidEmail(email) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"Please enter a valid email address."}`))
		return
	}

	// Length limits to prevent abuse
	if len(name) > 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"Name is too long (max 200 characters)."}`))
		return
	}
	if len(message) > 10000 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"Message is too long (max 10000 characters)."}`))
		return
	}

	// Create contact message
	contact := models.ContactMessage{
		Name:      name,
		Email:     email,
		Message:   message,
		IPAddress: ip,
		UserAgent: r.UserAgent(),
		Read:      false,
		CreatedAt: time.Now(),
	}

	if _, err := h.db.InsertOne(ctx, "contact_messages", contact); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"success":false,"error":"Failed to submit message. Please try again."}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"Thank you! Your message has been sent."}`))
}

// ListContactMessages shows all contact messages in admin
func (h *Handler) ListContactMessages(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "contact_messages", bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var messages []models.ContactMessage
	cursor.All(ctx, &messages)

	// Count unread
	unreadCount, _ := h.db.Count(ctx, "contact_messages", bson.M{"read": false})

	h.renderAdmin(w, r, "contact_messages_list", map[string]interface{}{
		"Messages":    messages,
		"UnreadCount": unreadCount,
	})
}

// ViewContactMessage shows a single contact message
func (h *Handler) ViewContactMessage(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var message models.ContactMessage
	ctx := r.Context()
	if err := h.db.FindOne(ctx, "contact_messages", bson.M{"_id": id}, &message); err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Mark as read
	if !message.Read {
		h.db.UpdateOne(ctx, "contact_messages", bson.M{"_id": id}, bson.M{"$set": bson.M{"read": true}})
		message.Read = true
	}

	h.renderAdmin(w, r, "contact_message_view", map[string]interface{}{
		"Message": message,
	})
}

// DeleteContactMessage removes a contact message
func (h *Handler) DeleteContactMessage(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.db.DeleteOne(ctx, "contact_messages", bson.M{"_id": id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/messages", http.StatusSeeOther)
}

// GetUnreadMessageCount returns the number of unread contact messages (for dashboard)
func (h *Handler) GetUnreadMessageCount(ctx context.Context) int64 {
	count, _ := h.db.Count(ctx, "contact_messages", bson.M{"read": false})
	return count
}

// ==================== Asset Library Handlers ====================

// AssetLibrary displays the asset library listing
func (h *Handler) AssetLibrary(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	folder := r.URL.Query().Get("folder")

	assets, err := h.db.ListAssets(ctx, folder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	folders, err := h.db.GetAssetFolders(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "asset_library", map[string]interface{}{
		"Assets":        assets,
		"Folders":       folders,
		"CurrentFolder": folder,
	})
}

// AssetUploadForm shows the upload form
func (h *Handler) AssetUploadForm(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	folders, err := h.db.GetAssetFolders(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "asset_upload", map[string]interface{}{
		"Folders": folders,
	})
}

// AssetUpload handles file upload
func (h *Handler) AssetUpload(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	// Parse multipart form - max 32MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Get form values
	filename := r.FormValue("filename")
	if filename == "" {
		filename = header.Filename
	}
	// Sanitize filename
	filename = strings.ReplaceAll(filename, "/", "-")
	filename = strings.ReplaceAll(filename, "\\", "-")

	folder := r.FormValue("folder")
	if folder == "" {
		folder = "/"
	}
	// Ensure folder starts with / and doesn't end with /
	if !strings.HasPrefix(folder, "/") {
		folder = "/" + folder
	}
	folder = strings.TrimSuffix(folder, "/")
	if folder == "" {
		folder = "/"
	}

	// Build full path
	var fullPath string
	if folder == "/" {
		fullPath = "/" + filename
	} else {
		fullPath = folder + "/" + filename
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)

	description := r.FormValue("description")

	ctx := r.Context()
	asset := &database.Asset{
		Filename:    filename,
		Folder:      folder,
		FullPath:    fullPath,
		MimeType:    mimeType,
		Size:        int64(len(data)),
		Data:        data,
		Description: description,
	}

	if err := h.db.SaveAsset(ctx, asset); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/assets", http.StatusSeeOther)
}

// ServeAsset serves an asset file by path
func (h *Handler) ServeAsset(w http.ResponseWriter, r *http.Request) {
	// Get full path from URL
	fullPath := r.URL.Path
	if !strings.HasPrefix(fullPath, "/assets") {
		http.NotFound(w, r)
		return
	}
	// Remove /assets prefix to get the actual path
	assetPath := strings.TrimPrefix(fullPath, "/assets")
	if assetPath == "" {
		assetPath = "/"
	}

	ctx := r.Context()
	asset, err := h.db.GetAssetByPath(ctx, assetPath)
	if err != nil || asset == nil {
		http.NotFound(w, r)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", asset.Size))
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	w.Write(asset.Data)
}

// DeleteAsset removes an asset
func (h *Handler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.db.DeleteAsset(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/assets", http.StatusSeeOther)
}

// ==================== Sitemap Generation ====================

// GenerateSitemap creates/updates the sitemap.xml file
func (h *Handler) GenerateSitemap(ctx context.Context, baseURL string) error {
	// Get all published content
	cursor, err := h.db.FindMany(ctx, "content", bson.M{"published": true})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return err
	}

	// Get all collections
	collCursor, err := h.db.FindMany(ctx, "collections", bson.M{})
	if err != nil {
		return err
	}
	defer collCursor.Close(ctx)

	var collections []models.Collection
	if err := collCursor.All(ctx, &collections); err != nil {
		return err
	}

	// Build sitemap XML following Google guidelines
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)

	// Add homepage
	sb.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/</loc>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
`, baseURL))

	// Add content pages
	for _, content := range contents {
		path := content.FullPath
		if path == "" {
			path = "/" + content.Slug
		}
		if path == "/" {
			continue // Skip homepage, already added
		}

		// Determine priority based on content type
		priority := "0.7"
		changefreq := "weekly"
		if content.Category == "blog" {
			priority = "0.6"
			changefreq = "monthly"
		}

		lastmod := content.UpdatedAt.Format("2006-01-02")

		sb.WriteString(fmt.Sprintf(`  <url>
    <loc>%s%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>%s</changefreq>
    <priority>%s</priority>
  </url>
`, baseURL, path, lastmod, changefreq, priority))
	}

	// Add collection pages
	for _, coll := range collections {
		sb.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/%s</loc>
    <changefreq>weekly</changefreq>
    <priority>0.8</priority>
  </url>
`, baseURL, coll.Slug))
	}

	sb.WriteString(`</urlset>
`)

	// Write to static file
	return os.WriteFile("static/sitemap.xml", []byte(sb.String()), 0644)
}

// ServeSitemap serves the sitemap.xml file
func (h *Handler) ServeSitemap(w http.ResponseWriter, r *http.Request) {
	// Try to serve static file first
	data, err := os.ReadFile("static/sitemap.xml")
	if err != nil {
		// Generate on the fly if file doesn't exist
		baseURL := h.baseURL
		if baseURL == "" {
			baseURL = "https://" + r.Host
			if r.TLS == nil {
				baseURL = "http://" + r.Host
			}
		}
		if err := h.GenerateSitemap(r.Context(), baseURL); err != nil {
			http.Error(w, "Failed to generate sitemap", http.StatusInternalServerError)
			return
		}
		data, _ = os.ReadFile("static/sitemap.xml")
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Write(data)
}

// ServeRobotsTxt serves robots.txt with sitemap reference
func (h *Handler) ServeRobotsTxt(w http.ResponseWriter, r *http.Request) {
	baseURL := h.baseURL
	if baseURL == "" {
		baseURL = "https://" + r.Host
		if r.TLS == nil {
			baseURL = "http://" + r.Host
		}
	}

	robotsTxt := fmt.Sprintf(`User-agent: *
Allow: /

Sitemap: %s/sitemap.xml
`, baseURL)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(robotsTxt))
}

// RegenerateSitemap is called after content changes to update sitemap
func (h *Handler) RegenerateSitemap(ctx context.Context) {
	baseURL := h.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:8082" // Fallback for development
	}
	h.GenerateSitemap(ctx, baseURL)
}

// ==================== Security Helpers ====================

// sanitizeEmail removes characters that could be used for mailto: injection
// Only allows alphanumeric, @, ., -, _, +
func sanitizeEmail(email string) string {
	var result strings.Builder
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '@' || r == '.' || r == '-' || r == '_' || r == '+' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// isValidEmail performs stricter email validation
func isValidEmail(email string) bool {
	// Must have exactly one @
	atCount := strings.Count(email, "@")
	if atCount != 1 {
		return false
	}

	// Split into local and domain parts
	parts := strings.Split(email, "@")
	local, domain := parts[0], parts[1]

	// Both parts must be non-empty
	if len(local) == 0 || len(domain) == 0 {
		return false
	}

	// Domain must have at least one dot
	if !strings.Contains(domain, ".") {
		return false
	}

	// Domain cannot start or end with a dot
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	// Check for double dots
	if strings.Contains(email, "..") {
		return false
	}

	// Length checks
	if len(local) > 64 || len(domain) > 253 || len(email) > 254 {
		return false
	}

	return true
}

// sanitizeContactInput removes potentially dangerous characters from user input
// Allows most text but removes control characters and null bytes
func sanitizeContactInput(input string) string {
	var result strings.Builder
	for _, r := range input {
		// Skip control characters (except newline, tab, carriage return)
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			continue
		}
		// Skip null bytes and other problematic characters
		if r == 0 {
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
