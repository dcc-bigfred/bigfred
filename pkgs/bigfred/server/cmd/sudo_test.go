package cmd_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

// freshLayoutSvc builds a LayoutService and seeds the bootstrap
// system layout (so VerifyAdminPIN has a comparable digest from the
// first call).
func freshLayoutSvc(t *testing.T, ctx context.Context, bundle repo.UsersBundle) *cmd.Layout {
	t.Helper()
	svc := cmd.NewLayout(
		bundle.Layouts,
		bundle.Interlockings,
		bundle.LayoutInterlockings,
		bundle.CommandStations,
		bundle.LayoutCommandStations,
	)
	if _, err := svc.EnsureSystemLayout(ctx); err != nil {
		t.Fatalf("seed system layout: %v", err)
	}
	return svc
}

func freshSudoSvc(t *testing.T, ctx context.Context, bundle repo.UsersBundle, ttl time.Duration) (*cmd.Sudo, *cmd.Layout, domain.Layout) {
	t.Helper()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	system, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system layout: %v", err)
	}
	cfg := cmd.SudoConfig{
		TTL:             ttl,
		FailWindow:      5 * time.Second,
		MaxFailures:     3,
		LockDuration:    100 * time.Millisecond,
		JanitorInterval: 50 * time.Millisecond,
	}
	return cmd.NewSudo(bundle.SudoElevations, bundle.LayoutSignalmen, layoutSvc, nil, cfg), layoutSvc, system
}

func TestSudoElevatesWithCorrectPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	row, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN)
	if err != nil {
		t.Fatalf("Sudo: %v", err)
	}
	if row.UserID != user.ID || row.LayoutID != system.ID {
		t.Fatalf("row mismatch: %+v", row)
	}
	if !row.IsActive(time.Now().UTC()) {
		t.Fatalf("row should be active immediately after grant")
	}
}

func TestSudoRejectsWrongPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	_, err := sudo.Sudo(ctx, user.ID, system.ID, "9999")
	if !errors.Is(err, svcerrors.ErrSudoInvalidPIN) {
		t.Fatalf("expected ErrSudoInvalidPIN, got %v", err)
	}
}

func TestSudoLocksAfterTooManyFailures(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	for i := 0; i < 3; i++ {
		_, err := sudo.Sudo(ctx, user.ID, system.ID, "9999")
		if !errors.Is(err, svcerrors.ErrSudoInvalidPIN) {
			t.Fatalf("attempt %d: expected ErrSudoInvalidPIN, got %v", i, err)
		}
	}

	_, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN)
	if !errors.Is(err, svcerrors.ErrSudoLocked) {
		t.Fatalf("expected ErrSudoLocked even with correct PIN, got %v", err)
	}
}

func TestSudoRevokeIsIdempotent(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	if err := sudo.Revoke(ctx, user.ID, system.ID); err != nil {
		t.Fatalf("revoke without grant: %v", err)
	}

	if _, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN); err != nil {
		t.Fatalf("Sudo: %v", err)
	}

	if err := sudo.Revoke(ctx, user.ID, system.ID); err != nil {
		t.Fatalf("revoke with grant: %v", err)
	}

	row, err := sudo.FindActive(ctx, user.ID, system.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if row != nil {
		t.Fatalf("expected no active row after revoke, got %+v", row)
	}
}

func TestSudoRenewsTimerOnRepeatedSuccess(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	first, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN)
	if err != nil {
		t.Fatalf("first Sudo: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	second, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN)
	if err != nil {
		t.Fatalf("second Sudo: %v", err)
	}
	if !second.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("second elevation should renew timer; first=%v second=%v", first.ExpiresAt, second.ExpiresAt)
	}
}

func TestSudoExpiresAfterTTL(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 50*time.Millisecond)

	if _, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN); err != nil {
		t.Fatalf("Sudo: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	row, err := sudo.FindActive(ctx, user.ID, system.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if row != nil {
		t.Fatalf("expected no active row after TTL elapsed, got %+v", row)
	}
}

func TestSignalmanSelfGrantIsPermanent(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, layoutSvc, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	if err := sudo.GrantSignalman(ctx, user.ID, system.ID, cmd.DefaultAdminPIN); err != nil {
		t.Fatalf("GrantSignalman: %v", err)
	}

	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	roles, err := auth.Effective(ctx, user, system.ID)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if !roles.Has(domain.RoleSignalman) {
		t.Fatalf("expected signalman in effective roles")
	}

	row, err := bundle.LayoutSignalmen.FindActiveGrant(ctx, system.ID, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("FindActiveGrant: %v", err)
	}
	if row.ExpiresAt != nil {
		t.Fatalf("signalman icon must produce a permanent grant; got expiresAt=%v", row.ExpiresAt)
	}

	// Idempotent revoke: missing row → nil; existing row → drop.
	if err := sudo.RevokeSignalman(ctx, user.ID, system.ID); err != nil {
		t.Fatalf("RevokeSignalman: %v", err)
	}
	if err := sudo.RevokeSignalman(ctx, user.ID, system.ID); err != nil {
		t.Fatalf("RevokeSignalman idempotent: %v", err)
	}

	roles, err = auth.Effective(ctx, user, system.ID)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if roles.Has(domain.RoleSignalman) {
		t.Fatalf("expected signalman to be cleared after revoke")
	}
}

func TestGrantSignalmanToUserByAdmin(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)
	target := insertUser(t, ctx, bundle.Users, "bob", domain.RoleDriver)
	sudo, layoutSvc, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	if err := sudo.GrantSignalmanToUser(ctx, admin.ID, target.ID, system.ID); err != nil {
		t.Fatalf("GrantSignalmanToUser: %v", err)
	}

	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	roles, err := auth.Effective(ctx, target, system.ID)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if !roles.Has(domain.RoleSignalman) {
		t.Fatalf("expected target to have signalman role")
	}

	row, err := bundle.LayoutSignalmen.FindActiveGrant(ctx, system.ID, target.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("FindActiveGrant: %v", err)
	}
	if row.GrantedBy != admin.ID {
		t.Fatalf("GrantedBy = %d, want %d", row.GrantedBy, admin.ID)
	}
}

func TestRevokeSignalmanFromUserByAdmin(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)
	target := insertUser(t, ctx, bundle.Users, "bob", domain.RoleDriver)
	sudo, layoutSvc, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	if err := sudo.GrantSignalmanToUser(ctx, admin.ID, target.ID, system.ID); err != nil {
		t.Fatalf("GrantSignalmanToUser: %v", err)
	}
	if err := sudo.RevokeSignalman(ctx, target.ID, system.ID); err != nil {
		t.Fatalf("RevokeSignalman: %v", err)
	}

	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})
	roles, err := auth.Effective(ctx, target, system.ID)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if roles.Has(domain.RoleSignalman) {
		t.Fatalf("expected signalman grant to be revoked")
	}
}

func TestSignalmanRejectsWrongPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	if err := sudo.GrantSignalman(ctx, user.ID, system.ID, "9999"); !errors.Is(err, svcerrors.ErrSudoInvalidPIN) {
		t.Fatalf("expected ErrSudoInvalidPIN, got %v", err)
	}
}

func TestLayoutUpdateAdminPINBlankIsNoop(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)

	system, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system: %v", err)
	}
	originalHash := system.AdminPINHash

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	if _, err := layoutSvc.UpdateAdminPIN(ctx, adminEff, system.ID, ""); err != nil {
		t.Fatalf("blank UpdateAdminPIN: %v", err)
	}

	again, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("re-get system: %v", err)
	}
	if again.AdminPINHash != originalHash {
		t.Fatalf("blank PIN must not rotate the hash")
	}
}

func TestLayoutUpdateAdminPINRejectsLetters(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	system, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	_, err = layoutSvc.UpdateAdminPIN(ctx, adminEff, system.ID, "abcd")
	if !errors.Is(err, svcerrors.ErrLayoutAdminPINInvalid) {
		t.Fatalf("expected ErrLayoutAdminPINInvalid, got %v", err)
	}
	_, err = layoutSvc.UpdateAdminPIN(ctx, adminEff, system.ID, "12")
	if !errors.Is(err, svcerrors.ErrLayoutAdminPINInvalid) {
		t.Fatalf("expected ErrLayoutAdminPINInvalid, got %v", err)
	}
}

func TestLayoutUpdateAdminPINRotates(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	system, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	if _, err := layoutSvc.UpdateAdminPIN(ctx, adminEff, system.ID, "1234"); err != nil {
		t.Fatalf("UpdateAdminPIN: %v", err)
	}

	if err := layoutSvc.VerifyAdminPIN(ctx, system.ID, "1234"); err != nil {
		t.Fatalf("VerifyAdminPIN(new): %v", err)
	}
	if err := layoutSvc.VerifyAdminPIN(ctx, system.ID, cmd.DefaultAdminPIN); !errors.Is(err, svcerrors.ErrLayoutAdminPINMismatch) {
		t.Fatalf("VerifyAdminPIN(old): expected ErrLayoutAdminPINMismatch, got %v", err)
	}
}

func TestEffectiveRolesIncludesSudoGrant(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, layoutSvc, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	if _, err := sudo.Sudo(ctx, user.ID, system.ID, cmd.DefaultAdminPIN); err != nil {
		t.Fatalf("Sudo: %v", err)
	}

	roles, err := auth.Effective(ctx, user, system.ID)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if !roles.Has(domain.RoleAdmin) {
		t.Fatalf("expected admin in effective roles")
	}
}
