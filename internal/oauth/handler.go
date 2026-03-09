package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/services"
)

const loginProofMaxAge = 5 * time.Minute

// Handler implements OAuth 2.1 endpoints for MCP authorization
type Handler struct {
	oauthService  *services.OAuthService
	authManager   *auth.Manager
	baseURL       string
	sessionSecret string
	tmpl          *template.Template
}

// NewHandler creates a new OAuth handler
func NewHandler(oauthService *services.OAuthService, authManager *auth.Manager, baseURL, sessionSecret string) *Handler {
	h := &Handler{
		oauthService:  oauthService,
		authManager:   authManager,
		baseURL:       baseURL,
		sessionSecret: sessionSecret,
	}
	h.tmpl = template.Must(template.New("authorize").Parse(oauthTemplates["authorize"]))
	return h
}

// Register handles Dynamic Client Registration (RFC 7591)
// POST /oauth/register
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
		GrantTypes   []string `json:"grant_types"`
		ResponseTypes []string `json:"response_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.ClientName == "" {
		req.ClientName = "Unknown Client"
	}
	if len(req.RedirectURIs) == 0 {
		jsonError(w, http.StatusBadRequest, "invalid_request", "redirect_uris is required")
		return
	}

	client, rawSecret, err := h.oauthService.RegisterClient(r.Context(), req.ClientName, req.RedirectURIs)
	if err != nil {
		log.Printf("[OAuth] Registration failed: %v", err)
		jsonError(w, http.StatusInternalServerError, "server_error", "Failed to register client")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"client_id":                client.ClientID,
		"client_secret":            rawSecret,
		"client_id_issued_at":      client.CreatedAt.Unix(),
		"client_secret_expires_at": 0,
		"client_name":              client.ClientName,
		"redirect_uris":            client.RedirectURIs,
		"grant_types":              []string{"authorization_code", "refresh_token"},
		"response_types":           []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	})
}

// Authorize handles the authorization page
// GET /oauth/authorize
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	state := q.Get("state")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	resource := q.Get("resource")
	scope := q.Get("scope")

	// Validate required params
	if responseType != "code" {
		jsonError(w, http.StatusBadRequest, "unsupported_response_type", "Only response_type=code is supported")
		return
	}
	if codeChallenge == "" || codeChallengeMethod != "S256" {
		jsonError(w, http.StatusBadRequest, "invalid_request", "PKCE with S256 is required")
		return
	}

	// Validate client
	_, err := h.oauthService.ValidateClient(r.Context(), clientID, redirectURI)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_client", err.Error())
		return
	}

	// Render login page
	h.renderAuthorize(w, map[string]interface{}{
		"ShowLogin":           true,
		"Error":               "",
		"ClientID":            clientID,
		"RedirectURI":         redirectURI,
		"ResponseType":        responseType,
		"State":               state,
		"CodeChallenge":       codeChallenge,
		"CodeChallengeMethod": codeChallengeMethod,
		"Resource":            resource,
		"Scope":               scope,
	})
}

// AuthorizeSubmit processes the login form or consent decision
// POST /oauth/authorize
func (h *Handler) AuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_request", "Invalid form data")
		return
	}

	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	responseType := r.FormValue("response_type")
	state := r.FormValue("state")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")
	resource := r.FormValue("resource")
	scope := r.FormValue("scope")
	action := r.FormValue("action")

	// Validate client
	client, err := h.oauthService.ValidateClient(r.Context(), clientID, redirectURI)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_client", err.Error())
		return
	}

	templateData := map[string]interface{}{
		"ClientID":            clientID,
		"RedirectURI":         redirectURI,
		"ResponseType":        responseType,
		"State":               state,
		"CodeChallenge":       codeChallenge,
		"CodeChallengeMethod": codeChallengeMethod,
		"Resource":            resource,
		"Scope":               scope,
	}

	switch action {
	case "login":
		email := r.FormValue("email")
		password := r.FormValue("password")

		// Check rate limiting
		if locked, duration := h.authManager.CheckRateLimit(r.Context(), r); locked {
			templateData["ShowLogin"] = true
			templateData["Error"] = "Too many attempts. Try again in " + duration
			h.renderAuthorize(w, templateData)
			return
		}

		user, err := h.authManager.ValidateCredentials(r.Context(), email, password)
		if err != nil || user == nil {
			h.authManager.RecordFailedLogin(r.Context(), r)
			templateData["ShowLogin"] = true
			templateData["Error"] = "Invalid email or password"
			h.renderAuthorize(w, templateData)
			return
		}

		h.authManager.ClearRateLimit(r.Context(), r)

		// Password valid — show consent page with login proof
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		proof := h.computeLoginProof(clientID, redirectURI, codeChallenge, ts)

		templateData["ShowLogin"] = false
		templateData["ClientName"] = client.ClientName
		templateData["LoginProof"] = proof
		templateData["LoginTS"] = ts
		h.renderAuthorize(w, templateData)

	case "approve":
		loginProof := r.FormValue("login_proof")
		loginTS := r.FormValue("login_ts")

		if !h.verifyLoginProof(clientID, redirectURI, codeChallenge, loginTS, loginProof) {
			templateData["ShowLogin"] = true
			templateData["Error"] = "Session expired. Please sign in again."
			h.renderAuthorize(w, templateData)
			return
		}

		// Generate auth code
		code, err := h.oauthService.CreateAuthCode(r.Context(), clientID, redirectURI, codeChallenge, codeChallengeMethod, resource)
		if err != nil {
			log.Printf("[OAuth] Failed to create auth code: %v", err)
			redirectWithError(w, r, redirectURI, state, "server_error", "Failed to generate authorization code")
			return
		}

		// Redirect with code
		redirectURL, _ := url.Parse(redirectURI)
		q := redirectURL.Query()
		q.Set("code", code)
		if state != "" {
			q.Set("state", state)
		}
		redirectURL.RawQuery = q.Encode()
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)

	case "deny":
		redirectWithError(w, r, redirectURI, state, "access_denied", "User denied access")

	default:
		jsonError(w, http.StatusBadRequest, "invalid_request", "Unknown action")
	}
}

// Token handles token exchange and refresh
// POST /oauth/token
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_request", "Invalid form data")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	grantType := r.FormValue("grant_type")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	// Validate client credentials
	if _, err := h.oauthService.ValidateClientCredentials(r.Context(), clientID, clientSecret); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client",
			"error_description": "Invalid client credentials",
		})
		return
	}

	switch grantType {
	case "authorization_code":
		code := r.FormValue("code")
		redirectURI := r.FormValue("redirect_uri")
		codeVerifier := r.FormValue("code_verifier")

		accessToken, refreshToken, expiresIn, err := h.oauthService.ExchangeAuthCode(r.Context(), code, clientID, redirectURI, codeVerifier)
		if err != nil {
			log.Printf("[OAuth] Token exchange failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_grant",
				"error_description": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  accessToken,
			"token_type":    "Bearer",
			"expires_in":    expiresIn,
			"refresh_token": refreshToken,
		})

	case "refresh_token":
		refreshToken := r.FormValue("refresh_token")

		accessToken, newRefreshToken, expiresIn, err := h.oauthService.RefreshAccessToken(r.Context(), refreshToken, clientID)
		if err != nil {
			log.Printf("[OAuth] Token refresh failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_grant",
				"error_description": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  accessToken,
			"token_type":    "Bearer",
			"expires_in":    expiresIn,
			"refresh_token": newRefreshToken,
		})

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "unsupported_grant_type",
			"error_description": "Only authorization_code and refresh_token are supported",
		})
	}
}

// computeLoginProof creates an HMAC proving the admin authenticated
func (h *Handler) computeLoginProof(clientID, redirectURI, codeChallenge, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(h.sessionSecret))
	mac.Write([]byte(clientID + "|" + redirectURI + "|" + codeChallenge + "|" + timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyLoginProof checks the HMAC and ensures it's not expired
func (h *Handler) verifyLoginProof(clientID, redirectURI, codeChallenge, timestamp, proof string) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > loginProofMaxAge {
		return false
	}
	expected := h.computeLoginProof(clientID, redirectURI, codeChallenge, timestamp)
	return hmac.Equal([]byte(expected), []byte(proof))
}

func (h *Handler) renderAuthorize(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, data); err != nil {
		log.Printf("[OAuth] Template render error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func redirectWithError(w http.ResponseWriter, r *http.Request, redirectURI, state, errCode, errDesc string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_request", "Invalid redirect_uri")
		return
	}
	q := u.Query()
	q.Set("error", errCode)
	q.Set("error_description", errDesc)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func jsonError(w http.ResponseWriter, status int, errCode, errDesc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": errDesc,
	})
}

// ProtectedResourceMetadata returns the RFC 9728 metadata handler
func ProtectedResourceMetadataHandler(baseURL string) http.HandlerFunc {
	metadata, _ := json.Marshal(map[string]interface{}{
		"resource":                 baseURL + "/mcp",
		"authorization_servers":    []string{baseURL},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "LightCMS MCP Server",
	})
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Write(metadata)
	}
}

// AuthorizationServerMetadataHandler returns the RFC 8414 metadata handler
func AuthorizationServerMetadataHandler(baseURL string) http.HandlerFunc {
	metadata, _ := json.Marshal(map[string]interface{}{
		"issuer":                                baseURL,
		"authorization_endpoint":                baseURL + "/oauth/authorize",
		"token_endpoint":                        baseURL + "/oauth/token",
		"registration_endpoint":                 baseURL + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{},
	})
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Write(metadata)
	}
}

// JWKSHandler returns an empty JWKS document (we use opaque tokens, not JWTs)
func JWKSHandler() http.HandlerFunc {
	body := []byte(`{"keys":[]}`)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(body)
	}
}

// TokenRevocationHandler handles token revocation (RFC 7009)
// POST /oauth/revoke
func (h *Handler) TokenRevocation(w http.ResponseWriter, r *http.Request) {
	// Per RFC 7009, always return 200 even if token is invalid
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "{}")
}
