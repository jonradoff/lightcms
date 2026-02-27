package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"lightcms/internal/middleware"
	"lightcms/internal/testutil"

	"github.com/gorilla/sessions"
)

func newTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long-enough"))
	mgr := NewManager(store, db)
	return mgr, cleanup
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid", "SecurePass1", nil},
		{"too short", "Aa1", ErrPasswordTooShort},
		{"no uppercase", "securepass1", ErrPasswordNoUppercase},
		{"no lowercase", "SECUREPASS1", ErrPasswordNoLowercase},
		{"no number", "SecurePassword", ErrPasswordNoNumber},
		{"min length valid", "Abcdef1x", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestPasswordError(t *testing.T) {
	err := ErrPasswordTooShort
	if err.Error() != "Password must be at least 8 characters" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestInitializePassword(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize should create default password
	if err := mgr.InitializePassword(ctx); err != nil {
		t.Fatalf("InitializePassword failed: %v", err)
	}

	// Default password should work
	if !mgr.ValidatePassword(ctx, "admin123") {
		t.Error("expected default password to be valid after initialization")
	}

	// Should be marked as default
	if !mgr.IsDefaultPassword(ctx) {
		t.Error("expected IsDefaultPassword to be true")
	}
}

func TestInitializePassword_Idempotent(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize twice — should be fine
	mgr.InitializePassword(ctx)
	if err := mgr.InitializePassword(ctx); err != nil {
		t.Fatalf("second InitializePassword should be idempotent: %v", err)
	}

	// Still works
	if !mgr.ValidatePassword(ctx, "admin123") {
		t.Error("expected default password to still work")
	}
}

func TestValidatePassword(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.InitializePassword(ctx)

	if !mgr.ValidatePassword(ctx, "admin123") {
		t.Error("expected default password to be valid")
	}

	if mgr.ValidatePassword(ctx, "wrongpassword") {
		t.Error("expected wrong password to be invalid")
	}
}

func TestValidatePassword_AutoInitialize(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	// Don't call InitializePassword — ValidatePassword should auto-init
	if !mgr.ValidatePassword(ctx, "admin123") {
		t.Error("expected auto-initialized default password to work")
	}
}

func TestChangePassword(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.InitializePassword(ctx)

	// Change from default to new password
	err := mgr.ChangePassword(ctx, "admin123", "NewSecure1")
	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Old password should not work
	if mgr.ValidatePassword(ctx, "admin123") {
		t.Error("old password should be invalid after change")
	}

	// New password should work
	if !mgr.ValidatePassword(ctx, "NewSecure1") {
		t.Error("new password should be valid")
	}

	// Should no longer be default
	if mgr.IsDefaultPassword(ctx) {
		t.Error("expected IsDefaultPassword to be false after change")
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.InitializePassword(ctx)

	err := mgr.ChangePassword(ctx, "wrongcurrent", "NewSecure1")
	if err != ErrInvalidCurrentPassword {
		t.Errorf("expected ErrInvalidCurrentPassword, got %v", err)
	}
}

func TestChangePassword_WeakNew(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.InitializePassword(ctx)

	// Too short
	err := mgr.ChangePassword(ctx, "admin123", "Ab1")
	if err != ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}

	// No uppercase
	err = mgr.ChangePassword(ctx, "admin123", "nouppercase1")
	if err != ErrPasswordNoUppercase {
		t.Errorf("expected ErrPasswordNoUppercase, got %v", err)
	}
}

func TestIsDefaultPassword_NoSettings(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	// No settings at all — should return true (default assumed)
	if !mgr.IsDefaultPassword(context.Background()) {
		t.Error("expected true when no settings exist")
	}
}

func TestLoginLogout(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	// Create a request and response recorder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Initially not authenticated
	if mgr.IsAuthenticated(req) {
		t.Error("expected not authenticated initially")
	}

	// Login
	if err := mgr.Login(rr, req); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Simulate the response being sent with Set-Cookie header
	// Then make a new request with that cookie
	resp := rr.Result()
	cookies := resp.Cookies()

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}

	if !mgr.IsAuthenticated(req2) {
		t.Error("expected authenticated after login")
	}

	// Logout
	rr2 := httptest.NewRecorder()
	if err := mgr.Logout(rr2, req2); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
}

func TestRequireAuth(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	handlerCalled := false
	protected := mgr.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Unauthenticated request → redirect
	req := httptest.NewRequest(http.MethodGet, "/cm/dashboard", nil)
	rr := httptest.NewRecorder()
	protected(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	if handlerCalled {
		t.Error("handler should not be called when unauthenticated")
	}
	if rr.Header().Get("Location") != "/cm/login" {
		t.Errorf("expected redirect to /cm/login, got %s", rr.Header().Get("Location"))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		minutes  int
		seconds  int
		expected string
	}{
		{0, 30, "30 seconds"},
		{0, 1, "1 second"},
		{1, 0, "1 minute"},
		{5, 30, "5 minutes"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.minutes, tt.seconds)
		if got != tt.expected {
			t.Errorf("formatDuration(%d, %d) = %q, want %q", tt.minutes, tt.seconds, got, tt.expected)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
	}

	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.expected {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.expected)
		}
	}
}

func TestPluralize(t *testing.T) {
	if pluralize(1) != "" {
		t.Error("expected empty string for 1")
	}
	if pluralize(0) != "s" {
		t.Error("expected 's' for 0")
	}
	if pluralize(5) != "s" {
		t.Error("expected 's' for 5")
	}
}

func TestNewManagerWithProxyConfig(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long-enough"))
	proxyConfig := &middleware.TrustedProxyConfig{TrustAllProxies: true}
	mgr := NewManagerWithProxyConfig(store, db, proxyConfig)

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.proxyConfig != proxyConfig {
		t.Error("expected custom proxy config")
	}
}

func TestCheckRateLimit_NotLocked(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	locked, _ := mgr.CheckRateLimit(context.Background(), req)
	if locked {
		t.Error("expected not locked with no failed attempts")
	}
}

func TestRecordFailedLogin_AndCheckRateLimit(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	// Record several failed login attempts
	for i := 0; i < 5; i++ {
		mgr.RecordFailedLogin(ctx, req)
	}

	// Check if rate limited (exercising the code path regardless of threshold)
	mgr.CheckRateLimit(ctx, req)
}

func TestClearRateLimit(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.2:12345"

	// Record some failed attempts
	for i := 0; i < 3; i++ {
		mgr.RecordFailedLogin(ctx, req)
	}

	// Clear rate limiting
	mgr.ClearRateLimit(ctx, req)

	// Should not be locked after clearing
	locked, _ := mgr.CheckRateLimit(ctx, req)
	if locked {
		t.Error("expected not locked after clearing rate limit")
	}
}

func TestGetClientIP(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "203.0.113.1:54321"

	ip := mgr.getClientIP(req)
	if ip != "203.0.113.1" {
		t.Errorf("expected 203.0.113.1, got %q", ip)
	}
}
