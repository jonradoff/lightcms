package services

import (
	"context"
	"testing"

	"github.com/jonradoff/lightcms/v6/internal/testutil"
)

func TestSnippetService_CreateAndGet(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSnippetService(db)
	ctx := context.Background()

	snip, err := svc.CreateSnippet(ctx, "header-cta", "<div>CTA</div>")
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	if snip.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
	if snip.Name != "header-cta" {
		t.Errorf("expected name 'header-cta', got %q", snip.Name)
	}
	if snip.HTML != "<div>CTA</div>" {
		t.Errorf("unexpected HTML: %q", snip.HTML)
	}

	got, err := svc.GetSnippet(ctx, snip.ID)
	if err != nil {
		t.Fatalf("GetSnippet: %v", err)
	}
	if got.Name != snip.Name {
		t.Errorf("expected %q, got %q", snip.Name, got.Name)
	}
}

func TestSnippetService_GetByName(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSnippetService(db)
	ctx := context.Background()

	_, err := svc.CreateSnippet(ctx, "footer-notice", "<p>notice</p>")
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}

	snip, err := svc.GetSnippetByName(ctx, "footer-notice")
	if err != nil {
		t.Fatalf("GetSnippetByName: %v", err)
	}
	if snip.HTML != "<p>notice</p>" {
		t.Errorf("unexpected HTML: %q", snip.HTML)
	}
}

func TestSnippetService_GetByName_NotFound(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSnippetService(db)
	ctx := context.Background()

	_, err := svc.GetSnippetByName(ctx, "nonexistent-snippet")
	if err == nil {
		t.Error("expected error for missing snippet")
	}
}

func TestSnippetService_ListSnippets(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSnippetService(db)
	ctx := context.Background()

	// Empty list
	list, err := svc.ListSnippets(ctx)
	if err != nil {
		t.Fatalf("ListSnippets: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 snippets, got %d", len(list))
	}

	svc.CreateSnippet(ctx, "beta", "<b>B</b>")
	svc.CreateSnippet(ctx, "alpha", "<b>A</b>")

	list, err = svc.ListSnippets(ctx)
	if err != nil {
		t.Fatalf("ListSnippets: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 snippets, got %d", len(list))
	}
	// sorted by name
	if list[0].Name != "alpha" {
		t.Errorf("expected first snippet to be 'alpha', got %q", list[0].Name)
	}
}

func TestSnippetService_UpdateSnippet(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSnippetService(db)
	ctx := context.Background()

	snip, err := svc.CreateSnippet(ctx, "old-name", "<p>old</p>")
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}

	updated, err := svc.UpdateSnippet(ctx, snip.ID, "new-name", "<p>new</p>")
	if err != nil {
		t.Fatalf("UpdateSnippet: %v", err)
	}
	if updated.Name != "new-name" {
		t.Errorf("expected 'new-name', got %q", updated.Name)
	}
	if updated.HTML != "<p>new</p>" {
		t.Errorf("expected updated HTML, got %q", updated.HTML)
	}
}

func TestSnippetService_DeleteSnippet(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSnippetService(db)
	ctx := context.Background()

	snip, err := svc.CreateSnippet(ctx, "to-delete", "<p>bye</p>")
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}

	if err := svc.DeleteSnippet(ctx, snip.ID); err != nil {
		t.Fatalf("DeleteSnippet: %v", err)
	}

	// Should be gone
	_, err = svc.GetSnippet(ctx, snip.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}
