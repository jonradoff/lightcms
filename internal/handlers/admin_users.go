package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UsersPage lists all users (admin only)
func (h *Handler) UsersPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	users, err := h.userService.ListUsers(r.Context())
	if err != nil {
		log.Printf("Failed to list users: %v", err)
	}

	h.renderAdmin(w, r, "users_list", map[string]interface{}{
		"Users": users,
	})
}

// NewUserPage shows the user creation form
func (h *Handler) NewUserPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	h.renderAdmin(w, r, "user_new", nil)
}

// CreateUser creates a new user and shows the temporary password
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(currentUser.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	role := r.FormValue("role")

	if email == "" {
		h.renderAdmin(w, r, "user_new", map[string]interface{}{
			"Error": "Email is required",
		})
		return
	}

	createdByID, _ := primitive.ObjectIDFromHex(currentUser.ID)
	newUser, tempPassword, err := h.userService.CreateUser(r.Context(), email, displayName, role, createdByID)
	if err != nil {
		h.renderAdmin(w, r, "user_new", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	// Audit log
	if h.auditService != nil {
		h.auditService.LogAsync(models.AuditLog{
			UserID:     createdByID,
			UserEmail:  currentUser.Email,
			Action:     "user.create",
			Resource:   "user",
			ResourceID: newUser.ID.Hex(),
			Details:    map[string]interface{}{"email": email, "role": role},
		})
	}

	h.renderAdmin(w, r, "user_created", map[string]interface{}{
		"NewUser":      newUser,
		"TempPassword": tempPassword,
	})
}

// EditUserPage shows the user edit form
func (h *Handler) EditUserPage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(currentUser.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	editUser, err := h.userService.GetByID(r.Context(), id)
	if err != nil || editUser == nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	h.renderAdmin(w, r, "user_edit", map[string]interface{}{
		"EditUser": editUser,
	})
}

// UpdateUser handles user role/name updates
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(currentUser.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	role := r.FormValue("role")

	oldUser, _ := h.userService.GetByID(r.Context(), id)

	if err := h.userService.UpdateUser(r.Context(), id, displayName, role); err != nil {
		editUser, _ := h.userService.GetByID(r.Context(), id)
		h.renderAdmin(w, r, "user_edit", map[string]interface{}{
			"EditUser": editUser,
			"Error":    err.Error(),
		})
		return
	}

	// Audit log
	if h.auditService != nil && oldUser != nil {
		details := map[string]interface{}{"display_name": displayName}
		if oldUser.Role != role {
			details["old_role"] = oldUser.Role
			details["new_role"] = role
		}
		h.auditService.LogAsync(models.AuditLog{
			UserID:     primitive.ObjectID{}, // will be set below
			UserEmail:  currentUser.Email,
			Action:     "user.update",
			Resource:   "user",
			ResourceID: id.Hex(),
			Details:    details,
		})
	}

	http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
}

// ToggleUserDisabled enables or disables a user account
func (h *Handler) ToggleUserDisabled(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(currentUser.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	// Don't let admins disable themselves
	if mux.Vars(r)["id"] == currentUser.ID {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	targetUser, _ := h.userService.GetByID(r.Context(), id)
	if targetUser == nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	newDisabled := !targetUser.Disabled
	if err := h.userService.DisableUser(r.Context(), id, newDisabled); err != nil {
		log.Printf("Failed to toggle user disabled: %v", err)
	}

	if h.auditService != nil {
		action := "user.enable"
		if newDisabled {
			action = "user.disable"
		}
		h.auditService.LogAsync(models.AuditLog{
			UserEmail:  currentUser.Email,
			Action:     action,
			Resource:   "user",
			ResourceID: id.Hex(),
			Details:    map[string]interface{}{"email": targetUser.Email},
		})
	}

	http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
}

// ResetUserPassword generates a new temporary password for a user
func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(currentUser.Role, auth.PermUserManage) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	targetUser, _ := h.userService.GetByID(r.Context(), id)
	if targetUser == nil {
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	tempPassword, err := h.userService.ResetPassword(r.Context(), id)
	if err != nil {
		log.Printf("Failed to reset password: %v", err)
		http.Redirect(w, r, "/cm/users", http.StatusSeeOther)
		return
	}

	if h.auditService != nil {
		h.auditService.LogAsync(models.AuditLog{
			UserEmail:  currentUser.Email,
			Action:     "user.password_reset",
			Resource:   "user",
			ResourceID: id.Hex(),
			Details:    map[string]interface{}{"email": targetUser.Email},
		})
	}

	h.renderAdmin(w, r, "user_password_reset", map[string]interface{}{
		"TargetUser":   targetUser,
		"TempPassword": tempPassword,
	})
}
