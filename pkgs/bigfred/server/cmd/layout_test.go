package cmd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

func TestLayoutCatalogAdminOnly(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := freshLayoutSvc(t, ctx, bundle)

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 Workshop")

	row, err := svc.Create(ctx, adminEff, cmd.LayoutCreateInput{
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

	if _, err := svc.Create(ctx, driverEff, cmd.LayoutCreateInput{
		Name:      "Other",
		CreatedBy: 1,
	}); !errors.Is(err, svcerrors.ErrLayoutForbidden) {
		t.Fatalf("expected ErrLayoutForbidden on create, got %v", err)
	}
	if _, err := svc.Rename(ctx, driverEff, row.ID, newName); !errors.Is(err, svcerrors.ErrLayoutForbidden) {
		t.Fatalf("expected ErrLayoutForbidden on rename, got %v", err)
	}
	if err := svc.Delete(ctx, driverEff, row.ID); !errors.Is(err, svcerrors.ErrLayoutForbidden) {
		t.Fatalf("expected ErrLayoutForbidden on delete, got %v", err)
	}

	if err := svc.Delete(ctx, adminEff, row.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}

func TestListSelectableFallsBackToLockedSystemLayout(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := freshLayoutSvc(t, ctx, bundle)
	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	system, err := svc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system layout: %v", err)
	}

	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 Workshop")
	custom, err := svc.Create(ctx, adminEff, cmd.LayoutCreateInput{
		Name:              "Club Night",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs.ID},
		AdminPIN:          "1234",
	})
	if err != nil {
		t.Fatalf("create custom layout: %v", err)
	}

	if _, err := svc.SetLocked(ctx, system.ID, true); err != nil {
		t.Fatalf("lock system layout: %v", err)
	}
	if _, err := svc.SetLocked(ctx, custom.ID, true); err != nil {
		t.Fatalf("lock custom layout: %v", err)
	}

	rows, err := svc.ListSelectable(ctx)
	if err != nil {
		t.Fatalf("ListSelectable: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 fallback row, got %d", len(rows))
	}
	if !rows[0].IsSystem {
		t.Fatalf("expected system layout fallback, got id=%d", rows[0].ID)
	}

	if _, err := svc.ValidateForLogin(ctx, system.ID); err != nil {
		t.Fatalf("ValidateForLogin system fallback: %v", err)
	}
	if _, err := svc.ValidateForLogin(ctx, custom.ID); !errors.Is(err, svcerrors.ErrLayoutLocked) {
		t.Fatalf("expected ErrLayoutLocked for locked custom layout, got %v", err)
	}
}

func TestListSelectableHidesLockedSystemWhenOthersUnlocked(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := freshLayoutSvc(t, ctx, bundle)
	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	system, err := svc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("get system layout: %v", err)
	}

	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 Workshop")
	custom, err := svc.Create(ctx, adminEff, cmd.LayoutCreateInput{
		Name:              "Club Night",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs.ID},
		AdminPIN:          "1234",
	})
	if err != nil {
		t.Fatalf("create custom layout: %v", err)
	}

	if _, err := svc.SetLocked(ctx, system.ID, true); err != nil {
		t.Fatalf("lock system layout: %v", err)
	}

	rows, err := svc.ListSelectable(ctx)
	if err != nil {
		t.Fatalf("ListSelectable: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != custom.ID {
		t.Fatalf("expected only unlocked custom layout, got %+v", rows)
	}

	if _, err := svc.ValidateForLogin(ctx, system.ID); !errors.Is(err, svcerrors.ErrLayoutLocked) {
		t.Fatalf("expected ErrLayoutLocked for locked system when others unlocked, got %v", err)
	}
}

func TestLayoutUpdateRadioChatEnabled(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := freshLayoutSvc(t, ctx, bundle)
	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)

	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Z21 Workshop")
	row, err := svc.Create(ctx, adminEff, cmd.LayoutCreateInput{
		Name:              "Radio Toggle",
		CreatedBy:         1,
		CommandStationIDs: []uint{cs.ID},
		AdminPIN:          "1234",
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	if !row.RadioChatEnabled {
		t.Fatalf("expected radio chat enabled by default, got false")
	}

	disabled, err := svc.UpdateRadioChatEnabled(ctx, adminEff, row.ID, false)
	if err != nil {
		t.Fatalf("disable radio chat: %v", err)
	}
	if disabled.RadioChatEnabled {
		t.Fatalf("expected radio chat disabled")
	}

	enabled, err := svc.UpdateRadioChatEnabled(ctx, adminEff, row.ID, true)
	if err != nil {
		t.Fatalf("enable radio chat: %v", err)
	}
	if !enabled.RadioChatEnabled {
		t.Fatalf("expected radio chat enabled again")
	}
}
