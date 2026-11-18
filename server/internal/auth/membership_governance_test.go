package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
)

func seedGovernanceUser(t *testing.T, store *memStore, email, name, globalRole string, isSuper bool) sqlc.User {
	t.Helper()
	pwHash, err := crypto.HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	now := store.now()
	u, err := store.CreateUser(context.Background(), sqlc.CreateUserParams{
		Lower:           email,
		PasswordHash:    pgtype.Text{String: pwHash, Valid: true},
		Name:            name,
		EmailVerifiedAt: &now,
		Role:            globalRole,
		IsSuperadmin:    isSuper,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s) failed: %v", email, err)
	}
	return u
}

func seedGovernanceWorkspace(t *testing.T, store *memStore, name, slug string, createdBy uuid.UUID) sqlc.Workspace {
	t.Helper()
	ws, err := store.CreateWorkspace(context.Background(), sqlc.CreateWorkspaceParams{
		Name:      name,
		Lower:     slug,
		Plan:      ProPlan,
		CreatedBy: uuid.NullUUID{UUID: createdBy, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace(%s) failed: %v", slug, err)
	}
	return ws
}

func seedGovernanceMember(t *testing.T, store *memStore, wsID, userID uuid.UUID, role string) sqlc.WorkspaceMember {
	t.Helper()
	m, err := store.CreateWorkspaceMember(context.Background(), sqlc.CreateWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        role,
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceMember failed: %v", err)
	}
	return m
}

func TestWorkspaceMembershipGovernanceRulesAndIsolation(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	ctx := context.Background()

	super := seedGovernanceUser(t, store, "super@gov.test", "Super Admin", RoleOwner, true)
	ownerA := seedGovernanceUser(t, store, "ownera@gov.test", "Owner A", RoleOwner, false)
	admin1 := seedGovernanceUser(t, store, "admin1@gov.test", "Admin One", RoleAdmin, false)
	admin2 := seedGovernanceUser(t, store, "admin2@gov.test", "Admin Two", RoleAdmin, false)
	member1 := seedGovernanceUser(t, store, "member1@gov.test", "Member One", RoleMember, false)
	member2 := seedGovernanceUser(t, store, "member2@gov.test", "Member Two", RoleMember, false)

	// Workspace A: only admin1 (as last admin) and member1 (no owner row, testing pure admin invariant).
	wsA := seedGovernanceWorkspace(t, store, "Workspace A", "workspace-a", super.ID)
	seedGovernanceMember(t, store, wsA.ID, admin1.ID, RoleAdmin)
	seedGovernanceMember(t, store, wsA.ID, member1.ID, RoleMember)

	// Workspace B: ownerA + member1 + member2 (for cross-workspace isolation).
	wsB := seedGovernanceWorkspace(t, store, "Workspace B", "workspace-b", ownerA.ID)
	seedGovernanceMember(t, store, wsB.ID, ownerA.ID, RoleOwner)
	seedGovernanceMember(t, store, wsB.ID, member1.ID, RoleMember)
	seedGovernanceMember(t, store, wsB.ID, member2.ID, RoleMember)

	// Workspace C: only a single active member (degenerate seeded case to test BR-02 directly).
	wsC := seedGovernanceWorkspace(t, store, "Workspace C", "workspace-c", super.ID)
	seedGovernanceMember(t, store, wsC.ID, member2.ID, RoleMember)

	// 1. Removing the last active Admin is rejected (BR-01).
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, super.ID, admin1.ID, "cleanup", testMeta()); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("expected ErrLastActiveAdmin removing last admin in wsA, got %v", err)
	}
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: admin1.ID}); err != nil || m.Status != "active" {
		t.Fatalf("expected admin1 to remain active after rejected remove, got m=%+v err=%v", m, err)
	}

	// 2. Suspending the last active Admin is rejected (BR-01).
	if _, err := svc.AdminSuspendUserInWorkspace(ctx, wsA.ID, super.ID, admin1.ID, "policy violation", testMeta()); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("expected ErrLastActiveAdmin suspending last admin in wsA, got %v", err)
	}
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: admin1.ID}); err != nil || m.Status != "active" {
		t.Fatalf("expected admin1 to remain active after rejected suspend, got m=%+v err=%v", m, err)
	}

	// 3. Demoting the last active Admin to member is rejected (BR-01, BR-05).
	if _, err := svc.AdminUpdateUserRoleInWorkspace(ctx, wsA.ID, super.ID, admin1.ID, RoleMember, testMeta()); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("expected ErrLastActiveAdmin demoting last admin to member in wsA, got %v", err)
	}
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: admin1.ID}); err != nil || m.Role != RoleAdmin {
		t.Fatalf("expected admin1 role to remain admin after rejected demotion, got m=%+v err=%v", m, err)
	}

	// 4. Removing the last active Member is rejected (BR-02).
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsC.ID, super.ID, member2.ID, "cleanup", testMeta()); !errors.Is(err, ErrLastActiveMember) {
		t.Fatalf("expected ErrLastActiveMember removing last member in wsC, got %v", err)
	}
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsC.ID, UserID: member2.ID}); err != nil || m.Status != "active" {
		t.Fatalf("expected member2 to remain active in wsC after rejected remove, got m=%+v err=%v", m, err)
	}

	// 5. Suspending the last active Member is rejected (BR-02).
	if _, err := svc.AdminSuspendUserInWorkspace(ctx, wsC.ID, super.ID, member2.ID, "policy violation", testMeta()); !errors.Is(err, ErrLastActiveMember) {
		t.Fatalf("expected ErrLastActiveMember suspending last member in wsC, got %v", err)
	}

	// 6. Removing a Member while another active member and Admin remain succeeds (BR-03).
	member1Login, err := svc.Login(ctx, member1.Email, "password123", "", testMeta())
	if err != nil {
		t.Fatalf("member1 login failed: %v", err)
	}
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, super.ID, member1.ID, "offboarded", testMeta()); err != nil {
		t.Fatalf("expected removing member1 from wsA to succeed, got %v", err)
	}
	removedMem, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: member1.ID})
	if err != nil || removedMem.Status != "removed" || removedMem.RemovedAt == nil || !removedMem.RemovedBy.Valid || removedMem.RemovedBy.UUID != super.ID || !removedMem.RemovalReason.Valid || removedMem.RemovalReason.String != "offboarded" {
		t.Fatalf("unexpected soft-removed membership state: %+v err=%v", removedMem, err)
	}

	// 8 & 9. Removing member1 from Workspace A does not affect Workspace B and preserves global account (BR-03).
	memInB, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsB.ID, UserID: member1.ID})
	if err != nil || memInB.Status != "active" || memInB.Role != RoleMember {
		t.Fatalf("expected member1 membership in wsB unaffected, got %+v err=%v", memInB, err)
	}
	globalMember1, err := store.GetUserByID(ctx, member1.ID)
	if err != nil || globalMember1.Status != "active" {
		t.Fatalf("expected global user account active after workspace removal, got %+v err=%v", globalMember1, err)
	}
	validatedUser, _, err := svc.ValidateSession(ctx, member1Login.SessionToken)
	if err != nil || validatedUser == nil {
		t.Fatalf("expected global session preserved after removal from wsA, got err=%v", err)
	}
	if _, err := svc.ResolveWorkspaceContext(ctx, validatedUser, &wsA.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected access to removed wsA rejected with ErrForbidden, got %v", err)
	}
	if wsBCtx, err := svc.ResolveWorkspaceContext(ctx, validatedUser, &wsB.ID); err != nil || wsBCtx.Workspace.ID != wsB.ID {
		t.Fatalf("expected access to wsB preserved after removal from wsA, got ctx=%+v err=%v", wsBCtx, err)
	}

	// 10. Historical audit records remain available after removal (BR-06).
	auditLogs, err := svc.ListWorkspaceAuditLogs(ctx, wsA.ID, 50)
	if err != nil || len(auditLogs) == 0 {
		t.Fatalf("expected preserved audit logs for wsA, got len=%d err=%v", len(auditLogs), err)
	}
	var foundRemoved, foundRejected bool
	for _, entry := range auditLogs {
		if entry.Event == AuditEventMemberRemoved && entry.UserID != nil && *entry.UserID == member1.ID {
			var meta map[string]any
			_ = json.Unmarshal(entry.Metadata, &meta)
			if meta["previous_role"] == RoleMember && meta["previous_status"] == "active" && meta["new_status"] == "removed" && meta["reason"] == "offboarded" {
				foundRemoved = true
			}
		}
		if entry.Event == AuditEventMembershipOpRejected {
			foundRejected = true
		}
	}
	if !foundRemoved {
		t.Fatalf("expected MEMBER_REMOVED audit event with full metadata in wsA, got %+v", auditLogs)
	}
	if !foundRejected {
		t.Fatalf("expected MEMBERSHIP_OPERATION_REJECTED audit event for blocked operations in wsA")
	}

	// 7. Adding a second Admin allows removing one of the two Admins (BR-01, BR-05).
	seedGovernanceMember(t, store, wsA.ID, admin2.ID, RoleAdmin)
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, super.ID, admin2.ID, "rotate admin", testMeta()); err != nil {
		t.Fatalf("expected removing one of two admins to succeed, got %v", err)
	}
	// Attempting to remove the remaining admin1 must now be rejected again.
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, super.ID, admin1.ID, "rotate admin", testMeta()); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("expected ErrLastActiveAdmin after only admin1 remains, got %v", err)
	}

	// 14. Rejoining Workspace A via re-invite + accept does not create duplicate memberships.
	reinvited, err := svc.AdminCreateUserInWorkspace(ctx, wsA.ID, super.ID, member1.Email, "", RoleAdmin, testMeta())
	if err != nil || reinvited.Status != "pending" || reinvited.Role != RoleAdmin {
		t.Fatalf("expected re-invited removed member to transition to pending admin, got %+v err=%v", reinvited, err)
	}
	if _, err := svc.AcceptWorkspaceInvite(ctx, member1.ID, uuid.Nil, wsA.ID, testMeta()); err != nil {
		t.Fatalf("AcceptWorkspaceInvite on rejoin failed: %v", err)
	}
	var matchingRows int
	store.mu.Lock()
	for _, m := range store.workspaceMembers {
		if m.WorkspaceID == wsA.ID && m.UserID == member1.ID {
			matchingRows++
		}
	}
	store.mu.Unlock()
	if matchingRows != 1 {
		t.Fatalf("expected exactly 1 membership row after rejoining wsA, got %d", matchingRows)
	}
	rejoinedMem, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: member1.ID})
	if err != nil || rejoinedMem.Status != "active" || rejoinedMem.Role != RoleAdmin || rejoinedMem.RemovedAt != nil {
		t.Fatalf("expected rejoined membership to be active admin with cleared removal metadata, got %+v err=%v", rejoinedMem, err)
	}

	// Role change (MEMBER -> ADMIN -> MEMBER) does not alter other workspace memberships (BR-05).
	if _, err := svc.AdminUpdateUserRoleInWorkspace(ctx, wsB.ID, ownerA.ID, member1.ID, RoleAdmin, testMeta()); err != nil {
		t.Fatalf("expected promoting member1 to admin in wsB to succeed, got %v", err)
	}
	if _, err := svc.AdminUpdateUserRoleInWorkspace(ctx, wsB.ID, ownerA.ID, member1.ID, RoleMember, testMeta()); err != nil {
		t.Fatalf("expected demoting member1 back to member in wsB (ownerA remains) to succeed, got %v", err)
	}
	memInAStillAdmin, _ := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: member1.ID})
	if memInAStillAdmin.Role != RoleAdmin {
		t.Fatalf("expected role change in wsB not to affect wsA role, got %s", memInAStillAdmin.Role)
	}
}

func TestGlobalSuspensionProtectionAndIsolation(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	ctx := context.Background()

	super := seedGovernanceUser(t, store, "super@global.test", "Super", RoleOwner, true)
	target := seedGovernanceUser(t, store, "target@global.test", "Target User", RoleOwner, false)
	coOwner := seedGovernanceUser(t, store, "coowner@global.test", "Co Owner", RoleOwner, false)

	// Workspace 1: target + coOwner (valid to suspend target).
	ws1 := seedGovernanceWorkspace(t, store, "Valid Multi-Admin WS", "ws-valid", target.ID)
	seedGovernanceMember(t, store, ws1.ID, target.ID, RoleOwner)
	seedGovernanceMember(t, store, ws1.ID, coOwner.ID, RoleOwner)

	// Workspace 2: target is the ONLY active owner/admin (member2 is regular member).
	member2 := seedGovernanceUser(t, store, "member2@global.test", "Member Two", RoleMember, false)
	ws2 := seedGovernanceWorkspace(t, store, "Single Admin WS", "ws-blocked", target.ID)
	seedGovernanceMember(t, store, ws2.ID, target.ID, RoleOwner)
	seedGovernanceMember(t, store, ws2.ID, member2.ID, RoleMember)

	targetLogin, err := svc.Login(ctx, target.Email, "password123", "", testMeta())
	if err != nil {
		t.Fatalf("target login failed: %v", err)
	}

	// 12. Global Suspend is rejected if ANY affected Workspace would violate BR-01/BR-02.
	impact, err := svc.GetGlobalSuspendImpact(ctx, super.ID, target.ID)
	if err != nil || impact.Allowed {
		t.Fatalf("expected GetGlobalSuspendImpact allowed=false, got allowed=%v err=%v", impact != nil && impact.Allowed, err)
	}
	if _, err := svc.PlatformSuspendUser(ctx, super.ID, target.ID, "security review", testMeta()); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("expected ErrLastActiveAdmin when globally suspending sole owner of ws2, got %v", err)
	}
	// Verify no partial mutation occurred: target remains active and session remains intact.
	targetAfterReject, _ := store.GetUserByID(ctx, target.ID)
	if targetAfterReject.Status != "active" {
		t.Fatalf("expected target status to remain active after rejected global suspend, got %s", targetAfterReject.Status)
	}
	if _, _, valErr := svc.ValidateSession(ctx, targetLogin.SessionToken); valErr != nil {
		t.Fatalf("expected target session preserved after rejected global suspend, got %v", valErr)
	}

	// 13. Once ws2 has a second active owner, Global Suspend succeeds atomically.
	seedGovernanceMember(t, store, ws2.ID, coOwner.ID, RoleOwner)
	impactOK, err := svc.GetGlobalSuspendImpact(ctx, super.ID, target.ID)
	if err != nil || !impactOK.Allowed {
		t.Fatalf("expected GetGlobalSuspendImpact allowed=true after adding coOwner, got allowed=%v err=%v", impactOK != nil && impactOK.Allowed, err)
	}
	suspendedDTO, err := svc.PlatformSuspendUser(ctx, super.ID, target.ID, "security review", testMeta())
	if err != nil || suspendedDTO.Status != "suspended" {
		t.Fatalf("expected PlatformSuspendUser to succeed, got %+v err=%v", suspendedDTO, err)
	}
	if _, _, valErr := svc.ValidateSession(ctx, targetLogin.SessionToken); !errors.Is(valErr, ErrUnauthorized) {
		t.Fatalf("expected target sessions revoked on global suspend, got %v", valErr)
	}
	// Membership records are preserved across workspaces.
	if m1, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: ws1.ID, UserID: target.ID}); err != nil || m1.Status != "active" {
		t.Fatalf("expected ws1 membership record preserved, got %+v err=%v", m1, err)
	}

	// Reactivating global account logs USER_GLOBALLY_UNSUSPENDED and restores login.
	reactivatedDTO, err := svc.PlatformActivateUser(ctx, super.ID, target.ID, testMeta())
	if err != nil || reactivatedDTO.Status != "active" {
		t.Fatalf("expected PlatformActivateUser to succeed, got %+v err=%v", reactivatedDTO, err)
	}
	if _, err := svc.Login(ctx, target.Email, "password123", "", testMeta()); err != nil {
		t.Fatalf("expected globally reactivated user to log in, got %v", err)
	}

	store.mu.Lock()
	var sawGlobalSuspend, sawGlobalUnsuspend, sawGlobalReject bool
	for _, entry := range store.auditLogs {
		if entry.UserID.Valid && entry.UserID.UUID == target.ID {
			switch entry.Event {
			case AuditEventUserGloballySuspended:
				if strings.Contains(string(entry.Metadata), "security review") {
					sawGlobalSuspend = true
				}
			case AuditEventUserGloballyUnsuspended:
				sawGlobalUnsuspend = true
			case AuditEventGlobalSuspendOpRejected:
				sawGlobalReject = true
			}
		}
	}
	store.mu.Unlock()
	if !sawGlobalSuspend || !sawGlobalUnsuspend || !sawGlobalReject {
		t.Fatalf("expected global audit events, got suspend=%v unsuspend=%v reject=%v", sawGlobalSuspend, sawGlobalUnsuspend, sawGlobalReject)
	}
}

func TestConcurrentAdminRemovalAndDemotionPreserveInvariant(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	ctx := context.Background()

	super := seedGovernanceUser(t, store, "super@conc.test", "Super", RoleOwner, true)
	adminA := seedGovernanceUser(t, store, "admina@conc.test", "Admin A", RoleAdmin, false)
	adminB := seedGovernanceUser(t, store, "adminb@conc.test", "Admin B", RoleAdmin, false)

	ws := seedGovernanceWorkspace(t, store, "Concurrent WS", "concurrent-ws", super.ID)
	seedGovernanceMember(t, store, ws.ID, adminA.ID, RoleAdmin)
	seedGovernanceMember(t, store, ws.ID, adminB.ID, RoleAdmin)

	// 11. Launch two concurrent removals of the only two admins: strictly 1 must succeed and 1 must be rejected.
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	targets := []uuid.UUID{adminA.ID, adminB.ID}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = svc.AdminRemoveUserInWorkspaceWithReason(ctx, ws.ID, super.ID, targets[idx], "concurrent remove", testMeta())
		}(i)
	}
	close(start)
	wg.Wait()

	var successes, rejected int
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrLastActiveAdmin) {
			rejected++
		} else {
			t.Fatalf("unexpected concurrent removal error: %v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("expected strictly 1 success and 1 ErrLastActiveAdmin rejection, got successes=%d rejected=%d errs=%v", successes, rejected, errs)
	}

	stats, err := store.GetWorkspaceActiveMembershipStats(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceActiveMembershipStats failed: %v", err)
	}
	if stats.ActiveAdmins < 1 || stats.ActiveMembers < 1 {
		t.Fatalf("invariant violated after concurrent removal: %+v", stats)
	}

	// Re-add second admin and test concurrent demotion + suspension.
	adminC := seedGovernanceUser(t, store, "adminc@conc.test", "Admin C", RoleAdmin, false)
	seedGovernanceMember(t, store, ws.ID, adminC.ID, RoleAdmin)

	var remainingAdminID uuid.UUID
	for _, id := range targets {
		if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: id}); err == nil && m.Status == "active" {
			remainingAdminID = id
		}
	}

	start2 := make(chan struct{})
	var wg2 sync.WaitGroup
	errs2 := make([]error, 2)
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		<-start2
		_, errs2[0] = svc.AdminUpdateUserRoleInWorkspace(ctx, ws.ID, super.ID, remainingAdminID, RoleMember, testMeta())
	}()
	go func() {
		defer wg2.Done()
		<-start2
		_, errs2[1] = svc.AdminSuspendUserInWorkspace(ctx, ws.ID, super.ID, adminC.ID, "concurrent suspend", testMeta())
	}()
	close(start2)
	wg2.Wait()

	var ok2, rej2 int
	for _, err := range errs2 {
		if err == nil {
			ok2++
		} else if errors.Is(err, ErrLastActiveAdmin) {
			rej2++
		} else {
			t.Fatalf("unexpected concurrent demote/suspend error: %v", err)
		}
	}
	if ok2 != 1 || rej2 != 1 {
		t.Fatalf("expected strictly 1 success and 1 ErrLastActiveAdmin under concurrent demote+suspend, got ok=%d rej=%d errs=%v", ok2, rej2, errs2)
	}
	stats2, _ := store.GetWorkspaceActiveMembershipStats(ctx, ws.ID)
	if stats2.ActiveAdmins < 1 || stats2.ActiveMembers < 1 {
		t.Fatalf("invariant violated after concurrent demote+suspend: %+v", stats2)
	}
}

