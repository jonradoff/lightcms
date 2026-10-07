package netguard

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsPrivateOrReservedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "127.8.9.10", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "198.18.0.1", "240.0.0.1", "255.255.255.255",
		"::1", "::", "fe80::1", "fc00::1", "fd12:3456::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254"}
	for _, s := range blocked {
		if !IsPrivateOrReservedIP(net.ParseIP(s)) {
			t.Errorf("%s is not blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "172.32.0.1", "2606:4700:4700::1111", "::ffff:8.8.8.8"} {
		if IsPrivateOrReservedIP(net.ParseIP(s)) {
			t.Errorf("%s is blocked", s)
		}
	}
}

// The guarded client never connects to a loopback listener, by address or
// by name, directly or through a redirect, and says why.
func TestClient_RefusesLoopback(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	port := srv.URL[strings.LastIndex(srv.URL, ":"):]

	c := NewClient(5 * time.Second)
	if c.Transport != Transport {
		t.Fatal("NewClient does not use the guarded transport")
	}
	for _, u := range []string{srv.URL, "http://localhost" + port, "http://[::1]" + port, "http://[::ffff:127.0.0.1]" + port, "http://169.254.169.254/"} {
		resp, err := c.Get(u)
		if err == nil {
			resp.Body.Close()
			t.Errorf("GET %s succeeded", u)
			continue
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("GET %s: %v, want ErrBlockedAddress", u, err)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("the loopback server was contacted %d time(s)", n)
	}
	if tr, ok := Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Error("the guarded transport must not use a proxy: the check would be made on the proxy's address")
	}
}
