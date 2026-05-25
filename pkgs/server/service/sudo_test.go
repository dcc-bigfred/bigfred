package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/service"
)

// freshLayoutSvc builds a LayoutService and seeds the bootstrap
// system layout (so VerifyAdminPIN has a comparable digest from the
// first call).
func freshLayoutSvc(t *testing.T, ctx context.Context, bundle repo.UsersBundle) *service.LayoutService {
	t.Helper()
	svc := service.NewLayoutService(bundle.Layouts, bundle.Interlockings, bundle.LayoutInterlockings)
	if _, err := svc.EnsureSystemLayout(ctx); err != nil {
		t.Fatalf("seed system layout: %v", err)
	}
	return svc
}

func freshSudoSvc(t *testing.T, ctx context.Context, bundle repo.UsersBundle, ttl time.Duration) (*service.SudoService, *service.LayoutService, domain.Layout) {
	t.Helper()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	system, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system layout: %v", err)
	}
	cfg := service.SudoConfig{
		TTL:             ttl,
		FailWindow:      5 * time.Second,
		MaxFailures:     3,
		LockDuration:    100 * time.Millisecond,
		JanitorInterval: 50 * time.Millisecond,
	}
	return service.NewSudoService(bundle.SudoElevations, bundle.LayoutSignalmen, layoutSvc, nil, cfg), layoutSvc, system
}

func TestSudoElevatesWithCorrectPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	row, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN)
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
	if !errors.Is(err, service.ErrSudoInvalidPIN) {
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
		if !errors.Is(err, service.ErrSudoInvalidPIN) {
			t.Fatalf("attempt %d: expected ErrSudoInvalidPIN, got %v", i, err)
		}
	}

	_, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN)
	if !errors.Is(err, service.ErrSudoLocked) {
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

	if _, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN); err != nil {
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

	first, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN)
	if err != nil {
		t.Fatalf("first Sudo: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	second, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN)
	if err != nil {
		t.Fatalf("second Sudo: %v", err)
	}
	if !second.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("second elevation should renew timer; first=%v second=%v", first.ExpiresAt, second.ExpiresAt)
	}
	if first.ID != second.ID {
		t.Fatalf("expected single row to be reused; got distinct IDs %d vs %d", first.ID, second.ID)
	}
}

func TestSudoExpiresAfterTTL(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 50*time.Millisecond)

	if _, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN); err != nil {
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

	if err := sudo.GrantSignalman(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN); err != nil {
		t.Fatalf("GrantSignalman: %v", err)
	}

	auth := service.NewAuthService(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		service.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

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

	auth := service.NewAuthService(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		service.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

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

func TestSignalmanRejectsWrongPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, _, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	if err := sudo.GrantSignalman(ctx, user.ID, system.ID, "9999"); !errors.Is(err, service.ErrSudoInvalidPIN) {
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

	if _, err := layoutSvc.UpdateAdminPIN(ctx, system.ID, ""); err != nil {
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

	_, err = layoutSvc.UpdateAdminPIN(ctx, system.ID, "abcd")
	if !errors.Is(err, service.ErrLayoutAdminPINInvalid) {
		t.Fatalf("expected ErrLayoutAdminPINInvalid, got %v", err)
	}
	_, err = layoutSvc.UpdateAdminPIN(ctx, system.ID, "12")
	if !errors.Is(err, service.ErrLayoutAdminPINInvalid) {
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

	if _, err := layoutSvc.UpdateAdminPIN(ctx, system.ID, "1234"); err != nil {
		t.Fatalf("UpdateAdminPIN: %v", err)
	}

	if err := layoutSvc.VerifyAdminPIN(ctx, system.ID, "1234"); err != nil {
		t.Fatalf("VerifyAdminPIN(new): %v", err)
	}
	if err := layoutSvc.VerifyAdminPIN(ctx, system.ID, service.SystemLayoutDefaultAdminPIN); !errors.Is(err, service.ErrLayoutAdminPINMismatch) {
		t.Fatalf("VerifyAdminPIN(old): expected ErrLayoutAdminPINMismatch, got %v", err)
	}
}

func TestEffectiveRolesIncludesSudoGrant(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "alice", domain.RoleDriver)
	sudo, layoutSvc, system := freshSudoSvc(t, ctx, bundle, 2*time.Minute)

	auth := service.NewAuthService(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		service.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	if _, err := sudo.Sudo(ctx, user.ID, system.ID, service.SystemLayoutDefaultAdminPIN); err != nil {
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
