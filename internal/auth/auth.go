package auth

import (
	"context"
	"log"
	"net/http"
	"strings"

	"lightcms/internal/database"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionName     = "lightcms-session"
	defaultPassword = "admin123"
	bcryptCost      = 12
)

type Manager struct {
	db    *database.DB
	store *sessions.CookieStore
}

func NewManager(store *sessions.CookieStore, db *database.DB) *Manager {
	return &Manager{
		db:    db,
		store: store,
	}
}

// InitializePassword sets up the password hash on first run
// If no password exists in the database, it creates one with the default password (admin123)
func (m *Manager) InitializePassword(ctx context.Context) error {
	settings, err := m.db.GetAdminSettings(ctx)
	if err != nil {
		return err
	}

	// If no settings exist or password hash is empty, create initial password hash with default
	if settings == nil || settings.PasswordHash == "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcryptCost)
		if err != nil {
			return err
		}

		settings = &database.AdminSettings{
			PasswordHash:      string(hash),
			IsDefaultPassword: true,
		}
		return m.db.SaveAdminSettings(ctx, settings)
	}

	return nil
}

// ValidatePassword checks if the provided password is correct
func (m *Manager) ValidatePassword(ctx context.Context, password string) bool {
	settings, err := m.db.GetAdminSettings(ctx)
	if err != nil {
		log.Printf("[AUTH] Error getting admin settings: %v", err)
		return false
	}

	// If no settings exist (no password hash in DB), initialize with default
	if settings == nil || settings.PasswordHash == "" {
		// Initialize default password on first check
		if err := m.InitializePassword(ctx); err != nil {
			log.Printf("[AUTH] Error initializing password: %v", err)
			return false
		}
		// Re-fetch settings after initialization
		settings, err = m.db.GetAdminSettings(ctx)
		if err != nil || settings == nil {
			log.Printf("[AUTH] Error getting settings after init: %v", err)
			return false
		}
	}

	// Compare password with stored hash
	err = bcrypt.CompareHashAndPassword([]byte(settings.PasswordHash), []byte(password))
	return err == nil
}

// ChangePassword updates the admin password
func (m *Manager) ChangePassword(ctx context.Context, currentPassword, newPassword string) error {
	// Verify current password first
	if !m.ValidatePassword(ctx, currentPassword) {
		return ErrInvalidCurrentPassword
	}

	// Validate new password strength
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	settings := &database.AdminSettings{
		PasswordHash:      string(hash),
		IsDefaultPassword: false,
	}
	return m.db.SaveAdminSettings(ctx, settings)
}

// IsDefaultPassword checks if the admin is still using the default password
func (m *Manager) IsDefaultPassword(ctx context.Context) bool {
	settings, err := m.db.GetAdminSettings(ctx)
	if err != nil || settings == nil {
		return true // No settings means default password will be used
	}
	return settings.IsDefaultPassword
}

// CheckRateLimit checks if the IP is rate limited
func (m *Manager) CheckRateLimit(ctx context.Context, r *http.Request) (bool, string) {
	ip := getClientIP(r)
	locked, duration := m.db.IsLoginLocked(ctx, ip)
	if locked {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		if minutes > 0 {
			return true, formatDuration(minutes, seconds)
		}
		return true, formatDuration(0, seconds)
	}
	return false, ""
}

// RecordFailedLogin records a failed login attempt
func (m *Manager) RecordFailedLogin(ctx context.Context, r *http.Request) {
	ip := getClientIP(r)
	m.db.RecordFailedLogin(ctx, ip)
}

// ClearRateLimit clears rate limiting on successful login
func (m *Manager) ClearRateLimit(ctx context.Context, r *http.Request) {
	ip := getClientIP(r)
	m.db.ClearLoginAttempts(ctx, ip)
}

func (m *Manager) Login(w http.ResponseWriter, r *http.Request) error {
	log.Printf("[LOGIN] Starting Login function")
	session, err := m.store.Get(r, sessionName)
	if err != nil {
		log.Printf("[LOGIN] store.Get error: %v, creating fresh session", err)
		// Create a completely new session, ignoring any existing cookie
		session = sessions.NewSession(m.store, sessionName)
		session.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   86400 * 7,
			HttpOnly: true,
		}
		session.IsNew = true
	}
	session.Values["authenticated"] = true
	log.Printf("[LOGIN] About to save session")
	err = session.Save(r, w)
	if err != nil {
		log.Printf("[LOGIN] session.Save error: %v", err)
	} else {
		log.Printf("[LOGIN] Session saved successfully")
	}
	return err
}

func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) error {
	session, err := m.store.Get(r, sessionName)
	if err != nil {
		return err
	}
	session.Values["authenticated"] = false
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

func (m *Manager) IsAuthenticated(r *http.Request) bool {
	session, err := m.store.Get(r, sessionName)
	if err != nil {
		return false
	}
	auth, ok := session.Values["authenticated"].(bool)
	return ok && auth
}

// Middleware wraps handlers that require authentication
func (m *Manager) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.IsAuthenticated(r) {
			http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// Password validation errors
type PasswordError struct {
	Message string
}

func (e PasswordError) Error() string {
	return e.Message
}

var (
	ErrInvalidCurrentPassword = PasswordError{"Current password is incorrect"}
	ErrPasswordTooShort       = PasswordError{"Password must be at least 8 characters"}
	ErrPasswordNoUppercase    = PasswordError{"Password must contain at least one uppercase letter"}
	ErrPasswordNoLowercase    = PasswordError{"Password must contain at least one lowercase letter"}
	ErrPasswordNoNumber       = PasswordError{"Password must contain at least one number"}
)

// ValidatePasswordStrength checks password meets security requirements
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	hasUpper := false
	hasLower := false
	hasNumber := false

	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasNumber = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUppercase
	}
	if !hasLower {
		return ErrPasswordNoLowercase
	}
	if !hasNumber {
		return ErrPasswordNoNumber
	}

	return nil
}

// getClientIP extracts the client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP in the list
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}
	return ip
}

func formatDuration(minutes, seconds int) string {
	if minutes > 0 {
		return strings.TrimSpace(strings.Join([]string{
			itoa(minutes), "minute",
			pluralize(minutes),
		}, " "))
	}
	return strings.TrimSpace(strings.Join([]string{
		itoa(seconds), "second",
		pluralize(seconds),
	}, " "))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
