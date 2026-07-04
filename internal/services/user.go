package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// UserService handles user management
type UserService struct {
	db *database.DB
}

// NewUserService creates a new user service
func NewUserService(db *database.DB) *UserService {
	return &UserService{db: db}
}

// CreateUser creates a new user with a temporary random password.
// Returns the user and the plaintext temporary password.
func (s *UserService) CreateUser(ctx context.Context, email, displayName, role string, createdBy primitive.ObjectID) (*models.User, string, error) {
	if email == "" {
		return nil, "", fmt.Errorf("email is required")
	}
	if role != models.RoleAdmin && role != models.RoleEditor &&
		role != models.RoleContributor && role != models.RoleViewer {
		return nil, "", fmt.Errorf("invalid role: %s", role)
	}

	// Check for duplicate email
	count, err := s.db.Count(ctx, "users", bson.M{"email": email})
	if err != nil {
		return nil, "", fmt.Errorf("checking email uniqueness: %w", err)
	}
	if count > 0 {
		return nil, "", fmt.Errorf("a user with email %q already exists", email)
	}

	// Generate temporary password
	tempPassword, err := generateTempPassword()
	if err != nil {
		return nil, "", fmt.Errorf("generating temp password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now()
	user := &models.User{
		Email:             email,
		DisplayName:       displayName,
		PasswordHash:      string(hash),
		Role:              role,
		IsDefaultPassword: true,
		Disabled:          false,
		CreatedBy:         createdBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	id, err := s.db.InsertOne(ctx, "users", user)
	if err != nil {
		return nil, "", fmt.Errorf("creating user: %w", err)
	}
	user.ID = id

	return user, tempPassword, nil
}

// CreateUserWithHash creates a user with an existing password hash (for migration)
func (s *UserService) CreateUserWithHash(ctx context.Context, email, displayName, role, passwordHash string, isDefault bool) (*models.User, error) {
	now := time.Now()
	user := &models.User{
		Email:             email,
		DisplayName:       displayName,
		PasswordHash:      passwordHash,
		Role:              role,
		IsDefaultPassword: isDefault,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	id, err := s.db.InsertOne(ctx, "users", user)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	user.ID = id
	return user, nil
}

// GetByID returns a user by ID
func (s *UserService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := s.db.FindOne(ctx, "users", bson.M{"_id": id}, &user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return &user, nil
}

// GetByEmail returns a user by email
func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := s.db.FindOne(ctx, "users", bson.M{"email": email}, &user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return &user, nil
}

// ListUsers returns all users sorted by email
func (s *UserService) ListUsers(ctx context.Context) ([]models.User, error) {
	cursor, err := s.db.FindMany(ctx, "users", bson.M{},
		options.Find().SetSort(bson.D{{Key: "email", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("decoding users: %w", err)
	}
	return users, nil
}

// UpdateUser updates a user's display name and role
func (s *UserService) UpdateUser(ctx context.Context, id primitive.ObjectID, displayName, role string) error {
	if role != models.RoleAdmin && role != models.RoleEditor &&
		role != models.RoleContributor && role != models.RoleViewer {
		return fmt.Errorf("invalid role: %s", role)
	}
	return s.db.UpdateOne(ctx, "users", bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"display_name": displayName,
			"role":         role,
			"updated_at":   time.Now(),
		},
	})
}

// DisableUser disables or enables a user
func (s *UserService) DisableUser(ctx context.Context, id primitive.ObjectID, disabled bool) error {
	return s.db.UpdateOne(ctx, "users", bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"disabled":   disabled,
			"updated_at": time.Now(),
		},
	})
}

// ChangePassword validates the current password and sets a new one
func (s *UserService) ChangePassword(ctx context.Context, id primitive.ObjectID, currentPassword, newPassword string) error {
	user, err := s.GetByID(ctx, id)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.db.UpdateOne(ctx, "users", bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"password_hash":       string(hash),
			"is_default_password": false,
			"updated_at":          time.Now(),
		},
	})
}

// ResetPassword generates a new temporary password for a user
func (s *UserService) ResetPassword(ctx context.Context, id primitive.ObjectID) (string, error) {
	tempPassword, err := generateTempPassword()
	if err != nil {
		return "", fmt.Errorf("generating temp password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}

	err = s.db.UpdateOne(ctx, "users", bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"password_hash":       string(hash),
			"is_default_password": true,
			"updated_at":          time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("updating password: %w", err)
	}
	return tempPassword, nil
}

// ValidateCredentials checks email/password and returns the user if valid
func (s *UserService) ValidateCredentials(ctx context.Context, email, password string) (*models.User, error) {
	user, err := s.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil // no such user
	}
	if user.Disabled {
		return nil, fmt.Errorf("account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil // wrong password
	}

	// Update last login
	now := time.Now()
	_ = s.db.UpdateOne(ctx, "users", bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"last_login_at": now},
	})
	user.LastLoginAt = &now

	return user, nil
}

// UserCount returns the total number of users
func (s *UserService) UserCount(ctx context.Context) (int64, error) {
	return s.db.Count(ctx, "users", bson.M{})
}

// generateTempPassword creates a random 12-character hex string
func generateTempPassword() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "Tmp" + hex.EncodeToString(b) + "1", nil // Ensures upper + lower + digit to pass strength checks
}
