package services

import (
	"context"
	"fmt"
	"os"

	"github.com/jonradoff/lightcms/v7/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ForkDamageReport is what RepairForkDamage found and, on a real run, fixed.
type ForkDamageReport struct {
	DryRun bool `json:"dry_run"`
	// Fork copies carrying published=true. On a real run the flag is cleared.
	ForkCopiesPublished []ForkDamagePage `json:"fork_copies_published"`
	ForkCopiesCleared   int64            `json:"fork_copies_cleared"`
	// Live published pages whose generated HTML file is missing. On a real
	// run each is regenerated.
	PagesChecked     int              `json:"pages_checked"`
	MissingStatic    []ForkDamagePage `json:"missing_static"`
	Regenerated      int              `json:"regenerated"`
	RegenerateFailed []ForkDamagePage `json:"regenerate_failed,omitempty"`
}

// ForkDamagePage identifies one page in a ForkDamageReport.
type ForkDamagePage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	// Fork copies only: the fork, and whether a live page exists at the path.
	// Without one the copy is a page created inside the fork, and its flag
	// meant "publish when the fork is merged" — merge with "Publish new
	// pages" to get that back.
	ForkID      string `json:"fork_id,omitempty"`
	HasLivePage *bool  `json:"has_live_page,omitempty"`
	Error       string `json:"error,omitempty"`
}

// staticPagePath is where GenerateStaticPage writes a live page's HTML.
func staticPagePath(fullPath string) string {
	if fullPath == "" || fullPath == "/" {
		fullPath = "/index"
	}
	return "content/generated" + fullPath + ".html"
}

// RepairForkDamage finds, and unless dryRun repairs, the two kinds of damage
// the admin editor could do before 7.4.1 when it saved a fork copy:
//
//   - a fork copy left with published=true. A fork copy is never a page of
//     the site; the flag is cleared (nothing else on the copy changes).
//   - a live published page whose generated HTML was removed. The file is
//     rendered again from the page as it is stored.
//
// Neither repair changes what a page says, so no versions are saved and the
// re-render runs under WithoutIndexNow: no search-engine pings and no change
// to content_modified_at.
func (s *ContentService) RepairForkDamage(ctx context.Context, dryRun bool) (*ForkDamageReport, error) {
	report := &ForkDamageReport{
		DryRun:              dryRun,
		ForkCopiesPublished: []ForkDamagePage{},
		MissingStatic:       []ForkDamagePage{},
	}
	proj := options.Find().SetProjection(bson.M{"title": 1, "full_path": 1, "fork_id": 1})

	// (i) fork copies flagged published
	forkFilter := bson.M{"fork_id": bson.M{"$exists": true, "$ne": nil}, "published": true}
	cursor, err := s.db.FindMany(ctx, "content", forkFilter, proj)
	if err != nil {
		return nil, fmt.Errorf("list published fork copies: %w", err)
	}
	var copies []models.Content
	if err := cursor.All(ctx, &copies); err != nil {
		return nil, fmt.Errorf("read published fork copies: %w", err)
	}
	for _, c := range copies {
		live := bson.M{"full_path": c.FullPath, "deleted": bson.M{"$ne": true}}
		liveOnly(live)
		n, _ := s.db.Count(ctx, "content", live)
		hasLive := n > 0
		page := ForkDamagePage{ID: c.ID.Hex(), Title: c.Title, Path: c.FullPath, HasLivePage: &hasLive}
		if c.ForkID != nil {
			page.ForkID = c.ForkID.Hex()
		}
		report.ForkCopiesPublished = append(report.ForkCopiesPublished, page)
	}
	if !dryRun && len(copies) > 0 {
		res, err := s.db.Collection("content").UpdateMany(ctx, forkFilter, bson.M{"$set": bson.M{"published": false}})
		if err != nil {
			return nil, fmt.Errorf("clear published flag on fork copies: %w", err)
		}
		report.ForkCopiesCleared = res.ModifiedCount
	}

	// (ii) live published pages with no generated file
	liveFilter := bson.M{"published": true, "deleted": bson.M{"$ne": true}}
	liveOnly(liveFilter)
	cursor, err = s.db.FindMany(ctx, "content", liveFilter, proj)
	if err != nil {
		return nil, fmt.Errorf("list published pages: %w", err)
	}
	var pages []models.Content
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, fmt.Errorf("read published pages: %w", err)
	}
	regenCtx := WithoutIndexNow(ctx)
	for _, p := range pages {
		// A legacy row with no full_path has no file of its own (an empty
		// path maps to the homepage's file): never guess one for it.
		if p.FullPath == "" {
			continue
		}
		report.PagesChecked++
		if _, err := os.Stat(staticPagePath(p.FullPath)); err == nil {
			continue
		}
		page := ForkDamagePage{ID: p.ID.Hex(), Title: p.Title, Path: p.FullPath}
		report.MissingStatic = append(report.MissingStatic, page)
		if dryRun {
			continue
		}
		if err := s.regenerateMissingStatic(regenCtx, p.ID); err != nil {
			page.Error = err.Error()
			report.RegenerateFailed = append(report.RegenerateFailed, page)
			continue
		}
		report.Regenerated++
	}
	return report, nil
}

// regenerateMissingStatic writes a live published page's HTML file again.
func (s *ContentService) regenerateMissingStatic(ctx context.Context, id primitive.ObjectID) error {
	content, err := s.GetContent(ctx, id)
	if err != nil {
		return err
	}
	if content.ForkID != nil || !content.Published || content.Deleted {
		return fmt.Errorf("no longer a live published page")
	}
	// GenerateStaticPage skips the write when the rendered HTML matches the
	// stored hash; the hash is still there, the file is not.
	content.ContentHash = ""
	if err := s.GenerateStaticPage(ctx, content); err != nil {
		return err
	}
	if _, err := os.Stat(staticPagePath(content.FullPath)); err != nil {
		return fmt.Errorf("file was not written")
	}
	return nil
}
