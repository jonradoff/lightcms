package services

import (
	"context"
	"log"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SchedulerService runs a background ticker that publishes content whose
// publish_at timestamp has passed.
type SchedulerService struct {
	db             *database.DB
	contentService *ContentService
	ticker         *time.Ticker
	done           chan struct{}
	// heldLogged remembers held items already reported, so a page that stays
	// on hold past its publish_at is logged once rather than every tick.
	heldLogged map[primitive.ObjectID]bool
}

// NewSchedulerService creates a new SchedulerService.
func NewSchedulerService(db *database.DB, cs *ContentService) *SchedulerService {
	return &SchedulerService{
		db:             db,
		contentService: cs,
		done:           make(chan struct{}),
		heldLogged:     make(map[primitive.ObjectID]bool),
	}
}

// Start launches the background goroutine with a 60-second ticker.
func (s *SchedulerService) Start(ctx context.Context) {
	s.ticker = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-ctx.Done():
				return
			case <-s.ticker.C:
				s.runOnce(ctx)
			}
		}
	}()
}

// Stop stops the ticker and signals the goroutine to exit.
func (s *SchedulerService) Stop() {
	close(s.done)
	if s.ticker != nil {
		s.ticker.Stop()
	}
}

// runOnce queries for due content and publishes each item. Fork copies are
// never due (they inherit publish_at from the page they were copied from);
// held pages are skipped until their hold is cleared.
func (s *SchedulerService) runOnce(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	filter := bson.M{
		"publish_at": bson.M{"$lte": time.Now()},
		"published":  false,
		"deleted":    bson.M{"$ne": true},
		"fork_id":    nil,
	}

	cursor, err := s.db.FindMany(runCtx, "content", filter)
	if err != nil {
		log.Printf("[scheduler] query failed: %v", err)
		return
	}
	defer cursor.Close(runCtx)

	type minContent struct {
		ID   primitive.ObjectID `bson:"_id"`
		Hold bool               `bson:"hold"`
	}

	for cursor.Next(runCtx) {
		var item minContent
		if err := cursor.Decode(&item); err != nil {
			log.Printf("[scheduler] decode error: %v", err)
			continue
		}
		if item.Hold {
			if !s.heldLogged[item.ID] {
				s.heldLogged[item.ID] = true
				log.Printf("[scheduler] skipped content %s: on hold", item.ID.Hex())
			}
			continue
		}
		delete(s.heldLogged, item.ID)
		if err := s.contentService.PublishContent(runCtx, item.ID); err != nil {
			log.Printf("[scheduler] failed to publish %s: %v", item.ID.Hex(), err)
		} else {
			log.Printf("[scheduler] published content %s", item.ID.Hex())
		}
	}
}
