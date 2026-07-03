package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/services/importer"
)

// ImportService manages import sources, jobs, and streaming log delivery.
type ImportService struct {
	db             *database.DB
	contentService *ContentService

	// SSE subscribers: jobID -> list of channels
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

// NewImportService creates a new ImportService.
func NewImportService(db *database.DB, cs *ContentService) *ImportService {
	return &ImportService{
		db:             db,
		contentService: cs,
		subscribers:    make(map[string][]chan string),
	}
}

// ---------------------------------------------------------------------------
// Source CRUD
// ---------------------------------------------------------------------------

// ListSources returns all import sources sorted by created_at descending.
func (s *ImportService) ListSources(ctx context.Context) ([]models.ImportSource, error) {
	cursor, err := s.db.FindMany(ctx, "import_sources", bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list import sources: %w", err)
	}
	var sources []models.ImportSource
	if err := cursor.All(ctx, &sources); err != nil {
		return nil, fmt.Errorf("decode import sources: %w", err)
	}
	return sources, nil
}

// GetSource returns a single import source by ID.
func (s *ImportService) GetSource(ctx context.Context, id primitive.ObjectID) (*models.ImportSource, error) {
	var src models.ImportSource
	if err := s.db.FindOne(ctx, "import_sources", bson.M{"_id": id}, &src); err != nil {
		return nil, fmt.Errorf("import source not found: %w", err)
	}
	return &src, nil
}

// CreateSource inserts a new import source.
func (s *ImportService) CreateSource(ctx context.Context, src *models.ImportSource) error {
	now := time.Now()
	src.CreatedAt = now
	src.UpdatedAt = now
	id, err := s.db.InsertOne(ctx, "import_sources", src)
	if err != nil {
		return fmt.Errorf("create import source: %w", err)
	}
	src.ID = id
	return nil
}

// UpdateSource applies a partial update to an import source.
func (s *ImportService) UpdateSource(ctx context.Context, id primitive.ObjectID, updates bson.M) error {
	updates["updated_at"] = time.Now()
	if err := s.db.UpdateOne(ctx, "import_sources", bson.M{"_id": id}, bson.M{"$set": updates}); err != nil {
		return fmt.Errorf("update import source: %w", err)
	}
	return nil
}

// DeleteSource removes an import source by ID.
func (s *ImportService) DeleteSource(ctx context.Context, id primitive.ObjectID) error {
	if err := s.db.DeleteOne(ctx, "import_sources", bson.M{"_id": id}); err != nil {
		return fmt.Errorf("delete import source: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Job management
// ---------------------------------------------------------------------------

// ListJobs returns recent import jobs, newest first.
func (s *ImportService) ListJobs(ctx context.Context, limit int) ([]models.ImportJob, error) {
	opts := options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cursor, err := s.db.FindMany(ctx, "import_jobs", bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list import jobs: %w", err)
	}
	var jobs []models.ImportJob
	if err := cursor.All(ctx, &jobs); err != nil {
		return nil, fmt.Errorf("decode import jobs: %w", err)
	}
	return jobs, nil
}

// GetJob returns a single import job by ID.
func (s *ImportService) GetJob(ctx context.Context, id primitive.ObjectID) (*models.ImportJob, error) {
	var job models.ImportJob
	if err := s.db.FindOne(ctx, "import_jobs", bson.M{"_id": id}, &job); err != nil {
		return nil, fmt.Errorf("import job not found: %w", err)
	}
	return &job, nil
}

// GetJobLogs returns log lines for a job with seq > afterSeq, in order.
func (s *ImportService) GetJobLogs(ctx context.Context, jobID primitive.ObjectID, afterSeq int) ([]models.ImportLog, error) {
	filter := bson.M{"job_id": jobID, "seq": bson.M{"$gt": afterSeq}}
	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}})
	cursor, err := s.db.FindMany(ctx, "import_logs", filter, opts)
	if err != nil {
		return nil, fmt.Errorf("get import logs: %w", err)
	}
	var logs []models.ImportLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("decode import logs: %w", err)
	}
	return logs, nil
}

// ---------------------------------------------------------------------------
// SSE pub/sub
// ---------------------------------------------------------------------------

// Subscribe returns a channel that receives SSE-formatted log lines for a job.
// The caller must call Unsubscribe when done.
func (s *ImportService) Subscribe(jobID string) chan string {
	ch := make(chan string, 64)
	s.mu.Lock()
	s.subscribers[jobID] = append(s.subscribers[jobID], ch)
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (s *ImportService) Unsubscribe(jobID string, ch chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.subscribers[jobID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[jobID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	close(ch)
}

// publish sends a log line to all SSE subscribers for a job.
func (s *ImportService) publish(jobID string, line string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.subscribers[jobID] {
		select {
		case ch <- line:
		default:
			// drop if subscriber is slow
		}
	}
}

// ---------------------------------------------------------------------------
// Internal log writer
// ---------------------------------------------------------------------------

// writeLog inserts a log line into import_logs and broadcasts to SSE subscribers.
func (s *ImportService) writeLog(ctx context.Context, jobID primitive.ObjectID, seq int, level models.ImportLogLevel, msg string, path string) {
	log := models.ImportLog{
		ID:        primitive.NewObjectID(),
		JobID:     jobID,
		Seq:       seq,
		Level:     level,
		Message:   msg,
		Path:      path,
		CreatedAt: time.Now(),
	}
	s.db.Collection("import_logs").InsertOne(ctx, log) //nolint:errcheck
	// SSE format: "data: {level}|{path}|{msg}\n\n"
	line := fmt.Sprintf("data: %s|%s|%s\n\n", level, path, msg)
	s.publish(jobID.Hex(), line)
}

// ---------------------------------------------------------------------------
// RSS import
// ---------------------------------------------------------------------------

// RunRSSImport runs an RSS import for a configured source.
// It creates a job record immediately and processes items asynchronously.
func (s *ImportService) RunRSSImport(ctx context.Context, sourceID primitive.ObjectID, triggeredBy string) (*models.ImportJob, error) {
	// 1. Load source
	src, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	// 2. Create job record
	job := &models.ImportJob{
		ID:         primitive.NewObjectID(),
		SourceID:   &sourceID,
		SourceName: src.Name,
		Type:       models.ImportTypeRSS,
		Status:     models.ImportStatusRunning,
		StartedAt:  time.Now(),
		CreatedBy:  triggeredBy,
	}
	s.db.Collection("import_jobs").InsertOne(ctx, job) //nolint:errcheck

	// 3. Run async
	go func() {
		bgCtx := context.Background()
		seq := 0
		logLine := func(level models.ImportLogLevel, msg, path string) {
			seq++
			s.writeLog(bgCtx, job.ID, seq, level, msg, path)
		}

		logLine(models.ImportLogInfo, fmt.Sprintf("Fetching feed: %s", src.URL), "")

		items, err := importer.ParseFeed(bgCtx, src.URL)
		if err != nil {
			logLine(models.ImportLogError, fmt.Sprintf("Feed fetch failed: %v", err), "")
			s.finalizeJob(bgCtx, job.ID, models.ImportStatusFailed, err.Error(), 0, 0, 0, 0)
			return
		}

		logLine(models.ImportLogInfo, fmt.Sprintf("Found %d items", len(items)), "")

		created, updated, failed, skipped := 0, 0, 0, 0
		for _, item := range items {
			if item.Title == "" || item.URL == "" {
				skipped++
				logLine(models.ImportLogWarn, "Skipped item with no title or URL", "")
				continue
			}

			// Dedup by source_url field
			existing := s.findBySourceURL(bgCtx, item.URL)

			// Build data fields
			data := map[string]interface{}{
				"title": item.Title,
				"body":  item.Description,
			}
			if item.Author != "" {
				data["author"] = item.Author
			}

			// Generate slug from title
			slug := slugify(item.Title)
			folderPath := src.FolderPath
			if folderPath == "" {
				folderPath = "/imports"
			}
			fullPath := strings.TrimSuffix(folderPath, "/") + "/" + slug

			if existing != nil {
				// Update existing: set data fields on the content object
				existing.Data = data
				existing.UpdatedAt = time.Now()
				err := s.contentService.UpdateContent(bgCtx, existing)
				if err != nil {
					failed++
					logLine(models.ImportLogError, fmt.Sprintf("Update failed: %v", err), fullPath)
				} else {
					updated++
					logLine(models.ImportLogInfo, "Updated", fullPath)
				}
			} else {
				// Create new
				content := &models.Content{
					Title:      item.Title,
					Slug:       slug,
					FolderPath: folderPath,
					FullPath:   fullPath,
					TemplateID: src.TemplateID,
					Data:       data,
					SourceURL:  item.URL,
				}
				err := s.contentService.CreateContent(bgCtx, content)
				if err != nil {
					failed++
					logLine(models.ImportLogError, fmt.Sprintf("Create failed: %v", err), fullPath)
				} else {
					created++
					logLine(models.ImportLogInfo, "Created", fullPath)
					if src.AutoPublish {
						s.contentService.PublishContent(bgCtx, content.ID) //nolint:errcheck
					}
				}
			}
		}

		logLine(models.ImportLogInfo, fmt.Sprintf("Done: %d created, %d updated, %d failed, %d skipped", created, updated, failed, skipped), "")
		s.finalizeJob(bgCtx, job.ID, models.ImportStatusDone, "", len(items), created, updated, failed)

		// Update source last_run fields
		now := time.Now()
		s.db.Collection("import_sources").UpdateOne(bgCtx,
			bson.M{"_id": sourceID},
			bson.M{"$set": bson.M{
				"last_run_at":     now,
				"last_run_status": "ok",
				"last_job_id":     job.ID,
			}},
		) //nolint:errcheck
	}()

	return job, nil
}

// ---------------------------------------------------------------------------
// Markdown import
// ---------------------------------------------------------------------------

// RunMarkdownImport imports one or more Markdown pages from parsed MarkdownPage structs.
// defaultTemplate and defaultFolder are used when not specified in frontmatter.
func (s *ImportService) RunMarkdownImport(ctx context.Context, pages []importer.MarkdownPage, defaultTemplate string, defaultFolder string, autoPublish bool, triggeredBy string) (*models.ImportJob, error) {
	job := &models.ImportJob{
		ID:         primitive.NewObjectID(),
		SourceName: "markdown-upload",
		Type:       models.ImportTypeMarkdown,
		Status:     models.ImportStatusRunning,
		TotalPages: len(pages),
		StartedAt:  time.Now(),
		CreatedBy:  triggeredBy,
	}
	s.db.Collection("import_jobs").InsertOne(ctx, job) //nolint:errcheck

	go func() {
		bgCtx := context.Background()
		seq := 0
		logLine := func(level models.ImportLogLevel, msg, path string) {
			seq++
			s.writeLog(bgCtx, job.ID, seq, level, msg, path)
		}

		created, updated, failed, skipped := 0, 0, 0, 0

		for _, page := range pages {
			title := importer.FrontmatterGet(page.Frontmatter, "title")
			if title == "" {
				// Derive from filename
				base := strings.TrimSuffix(filepath.Base(page.Filename), filepath.Ext(page.Filename))
				title = strings.ReplaceAll(base, "-", " ")
				title = strings.ReplaceAll(title, "_", " ")
			}
			if title == "" {
				skipped++
				logLine(models.ImportLogWarn, fmt.Sprintf("Skipped %s: no title", page.Filename), "")
				continue
			}

			slug := importer.FrontmatterGet(page.Frontmatter, "slug")
			if slug == "" {
				slug = slugify(title)
			}

			folder := importer.FrontmatterGet(page.Frontmatter, "folder")
			if folder == "" {
				folder = defaultFolder
			}
			if folder == "" {
				folder = "/imports"
			}
			// Sanitize folder from frontmatter — reject path traversal
			if !safeFolderPath(folder) {
				failed++
				logLine(models.ImportLogError, fmt.Sprintf("Skipped %s: invalid folder path %q", page.Filename, folder), "")
				continue
			}
			fullPath := strings.TrimSuffix(folder, "/") + "/" + slug

			templateName := importer.FrontmatterGet(page.Frontmatter, "template")
			if templateName == "" {
				templateName = defaultTemplate
			}

			data := map[string]interface{}{
				"title": title,
				"body":  page.Body,
			}
			// Map additional frontmatter keys to data fields
			for k, v := range page.Frontmatter {
				lower := strings.ToLower(k)
				if lower != "title" && lower != "slug" && lower != "folder" && lower != "template" && lower != "published" && lower != "publish_at" {
					data[k] = v
				}
			}

			// Resolve template ID
			var templateID primitive.ObjectID
			if templateName != "" {
				tmpl := s.findTemplateByName(bgCtx, templateName)
				if tmpl != nil {
					templateID = tmpl.ID
				}
			}

			existing := s.findByPath(bgCtx, fullPath)

			if existing != nil {
				existing.Data = data
				err := s.contentService.UpdateContent(bgCtx, existing)
				if err != nil {
					failed++
					logLine(models.ImportLogError, fmt.Sprintf("Update failed: %v", err), fullPath)
				} else {
					updated++
					logLine(models.ImportLogInfo, "Updated", fullPath)
				}
			} else {
				content := &models.Content{
					Title:      title,
					Slug:       slug,
					FolderPath: folder,
					FullPath:   fullPath,
					TemplateID: templateID,
					Data:       data,
				}

				// Handle publish_at from frontmatter
				if paStr := importer.FrontmatterGet(page.Frontmatter, "publish_at"); paStr != "" {
					if t := importer.ParseTimeStr(paStr); t != nil {
						content.PublishAt = t
					}
				}

				err := s.contentService.CreateContent(bgCtx, content)
				if err != nil {
					failed++
					logLine(models.ImportLogError, fmt.Sprintf("Create failed: %v", err), fullPath)
				} else {
					created++
					logLine(models.ImportLogInfo, "Created", fullPath)
					pubStr := importer.FrontmatterGet(page.Frontmatter, "published")
					if autoPublish || pubStr == "true" {
						s.contentService.PublishContent(bgCtx, content.ID) //nolint:errcheck
					}
				}
			}
		}

		logLine(models.ImportLogInfo, fmt.Sprintf("Done: %d created, %d updated, %d failed, %d skipped", created, updated, failed, skipped), "")
		s.finalizeJob(bgCtx, job.ID, models.ImportStatusDone, "", len(pages), created, updated, failed)
	}()

	return job, nil
}

// ---------------------------------------------------------------------------
// CSV import
// ---------------------------------------------------------------------------

// RunCSVImport imports rows from a CSV, mapping columns to content data fields.
// columnMap maps CSV header name -> content field name.
// titleColumn is the CSV column used as the page title.
// slugColumn (optional) provides the URL slug; falls back to slugifying the title.
func (s *ImportService) RunCSVImport(ctx context.Context, records []importer.CSVRecord, columnMap map[string]string, titleColumn string, slugColumn string, templateID primitive.ObjectID, folderPath string, autoPublish bool, triggeredBy string) (*models.ImportJob, error) {
	job := &models.ImportJob{
		ID:         primitive.NewObjectID(),
		SourceName: "csv-upload",
		Type:       models.ImportTypeCSV,
		Status:     models.ImportStatusRunning,
		TotalPages: len(records),
		StartedAt:  time.Now(),
		CreatedBy:  triggeredBy,
	}
	s.db.Collection("import_jobs").InsertOne(ctx, job) //nolint:errcheck

	go func() {
		bgCtx := context.Background()
		seq := 0
		logLine := func(level models.ImportLogLevel, msg, path string) {
			seq++
			s.writeLog(bgCtx, job.ID, seq, level, msg, path)
		}

		created, updated, failed, skipped := 0, 0, 0, 0
		if folderPath == "" {
			folderPath = "/imports"
		}

		for _, rec := range records {
			title := rec.Fields[titleColumn]
			if title == "" {
				skipped++
				logLine(models.ImportLogWarn, fmt.Sprintf("Row %d: empty title column %q", rec.Row, titleColumn), "")
				continue
			}

			slug := ""
			if slugColumn != "" {
				slug = slugify(rec.Fields[slugColumn])
			}
			if slug == "" {
				slug = slugify(title)
			}
			fullPath := strings.TrimSuffix(folderPath, "/") + "/" + slug

			// Map columns to data fields
			data := map[string]interface{}{"title": title}
			for csvCol, fieldName := range columnMap {
				if val, ok := rec.Fields[csvCol]; ok && val != "" {
					data[fieldName] = val
				}
			}

			existing := s.findByPath(bgCtx, fullPath)
			if existing != nil {
				existing.Data = data
				err := s.contentService.UpdateContent(bgCtx, existing)
				if err != nil {
					failed++
					logLine(models.ImportLogError, fmt.Sprintf("Row %d update failed: %v", rec.Row, err), fullPath)
				} else {
					updated++
					logLine(models.ImportLogInfo, fmt.Sprintf("Row %d updated", rec.Row), fullPath)
				}
			} else {
				content := &models.Content{
					Title:      title,
					Slug:       slug,
					FolderPath: folderPath,
					FullPath:   fullPath,
					TemplateID: templateID,
					Data:       data,
				}
				err := s.contentService.CreateContent(bgCtx, content)
				if err != nil {
					failed++
					logLine(models.ImportLogError, fmt.Sprintf("Row %d create failed: %v", rec.Row, err), fullPath)
				} else {
					created++
					logLine(models.ImportLogInfo, fmt.Sprintf("Row %d created", rec.Row), fullPath)
					if autoPublish {
						s.contentService.PublishContent(bgCtx, content.ID) //nolint:errcheck
					}
				}
			}
		}

		logLine(models.ImportLogInfo, fmt.Sprintf("Done: %d created, %d updated, %d failed, %d skipped", created, updated, failed, skipped), "")
		s.finalizeJob(bgCtx, job.ID, models.ImportStatusDone, "", len(records), created, updated, failed)
	}()

	return job, nil
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

func (s *ImportService) finalizeJob(ctx context.Context, jobID primitive.ObjectID, status models.ImportStatus, errMsg string, total, created, updated, failed int) {
	now := time.Now()
	s.db.Collection("import_jobs").UpdateOne(ctx,
		bson.M{"_id": jobID},
		bson.M{"$set": bson.M{
			"status":      status,
			"error_msg":   errMsg,
			"total_pages": total,
			"created":     created,
			"updated":     updated,
			"failed":      failed,
			"finished_at": now,
		}},
	) //nolint:errcheck
	// Send SSE "done" sentinel
	doneMsg := fmt.Sprintf("data: done|%s|import finished\n\n", status)
	s.publish(jobID.Hex(), doneMsg)
}

func (s *ImportService) findByPath(ctx context.Context, fullPath string) *models.Content {
	var content models.Content
	err := s.db.FindOne(ctx, "content", bson.M{"full_path": fullPath, "deleted": bson.M{"$ne": true}}, &content)
	if err != nil {
		return nil
	}
	return &content
}

func (s *ImportService) findBySourceURL(ctx context.Context, sourceURL string) *models.Content {
	var content models.Content
	err := s.db.FindOne(ctx, "content", bson.M{"source_url": sourceURL, "deleted": bson.M{"$ne": true}}, &content)
	if err != nil {
		return nil
	}
	return &content
}

func (s *ImportService) findTemplateByName(ctx context.Context, name string) *models.Template {
	var tmpl models.Template
	err := s.db.FindOne(ctx, "templates", bson.M{
		"name":    bson.M{"$regex": name, "$options": "i"},
		"deleted": bson.M{"$ne": true},
	}, &tmpl)
	if err != nil {
		return nil
	}
	return &tmpl
}

// CancelJob sets a running import job's status to cancelled.
func (s *ImportService) CancelJob(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	if err := s.db.UpdateOne(ctx, "import_jobs",
		bson.M{"_id": id, "status": models.ImportStatusRunning},
		bson.M{"$set": bson.M{
			"status":      models.ImportStatusCancelled,
			"finished_at": now,
		}},
	); err != nil {
		return fmt.Errorf("cancel import job: %w", err)
	}
	// Broadcast a done sentinel to any SSE subscribers
	doneMsg := fmt.Sprintf("data: done|%s|import cancelled\n\n", models.ImportStatusCancelled)
	s.publish(id.Hex(), doneMsg)
	return nil
}

// slugify converts a string to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}

// safeFolderPath rejects path traversal sequences and other unsafe folder values.
// Mirrors the isValidImportFolder check in the API handler layer.
func safeFolderPath(path string) bool {
	if path == "" || path[0] != '/' {
		return false
	}
	for _, bad := range []string{"..", "\x00", "\\", "//"} {
		if strings.Contains(path, bad) {
			return false
		}
	}
	return true
}
