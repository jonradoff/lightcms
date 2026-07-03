package services

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LinkCheckJob tracks a single async link-check run.
type LinkCheckJob struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Status      string             `bson:"status" json:"status"` // "running", "done", "failed"
	TotalPages  int                `bson:"total_pages" json:"total_pages"`
	BrokenLinks []BrokenLink       `bson:"broken_links" json:"broken_links"`
	StartedAt   time.Time          `bson:"started_at" json:"started_at"`
	FinishedAt  *time.Time         `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
}

// BrokenLink describes a single broken link found in content.
type BrokenLink struct {
	SourcePath string `bson:"source_path" json:"source_path"`
	TargetPath string `bson:"target_path" json:"target_path"`
	LinkText   string `bson:"link_text" json:"link_text"`
}

// LinkCheckerService manages async link-check jobs.
type LinkCheckerService struct {
	db         *database.DB
	httpClient *http.Client
}

// NewLinkCheckerService creates a new LinkCheckerService.
func NewLinkCheckerService(db *database.DB) *LinkCheckerService {
	return &LinkCheckerService{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// StartJob creates a new job record and launches the scan in the background.
// Returns the job ID immediately.
func (s *LinkCheckerService) StartJob(ctx context.Context) (primitive.ObjectID, error) {
	job := LinkCheckJob{
		Status:      "running",
		BrokenLinks: []BrokenLink{},
		StartedAt:   time.Now(),
	}
	id, err := s.db.InsertOne(ctx, "link_check_jobs", job)
	if err != nil {
		return primitive.NilObjectID, err
	}
	job.ID = id

	go s.runJob(id)
	return id, nil
}

// GetJob retrieves the current state of a link-check job.
func (s *LinkCheckerService) GetJob(ctx context.Context, id primitive.ObjectID) (*LinkCheckJob, error) {
	var job LinkCheckJob
	err := s.db.FindOne(ctx, "link_check_jobs", bson.M{"_id": id}, &job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// runJob performs the actual scan in a background goroutine.
func (s *LinkCheckerService) runJob(jobID primitive.ObjectID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Build a set of all published content paths for fast lookup.
	cursor, err := s.db.FindMany(ctx, "content", bson.M{
		"published": true,
		"deleted":   bson.M{"$ne": true},
	}, options.Find().SetProjection(bson.M{"full_path": 1, "title": 1, "data": 1}))
	if err != nil {
		s.markFailed(jobID, err)
		return
	}
	defer cursor.Close(ctx)

	type pageEntry struct {
		ID       primitive.ObjectID     `bson:"_id"`
		FullPath string                 `bson:"full_path"`
		Title    string                 `bson:"title"`
		Data     map[string]interface{} `bson:"data"`
	}

	var pages []pageEntry
	for cursor.Next(ctx) {
		var p pageEntry
		if cursor.Decode(&p) == nil {
			pages = append(pages, p)
		}
	}

	// Build path set (lowercase for case-insensitive match)
	pathSet := make(map[string]bool, len(pages))
	titleSet := make(map[string]bool, len(pages))
	for _, p := range pages {
		pathSet[strings.ToLower(p.FullPath)] = true
		titleSet[strings.ToLower(p.Title)] = true
	}

	var broken []BrokenLink

	for _, p := range pages {
		// Extract all wikilinks from every string field in Data
		for _, v := range p.Data {
			str, ok := v.(string)
			if !ok {
				continue
			}
			matches := wikilinkRe.FindAllStringSubmatch(str, -1)
			for _, m := range matches {
				target := strings.TrimSpace(m[1])
				if target == "" {
					continue
				}
				// Exclude snippet includes
				if strings.HasPrefix(target, "include:") {
					continue
				}
				// Determine if it's a path link or title link
				if strings.HasPrefix(target, "/") {
					if !pathSet[strings.ToLower(target)] {
						broken = append(broken, BrokenLink{
							SourcePath: p.FullPath,
							TargetPath: target,
							LinkText:   target,
						})
					}
				} else {
					if !titleSet[strings.ToLower(target)] {
						broken = append(broken, BrokenLink{
							SourcePath: p.FullPath,
							TargetPath: target,
							LinkText:   target,
						})
					}
				}
			}
		}
		// Also check the title field itself for wikilinks (it may contain them)
		titleMatches := wikilinkRe.FindAllStringSubmatch(p.Title, -1)
		for _, m := range titleMatches {
			target := strings.TrimSpace(m[1])
			if target == "" || strings.HasPrefix(target, "include:") {
				continue
			}
			if strings.HasPrefix(target, "/") {
				if !pathSet[strings.ToLower(target)] {
					broken = append(broken, BrokenLink{
						SourcePath: p.FullPath,
						TargetPath: target,
						LinkText:   target,
					})
				}
			} else {
				if !titleSet[strings.ToLower(target)] {
					broken = append(broken, BrokenLink{
						SourcePath: p.FullPath,
						TargetPath: target,
						LinkText:   target,
					})
				}
			}
		}
	}

	// Also scan internal_links field if available (cached link list)
	allContent, err := s.db.FindMany(ctx, "content", bson.M{
		"published":      true,
		"deleted":        bson.M{"$ne": true},
		"internal_links": bson.M{"$exists": true, "$ne": []interface{}{}},
	}, options.Find().SetProjection(bson.M{"full_path": 1, "internal_links": 1}))
	if err == nil {
		defer allContent.Close(ctx)
		for allContent.Next(ctx) {
			var item models.Content
			if allContent.Decode(&item) != nil {
				continue
			}
			for _, link := range item.InternalLinks {
				if !pathSet[strings.ToLower(link)] && !titleSet[strings.ToLower(link)] {
					broken = append(broken, BrokenLink{
						SourcePath: item.FullPath,
						TargetPath: link,
						LinkText:   link,
					})
				}
			}
		}
	}

	if broken == nil {
		broken = []BrokenLink{}
	}

	now := time.Now()
	_ = s.db.UpdateOne(ctx, "link_check_jobs", bson.M{"_id": jobID}, bson.M{
		"$set": bson.M{
			"status":       "done",
			"total_pages":  len(pages),
			"broken_links": broken,
			"finished_at":  now,
		},
	})
}

func (s *LinkCheckerService) markFailed(jobID primitive.ObjectID, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()
	_ = s.db.UpdateOne(ctx, "link_check_jobs", bson.M{"_id": jobID}, bson.M{
		"$set": bson.M{
			"status":      "failed",
			"finished_at": now,
		},
	})
}
