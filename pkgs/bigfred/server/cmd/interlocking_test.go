package cmd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

func TestInterlockingCatalogAdminOnly(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := cmd.NewInterlocking(bundle.Interlockings, bundle.LayoutInterlockings)

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	row, err := svc.Create(ctx, adminEff, cmd.InterlockingCreateInput{
		Name:     "North",
		Location: "Sector A",
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}

	newName := "North Renamed"
	if _, err := svc.Update(ctx, adminEff, row.ID, cmd.InterlockingUpdateInput{
		Name: &newName,
	}); err != nil {
		t.Fatalf("admin update: %v", err)
	}

	if _, err := svc.Create(ctx, driverEff, cmd.InterlockingCreateInput{
		Name: "South",
	}); !errors.Is(err, svcerrors.ErrInterlockingForbidden) {
		t.Fatalf("expected ErrInterlockingForbidden on create, got %v", err)
	}
	if _, err := svc.Update(ctx, driverEff, row.ID, cmd.InterlockingUpdateInput{
		Name: &newName,
	}); !errors.Is(err, svcerrors.ErrInterlockingForbidden) {
		t.Fatalf("expected ErrInterlockingForbidden on update, got %v", err)
	}
	if err := svc.Delete(ctx, driverEff, row.ID); !errors.Is(err, svcerrors.ErrInterlockingForbidden) {
		t.Fatalf("expected ErrInterlockingForbidden on delete, got %v", err)
	}

	if err := svc.Delete(ctx, adminEff, row.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}
