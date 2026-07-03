package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// WebhookDoc represents a registered webhook endpoint.
type WebhookDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	URL       string             `bson:"url" json:"url"`
	Secret    string             `bson:"secret" json:"secret"`
	Events    []string           `bson:"events" json:"events"`
	Active    bool               `bson:"active" json:"active"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// WebhookDeliveryDoc records a single delivery attempt for a webhook.
type WebhookDeliveryDoc struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WebhookID   primitive.ObjectID `bson:"webhook_id" json:"webhook_id"`
	Event       string             `bson:"event" json:"event"`
	Attempt     int                `bson:"attempt" json:"attempt"`
	StatusCode  int                `bson:"status_code" json:"status_code"`
	Success     bool               `bson:"success" json:"success"`
	Error       string             `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	DeliveredAt *time.Time         `bson:"delivered_at,omitempty" json:"delivered_at,omitempty"`
}

// WebhookService manages webhook registration and event delivery.
type WebhookService struct {
	db         *database.DB
	httpClient *http.Client
}

// NewWebhookService creates a WebhookService with a 10-second HTTP client that
// does not follow redirects.
func NewWebhookService(db *database.DB) *WebhookService {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &WebhookService{db: db, httpClient: client}
}

// webhookPayload is the JSON body sent to webhook endpoints.
type webhookPayload struct {
	Event     string      `json:"event"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// FireEvent asynchronously fires event to all active matching webhooks.
func (s *WebhookService) FireEvent(ctx context.Context, event string, data interface{}) {
	go func() {
		// Build payload once.
		p := webhookPayload{
			Event:     event,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data:      data,
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return
		}

		// Fetch active webhooks that subscribe to this event.
		filter := bson.M{
			"active": true,
			"events": event,
		}
		cursor, err := s.db.FindMany(ctx, "webhooks", filter)
		if err != nil {
			return
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) {
			var wh WebhookDoc
			if err := cursor.Decode(&wh); err != nil {
				continue
			}
			if err := s.deliver(ctx, wh, event, payload); err != nil {
				go s.retryWithBackoff(wh, event, payload)
			}
		}
	}()
}

// deliver sends the payload to the webhook URL and logs the delivery attempt.
func (s *WebhookService) deliver(ctx context.Context, wh WebhookDoc, event string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(payload))
	if err != nil {
		s.logDelivery(wh.ID, event, 1, 0, false, err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-LightCMS-Event", event)
	req.Header.Set("X-LightCMS-Signature", s.sign(wh.Secret, payload))
	req.Header.Set("User-Agent", "LightCMS-Webhook/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logDelivery(wh.ID, event, 1, 0, false, err.Error())
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	errMsg := ""
	if !success {
		errMsg = fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	s.logDelivery(wh.ID, event, 1, resp.StatusCode, success, errMsg)
	if !success {
		return fmt.Errorf("webhook %s returned status %d", wh.URL, resp.StatusCode)
	}
	return nil
}

// sign returns the HMAC-SHA256 signature in the form sha256=<hex>.
func (s *WebhookService) sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// retryWithBackoff attempts delivery again at 30 s and 5 min after the first failure.
func (s *WebhookService) retryWithBackoff(wh WebhookDoc, event string, payload []byte) {
	delays := []time.Duration{30 * time.Second, 5 * time.Minute}
	for attempt, delay := range delays {
		time.Sleep(delay)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(payload))
		if err != nil {
			cancel()
			s.logDelivery(wh.ID, event, attempt+2, 0, false, err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-LightCMS-Event", event)
		req.Header.Set("X-LightCMS-Signature", s.sign(wh.Secret, payload))
		req.Header.Set("User-Agent", "LightCMS-Webhook/1.0")

		resp, err := s.httpClient.Do(req)
		cancel()
		if err != nil {
			s.logDelivery(wh.ID, event, attempt+2, 0, false, err.Error())
			continue
		}
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()

		success := resp.StatusCode >= 200 && resp.StatusCode < 300
		errMsg := ""
		if !success {
			errMsg = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		}
		s.logDelivery(wh.ID, event, attempt+2, resp.StatusCode, success, errMsg)
		if success {
			return
		}
	}
}

// logDelivery writes a WebhookDeliveryDoc to the database.
func (s *WebhookService) logDelivery(webhookID primitive.ObjectID, event string, attempt, statusCode int, success bool, errMsg string) {
	now := time.Now()
	doc := WebhookDeliveryDoc{
		WebhookID:  webhookID,
		Event:      event,
		Attempt:    attempt,
		StatusCode: statusCode,
		Success:    success,
		Error:      errMsg,
		CreatedAt:  now,
	}
	if success {
		doc.DeliveredAt = &now
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.db.InsertOne(ctx, "webhook_deliveries", doc)
}

// List returns up to limit webhooks sorted by created_at descending.
func (s *WebhookService) List(ctx context.Context, limit int) ([]WebhookDoc, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))
	cursor, err := s.db.FindMany(ctx, "webhooks", bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []WebhookDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// Get returns a single webhook by ID.
func (s *WebhookService) Get(ctx context.Context, id primitive.ObjectID) (*WebhookDoc, error) {
	var doc WebhookDoc
	err := s.db.FindOne(ctx, "webhooks", bson.M{"_id": id}, &doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &doc, err
}

// Create inserts a new webhook document.
func (s *WebhookService) Create(ctx context.Context, name, url, secret string, events []string, active bool) (*WebhookDoc, error) {
	now := time.Now()
	doc := WebhookDoc{
		Name:      name,
		URL:       url,
		Secret:    secret,
		Events:    events,
		Active:    active,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := s.db.InsertOne(ctx, "webhooks", doc)
	if err != nil {
		return nil, err
	}
	doc.ID = id
	return &doc, nil
}

// Update modifies an existing webhook document.
func (s *WebhookService) Update(ctx context.Context, id primitive.ObjectID, name, url, secret string, events []string, active bool) error {
	return s.db.UpdateOne(ctx, "webhooks", bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"name":       name,
			"url":        url,
			"secret":     secret,
			"events":     events,
			"active":     active,
			"updated_at": time.Now(),
		},
	})
}

// RegenerateSecret replaces the secret for a webhook and returns the new plaintext secret.
func (s *WebhookService) RegenerateSecret(ctx context.Context, id primitive.ObjectID, newSecret string) error {
	return s.db.UpdateOne(ctx, "webhooks", bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"secret":     newSecret,
			"updated_at": time.Now(),
		},
	})
}

// Delete removes a webhook document by ID.
func (s *WebhookService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "webhooks", bson.M{"_id": id})
}

// ListDeliveries returns up to limit delivery records for a given webhook.
func (s *WebhookService) ListDeliveries(ctx context.Context, webhookID primitive.ObjectID, limit int) ([]WebhookDeliveryDoc, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))
	cursor, err := s.db.FindMany(ctx, "webhook_deliveries", bson.M{"webhook_id": webhookID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []WebhookDeliveryDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}
