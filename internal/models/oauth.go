package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OAuthClient represents a dynamically registered OAuth client (RFC 7591)
type OAuthClient struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClientID         string             `bson:"client_id" json:"client_id"`
	ClientSecretHash string             `bson:"client_secret_hash" json:"-"`
	ClientName       string             `bson:"client_name" json:"client_name"`
	RedirectURIs     []string           `bson:"redirect_uris" json:"redirect_uris"`
	GrantTypes       []string           `bson:"grant_types" json:"grant_types"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
}

// OAuthAuthCode represents a pending authorization code
type OAuthAuthCode struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	CodeHash            string             `bson:"code" json:"-"`
	ClientID            string             `bson:"client_id"`
	RedirectURI         string             `bson:"redirect_uri"`
	CodeChallenge       string             `bson:"code_challenge"`
	CodeChallengeMethod string             `bson:"code_challenge_method"`
	Resource            string             `bson:"resource"`
	ExpiresAt           time.Time          `bson:"expires_at"`
	Used                bool               `bson:"used"`
	CreatedAt           time.Time          `bson:"created_at"`
}

// OAuthAccessToken represents an issued access token
type OAuthAccessToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	TokenHash string             `bson:"token_hash"`
	ClientID  string             `bson:"client_id"`
	Resource  string             `bson:"resource"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}

// OAuthRefreshToken represents an issued refresh token
type OAuthRefreshToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	TokenHash string             `bson:"token_hash"`
	ClientID  string             `bson:"client_id"`
	Resource  string             `bson:"resource"`
	ExpiresAt time.Time          `bson:"expires_at"`
	Revoked   bool               `bson:"revoked"`
	CreatedAt time.Time          `bson:"created_at"`
}
