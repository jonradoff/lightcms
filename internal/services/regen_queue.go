package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ISRBatchSize is the number of content items processed per batch.
const ISRBatchSize = 20

// RegenJobDoc records the state of a batch regeneration job.
type RegenJobDoc struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TemplateID   primitive.ObjectID `bson:"template_id" json:"template_id"`
	TemplateName string             `bson:"template_name" json:"template_name"`
	Status       string             `bson:"status" json:"status"` // pending, running, done, failed
	Total        int                `bson:"total" json:"total"`
	Processed    int                `bson:"processed" json:"processed"`
	Errors       int                `bson:"errors" json:"errors"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
	CompletedAt  *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
}

// regenRequest carries the template information for a queued regeneration job.
type regenRequest struct {
	templateID   primitive.ObjectID
	templateName string
}

// RegenQueue manages ISR batch regeneration jobs.
type RegenQueue struct {
	db             *database.DB
	contentService *ContentService
	jobCh          chan regenRequest
	pending        map[primitive.ObjectID]bool
	mu             sync.Mutex
}

// NewRegenQueue creates a new RegenQueue.
func NewRegenQueue(db *database.DB, cs *ContentService) *RegenQueue {
	return &RegenQueue{
		db:             db,
		contentService: cs,
		jobCh:          make(chan regenRequest, 64),
		pending:        make(map[primitive.ObjectID]bool),
	}
}

// Start launches the worker goroutine.
func (q *RegenQueue) Start(ctx context.Context) {
	go q.worker(ctx)
}

// Enqueue creates a job document and queues the request if not already pending.
func (q *RegenQueue) Enqueue(ctx context.Context, templateID primitive.ObjectID, templateName string) {
	q.mu.Lock()
	if q.pending[templateID] {
		q.mu.Unlock()
		return
	}
	q.pending[templateID] = true
	q.mu.Unlock()

	now := time.Now()
	job := RegenJobDoc{
		TemplateID:   templateID,
		TemplateName: templateName,
		Status:       "pending",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := q.db.InsertOne(ctx, "regen_jobs", job); err != nil {
		log.Printf("[regen_queue] failed to create job doc for template %s: %v", templateName, err)
		q.mu.Lock()
		delete(q.pending, templateID)
		q.mu.Unlock()
		return
	}

	q.jobCh <- regenRequest{templateID: templateID, templateName: templateName}
}

// worker reads from jobCh and processes regeneration jobs.
func (q *RegenQueue) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-q.jobCh:
			if !ok {
				return
			}
			q.processJob(ctx, req)

			q.mu.Lock()
			delete(q.pending, req.templateID)
			q.mu.Unlock()
		}
	}
}

// processJob runs a single regeneration job with a 5-minute timeout.
func (q *RegenQueue) processJob(ctx context.Context, req regenRequest) {
	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Mark job as running.
	now := time.Now()
	_ = q.db.UpdateOne(jobCtx, "regen_jobs",
		bson.M{"template_id": req.templateID, "status": "pending"},
		bson.M{"$set": bson.M{"status": "running", "updated_at": now}},
	)

	// Find all published, non-deleted content for this template.
	cursor, err := q.db.FindMany(jobCtx, "content", bson.M{
		"template_id": req.templateID,
		"published":   true,
		"deleted":     bson.M{"$ne": true},
	})
	if err != nil {
		log.Printf("[regen_queue] query failed for template %s: %v", req.templateName, err)
		_ = q.db.UpdateOne(jobCtx, "regen_jobs",
			bson.M{"template_id": req.templateID, "status": "running"},
			bson.M{"$set": bson.M{"status": "failed", "updated_at": time.Now()}},
		)
		return
	}
	defer cursor.Close(jobCtx)

	type minContent struct {
		ID primitive.ObjectID `bson:"_id"`
	}

	// Collect all IDs first.
	var ids []primitive.ObjectID
	for cursor.Next(jobCtx) {
		var item minContent
		if err := cursor.Decode(&item); err != nil {
			continue
		}
		ids = append(ids, item.ID)
	}

	total := len(ids)
	processed := 0
	errors := 0

	// Update total.
	_ = q.db.UpdateOne(jobCtx, "regen_jobs",
		bson.M{"template_id": req.templateID, "status": "running"},
		bson.M{"$set": bson.M{"total": total, "updated_at": time.Now()}},
	)

	// Process in batches of ISRBatchSize.
	for batchStart := 0; batchStart < len(ids); batchStart += ISRBatchSize {
		end := batchStart + ISRBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[batchStart:end]

		for _, id := range batch {
			content, err := q.contentService.GetContent(jobCtx, id)
			if err != nil || content == nil {
				log.Printf("[regen_queue] get content %s failed: %v", id.Hex(), err)
				errors++
				continue
			}
			if err := q.contentService.GenerateStaticPage(jobCtx, content); err != nil {
				log.Printf("[regen_queue] regen %s failed: %v", id.Hex(), err)
				errors++
			} else {
				processed++
			}
		}

		// Persist progress after each batch.
		_ = q.db.UpdateOne(jobCtx, "regen_jobs",
			bson.M{"template_id": req.templateID, "status": "running"},
			bson.M{"$set": bson.M{
				"processed":  processed,
				"errors":     errors,
				"updated_at": time.Now(),
			}},
		)

		// Throttle between batches.
		if end < len(ids) {
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Mark complete.
	completedAt := time.Now()
	_ = q.db.UpdateOne(jobCtx, "regen_jobs",
		bson.M{"template_id": req.templateID, "status": "running"},
		bson.M{"$set": bson.M{
			"status":       "done",
			"processed":    processed,
			"errors":       errors,
			"updated_at":   completedAt,
			"completed_at": completedAt,
		}},
	)
	log.Printf("[regen_queue] template %s: %d/%d regenerated, %d errors", req.templateName, processed, total, errors)
}

// ListRecentJobs returns up to limit regen jobs sorted by created_at descending.
func (q *RegenQueue) ListRecentJobs(ctx context.Context, limit int) ([]RegenJobDoc, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))
	cursor, err := q.db.FindMany(ctx, "regen_jobs", bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var jobs []RegenJobDoc
	if err := cursor.All(ctx, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}
