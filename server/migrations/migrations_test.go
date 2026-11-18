package migrations_test

import (
	"io/fs"
	"strings"
	"testing"

	"riralicense/migrations"
)

func TestRBACMigrationsEmbeddedAndCorrective(t *testing.T) {
	m11, err := fs.ReadFile(migrations.FS, "000011_add_rbac_to_users.up.sql")
	if err != nil {
		t.Fatalf("read 000011 up migration: %v", err)
	}
	s11 := string(m11)
	if !strings.Contains(s11, "CASE WHEN u.id = e.id THEN 'owner' ELSE 'member' END") {
		t.Fatalf("000011 must assign owner only to earliest user and member to all others")
	}
	if !strings.Contains(s11, "is_superadmin = (u.id = e.id)") {
		t.Fatalf("000011 must assign is_superadmin only to earliest user")
	}

	m12, err := fs.ReadFile(migrations.FS, "000012_fix_single_owner_rbac.up.sql")
	if err != nil {
		t.Fatalf("read 000012 up migration: %v", err)
	}
	s12 := string(m12)
	if !strings.Contains(s12, "CASE WHEN u.id = e.id THEN 'owner' ELSE 'member' END") {
		t.Fatalf("000012 must keep owner only for earliest user and set member for others")
	}
	if !strings.Contains(s12, "'role_corrected'") || !strings.Contains(s12, "INSERT INTO auth_audit_log") {
		t.Fatalf("000012 must log 'role_corrected' into auth_audit_log for changed users")
	}

	if _, err := fs.ReadFile(migrations.FS, "000012_fix_single_owner_rbac.down.sql"); err != nil {
		t.Fatalf("read 000012 down migration: %v", err)
	}

	m13, err := fs.ReadFile(migrations.FS, "000013_create_workspaces.up.sql")
	if err != nil {
		t.Fatalf("read 000013 up migration: %v", err)
	}
	s13 := string(m13)
	if !strings.Contains(s13, "CREATE TABLE IF NOT EXISTS workspaces") || !strings.Contains(s13, "CREATE TABLE IF NOT EXISTS workspace_members") {
		t.Fatalf("000013 must create workspaces and workspace_members tables")
	}
	if _, err := fs.ReadFile(migrations.FS, "000013_create_workspaces.down.sql"); err != nil {
		t.Fatalf("read 000013 down migration: %v", err)
	}

	m14, err := fs.ReadFile(migrations.FS, "000014_exclude_superadmin_from_workspace_members.up.sql")
	if err != nil {
		t.Fatalf("read 000014 up migration: %v", err)
	}
	if !strings.Contains(string(m14), "is_superadmin = TRUE") {
		t.Fatalf("000014 must remove superadmins from workspace_members")
	}

	m16, err := fs.ReadFile(migrations.FS, "000016_workspace_membership_governance.up.sql")
	if err != nil {
		t.Fatalf("read 000016 up migration: %v", err)
	}
	s16 := string(m16)
	for _, token := range []string{"'removed'", "removed_at", "removed_by", "removal_reason"} {
		if !strings.Contains(s16, token) {
			t.Fatalf("000016 up migration must contain %q", token)
		}
	}
	if _, err := fs.ReadFile(migrations.FS, "000016_workspace_membership_governance.down.sql"); err != nil {
		t.Fatalf("read 000016 down migration: %v", err)
	}
}
