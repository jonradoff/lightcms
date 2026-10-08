package importer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/netguard"
)

// The package's feeds are served by httptest on 127.0.0.1, which the SSRF
// guard refuses: the tests fetch with a plain client, and the guard itself
// is tested below with the production one.
func TestMain(m *testing.M) {
	guarded := FeedClient
	FeedClient = &http.Client{Timeout: 30 * time.Second}
	productionFeedClient = guarded
	os.Exit(m.Run())
}

var productionFeedClient *http.Client

// A feed URL on the server's own network is never fetched.
func TestParseFeed_BlocksPrivateDestinations(t *testing.T) {
	if productionFeedClient.Transport != netguard.Transport {
		t.Fatal("FeedClient does not dial through netguard.Transport")
	}
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	prev := FeedClient
	FeedClient = productionFeedClient
	defer func() { FeedClient = prev }()

	for _, u := range []string{srv.URL, "http://169.254.169.254/latest/meta-data/"} {
		_, err := ParseFeed(context.Background(), u)
		if !errors.Is(err, netguard.ErrBlockedAddress) {
			t.Errorf("ParseFeed(%s): err = %v, want a blocked-address error", u, err)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("the loopback feed server was contacted %d time(s)", n)
	}
}
