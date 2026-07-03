package auth

import (
	"context"
	"net/http/httptest"
	"testing"
)

// TestCheckRateLimit covers the not-locked, seconds-only, and minutes+seconds
// branches of Manager.CheckRateLimit, plus RecordFailedLogin/ClearRateLimit.
func TestCheckRateLimit(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()

	req := httptest.NewRequest("POST", "/cm/login", nil)
	req.RemoteAddr = "198.51.100.42:12345"

	// Fresh IP — not locked.
	if locked, msg := mgr.CheckRateLimit(ctx, req); locked || msg != "" {
		t.Fatalf("fresh IP should not be locked, got locked=%v msg=%q", locked, msg)
	}

	// 10 failed attempts → 1-minute lockout → duration < 1 min → seconds branch.
	for i := 0; i < 10; i++ {
		mgr.RecordFailedLogin(ctx, req)
	}
	locked, msg := mgr.CheckRateLimit(ctx, req)
	if !locked || msg == "" {
		t.Fatalf("expected lock after 10 failures, got locked=%v msg=%q", locked, msg)
	}

	// 15 failed attempts → 5-minute lockout → minutes > 0 branch.
	for i := 0; i < 5; i++ {
		mgr.RecordFailedLogin(ctx, req)
	}
	locked, msg = mgr.CheckRateLimit(ctx, req)
	if !locked || msg == "" {
		t.Fatalf("expected lock after 15 failures, got locked=%v msg=%q", locked, msg)
	}

	// Clearing removes the lock.
	mgr.ClearRateLimit(ctx, req)
	if locked, _ := mgr.CheckRateLimit(ctx, req); locked {
		t.Error("expected unlock after ClearRateLimit")
	}
}
