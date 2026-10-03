package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IndexNow (https://www.indexnow.org) lets a site tell participating search
// engines (Bing, Yandex, Seznam, Naver, Yep, ...) that URLs changed, instead
// of waiting for a recrawl. One POST to api.indexnow.org fans out to all of
// them.
//
// The service is built to be safe on any LightCMS install, not just one site:
//   - Every install gets its own random key, generated on first use and
//     served at /{key}.txt. No configuration is required.
//   - It only runs when the configured BASE_URL is a public domain and the
//     server is not in development mode.
//   - Before submitting, it fetches its own key file through BASE_URL and
//     checks the key matches. A staging copy, a misconfigured BASE_URL, or an
//     instance that does not actually serve that domain never submits.
//   - Changes are batched and debounced, and each URL is resubmitted at most
//     once per cooldown window, so a burst of saves sends one request.
//   - Site-wide re-renders (template/theme changes, "regenerate all") are not
//     submitted: the content did not change.
const (
	IndexNowEndpoint = "https://api.indexnow.org/indexnow"

	indexNowMaxBatch     = 10000 // protocol limit per request
	indexNowMaxPending   = 50000 // memory guard
	indexNowCooldown     = 10 * time.Minute
	indexNowDebounce     = 5 * time.Second
	indexNowTick         = 10 * time.Second
	indexNowVerifyTTL    = 30 * time.Minute
	indexNowKeyCacheTTL  = time.Minute
	indexNowCatchUpDelay = 90 * time.Second
	indexNowCatchUpCheck = time.Hour
	indexNowFullMinGap   = time.Hour // minimum gap between full-site submissions
	indexNowMaxAttempts  = 4
	// A brand-new key is verified asynchronously by the engine; until then
	// submissions get 403 SiteVerificationNotCompleted. Retry patiently.
	indexNowVerifyPendingRetry       = 10 * time.Minute
	indexNowVerifyPendingMaxAttempts = 18 // ~3 hours
	indexNowHistorySize              = 20
	indexNowSampleSize               = 5
	indexNowConfigType               = "indexnow_config"
)

// IndexNowSubmission is one recorded POST to the IndexNow endpoint.
type IndexNowSubmission struct {
	At       time.Time `bson:"at" json:"at"`
	Trigger  string    `bson:"trigger" json:"trigger"` // "change" | "catch-up" | "manual"
	URLCount int       `bson:"url_count" json:"url_count"`
	Status   int       `bson:"status" json:"status"` // HTTP status (0 = request not sent)
	Error    string    `bson:"error,omitempty" json:"error,omitempty"`
	Sample   []string  `bson:"sample,omitempty" json:"sample,omitempty"`
}

// IndexNowConfig is stored in the settings collection as type "indexnow_config".
// It is kept out of SiteConfig deliberately: SaveSiteConfig $sets the whole
// struct, which would race with (and could wipe) the background bookkeeping
// fields here.
type IndexNowConfig struct {
	Key             string               `bson:"key" json:"key"`
	Disabled        bool                 `bson:"disabled" json:"disabled"` // on by default
	InitialSubmitAt *time.Time           `bson:"initial_submit_at,omitempty" json:"initial_submit_at,omitempty"`
	LastFullAt      *time.Time           `bson:"last_full_at,omitempty" json:"last_full_at,omitempty"`
	LastSubmitAt    *time.Time           `bson:"last_submit_at,omitempty" json:"last_submit_at,omitempty"`
	LastStatus      int                  `bson:"last_status,omitempty" json:"last_status,omitempty"`
	LastError       string               `bson:"last_error,omitempty" json:"last_error,omitempty"`
	TotalSubmitted  int                  `bson:"total_submitted" json:"total_submitted"`
	History         []IndexNowSubmission `bson:"history,omitempty" json:"history,omitempty"`
}

// IndexNowStatus is the read model for the admin page, API, and MCP.
type IndexNowStatus struct {
	Enabled      bool   `json:"enabled"`
	Active       bool   `json:"active"`               // enabled AND eligible: changes are being submitted
	Ineligible   string `json:"ineligible,omitempty"` // why this install cannot submit
	KeyURL       string `json:"key_url"`
	Verified     bool   `json:"verified"` // key file last confirmed reachable via BASE_URL
	VerifyError  string `json:"verify_error,omitempty"`
	PendingCount int    `json:"pending_count"`
	Endpoint     string `json:"endpoint"`
	IndexNowConfig
}

// errIndexNowVerificationPending marks a 403 SiteVerificationNotCompleted:
// the engine hasn't finished checking the key file yet. Not a real failure.
var errIndexNowVerificationPending = errors.New("IndexNow site verification still in progress")

type indexNowPending struct {
	queuedAt  time.Time
	notBefore time.Time
	attempts  int
}

// IndexNowService batches changed URLs and submits them to IndexNow.
type IndexNowService struct {
	db       *database.DB
	baseURL  string
	base     *url.URL
	dev      bool
	endpoint string
	client   *http.Client

	allowPrivateHosts bool // tests only: httptest servers live on 127.0.0.1

	mu          sync.Mutex
	pending     map[string]*indexNowPending // path → state
	lastSent    map[string]time.Time        // path → last submission
	verifiedKey string
	verifiedAt  time.Time
	verifyErr   string
	cachedKey   string
	cachedKeyAt time.Time
}

// NewIndexNowService creates the service. baseURL is the site's public URL;
// dev disables submission entirely.
func NewIndexNowService(db *database.DB, baseURL string, dev bool) *IndexNowService {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, _ := url.Parse(baseURL)
	return &IndexNowService{
		db:       db,
		baseURL:  baseURL,
		base:     u,
		dev:      dev,
		endpoint: IndexNowEndpoint,
		client:   &http.Client{Timeout: 20 * time.Second},
		pending:  make(map[string]*indexNowPending),
		lastSent: make(map[string]time.Time),
	}
}

// SetEndpoint overrides the IndexNow endpoint (tests only).
func (s *IndexNowService) SetEndpoint(u string) { s.endpoint = u }

// Ineligible returns why this install cannot submit to IndexNow, or "" if it can.
// This is the static check (mode + BASE_URL shape); key-file verification is
// checked separately at submit time.
func (s *IndexNowService) Ineligible() string {
	if s.dev {
		return "server is running in development mode"
	}
	if s.base == nil || s.baseURL == "" {
		return "BASE_URL is not set"
	}
	if s.base.Scheme != "https" && s.base.Scheme != "http" {
		return "BASE_URL must start with https:// or http://"
	}
	if s.allowPrivateHosts {
		return ""
	}
	return indexNowHostIneligible(s.base.Hostname())
}

// indexNowHostIneligible rejects hosts search engines can't reach or that
// clearly aren't a real public site.
func indexNowHostIneligible(host string) string {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" {
		return "BASE_URL has no host"
	}
	if net.ParseIP(h) != nil {
		return "BASE_URL is an IP address, not a public domain"
	}
	if h == "localhost" || !strings.Contains(h, ".") {
		return "BASE_URL (" + h + ") is not a public domain"
	}
	for _, suffix := range []string{".localhost", ".local", ".test", ".example", ".invalid", ".internal", ".lan", ".home.arpa", ".corp"} {
		if strings.HasSuffix(h, suffix) || h == strings.TrimPrefix(suffix, ".") {
			return "BASE_URL (" + h + ") is a reserved/private domain"
		}
	}
	for _, reserved := range []string{"example.com", "example.net", "example.org"} {
		if h == reserved || strings.HasSuffix(h, "."+reserved) {
			return "BASE_URL (" + h + ") is a reserved example domain"
		}
	}
	return ""
}

// ==================== Configuration ====================

// GetConfig loads the stored configuration, generating the key on first use.
func (s *IndexNowService) GetConfig(ctx context.Context) (IndexNowConfig, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return cfg, err
	}
	if cfg.Key != "" {
		return cfg, nil
	}
	key, err := newIndexNowKey()
	if err != nil {
		return cfg, err
	}
	// Insert the doc with the key if it doesn't exist yet, or fill in the key
	// on a doc that lacks one. Then reread: if two machines race on first
	// start, both converge on whichever key was written first.
	if _, err := s.db.Settings().UpdateOne(ctx,
		bson.M{"type": indexNowConfigType},
		bson.M{"$setOnInsert": bson.M{"type": indexNowConfigType, "key": key, "disabled": false, "total_submitted": 0}},
		options.Update().SetUpsert(true),
	); err != nil {
		return cfg, fmt.Errorf("indexnow: save key: %w", err)
	}
	if _, err := s.db.Settings().UpdateOne(ctx,
		bson.M{"type": indexNowConfigType, "key": bson.M{"$in": bson.A{nil, ""}}},
		bson.M{"$set": bson.M{"key": key}},
	); err != nil {
		return cfg, fmt.Errorf("indexnow: save key: %w", err)
	}
	return s.loadConfig(ctx)
}

func (s *IndexNowService) loadConfig(ctx context.Context) (IndexNowConfig, error) {
	var cfg IndexNowConfig
	err := s.db.Settings().FindOne(ctx, bson.M{"type": indexNowConfigType}).Decode(&cfg)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return cfg, fmt.Errorf("indexnow: load config: %w", err)
	}
	return cfg, nil
}

func newIndexNowKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("indexnow: generate key: %w", err)
	}
	return hex.EncodeToString(b), nil // 32 chars of [0-9a-f], within the 8–128 the protocol allows
}

// SetEnabled turns automatic submission on or off.
func (s *IndexNowService) SetEnabled(ctx context.Context, enabled bool) error {
	if _, err := s.GetConfig(ctx); err != nil {
		return err
	}
	if !enabled {
		s.mu.Lock()
		s.pending = make(map[string]*indexNowPending)
		s.mu.Unlock()
	}
	return s.db.Settings().FindOneAndUpdate(ctx, bson.M{"type": indexNowConfigType},
		bson.M{"$set": bson.M{"disabled": !enabled}}).Err()
}

// RegenerateKey replaces the key (e.g. if it was leaked or rejected).
func (s *IndexNowService) RegenerateKey(ctx context.Context) (string, error) {
	if _, err := s.GetConfig(ctx); err != nil {
		return "", err
	}
	key, err := newIndexNowKey()
	if err != nil {
		return "", err
	}
	if err := s.db.Settings().FindOneAndUpdate(ctx, bson.M{"type": indexNowConfigType},
		bson.M{"$set": bson.M{"key": key}}).Err(); err != nil {
		return "", fmt.Errorf("indexnow: save key: %w", err)
	}
	s.mu.Lock()
	s.cachedKey, s.cachedKeyAt = key, time.Now()
	s.verifiedKey, s.verifiedAt, s.verifyErr = "", time.Time{}, ""
	s.mu.Unlock()
	return key, nil
}

// KeyForFile returns the key to serve at /{key}.txt (cached briefly so the
// public route doesn't hit MongoDB on every *.txt request).
func (s *IndexNowService) KeyForFile(ctx context.Context) string {
	s.mu.Lock()
	if s.cachedKey != "" && time.Since(s.cachedKeyAt) < indexNowKeyCacheTTL {
		k := s.cachedKey
		s.mu.Unlock()
		return k
	}
	s.mu.Unlock()
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return ""
	}
	s.mu.Lock()
	s.cachedKey, s.cachedKeyAt = cfg.Key, time.Now()
	s.mu.Unlock()
	return cfg.Key
}

// KeyURL is where the key file is published for this install.
func (s *IndexNowService) KeyURL(key string) string {
	if key == "" {
		return ""
	}
	return s.baseURL + "/" + key + ".txt"
}

// Status returns the current state for display.
func (s *IndexNowService) Status(ctx context.Context) (IndexNowStatus, error) {
	cfg, err := s.GetConfig(ctx)
	st := IndexNowStatus{IndexNowConfig: cfg, Endpoint: s.endpoint}
	if err != nil {
		return st, err
	}
	st.Enabled = !cfg.Disabled
	st.Ineligible = s.Ineligible()
	st.Active = st.Enabled && st.Ineligible == ""
	st.KeyURL = s.KeyURL(cfg.Key)
	s.mu.Lock()
	st.PendingCount = len(s.pending)
	st.Verified = s.verifiedKey == cfg.Key && cfg.Key != "" && time.Since(s.verifiedAt) < indexNowVerifyTTL
	st.VerifyError = s.verifyErr
	s.mu.Unlock()
	return st, nil
}

// ==================== Change notification ====================

// Notify queues site paths (e.g. "/blog/post") whose public content changed,
// appeared, or disappeared. It never blocks or touches the network.
func (s *IndexNowService) Notify(paths ...string) {
	if s == nil || s.Ineligible() != "" {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range paths {
		p = normalizeIndexNowPath(p)
		if p == "" {
			continue
		}
		if existing, ok := s.pending[p]; ok {
			existing.queuedAt = now // restart debounce for this path
			continue
		}
		if len(s.pending) >= indexNowMaxPending {
			return
		}
		notBefore := now
		if last, ok := s.lastSent[p]; ok && now.Sub(last) < indexNowCooldown {
			notBefore = last.Add(indexNowCooldown)
		}
		s.pending[p] = &indexNowPending{queuedAt: now, notBefore: notBefore}
	}
}

func normalizeIndexNowPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// Start runs the flush loop and the one-time catch-up. It returns when ctx ends.
func (s *IndexNowService) Start(ctx context.Context) {
	if reason := s.Ineligible(); reason != "" {
		log.Printf("IndexNow: inactive (%s)", reason)
		return
	}
	log.Printf("IndexNow: active for %s", s.baseURL)

	ticker := time.NewTicker(indexNowTick)
	defer ticker.Stop()
	catchUp := time.NewTimer(indexNowCatchUpDelay)
	defer catchUp.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flush(ctx, time.Now())
		case <-catchUp.C:
			next := indexNowCatchUpCheck
			if n, err := s.CatchUp(ctx); err != nil {
				log.Printf("IndexNow: catch-up not done: %v", err)
				if errors.Is(err, errIndexNowVerificationPending) {
					next = indexNowVerifyPendingRetry
				}
			} else if n > 0 {
				log.Printf("IndexNow: catch-up submitted %d URLs", n)
			}
			catchUp.Reset(next)
		}
	}
}

// flush submits pending paths that are past their debounce and cooldown.
func (s *IndexNowService) flush(ctx context.Context, now time.Time) {
	s.mu.Lock()
	var ready []string
	for p, st := range s.pending {
		if !now.Before(st.notBefore) && now.Sub(st.queuedAt) >= indexNowDebounce {
			ready = append(ready, p)
		}
	}
	s.mu.Unlock()
	if len(ready) == 0 {
		return
	}
	sort.Strings(ready)

	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return // DB hiccup: keep pending, retry next tick
	}
	if cfg.Disabled {
		s.mu.Lock()
		s.pending = make(map[string]*indexNowPending)
		s.mu.Unlock()
		return
	}

	for start := 0; start < len(ready); start += indexNowMaxBatch {
		end := start + indexNowMaxBatch
		if end > len(ready) {
			end = len(ready)
		}
		batch := ready[start:end]
		status, err := s.submit(ctx, cfg.Key, batch, "change")

		s.mu.Lock()
		for _, p := range batch {
			st := s.pending[p]
			if st == nil {
				continue
			}
			if errors.Is(err, errIndexNowVerificationPending) && st.attempts+1 < indexNowVerifyPendingMaxAttempts {
				st.attempts++
				st.notBefore = now.Add(indexNowVerifyPendingRetry)
				continue
			}
			if err != nil && indexNowRetryable(status) && st.attempts+1 < indexNowMaxAttempts {
				st.attempts++
				st.notBefore = now.Add(time.Duration(1<<st.attempts) * time.Minute)
				continue
			}
			delete(s.pending, p)
		}
		s.mu.Unlock()
	}
}

// indexNowRetryable: network errors (0), throttling, and server errors.
// 400/403/422 mean the request itself is wrong; retrying won't help.
func indexNowRetryable(status int) bool {
	return status == 0 || status == http.StatusTooManyRequests || status >= 500
}

// ==================== Full-site submission ====================

// CatchUp submits every published URL once, the first time IndexNow is
// active on this install (sites that predate IndexNow support otherwise
// only get changed pages submitted). Claimed atomically so multiple machines
// sharing a database submit once; released on failure so it retries later.
func (s *IndexNowService) CatchUp(ctx context.Context) (int, error) {
	if reason := s.Ineligible(); reason != "" {
		return 0, fmt.Errorf("%s", reason)
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return 0, err
	}
	if cfg.Disabled || cfg.InitialSubmitAt != nil {
		return 0, nil
	}
	now := time.Now()
	res, err := s.db.Settings().UpdateOne(ctx,
		bson.M{"type": indexNowConfigType, "disabled": bson.M{"$ne": true}, "initial_submit_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"initial_submit_at": now}})
	if err != nil {
		return 0, err
	}
	if res.ModifiedCount == 0 {
		return 0, nil // another machine claimed it, or it was disabled meanwhile
	}
	n, err := s.submitAll(ctx, cfg.Key, "catch-up")
	if err != nil {
		s.db.Settings().UpdateOne(ctx, bson.M{"type": indexNowConfigType}, //nolint:errcheck
			bson.M{"$unset": bson.M{"initial_submit_at": ""}})
		return 0, err
	}
	return n, nil
}

// SubmitAll submits every published URL now (admin "submit all" action).
// Limited to once per hour: engines treat repeated full-site pings as abuse.
func (s *IndexNowService) SubmitAll(ctx context.Context) (int, error) {
	if reason := s.Ineligible(); reason != "" {
		return 0, fmt.Errorf("IndexNow is inactive: %s", reason)
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return 0, err
	}
	if cfg.Disabled {
		return 0, fmt.Errorf("IndexNow is disabled for this site")
	}
	if cfg.LastFullAt != nil && time.Since(*cfg.LastFullAt) < indexNowFullMinGap {
		wait := indexNowFullMinGap - time.Since(*cfg.LastFullAt)
		return 0, fmt.Errorf("all URLs were submitted %s ago; try again in %s",
			time.Since(*cfg.LastFullAt).Round(time.Minute), wait.Round(time.Minute))
	}
	return s.submitAll(ctx, cfg.Key, "manual")
}

func (s *IndexNowService) submitAll(ctx context.Context, key, trigger string) (int, error) {
	paths, err := s.publishedPaths(ctx)
	if err != nil {
		return 0, err
	}
	if len(paths) == 0 {
		return 0, nil
	}
	sent := 0
	for start := 0; start < len(paths); start += indexNowMaxBatch {
		end := start + indexNowMaxBatch
		if end > len(paths) {
			end = len(paths)
		}
		if _, err := s.submit(ctx, key, paths[start:end], trigger); err != nil {
			return sent, err
		}
		sent += end - start
	}
	now := time.Now()
	s.db.Settings().UpdateOne(ctx, bson.M{"type": indexNowConfigType}, //nolint:errcheck
		bson.M{"$set": bson.M{"last_full_at": now}})
	return sent, nil
}

// publishedPaths lists live public pages: published, not deleted, not fork copies.
func (s *IndexNowService) publishedPaths(ctx context.Context) ([]string, error) {
	cursor, err := s.db.Collection("content").Find(ctx,
		bson.M{"published": true, "deleted": bson.M{"$ne": true}, "fork_id": nil, "noindex": bson.M{"$ne": true}},
		options.Find().SetProjection(bson.M{"full_path": 1}))
	if err != nil {
		return nil, fmt.Errorf("indexnow: list content: %w", err)
	}
	defer cursor.Close(ctx)
	seen := map[string]bool{}
	var paths []string
	for cursor.Next(ctx) {
		var doc struct {
			FullPath string `bson:"full_path"`
		}
		if cursor.Decode(&doc) != nil {
			continue
		}
		p := normalizeIndexNowPath(doc.FullPath)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, cursor.Err()
}

// ==================== Wire protocol ====================

// submit verifies the key file, POSTs the URLs, and records the outcome.
// Returns the HTTP status (0 if no request was sent).
func (s *IndexNowService) submit(ctx context.Context, key string, paths []string, trigger string) (int, error) {
	urls := make([]string, 0, len(paths))
	for _, p := range paths {
		urls = append(urls, s.pageURL(p))
	}

	if err := s.verifyKeyFile(ctx, key); err != nil {
		s.record(ctx, IndexNowSubmission{At: time.Now(), Trigger: trigger, URLCount: len(urls), Error: err.Error(), Sample: sampleURLs(urls)})
		return 0, err
	}

	body, err := json.Marshal(map[string]interface{}{
		"host":        s.base.Host,
		"key":         key,
		"keyLocation": s.KeyURL(key),
		"urlList":     urls,
	})
	if err != nil {
		return 0, fmt.Errorf("indexnow: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("indexnow: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "LightCMS-IndexNow/1.0 (+"+s.baseURL+")")

	resp, err := s.client.Do(req)
	if err != nil {
		err = fmt.Errorf("indexnow: request failed: %w", err)
		s.record(ctx, IndexNowSubmission{At: time.Now(), Trigger: trigger, URLCount: len(urls), Error: err.Error(), Sample: sampleURLs(urls)})
		return 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	sub := IndexNowSubmission{At: time.Now(), Trigger: trigger, URLCount: len(urls), Status: resp.StatusCode, Sample: sampleURLs(urls)}
	var subErr error
	// 200 = accepted; 202 = accepted, key validation pending.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body := strings.TrimSpace(string(respBody))
		if resp.StatusCode == http.StatusForbidden && strings.Contains(body, "SiteVerificationNotCompleted") {
			subErr = fmt.Errorf("indexnow: %w — the search engine is still checking the key file; will retry", errIndexNowVerificationPending)
		} else {
			subErr = fmt.Errorf("indexnow: %s", indexNowStatusText(resp.StatusCode, body))
		}
		sub.Error = subErr.Error()
	}
	s.record(ctx, sub)

	if subErr == nil {
		now := time.Now()
		s.mu.Lock()
		for _, p := range paths {
			s.lastSent[p] = now
		}
		for p, t := range s.lastSent { // prune expired cooldowns
			if now.Sub(t) > indexNowCooldown {
				delete(s.lastSent, p)
			}
		}
		s.mu.Unlock()
	}
	return resp.StatusCode, subErr
}

func indexNowStatusText(code int, body string) string {
	var msg string
	switch code {
	case http.StatusBadRequest:
		msg = "400 bad request (invalid format)"
	case http.StatusForbidden:
		msg = "403 forbidden — key not valid (key file not found or doesn't match)"
	case http.StatusUnprocessableEntity:
		msg = "422 unprocessable — URLs don't belong to the host, or key doesn't match the protocol schema"
	case http.StatusTooManyRequests:
		msg = "429 too many requests (throttled)"
	default:
		msg = fmt.Sprintf("unexpected status %d", code)
	}
	if body != "" && len(body) < 300 {
		msg += ": " + body
	}
	return msg
}

// pageURL builds the absolute URL for a site path, escaping as needed.
func (s *IndexNowService) pageURL(p string) string {
	u := *s.base
	u.Path = strings.TrimRight(s.base.Path, "/") + p
	u.RawPath = ""
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

// verifyKeyFile confirms that BASE_URL really serves this install's key —
// i.e. that this server is the site it claims to be. Cached for a while.
func (s *IndexNowService) verifyKeyFile(ctx context.Context, key string) error {
	s.mu.Lock()
	if s.verifiedKey == key && time.Since(s.verifiedAt) < indexNowVerifyTTL {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	err := func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.KeyURL(key), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("key file not reachable at %s: %v", s.KeyURL(key), err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("key file at %s returned %d — is BASE_URL pointing at this server?", s.KeyURL(key), resp.StatusCode)
		}
		if strings.TrimSpace(string(b)) != key {
			return fmt.Errorf("key file at %s does not match this server's key — BASE_URL is served by a different instance", s.KeyURL(key))
		}
		return nil
	}()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.verifyErr = err.Error()
		return err
	}
	s.verifiedKey, s.verifiedAt, s.verifyErr = key, time.Now(), ""
	return nil
}

func (s *IndexNowService) record(ctx context.Context, sub IndexNowSubmission) {
	set := bson.M{"last_submit_at": sub.At, "last_status": sub.Status, "last_error": sub.Error}
	update := bson.M{
		"$set":  set,
		"$push": bson.M{"history": bson.M{"$each": bson.A{sub}, "$slice": -indexNowHistorySize}},
	}
	if sub.Error == "" {
		update["$inc"] = bson.M{"total_submitted": sub.URLCount}
	}
	if _, err := s.db.Settings().UpdateOne(ctx, bson.M{"type": indexNowConfigType}, update); err != nil {
		log.Printf("IndexNow: failed to record submission: %v", err)
	}
	if sub.Error != "" {
		log.Printf("IndexNow: submission of %d URLs failed: %s", sub.URLCount, sub.Error)
	} else {
		log.Printf("IndexNow: submitted %d URLs (%s, status %d)", sub.URLCount, sub.Trigger, sub.Status)
	}
}

func sampleURLs(urls []string) []string {
	if len(urls) <= indexNowSampleSize {
		return append([]string(nil), urls...)
	}
	return append([]string(nil), urls[:indexNowSampleSize]...)
}
