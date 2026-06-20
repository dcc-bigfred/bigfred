package cmd_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

func insertUserWithPIN(t *testing.T, ctx context.Context, bundle repo.UsersBundle, login, pin string, role domain.Role) domain.User {
	t.Helper()
	hash, err := helpers.HashPIN(pin)
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}
	now := time.Now().UTC()
	u := domain.User{Login: login, PINHash: hash, Role: role, Active: true, CreatedAt: now, UpdatedAt: now}
	if err := bundle.Users.Insert(ctx, &u); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return u
}

func TestAuthChangePINRotatesHash(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUserWithPIN(t, ctx, bundle, "alice", "1234", domain.RoleDriver)
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	if err := auth.ChangePIN(ctx, user.ID, "1234", "5678"); err != nil {
		t.Fatalf("ChangePIN: %v", err)
	}
	if err := auth.ChangePIN(ctx, user.ID, "1234", "9999"); !errors.Is(err, svcerrors.ErrInvalidCredentials) {
		t.Fatalf("old pin should fail after rotation, got %v", err)
	}
	if err := auth.ChangePIN(ctx, user.ID, "5678", "9999"); err != nil {
		t.Fatalf("ChangePIN with new pin: %v", err)
	}
}

func TestAuthChangePINRejectsInvalidNewPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()
	user := insertUserWithPIN(t, ctx, bundle, "alice", "1234", domain.RoleDriver)
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	err := auth.ChangePIN(ctx, user.ID, "1234", "12")
	if !errors.Is(err, svcerrors.ErrUserPINInvalid) {
		t.Fatalf("expected ErrUserPINInvalid, got %v", err)
	}
}
