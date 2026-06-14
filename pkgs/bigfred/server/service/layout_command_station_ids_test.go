package service_test

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func TestLayoutCommandStationIDsForLayout(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := freshLayoutSvc(t, ctx, bundle)
	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 A")
	cs2 := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 B")

	system, err := svc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("system layout: %v", err)
	}
	systemIDs, err := svc.CommandStationIDsForLayout(ctx, system.ID)
	if err != nil {
		t.Fatalf("system ids: %v", err)
	}
	if len(systemIDs) != 2 {
		t.Fatalf("system layout should see full catalogue, got %v", systemIDs)
	}

	layout, err := svc.Create(ctx, adminEff, service.CreateInput{
		Name:              "Ops",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs.ID},
		AdminPIN:          "1234",
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	ids, err := svc.CommandStationIDsForLayout(ctx, layout.ID)
	if err != nil {
		t.Fatalf("layout ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != cs.ID {
		t.Fatalf("expected [%d], got %v", cs.ID, ids)
	}

	if _, err := svc.SetCommandStations(ctx, adminEff, layout.ID, 1, []uint{cs2.ID}); err != nil {
		t.Fatalf("set command stations: %v", err)
	}
	ids, err = svc.CommandStationIDsForLayout(ctx, layout.ID)
	if err != nil {
		t.Fatalf("layout ids after set: %v", err)
	}
	if len(ids) != 1 || ids[0] != cs2.ID {
		t.Fatalf("expected [%d], got %v", cs2.ID, ids)
	}
}
