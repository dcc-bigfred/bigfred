package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func TestLayoutCatalogAdminOnly(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := freshLayoutSvc(t, ctx, bundle)

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 Workshop")

	row, err := svc.Create(ctx, adminEff, service.CreateInput{
		Name:              "Club Night",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs.ID},
		AdminPIN:          "1234",
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}

	newName := "Club Night Renamed"
	if _, err := svc.Rename(ctx, adminEff, row.ID, newName); err != nil {
		t.Fatalf("admin rename: %v", err)
	}

	if _, err := svc.Create(ctx, driverEff, service.CreateInput{
		Name:      "Other",
		CreatedBy: 1,
	}); !errors.Is(err, service.ErrLayoutForbidden) {
		t.Fatalf("expected ErrLayoutForbidden on create, got %v", err)
	}
	if _, err := svc.Rename(ctx, driverEff, row.ID, newName); !errors.Is(err, service.ErrLayoutForbidden) {
		t.Fatalf("expected ErrLayoutForbidden on rename, got %v", err)
	}
	if err := svc.Delete(ctx, driverEff, row.ID); !errors.Is(err, service.ErrLayoutForbidden) {
		t.Fatalf("expected ErrLayoutForbidden on delete, got %v", err)
	}

	if err := svc.Delete(ctx, adminEff, row.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}
