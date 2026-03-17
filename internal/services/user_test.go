package services

import (
	"context"
	"testing"

	"lightcms/internal/models"
	"lightcms/internal/testutil"

	"golang.org/x/crypto/bcrypt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestUserService_CreateUser(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()
	createdBy := primitive.NewObjectID()

	user, tempPw, err := svc.CreateUser(ctx, "alice@example.com", "Alice", models.RoleEditor, createdBy)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("unexpected email: %q", user.Email)
	}
	if user.Role != models.RoleEditor {
		t.Errorf("unexpected role: %q", user.Role)
	}
	if !user.IsDefaultPassword {
		t.Error("expected IsDefaultPassword=true")
	}
	if len(tempPw) == 0 {
		t.Error("expected non-empty temp password")
	}
}

func TestUserService_CreateUser_DuplicateEmail(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	svc.CreateUser(ctx, "dup@example.com", "First", models.RoleViewer, primitive.NewObjectID())
	_, _, err := svc.CreateUser(ctx, "dup@example.com", "Second", models.RoleViewer, primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestUserService_CreateUser_InvalidRole(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	_, _, err := svc.CreateUser(ctx, "x@x.com", "X", "superadmin", primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestUserService_CreateUser_EmptyEmail(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	_, _, err := svc.CreateUser(ctx, "", "Nobody", models.RoleViewer, primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for empty email")
	}
}

func TestUserService_GetByID(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	user, _, _ := svc.CreateUser(ctx, "getbyid@example.com", "Test", models.RoleAdmin, primitive.NewObjectID())

	got, err := svc.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Email != user.Email {
		t.Errorf("expected %q, got %q", user.Email, got.Email)
	}

	// Non-existent ID
	missing, err := svc.GetByID(ctx, primitive.NewObjectID())
	if err != nil {
		t.Fatalf("GetByID missing: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing user")
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	svc.CreateUser(ctx, "byemail@example.com", "By Email", models.RoleViewer, primitive.NewObjectID())

	got, err := svc.GetByEmail(ctx, "byemail@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Email != "byemail@example.com" {
		t.Errorf("unexpected email: %q", got.Email)
	}

	missing, err := svc.GetByEmail(ctx, "nobody@example.com")
	if err != nil {
		t.Fatalf("GetByEmail missing: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing user")
	}
}

func TestUserService_ListUsers(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers empty: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0, got %d", len(users))
	}

	svc.CreateUser(ctx, "zeta@example.com", "Zeta", models.RoleViewer, primitive.NewObjectID())
	svc.CreateUser(ctx, "alpha@example.com", "Alpha", models.RoleEditor, primitive.NewObjectID())

	users, err = svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	if users[0].Email != "alpha@example.com" {
		t.Errorf("expected sorted by email; got %q first", users[0].Email)
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	user, _, _ := svc.CreateUser(ctx, "update@example.com", "Old Name", models.RoleViewer, primitive.NewObjectID())

	if err := svc.UpdateUser(ctx, user.ID, "New Name", models.RoleEditor); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got, _ := svc.GetByID(ctx, user.ID)
	if got.DisplayName != "New Name" {
		t.Errorf("expected 'New Name', got %q", got.DisplayName)
	}
	if got.Role != models.RoleEditor {
		t.Errorf("expected editor role, got %q", got.Role)
	}
}

func TestUserService_UpdateUser_InvalidRole(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	user, _, _ := svc.CreateUser(ctx, "badrole@example.com", "X", models.RoleViewer, primitive.NewObjectID())
	err := svc.UpdateUser(ctx, user.ID, "X", "overlord")
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestUserService_DisableUser(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	user, _, _ := svc.CreateUser(ctx, "disable@example.com", "Disable Me", models.RoleViewer, primitive.NewObjectID())

	if err := svc.DisableUser(ctx, user.ID, true); err != nil {
		t.Fatalf("DisableUser: %v", err)
	}

	got, _ := svc.GetByID(ctx, user.ID)
	if !got.Disabled {
		t.Error("expected user to be disabled")
	}

	if err := svc.DisableUser(ctx, user.ID, false); err != nil {
		t.Fatalf("DisableUser re-enable: %v", err)
	}
	got, _ = svc.GetByID(ctx, user.ID)
	if got.Disabled {
		t.Error("expected user to be re-enabled")
	}
}

func TestUserService_UserCount(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	count, err := svc.UserCount(ctx)
	if err != nil {
		t.Fatalf("UserCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	svc.CreateUser(ctx, "c1@example.com", "One", models.RoleViewer, primitive.NewObjectID())
	svc.CreateUser(ctx, "c2@example.com", "Two", models.RoleViewer, primitive.NewObjectID())

	count, _ = svc.UserCount(ctx)
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestUserService_ValidateCredentials(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	// Use cost 4 to keep this test fast
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pw"), 4)
	svc.CreateUserWithHash(ctx, "validate@example.com", "Validate", models.RoleAdmin, string(hash), false)

	got, err := svc.ValidateCredentials(ctx, "validate@example.com", "correct-pw")
	if err != nil {
		t.Fatalf("ValidateCredentials: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.LastLoginAt == nil {
		t.Error("expected LastLoginAt to be set")
	}

	// Wrong password
	got, err = svc.ValidateCredentials(ctx, "validate@example.com", "wrong-pw")
	if err != nil {
		t.Fatalf("ValidateCredentials wrong pw: %v", err)
	}
	if got != nil {
		t.Error("expected nil for wrong password")
	}

	// Non-existent user
	got, err = svc.ValidateCredentials(ctx, "ghost@example.com", "any")
	if err != nil {
		t.Fatalf("ValidateCredentials missing user: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing user")
	}
}

func TestUserService_ValidateCredentials_DisabledUser(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), 4)
	user, _ := svc.CreateUserWithHash(ctx, "disabled@example.com", "D", models.RoleViewer, string(hash), false)
	svc.DisableUser(ctx, user.ID, true)

	_, err := svc.ValidateCredentials(ctx, "disabled@example.com", "pw")
	if err == nil {
		t.Error("expected error for disabled user")
	}
}

func TestUserService_ResetPassword(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewUserService(db)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("original"), 4)
	user, _ := svc.CreateUserWithHash(ctx, "reset@example.com", "Reset", models.RoleViewer, string(hash), false)

	newPw, err := svc.ResetPassword(ctx, user.ID)
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if len(newPw) == 0 {
		t.Error("expected non-empty new password")
	}

	got, _ := svc.GetByID(ctx, user.ID)
	if !got.IsDefaultPassword {
		t.Error("expected IsDefaultPassword=true after reset")
	}
}
