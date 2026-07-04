package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestWebhookService_RetryWithBackoff_EventualSuccess(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&hits, 1)
		if n < 2 {
			http.Error(w, "not yet", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ws := NewWebhookService(db)
	ws.retryDelays = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond}

	wh := WebhookDoc{ID: primitive.NewObjectID(), URL: srv.URL, Secret: "s3cret"}
	ws.retryWithBackoff(wh, "content.publish", []byte(`{"event":"content.publish"}`))

	// Attempt 1 fails, attempt 2 succeeds and stops the loop.
	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Errorf("endpoint hits = %d, want 2", got)
	}

	deliveries, err := ws.ListDeliveries(context.Background(), wh.ID, 10)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %d, want 2", len(deliveries))
	}
	var successes int
	for _, d := range deliveries {
		if d.Success {
			successes++
			if d.DeliveredAt == nil {
				t.Error("successful delivery missing delivered_at")
			}
		}
	}
	if successes != 1 {
		t.Errorf("successful deliveries = %d, want 1", successes)
	}
}

func TestWebhookService_RetryWithBackoff_AllFail(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		http.Error(w, "still broken", http.StatusBadGateway)
	}))
	defer srv.Close()

	ws := NewWebhookService(db)
	ws.retryDelays = []time.Duration{time.Millisecond, time.Millisecond}

	wh := WebhookDoc{ID: primitive.NewObjectID(), URL: srv.URL, Secret: "s"}
	ws.retryWithBackoff(wh, "content.update", []byte(`{}`))

	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Errorf("endpoint hits = %d, want 2", got)
	}
	deliveries, err := ws.ListDeliveries(context.Background(), wh.ID, 10)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	for _, d := range deliveries {
		if d.Success {
			t.Errorf("unexpected successful delivery: %+v", d)
		}
	}
}

func TestWebhookService_RetryWithBackoff_ConnectionError(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ws := NewWebhookService(db)
	ws.retryDelays = []time.Duration{time.Millisecond}

	// Closed server → connection refused on every attempt.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	wh := WebhookDoc{ID: primitive.NewObjectID(), URL: url, Secret: "s"}
	ws.retryWithBackoff(wh, "content.delete", []byte(`{}`))

	deliveries, err := ws.ListDeliveries(context.Background(), wh.ID, 10)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(deliveries))
	}
	if deliveries[0].Success || deliveries[0].Error == "" {
		t.Errorf("expected failed delivery with error, got %+v", deliveries[0])
	}
}

func TestAnalyticsService_UptimePingAndSummary(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewAnalyticsService(ctx, db, "https://example.com")
	defer svc.Stop()

	// Zero-range summary short-circuits to 100%.
	pct, total, human := svc.GetUptimeSummary(ctx, time.Now().Add(time.Minute))
	if pct != 100.0 || total != 0 || human != 0 {
		t.Errorf("empty summary = %v/%d/%d, want 100/0/0", pct, total, human)
	}

	// Record heartbeats for the current hour.
	svc.recordUptimePing()
	svc.recordUptimePing()

	// GetUptimeSummary only counts fully-elapsed hours, so seed a past-hour
	// bucket to make the percentage non-zero.
	if _, err := db.InsertOne(ctx, activityCollection, bson.M{
		"user_id": hourlyUserID, "date": hourKey(time.Now().Add(-time.Hour)),
		"uptime_pings": 3, "visitors": bson.A{"v1"}, "created_at": time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed past bucket: %v", err)
	}

	pct, total, _ = svc.GetUptimeSummary(ctx, time.Now().Add(-2*time.Hour))
	if pct <= 0 {
		t.Errorf("uptime pct = %v, want > 0 after heartbeats", pct)
	}
	if total < 1 {
		t.Errorf("total visitors = %d, want >= 1", total)
	}

	stats, err := svc.GetHourlyStats(ctx, time.Now().Add(-2*time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("GetHourlyStats: %v", err)
	}
	foundPings := false
	for _, st := range stats {
		if st.UptimePings >= 2 {
			foundPings = true
		}
	}
	if !foundPings {
		t.Errorf("expected an hourly bucket with >=2 uptime pings, got %+v", stats)
	}
}

func TestSettingsService_DeleteCollectionError(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSettingsService(db, NewContentService(db))

	db.SetFaultHook(testutil.FailOp("DeleteOne"))
	err := svc.DeleteCollection(ctx, primitive.NewObjectID())
	db.SetFaultHook(nil)
	if err == nil {
		t.Error("expected DeleteCollection error with injected DeleteOne fault")
	}

	// Success path with a real document.
	id, insertErr := db.InsertOne(ctx, "collections", bson.M{"name": "C", "slug": "c"})
	if insertErr != nil {
		t.Fatalf("insert collection: %v", insertErr)
	}
	if err := svc.DeleteCollection(ctx, id); err != nil {
		t.Errorf("DeleteCollection: %v", err)
	}
}
