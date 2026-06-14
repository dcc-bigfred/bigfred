package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func TestInterlockingCatalogAdminOnly(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewInterlockingService(bundle.Interlockings, bundle.LayoutInterlockings)

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	row, err := svc.Create(ctx, adminEff, service.InterlockingCreateInput{
		Name:     "North",
		Location: "Sector A",
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}

	newName := "North Renamed"
	if _, err := svc.Update(ctx, adminEff, row.ID, service.InterlockingUpdateInput{
		Name: &newName,
	}); err != nil {
		t.Fatalf("admin update: %v", err)
	}

	if _, err := svc.Create(ctx, driverEff, service.InterlockingCreateInput{
		Name: "South",
	}); !errors.Is(err, service.ErrInterlockingForbidden) {
		t.Fatalf("expected ErrInterlockingForbidden on create, got %v", err)
	}
	if _, err := svc.Update(ctx, driverEff, row.ID, service.InterlockingUpdateInput{
		Name: &newName,
	}); !errors.Is(err, service.ErrInterlockingForbidden) {
		t.Fatalf("expected ErrInterlockingForbidden on update, got %v", err)
	}
	if err := svc.Delete(ctx, driverEff, row.ID); !errors.Is(err, service.ErrInterlockingForbidden) {
		t.Fatalf("expected ErrInterlockingForbidden on delete, got %v", err)
	}

	if err := svc.Delete(ctx, adminEff, row.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}
