package auth

import "testing"

func TestUserHasPermission(t *testing.T) {
	tests := []struct {
		name string
		user *SessionUser
		perm string
		want bool
	}{
		{"nil user", nil, PermContentView, false},
		{"admin full", &SessionUser{Role: "admin"}, PermUserManage, true},
		{"viewer denied edit", &SessionUser{Role: "viewer"}, PermContentEdit, false},
		{"scoped key allows listed perm", &SessionUser{Role: "admin", Scopes: []string{PermContentView, PermContentEdit}}, PermContentEdit, true},
		{"scoped key denies unlisted perm", &SessionUser{Role: "admin", Scopes: []string{PermContentView}}, PermContentDelete, false},
		{"scope cannot exceed role", &SessionUser{Role: "viewer", Scopes: []string{PermContentEdit}}, PermContentEdit, false},
		{"sandbox-only allows content edit", &SessionUser{Role: "admin", SandboxOnly: true}, PermContentEdit, true},
		{"sandbox-only allows fork create", &SessionUser{Role: "editor", SandboxOnly: true}, PermForkCreate, true},
		{"sandbox-only denies publish", &SessionUser{Role: "admin", SandboxOnly: true}, PermContentPublish, false},
		{"sandbox-only denies delete", &SessionUser{Role: "admin", SandboxOnly: true}, PermContentDelete, false},
		{"sandbox-only denies search replace", &SessionUser{Role: "admin", SandboxOnly: true}, PermSearchReplace, false},
		{"sandbox-only denies settings edit", &SessionUser{Role: "admin", SandboxOnly: true}, PermSettingsEdit, false},
		{"sandbox-only denies fork merge", &SessionUser{Role: "admin", SandboxOnly: true}, PermForkMerge, false},
		{"sandbox + scopes combine", &SessionUser{Role: "admin", SandboxOnly: true, Scopes: []string{PermContentView}}, PermContentEdit, false},
	}
	for _, tc := range tests {
		if got := UserHasPermission(tc.user, tc.perm); got != tc.want {
			t.Errorf("%s: UserHasPermission = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsKnownPermission(t *testing.T) {
	if !IsKnownPermission(PermContentEdit) {
		t.Error("content.edit should be known")
	}
	if !IsKnownPermission(PermUserManage) {
		t.Error("user.manage should be known")
	}
	if IsKnownPermission("bogus.permission") {
		t.Error("bogus.permission should not be known")
	}
	if IsKnownPermission("") {
		t.Error("empty string should not be known")
	}
}
