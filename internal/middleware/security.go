package middleware

import (
	"net"
	"net/http"
	"strings"
)

// SecurityHeaders adds security headers to all responses
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers for all responses
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// HSTS - only in production when using HTTPS
		// This will be applied when SecureCookies is true
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content Security Policy
		if strings.HasPrefix(r.URL.Path, "/cm") {
			// Admin pages - need inline scripts/styles for the UI
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline'; "+
					"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
					"font-src 'self' https://fonts.gstatic.com; "+
					"img-src 'self' data: https:; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")
		} else if !strings.HasPrefix(r.URL.Path, "/static/") && !strings.HasPrefix(r.URL.Path, "/assets/") {
			// Public pages - more permissive for CMS content but still protective
			// Allows inline scripts/styles for admin-authored content (analytics, widgets, etc.)
			// Allows external resources for embeds, CDN images, etc.
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' https:; "+
					"style-src 'self' 'unsafe-inline' https:; "+
					"font-src 'self' https: data:; "+
					"img-src 'self' data: https:; "+
					"media-src 'self' https:; "+
					"frame-src https:; "+
					"connect-src 'self' https:; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self' https:")
		}

		next.ServeHTTP(w, r)
	})
}

// TrustedProxyConfig holds configuration for trusted proxy handling
type TrustedProxyConfig struct {
	// TrustedProxies is a list of trusted proxy CIDRs
	// If empty, X-Forwarded-For is not trusted
	TrustedProxies []string
	// TrustAllProxies trusts any proxy — DEPRECATED, do not use in production.
	// Use TrustFlyProxy or TrustedProxies instead.
	TrustAllProxies bool
	// TrustFlyProxy enables Fly.io-specific IP extraction. When true, the
	// Fly-Client-IP header (set by Fly.io's edge proxy and not spoofable by
	// clients) is used as the authoritative client IP. Falls back to RemoteAddr.
	TrustFlyProxy bool
}

// DefaultCloudConfig returns a config that trusts Fly.io's edge proxy.
// Uses the Fly-Client-IP header which is set by the Fly.io edge and cannot
// be spoofed by end-users, preventing rate-limit and audit-log bypass.
func DefaultCloudConfig() *TrustedProxyConfig {
	return &TrustedProxyConfig{
		TrustFlyProxy: true,
	}
}

// GetClientIP extracts the real client IP from a request
// It properly handles X-Forwarded-For based on trusted proxy configuration
func GetClientIP(r *http.Request, config *TrustedProxyConfig) string {
	if config == nil {
		return extractIP(r.RemoteAddr)
	}

	// Fly.io sets Fly-Client-IP at the edge; it cannot be spoofed by clients.
	// Prefer it over X-Forwarded-For which can be set by anyone.
	if config.TrustFlyProxy {
		if flyIP := r.Header.Get("Fly-Client-IP"); flyIP != "" && isValidIP(strings.TrimSpace(flyIP)) {
			return strings.TrimSpace(flyIP)
		}
		return extractIP(r.RemoteAddr)
	}

	// No trusted proxies configured: use direct connection IP
	if !config.TrustAllProxies && len(config.TrustedProxies) == 0 {
		return extractIP(r.RemoteAddr)
	}

	// Get the remote address (the immediate connection)
	remoteIP := extractIP(r.RemoteAddr)

	// Check if the immediate connection is from a trusted proxy
	if !config.TrustAllProxies && !isTrustedProxy(remoteIP, config.TrustedProxies) {
		return remoteIP
	}

	// Get X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remoteIP
	}

	// X-Forwarded-For is a comma-separated list: client, proxy1, proxy2, ...
	ips := strings.Split(xff, ",")

	if config.TrustAllProxies {
		// Last resort: trust the first IP from X-Forwarded-For
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" && isValidIP(clientIP) {
				return clientIP
			}
		}
	} else {
		// Find the first non-trusted IP from the right
		for i := len(ips) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(ips[i])
			if ip == "" {
				continue
			}
			if !isTrustedProxy(ip, config.TrustedProxies) {
				if isValidIP(ip) {
					return ip
				}
			}
		}
	}

	return remoteIP
}

// extractIP extracts just the IP from an address that might include a port
func extractIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port, return as-is
		return addr
	}
	return host
}

// isTrustedProxy checks if an IP is in the trusted proxy list
func isTrustedProxy(ip string, trustedProxies []string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, trusted := range trustedProxies {
		// Check if it's a CIDR
		if strings.Contains(trusted, "/") {
			_, network, err := net.ParseCIDR(trusted)
			if err != nil {
				continue
			}
			if network.Contains(parsedIP) {
				return true
			}
		} else {
			// Single IP
			if trusted == ip {
				return true
			}
		}
	}
	return false
}

// isValidIP checks if a string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// AllowedFileTypes defines the allowed file extensions and their MIME types for uploads
var AllowedFileTypes = map[string][]string{
	// Images
	".jpg":  {"image/jpeg"},
	".jpeg": {"image/jpeg"},
	".png":  {"image/png"},
	".gif":  {"image/gif"},
	".webp": {"image/webp"},
	".svg":  {"image/svg+xml"},
	".ico":  {"image/x-icon", "image/vnd.microsoft.icon"},
	".bmp":  {"image/bmp"},
	".avif": {"image/avif"},

	// Documents
	".pdf": {"application/pdf"},

	// Web assets
	".css":   {"text/css"},
	".js":    {"application/javascript", "text/javascript"},
	".json":  {"application/json"},
	".woff":  {"font/woff", "application/font-woff"},
	".woff2": {"font/woff2"},
	".ttf":   {"font/ttf", "application/x-font-ttf"},
	".eot":   {"application/vnd.ms-fontobject"},
	".otf":   {"font/otf"},

	// Data
	".xml":  {"application/xml", "text/xml"},
	".txt":  {"text/plain"},
	".csv":  {"text/csv"},
	".html": {"text/html"},
}

// IsAllowedFileType checks if a file extension is allowed
func IsAllowedFileType(ext string) bool {
	ext = strings.ToLower(ext)
	_, ok := AllowedFileTypes[ext]
	return ok
}

// ValidateMIMEType checks if the detected MIME type matches what's expected for the extension
func ValidateMIMEType(ext string, detectedMIME string) bool {
	ext = strings.ToLower(ext)
	allowedMIMEs, ok := AllowedFileTypes[ext]
	if !ok {
		return false
	}

	// Normalize detected MIME (remove charset etc.)
	detectedMIME = strings.Split(detectedMIME, ";")[0]
	detectedMIME = strings.TrimSpace(detectedMIME)

	for _, allowed := range allowedMIMEs {
		if detectedMIME == allowed {
			return true
		}
	}

	// Special case: http.DetectContentType often returns application/octet-stream
	// for files it doesn't recognize. Allow this for font files and some others.
	if detectedMIME == "application/octet-stream" {
		switch ext {
		case ".woff", ".woff2", ".ttf", ".eot", ".otf", ".ico":
			return true
		}
	}

	// Special case: SVG files might be detected as text/xml or text/plain
	if ext == ".svg" && (detectedMIME == "text/xml" || detectedMIME == "text/plain" || detectedMIME == "text/html") {
		return true
	}

	// Special case: http.DetectContentType returns text/plain for text-based
	// web assets (CSS, JS, JSON, etc.) since it can't distinguish them from
	// plain text at the byte level. The extension whitelist gates what's allowed.
	if detectedMIME == "text/plain" {
		switch ext {
		case ".css", ".js", ".json", ".xml", ".csv", ".html":
			return true
		}
	}

	return false
}

// ValidateFilePath checks if a file path is safe (no directory traversal)
func ValidateFilePath(path string) bool {
	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return false
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return false
	}

	// Check for backslashes (Windows-style paths that could be problematic)
	if strings.Contains(path, "\\") {
		return false
	}

	// Check for double slashes
	if strings.Contains(path, "//") {
		return false
	}

	return true
}
