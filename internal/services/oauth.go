package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	AccessTokenTTL  = 1 * time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour
	AuthCodeTTL     = 10 * time.Minute
)

// OAuthService handles OAuth 2.1 authorization
type OAuthService struct {
	db *database.DB
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(db *database.DB) *OAuthService {
	return &OAuthService{db: db}
}

// RegisterClient creates a new OAuth client via Dynamic Client Registration (RFC 7591)
func (s *OAuthService) RegisterClient(ctx context.Context, clientName string, redirectURIs []string) (*models.OAuthClient, string, error) {
	if clientName == "" {
		return nil, "", fmt.Errorf("client_name is required")
	}
	if len(redirectURIs) == 0 {
		return nil, "", fmt.Errorf("redirect_uris is required")
	}

	clientID, err := generateToken(16)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate client_id: %w", err)
	}

	rawSecret, err := generateToken(24)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate client_secret: %w", err)
	}

	now := time.Now()
	client := &models.OAuthClient{
		ClientID:         clientID,
		ClientSecretHash: hashToken(rawSecret),
		ClientName:       clientName,
		RedirectURIs:     redirectURIs,
		GrantTypes:       []string{"authorization_code", "refresh_token"},
		CreatedAt:        now,
	}

	id, err := s.db.InsertOne(ctx, "oauth_clients", client)
	if err != nil {
		return nil, "", fmt.Errorf("failed to register client: %w", err)
	}
	client.ID = id

	return client, rawSecret, nil
}

// ValidateClient looks up a client by ID and validates the redirect URI
func (s *OAuthService) ValidateClient(ctx context.Context, clientID, redirectURI string) (*models.OAuthClient, error) {
	var client models.OAuthClient
	err := s.db.FindOne(ctx, "oauth_clients", bson.M{"client_id": clientID}, &client)
	if err != nil {
		return nil, fmt.Errorf("unknown client_id")
	}

	if redirectURI != "" {
		found := false
		for _, uri := range client.RedirectURIs {
			if uri == redirectURI {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("redirect_uri not registered")
		}
	}

	return &client, nil
}

// ValidateClientCredentials validates client_id and client_secret
func (s *OAuthService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (*models.OAuthClient, error) {
	var client models.OAuthClient
	err := s.db.FindOne(ctx, "oauth_clients", bson.M{"client_id": clientID}, &client)
	if err != nil {
		return nil, fmt.Errorf("unknown client_id")
	}

	if client.ClientSecretHash != hashToken(clientSecret) {
		return nil, fmt.Errorf("invalid client_secret")
	}

	return &client, nil
}

// CreateAuthCode generates an authorization code for the given client
func (s *OAuthService) CreateAuthCode(ctx context.Context, clientID, redirectURI, codeChallenge, codeChallengeMethod, resource string) (string, error) {
	rawCode, err := generateToken(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate auth code: %w", err)
	}

	now := time.Now()
	authCode := &models.OAuthAuthCode{
		CodeHash:            hashToken(rawCode),
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Resource:            resource,
		ExpiresAt:           now.Add(AuthCodeTTL),
		Used:                false,
		CreatedAt:           now,
	}

	if _, err := s.db.InsertOne(ctx, "oauth_auth_codes", authCode); err != nil {
		return "", fmt.Errorf("failed to store auth code: %w", err)
	}

	return rawCode, nil
}

// ExchangeAuthCode validates an auth code and returns access + refresh tokens
func (s *OAuthService) ExchangeAuthCode(ctx context.Context, rawCode, clientID, redirectURI, codeVerifier string) (accessToken, refreshToken string, expiresIn int, err error) {
	codeHash := hashToken(rawCode)

	var authCode models.OAuthAuthCode
	if err := s.db.FindOne(ctx, "oauth_auth_codes", bson.M{"code": codeHash}, &authCode); err != nil {
		return "", "", 0, fmt.Errorf("invalid authorization code")
	}

	// Validate code
	if authCode.Used {
		return "", "", 0, fmt.Errorf("authorization code already used")
	}
	if time.Now().After(authCode.ExpiresAt) {
		return "", "", 0, fmt.Errorf("authorization code expired")
	}
	if authCode.ClientID != clientID {
		return "", "", 0, fmt.Errorf("client_id mismatch")
	}
	if authCode.RedirectURI != redirectURI {
		return "", "", 0, fmt.Errorf("redirect_uri mismatch")
	}

	// Verify PKCE
	if !verifyPKCE(codeVerifier, authCode.CodeChallenge, authCode.CodeChallengeMethod) {
		return "", "", 0, fmt.Errorf("PKCE verification failed")
	}

	// Mark code as used
	_ = s.db.UpdateOne(ctx, "oauth_auth_codes", bson.M{"_id": authCode.ID}, bson.M{"$set": bson.M{"used": true}})

	// Generate tokens
	accessToken, refreshToken, expiresIn, err = s.generateTokenPair(ctx, clientID, authCode.Resource)
	if err != nil {
		return "", "", 0, err
	}

	return accessToken, refreshToken, expiresIn, nil
}

// RefreshAccessToken validates a refresh token and issues new tokens (with rotation)
func (s *OAuthService) RefreshAccessToken(ctx context.Context, rawRefreshToken, clientID string) (newAccessToken, newRefreshToken string, expiresIn int, err error) {
	tokenHash := hashToken(rawRefreshToken)

	var refreshToken models.OAuthRefreshToken
	if err := s.db.FindOne(ctx, "oauth_refresh_tokens", bson.M{"token_hash": tokenHash}, &refreshToken); err != nil {
		return "", "", 0, fmt.Errorf("invalid refresh token")
	}

	if refreshToken.Revoked {
		return "", "", 0, fmt.Errorf("refresh token revoked")
	}
	if time.Now().After(refreshToken.ExpiresAt) {
		return "", "", 0, fmt.Errorf("refresh token expired")
	}
	if refreshToken.ClientID != clientID {
		return "", "", 0, fmt.Errorf("client_id mismatch")
	}

	// Revoke old refresh token (rotation)
	_ = s.db.UpdateOne(ctx, "oauth_refresh_tokens",
		bson.M{"_id": refreshToken.ID},
		bson.M{"$set": bson.M{"revoked": true}})

	// Generate new token pair
	return s.generateTokenPair(ctx, clientID, refreshToken.Resource)
}

// ValidateAccessToken checks a raw access token and returns the token record
func (s *OAuthService) ValidateAccessToken(ctx context.Context, rawToken string) (*models.OAuthAccessToken, error) {
	tokenHash := hashToken(rawToken)

	var token models.OAuthAccessToken
	if err := s.db.FindOne(ctx, "oauth_access_tokens", bson.M{"token_hash": tokenHash}, &token); err != nil {
		return nil, fmt.Errorf("invalid access token")
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("access token expired")
	}

	return &token, nil
}

// generateTokenPair creates and stores a new access token + refresh token
func (s *OAuthService) generateTokenPair(ctx context.Context, clientID, resource string) (string, string, int, error) {
	rawAccess, err := generateToken(48)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate access token: %w", err)
	}

	rawRefresh, err := generateToken(64)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	now := time.Now()

	accessToken := &models.OAuthAccessToken{
		TokenHash: hashToken(rawAccess),
		ClientID:  clientID,
		Resource:  resource,
		ExpiresAt: now.Add(AccessTokenTTL),
		CreatedAt: now,
	}

	refreshToken := &models.OAuthRefreshToken{
		TokenHash: hashToken(rawRefresh),
		ClientID:  clientID,
		Resource:  resource,
		ExpiresAt: now.Add(RefreshTokenTTL),
		Revoked:   false,
		CreatedAt: now,
	}

	if _, err := s.db.InsertOne(ctx, "oauth_access_tokens", accessToken); err != nil {
		return "", "", 0, fmt.Errorf("failed to store access token: %w", err)
	}

	if _, err := s.db.InsertOne(ctx, "oauth_refresh_tokens", refreshToken); err != nil {
		// Clean up the access token we just created
		_ = s.db.DeleteOne(ctx, "oauth_access_tokens", bson.M{"_id": accessToken.ID})
		return "", "", 0, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return rawAccess, rawRefresh, int(AccessTokenTTL.Seconds()), nil
}

// DeleteClientByID removes a client (used in cleanup/admin)
func (s *OAuthService) DeleteClientByID(ctx context.Context, id primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "oauth_clients", bson.M{"_id": id})
}

// hashToken returns the SHA-256 hex digest of a raw token
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// generateToken returns a base64url-encoded random token of nBytes length
func generateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// verifyPKCE verifies the PKCE code_verifier against the stored code_challenge
func verifyPKCE(codeVerifier, codeChallenge, method string) bool {
	if method != "S256" {
		return false
	}
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == codeChallenge
}
