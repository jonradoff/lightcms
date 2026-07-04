package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/middleware"
	"github.com/jonradoff/lightcms/v7/internal/services"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

func newTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long-enough"))
	userService := services.NewUserService(db)
	mgr := NewManager(store, db, userService)
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

func TestMigrateToMultiUser(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Migration should create a default admin user
	if err := mgr.MigrateToMultiUser(ctx); err != nil {
		t.Fatalf("MigrateToMultiUser failed: %v", err)
	}

	// Validate credentials with default password
	user, err := mgr.ValidateCredentials(ctx, "admin@localhost", "admin123")
	if err != nil {
		t.Fatalf("ValidateCredentials error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be returned for default credentials")
	}
	if user.Role != "admin" {
		t.Errorf("expected admin role, got %s", user.Role)
	}

	// Second call should be idempotent
	if err := mgr.MigrateToMultiUser(ctx); err != nil {
		t.Fatalf("second MigrateToMultiUser should be idempotent: %v", err)
	}
}

func TestValidateCredentials(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.MigrateToMultiUser(ctx)

	// Wrong email
	user, _ := mgr.ValidateCredentials(ctx, "nobody@example.com", "admin123")
	if user != nil {
		t.Error("expected nil user for wrong email")
	}

	// Wrong password
	user, _ = mgr.ValidateCredentials(ctx, "admin@localhost", "wrongpassword")
	if user != nil {
		t.Error("expected nil user for wrong password")
	}

	// Correct credentials
	user, err := mgr.ValidateCredentials(ctx, "admin@localhost", "admin123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user for correct credentials")
	}
}

func TestChangePassword_MultiUser(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	userSvc := services.NewUserService(db)
	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long-enough"))
	mgr := NewManager(store, db, userSvc)

	// Create user directly with bcrypt cost 4 to avoid slow hashing in tests.
	hash, err := bcrypt.GenerateFromPassword([]byte("OldPass1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	user, err := userSvc.CreateUserWithHash(ctx, "changepw@example.com", "Test User", "admin", string(hash), false)
	if err != nil {
		t.Fatalf("CreateUserWithHash: %v", err)
	}

	// Change password
	if err := mgr.ChangePassword(ctx, user.ID, "OldPass1", "NewSecure1"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Old password should not work
	u2, _ := mgr.ValidateCredentials(ctx, "changepw@example.com", "OldPass1")
	if u2 != nil {
		t.Error("old password should be invalid after change")
	}

	// New password should work
	u3, _ := mgr.ValidateCredentials(ctx, "changepw@example.com", "NewSecure1")
	if u3 == nil {
		t.Error("new password should be valid")
	}
}

func TestLoginLogout(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.MigrateToMultiUser(ctx)

	user, _ := mgr.ValidateCredentials(ctx, "admin@localhost", "admin123")
	if user == nil {
		t.Fatal("expected user")
	}

	// Create a request and response recorder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Initially not authenticated
	if mgr.IsAuthenticated(req) {
		t.Error("expected not authenticated initially")
	}

	// Login
	if err := mgr.LoginUser(rr, req, user); err != nil {
		t.Fatalf("LoginUser failed: %v", err)
	}

	// Simulate the response being sent with Set-Cookie header
	resp := rr.Result()
	cookies := resp.Cookies()

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}

	if !mgr.IsAuthenticated(req2) {
		t.Error("expected authenticated after login")
	}

	// Check user info in session
	su, ok := mgr.GetCurrentUser(req2)
	if !ok || su == nil {
		t.Fatal("expected current user in session")
	}
	if su.Email != "admin@localhost" {
		t.Errorf("expected admin@localhost, got %s", su.Email)
	}
	if su.Role != "admin" {
		t.Errorf("expected admin role, got %s", su.Role)
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
	userService := services.NewUserService(db)
	proxyConfig := &middleware.TrustedProxyConfig{TrustAllProxies: true}
	mgr := NewManagerWithProxyConfig(store, db, userService, proxyConfig)

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

	for i := 0; i < 5; i++ {
		mgr.RecordFailedLogin(ctx, req)
	}

	mgr.CheckRateLimit(ctx, req)
}

func TestClearRateLimit(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.2:12345"

	for i := 0; i < 3; i++ {
		mgr.RecordFailedLogin(ctx, req)
	}

	mgr.ClearRateLimit(ctx, req)

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

func TestMustChangePassword(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.MigrateToMultiUser(ctx)

	// admin123 sets is_default_password=true → force_password_change=true in session
	user, _ := mgr.ValidateCredentials(ctx, "admin@localhost", "admin123")
	if user == nil {
		t.Fatal("expected user")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mgr.LoginUser(rr, req, user)

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req2.AddCookie(c)
	}

	if !mgr.MustChangePassword(req2) {
		t.Error("expected MustChangePassword=true for default-password user")
	}

	// After clearing, should be false
	rr2 := httptest.NewRecorder()
	if err := mgr.ClearForcePasswordChange(rr2, req2); err != nil {
		t.Fatalf("ClearForcePasswordChange: %v", err)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr2.Result().Cookies() {
		req3.AddCookie(c)
	}
	if mgr.MustChangePassword(req3) {
		t.Error("expected MustChangePassword=false after clearing")
	}
}

func TestMustChangePassword_NoSession(t *testing.T) {
	mgr, cleanup := newTestManager(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if mgr.MustChangePassword(req) {
		t.Error("expected false with no session")
	}
}

func TestUserFromAPIContext_Empty(t *testing.T) {
	_, ok := UserFromAPIContext(context.Background())
	if ok {
		t.Error("expected ok=false for empty context")
	}
}

func TestHasPermission(t *testing.T) {
	// Admin should have all permissions
	if !HasPermission("admin", PermContentCreate) {
		t.Error("admin should have content.create")
	}
	if !HasPermission("admin", PermUserManage) {
		t.Error("admin should have user.manage")
	}

	// Editor should have content but not template create
	if !HasPermission("editor", PermContentCreate) {
		t.Error("editor should have content.create")
	}
	if HasPermission("editor", PermTemplateCreate) {
		t.Error("editor should not have template.create")
	}

	// Viewer should only view
	if !HasPermission("viewer", PermContentView) {
		t.Error("viewer should have content.view")
	}
	if HasPermission("viewer", PermContentCreate) {
		t.Error("viewer should not have content.create")
	}

	// Unknown role
	if HasPermission("superadmin", PermContentView) {
		t.Error("unknown role should have no permissions")
	}
}
