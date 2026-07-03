package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/services"
	"github.com/jonradoff/lightcms/v6/internal/services/importer"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SetImportService attaches an ImportService to the Handler.
func (h *Handler) SetImportService(is *services.ImportService) {
	h.importService = is
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func (h *Handler) listTemplatesForImport(r *http.Request) ([]models.Template, error) {
	ctx := r.Context()
	cursor, err := h.db.FindMany(ctx, "templates",
		bson.M{"deleted": bson.M{"$ne": true}},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var templates []models.Template
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// ---------------------------------------------------------------------------
// ImportsPage — GET /cm/imports
// ---------------------------------------------------------------------------

func (h *Handler) ImportsPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	sources, err := h.importService.ListSources(ctx)
	if err != nil {
		http.Error(w, "Failed to load import sources", http.StatusInternalServerError)
		return
	}
	jobs, err := h.importService.ListJobs(ctx, 50)
	if err != nil {
		http.Error(w, "Failed to load import jobs", http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "imports", map[string]interface{}{
		"Sources": sources,
		"Jobs":    jobs,
	})
}

// ---------------------------------------------------------------------------
// ImportJobPage — GET /cm/imports/{id}
// ---------------------------------------------------------------------------

func (h *Handler) ImportJobPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	job, err := h.importService.GetJob(ctx, id)
	if err != nil || job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	logs, err := h.importService.GetJobLogs(ctx, id, 0)
	if err != nil {
		logs = []models.ImportLog{}
	}

	h.renderAdmin(w, r, "import-job", map[string]interface{}{
		"Job":  job,
		"Logs": logs,
	})
}

// ---------------------------------------------------------------------------
// ImportJobSSE — GET /cm/imports/{id}/stream
// ---------------------------------------------------------------------------

func (h *Handler) ImportJobSSE(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// 1. Replay existing logs
	existingLogs, _ := h.importService.GetJobLogs(ctx, id, 0)
	for _, logLine := range existingLogs {
		line := fmt.Sprintf("data: %s|%s|%s\n\n", logLine.Level, logLine.Path, logLine.Message)
		fmt.Fprint(w, line)
		flusher.Flush()
	}

	// 2. Check if already done
	job, err := h.importService.GetJob(ctx, id)
	if err == nil && job != nil && job.Status != models.ImportStatusRunning {
		fmt.Fprintf(w, "data: done|%s|import finished\n\n", job.Status)
		flusher.Flush()
		return
	}

	// 3. Subscribe to live events
	ch := h.importService.Subscribe(id.Hex())
	defer h.importService.Unsubscribe(id.Hex(), ch)

	// 4. Stream until done or disconnect
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-ch:
			if !open {
				return
			}
			fmt.Fprint(w, msg)
			flusher.Flush()
			// Check for done sentinel
			if strings.HasPrefix(msg, "data: done|") {
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// NewRSSSourcePage — GET /cm/imports/sources/new
// ---------------------------------------------------------------------------

func (h *Handler) NewRSSSourcePage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	templates, _ := h.listTemplatesForImport(r)
	h.renderAdmin(w, r, "import-source-form", map[string]interface{}{
		"IsNew":     true,
		"Templates": templates,
	})
}

// ---------------------------------------------------------------------------
// CreateRSSSource — POST /cm/imports/sources
// ---------------------------------------------------------------------------

func (h *Handler) CreateRSSSource(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	url := strings.TrimSpace(r.FormValue("url"))
	templateIDStr := strings.TrimSpace(r.FormValue("template_id"))
	folderPath := strings.TrimSpace(r.FormValue("folder_path"))
	autoPublish := r.FormValue("auto_publish") == "on" || r.FormValue("auto_publish") == "true"
	schedule := strings.TrimSpace(r.FormValue("schedule"))
	active := r.FormValue("active") == "on" || r.FormValue("active") == "true"

	src := &models.ImportSource{
		Name:        name,
		URL:         url,
		FolderPath:  folderPath,
		AutoPublish: autoPublish,
		Schedule:    schedule,
		Active:      active,
	}
	if oid, err := primitive.ObjectIDFromHex(user.ID); err == nil {
		src.CreatedBy = oid
	}

	if templateIDStr != "" {
		if tid, err := primitive.ObjectIDFromHex(templateIDStr); err == nil {
			src.TemplateID = tid
		}
	}

	if err := h.importService.CreateSource(r.Context(), src); err != nil {
		templates, _ := h.listTemplatesForImport(r)
		h.renderAdmin(w, r, "import-source-form", map[string]interface{}{
			"IsNew":     true,
			"Error":     err.Error(),
			"Templates": templates,
		})
		return
	}

	http.Redirect(w, r, "/cm/imports", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// EditRSSSourcePage — GET /cm/imports/sources/{id}/edit
// ---------------------------------------------------------------------------

func (h *Handler) EditRSSSourcePage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid source ID", http.StatusBadRequest)
		return
	}

	src, err := h.importService.GetSource(r.Context(), id)
	if err != nil || src == nil {
		http.Error(w, "Source not found", http.StatusNotFound)
		return
	}

	templates, _ := h.listTemplatesForImport(r)
	h.renderAdmin(w, r, "import-source-form", map[string]interface{}{
		"IsNew":     false,
		"Source":    src,
		"Templates": templates,
	})
}

// ---------------------------------------------------------------------------
// UpdateRSSSource — POST /cm/imports/sources/{id}
// ---------------------------------------------------------------------------

func (h *Handler) UpdateRSSSource(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid source ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	updates := bson.M{
		"name":         strings.TrimSpace(r.FormValue("name")),
		"url":          strings.TrimSpace(r.FormValue("url")),
		"folder_path":  strings.TrimSpace(r.FormValue("folder_path")),
		"auto_publish": r.FormValue("auto_publish") == "on" || r.FormValue("auto_publish") == "true",
		"schedule":     strings.TrimSpace(r.FormValue("schedule")),
		"active":       r.FormValue("active") == "on" || r.FormValue("active") == "true",
	}

	templateIDStr := strings.TrimSpace(r.FormValue("template_id"))
	if templateIDStr != "" {
		if tid, err := primitive.ObjectIDFromHex(templateIDStr); err == nil {
			updates["template_id"] = tid
		}
	}

	if err := h.importService.UpdateSource(r.Context(), id, updates); err != nil {
		src, _ := h.importService.GetSource(r.Context(), id)
		templates, _ := h.listTemplatesForImport(r)
		h.renderAdmin(w, r, "import-source-form", map[string]interface{}{
			"IsNew":     false,
			"Source":    src,
			"Templates": templates,
			"Error":     err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/cm/imports", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// DeleteRSSSource — POST /cm/imports/sources/{id}/delete
// ---------------------------------------------------------------------------

func (h *Handler) DeleteRSSSource(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid source ID", http.StatusBadRequest)
		return
	}

	if err := h.importService.DeleteSource(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete source", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/imports", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// TriggerRSSSource — POST /cm/imports/sources/{id}/trigger
// ---------------------------------------------------------------------------

func (h *Handler) TriggerRSSSource(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid source ID", http.StatusBadRequest)
		return
	}

	job, err := h.importService.RunRSSImport(r.Context(), id, user.Email)
	if err != nil {
		http.Error(w, "Failed to start import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/imports/"+job.ID.Hex(), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// ImportMarkdownPage — GET /cm/imports/markdown
// ---------------------------------------------------------------------------

func (h *Handler) ImportMarkdownPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	templates, _ := h.listTemplatesForImport(r)
	h.renderAdmin(w, r, "import-markdown-form", map[string]interface{}{
		"Templates": templates,
	})
}

// ---------------------------------------------------------------------------
// DoImportMarkdown — POST /cm/imports/markdown
// ---------------------------------------------------------------------------

func (h *Handler) DoImportMarkdown(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "File too large (max 20MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	defaultTemplate := strings.TrimSpace(r.FormValue("default_template"))
	defaultFolder := strings.TrimSpace(r.FormValue("default_folder"))
	autoPublish := r.FormValue("auto_publish") == "on" || r.FormValue("auto_publish") == "true"

	var pages []importer.MarkdownPage
	name := strings.ToLower(header.Filename)
	if strings.HasSuffix(name, ".zip") {
		pages, err = importer.ParseMarkdownZip(data)
		if err != nil {
			http.Error(w, "Failed to parse ZIP: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		pages = []importer.MarkdownPage{importer.ParseMarkdownFile(header.Filename, string(data))}
	}

	job, err := h.importService.RunMarkdownImport(r.Context(), pages, defaultTemplate, defaultFolder, autoPublish, user.Email)
	if err != nil {
		http.Error(w, "Failed to start import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/imports/"+job.ID.Hex(), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// ImportCSVPage — GET /cm/imports/csv
// ---------------------------------------------------------------------------

func (h *Handler) ImportCSVPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	templates, _ := h.listTemplatesForImport(r)
	h.renderAdmin(w, r, "import-csv-form", map[string]interface{}{
		"Templates": templates,
	})
}

// ---------------------------------------------------------------------------
// DoImportCSV — POST /cm/imports/csv
// ---------------------------------------------------------------------------

func (h *Handler) DoImportCSV(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.importService == nil {
		http.Error(w, "Import service unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "File too large (max 20MB)", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	templateIDStr := strings.TrimSpace(r.FormValue("template_id"))
	folderPath := strings.TrimSpace(r.FormValue("folder_path"))
	titleColumn := strings.TrimSpace(r.FormValue("title_column"))
	if titleColumn == "" {
		titleColumn = "title"
	}
	autoPublish := r.FormValue("auto_publish") == "on" || r.FormValue("auto_publish") == "true"

	var templateID primitive.ObjectID
	if templateIDStr != "" {
		if tid, err := primitive.ObjectIDFromHex(templateIDStr); err == nil {
			templateID = tid
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	headers, records, err := importer.ParseCSV(bytes.NewReader(data))
	if err != nil {
		http.Error(w, "Failed to parse CSV: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build column map: every column maps to itself
	columnMap := make(map[string]string, len(headers))
	for _, h := range headers {
		columnMap[h] = h
	}

	job, err := h.importService.RunCSVImport(r.Context(), records, columnMap, titleColumn, "", templateID, folderPath, autoPublish, user.Email)
	if err != nil {
		http.Error(w, "Failed to start import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/imports/"+job.ID.Hex(), http.StatusSeeOther)
}
