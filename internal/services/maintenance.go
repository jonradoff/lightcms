package services

import (
	"context"
	"log"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MaintenanceService runs periodic site-health scans and stores the
// findings as maintenance reports. Agents (via MCP) and the admin copilot
// read the latest report and fix issues through the normal editing flow —
// the site surfaces its own work queue.
type MaintenanceService struct {
	db          *database.DB
	linkChecker *LinkCheckerService
	ticker      *time.Ticker
	done        chan struct{}
	// StaleAfter marks pages as stale when not updated for this long.
	StaleAfter time.Duration
}

// MaintenanceReport is one scan's findings.
type MaintenanceReport struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	GeneratedAt time.Time          `bson:"generated_at" json:"generated_at"`
	StalePages  []StalePage        `bson:"stale_pages,omitempty" json:"stale_pages,omitempty"`
	MissingMeta []PageRef          `bson:"missing_meta,omitempty" json:"missing_meta,omitempty"`
	Drafts      []PageRef          `bson:"drafts,omitempty" json:"drafts,omitempty"`
	LinkJobID   string             `bson:"link_job_id,omitempty" json:"link_job_id,omitempty"` // async link-check job started by this scan
	PageCount   int                `bson:"page_count" json:"page_count"`
}

type PageRef struct {
	ID    string `bson:"id" json:"id"`
	Title string `bson:"title" json:"title"`
	Path  string `bson:"path" json:"path"`
}

type StalePage struct {
	PageRef   `bson:",inline"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
	AgeDays   int       `bson:"age_days" json:"age_days"`
}

func NewMaintenanceService(db *database.DB, linkChecker *LinkCheckerService) *MaintenanceService {
	return &MaintenanceService{
		db:          db,
		linkChecker: linkChecker,
		done:        make(chan struct{}),
		StaleAfter:  180 * 24 * time.Hour,
	}
}

// Start schedules a daily scan (first run one minute after startup).
func (s *MaintenanceService) Start(ctx context.Context) {
	s.ticker = time.NewTicker(24 * time.Hour)
	go func() {
		select {
		case <-time.After(time.Minute):
			s.scanAndLog(ctx)
		case <-s.done:
			return
		case <-ctx.Done():
			return
		}
		for {
			select {
			case <-s.done:
				return
			case <-ctx.Done():
				return
			case <-s.ticker.C:
				s.scanAndLog(ctx)
			}
		}
	}()
}

// Stop terminates the background loop.
func (s *MaintenanceService) Stop() {
	close(s.done)
	if s.ticker != nil {
		s.ticker.Stop()
	}
}

func (s *MaintenanceService) scanAndLog(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := s.RunScan(runCtx, false); err != nil {
		log.Printf("[maintenance] scan failed: %v", err)
	}
}

// RunScan performs one maintenance scan and stores the report. When
// withLinkCheck is true it also kicks off an async broken-link job and
// records its ID in the report.
func (s *MaintenanceService) RunScan(ctx context.Context, withLinkCheck bool) (*MaintenanceReport, error) {
	report := &MaintenanceReport{GeneratedAt: time.Now()}

	cursor, err := s.db.FindMany(ctx, "content", bson.M{
		"deleted": bson.M{"$ne": true},
		"fork_id": bson.M{"$exists": false},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type page struct {
		ID              primitive.ObjectID `bson:"_id"`
		Title           string             `bson:"title"`
		FullPath        string             `bson:"full_path"`
		MetaDescription string             `bson:"meta_description"`
		Published       bool               `bson:"published"`
		UpdatedAt       time.Time          `bson:"updated_at"`
	}

	now := time.Now()
	for cursor.Next(ctx) {
		var p page
		if err := cursor.Decode(&p); err != nil {
			continue
		}
		report.PageCount++
		ref := PageRef{ID: p.ID.Hex(), Title: p.Title, Path: p.FullPath}

		if !p.Published {
			report.Drafts = append(report.Drafts, ref)
			continue
		}
		if p.MetaDescription == "" {
			report.MissingMeta = append(report.MissingMeta, ref)
		}
		if age := now.Sub(p.UpdatedAt); age > s.StaleAfter {
			report.StalePages = append(report.StalePages, StalePage{
				PageRef: ref, UpdatedAt: p.UpdatedAt, AgeDays: int(age.Hours() / 24),
			})
		}
	}

	if withLinkCheck && s.linkChecker != nil {
		if jobID, err := s.linkChecker.StartJob(ctx); err == nil {
			report.LinkJobID = jobID.Hex()
		}
	}

	id, err := s.db.InsertOne(ctx, "maintenance_reports", report)
	if err != nil {
		return nil, err
	}
	report.ID = id
	return report, nil
}

// LatestReport returns the most recent maintenance report, if any.
func (s *MaintenanceService) LatestReport(ctx context.Context) (*MaintenanceReport, error) {
	var report MaintenanceReport
	err := s.db.Collection("maintenance_reports").
		FindOne(ctx, bson.M{}, options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})).
		Decode(&report)
	if err != nil {
		return nil, err
	}
	return &report, nil
}
