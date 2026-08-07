package cmd_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/repo/migrations"
)

func freshRedisLeaseStores(t *testing.T) (repo.VehicleLeaseStore, repo.TrainLeaseStore) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return repo.NewRedisVehicleLeases(client), repo.NewRedisTrainLeases(client)
}

// freshRepo opens a brand-new SQLite db in a temp dir and runs every
// migration. Returns the wired *repo.* + cleanup.
func freshRepo(t *testing.T) (repo.UsersBundle, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "service_test.db")

	r, db, err := repo.Open(path, nil, nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	migrations.MigrateUp(context.Background(), r)

	vehicleLeases, trainLeases := freshRedisLeaseStores(t)
	bundle := repo.UsersBundle{
		Repo:                  r,
		Users:                 repo.NewUsers(r),
		Pool:                  repo.NewDCCAddressRanges(r),
		Vehicles:              repo.NewVehicles(r),
		Trains:                repo.NewTrains(r),
		TrainMembers:          repo.NewTrainMembers(r),
		LayoutVehicles:        repo.NewLayoutVehicles(r),
		LayoutTrains:          repo.NewLayoutTrains(r),
		LayoutSignalmen:       repo.NewLayoutSignalmen(r),
		Layouts:               repo.NewLayouts(r),
		Interlockings:         repo.NewInterlockings(r),
		LayoutInterlockings:   repo.NewLayoutInterlockings(r),
		CommandStations:       repo.NewCommandStations(r),
		LayoutCommandStations: repo.NewLayoutCommandStations(r),
		DccFunctions:          repo.NewDccFunctions(r),
		SudoElevations:        repo.NewSudoElevations(r),
		VehicleLeases:         vehicleLeases,
		TrainLeases:           trainLeases,
	}
	cleanup := func() {
		_ = db.Close()
		_ = os.Remove(path)
	}
	return bundle, cleanup
}

func insertCommandStation(t *testing.T, ctx context.Context, stations *repo.CommandStations, name string) domain.CommandStation {
	t.Helper()
	now := time.Now().UTC()
	cs := domain.CommandStation{
		Name:          name,
		Kind:          domain.CommandStationKindZ21,
		ConnectionURI: "udp://127.0.0.1:21105",
		SpeedSteps:    128,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := stations.Insert(ctx, &cs); err != nil {
		t.Fatalf("insert command station: %v", err)
	}
	return cs
}

func insertUser(t *testing.T, ctx context.Context, users *repo.Users, login string, role domain.Role) domain.User {
	t.Helper()
	now := time.Now().UTC()
	u := domain.User{Login: login, PINHash: "x", Role: role, Active: true, CreatedAt: now, UpdatedAt: now}
	if err := users.Insert(ctx, &u); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return u
}

func TestVehicleCreateAcceptsDummy(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Dummy boxcar",
		Kind:        domain.VehicleKindWagon,
		Number:      "92510",
	})
	if err != nil {
		t.Fatalf("create dummy: %v", err)
	}
	if v.DCCAddress != nil {
		t.Fatalf("expected dummy (nil DCC), got %v", v.DCCAddress)
	}
	if !v.IsDummy() {
		t.Fatalf("expected IsDummy true")
	}
	if !v.ID.Valid() {
		t.Fatalf("expected V- prefixed catalogue id, got %q", v.ID)
	}
}

func TestVehicleCreateRejectsOutsidePool(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, user.ID, []cmd.PoolRange{{From: 100, To: 199}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	addrInside := uint16(150)
	if _, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco inside",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addrInside,
	}); err != nil {
		t.Fatalf("expected inside-pool create to succeed, got %v", err)
	}

	addrOutside := uint16(7000)
	_, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco outside",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addrOutside,
	})
	if !errors.Is(err, svcerrors.ErrDCCAddressOutsidePool) {
		t.Fatalf("expected ErrDCCAddressOutsidePool, got %v", err)
	}
}

func TestVehicleCreateRejectsDuplicateDCC(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, user.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	addr := uint16(33)
	if _, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "A", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "B", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if !errors.Is(err, svcerrors.ErrDCCAddressTaken) {
		t.Fatalf("expected ErrDCCAddressTaken, got %v", err)
	}
}

func TestVehicleDeleteRefusedWhenInTrain(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, user.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vSvc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)
	tSvc := cmd.NewTrain(bundle.Repo, bundle.Trains, bundle.TrainMembers, bundle.Vehicles, bundle.LayoutTrains, bundle.Users)

	addr := uint16(7)
	v, err := vSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	if _, err := tSvc.Create(ctx, cmd.TrainCreateInput{
		OwnerUserID: user.ID,
		Name:        "Test",
		Members:     []cmd.TrainMemberInput{{VehicleID: v.ID}},
	}); err != nil {
		t.Fatalf("create train: %v", err)
	}

	if _, err := vSvc.Delete(ctx, user.ID, v.ID, domain.NewEffectiveRoles(domain.RoleDriver)); !errors.Is(err, svcerrors.ErrVehicleInUse) {
		t.Fatalf("expected ErrVehicleInUse, got %v", err)
	}
}

func TestVehicleAdminCanMutateOthersVehicle(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed owner pool: %v", err)
	}
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	newName := "Renamed by admin"
	updated, err := svc.Update(ctx, admin.ID, v.ID, adminEff, cmd.VehicleUpdateInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("admin update: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("name = %q, want %q", updated.Name, newName)
	}

	if _, err := svc.Delete(ctx, admin.ID, v.ID, adminEff); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}

func TestVehicleNonOwnerDriverCannotMutateOthersVehicle(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	other := insertUser(t, ctx, bundle.Users, "other", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	newName := "Hijacked"
	_, err = svc.Update(ctx, other.ID, v.ID, driverEff, cmd.VehicleUpdateInput{Name: &newName})
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned on update, got %v", err)
	}
	_, err = svc.Delete(ctx, other.ID, v.ID, driverEff)
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned on delete, got %v", err)
	}
}

func TestVehicleCreateStoresCatalogMetadata(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	revision := "2020-05-15"
	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID:  user.ID,
		Name:         "ET22",
		Kind:         domain.VehicleKindLoco,
		Carrier:      " PKP ",
		Assignment:   " Cargo ",
		Epoch:        "IIIa",
		RevisionDate: &revision,
	})
	if err != nil {
		t.Fatalf("create with catalog metadata: %v", err)
	}
	if v.Carrier != "PKP" {
		t.Fatalf("carrier = %q, want %q", v.Carrier, "PKP")
	}
	if v.Assignment != "Cargo" {
		t.Fatalf("assignment = %q, want %q", v.Assignment, "Cargo")
	}
	if v.Epoch != domain.VehicleEpochIIIa {
		t.Fatalf("epoch = %q, want %q", v.Epoch, domain.VehicleEpochIIIa)
	}
	if v.RevisionDate == nil {
		t.Fatal("expected revision date to be set")
	}
	if v.RevisionDate.UTC().Format("2006-01-02") != revision {
		t.Fatalf("revision date = %q, want %q", v.RevisionDate.UTC().Format("2006-01-02"), revision)
	}
}

func TestVehicleCreateRejectsInvalidEpoch(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	_, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
		Epoch:       "VII",
	})
	if !errors.Is(err, svcerrors.ErrVehicleEpochInvalid) {
		t.Fatalf("expected ErrVehicleEpochInvalid, got %v", err)
	}
}

func TestVehicleCreateRejectsInvalidRevisionDate(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	bad := "not-a-date"
	_, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID:  user.ID,
		Name:         "Loco",
		Kind:         domain.VehicleKindLoco,
		RevisionDate: &bad,
	})
	if !errors.Is(err, svcerrors.ErrVehicleRevisionDateInvalid) {
		t.Fatalf("expected ErrVehicleRevisionDateInvalid, got %v", err)
	}
}

func TestVehicleUpdateAlwaysOverwritesCatalogMetadata(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	revision := "2019-01-01"
	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID:  user.ID,
		Name:         "ET22",
		Kind:         domain.VehicleKindLoco,
		Carrier:      "PKP",
		Assignment:   "Cargo",
		Epoch:        "IV",
		RevisionDate: &revision,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	newName := "ET22-1175"
	newRevision := "2021-06-01"
	updated, err := svc.Update(ctx, user.ID, v.ID, driverEff, cmd.VehicleUpdateInput{
		Name:         &newName,
		Carrier:      "IC",
		Assignment:   "Pasażerski",
		Epoch:        "V",
		RevisionDate: &newRevision,
	})
	if err != nil {
		t.Fatalf("update catalog metadata: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("name = %q, want %q", updated.Name, newName)
	}
	if updated.Carrier != "IC" {
		t.Fatalf("carrier = %q, want %q", updated.Carrier, "IC")
	}
	if updated.Assignment != "Pasażerski" {
		t.Fatalf("assignment = %q, want %q", updated.Assignment, "Pasażerski")
	}
	if updated.Epoch != domain.VehicleEpochV {
		t.Fatalf("epoch = %q, want %q", updated.Epoch, domain.VehicleEpochV)
	}
	if updated.RevisionDate == nil || updated.RevisionDate.UTC().Format("2006-01-02") != newRevision {
		t.Fatalf("revision date = %v, want %q", updated.RevisionDate, newRevision)
	}

	// Every update applies catalog fields — a name-only patch with zero values clears them.
	renamed := "Renamed"
	cleared, err := svc.Update(ctx, user.ID, v.ID, driverEff, cmd.VehicleUpdateInput{
		Name: &renamed,
	})
	if err != nil {
		t.Fatalf("name-only update: %v", err)
	}
	if cleared.Carrier != "" {
		t.Fatalf("carrier = %q, want empty after overwrite", cleared.Carrier)
	}
	if cleared.Assignment != "" {
		t.Fatalf("assignment = %q, want empty after overwrite", cleared.Assignment)
	}
	if cleared.Epoch != domain.VehicleEpochNone {
		t.Fatalf("epoch = %q, want empty after overwrite", cleared.Epoch)
	}
	if cleared.RevisionDate != nil {
		t.Fatalf("revision date = %v, want nil after overwrite", cleared.RevisionDate)
	}
}

func TestVehicleUpsertByExternalIDCreatesThenOverwrites(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	created, wasNew, err := svc.UpsertByExternalID(ctx, user.ID, driverEff, "ext-1", cmd.VehicleCreateInput{
		Name:     "Lok",
		Kind:     domain.VehicleKindLoco,
		Carrier:  "PKP",
		Number:   "ET22",
	})
	if err != nil {
		t.Fatalf("upsert create: %v", err)
	}
	if !wasNew {
		t.Fatal("expected created=true on first upsert")
	}
	if created.Source != domain.EntitySourceAndroidCatalog {
		t.Fatalf("source = %q, want %q", created.Source, domain.EntitySourceAndroidCatalog)
	}
	if created.ExternalID == nil || *created.ExternalID != "ext-1" {
		t.Fatalf("external_id = %v, want ext-1", created.ExternalID)
	}
	if created.Carrier != "PKP" {
		t.Fatalf("carrier = %q, want PKP", created.Carrier)
	}

	updated, wasNew2, err := svc.UpsertByExternalID(ctx, user.ID, driverEff, "ext-1", cmd.VehicleCreateInput{
		Name: "Lok 2",
		Kind: domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("upsert overwrite: %v", err)
	}
	if wasNew2 {
		t.Fatal("expected created=false on second upsert")
	}
	if updated.ID != created.ID {
		t.Fatalf("id = %s, want %s", updated.ID, created.ID)
	}
	if updated.Name != "Lok 2" {
		t.Fatalf("name = %q, want Lok 2", updated.Name)
	}
	if updated.Carrier != "" {
		t.Fatalf("carrier = %q, want empty after overwrite", updated.Carrier)
	}
}

func TestVehicleUpsertByExternalIDRejectsOtherOwner(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	other := insertUser(t, ctx, bundle.Users, "other", domain.RoleDriver)
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	_, _, err := svc.UpsertByExternalID(ctx, owner.ID, driverEff, "ext-owned", cmd.VehicleCreateInput{
		Name: "Owned",
		Kind: domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("owner upsert: %v", err)
	}

	_, _, err = svc.UpsertByExternalID(ctx, other.ID, driverEff, "ext-owned", cmd.VehicleCreateInput{
		Name: "Hijack",
		Kind: domain.VehicleKindLoco,
	})
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned on upsert, got %v", err)
	}

	_, err = svc.DeleteByExternalID(ctx, other.ID, driverEff, "ext-owned")
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned on delete, got %v", err)
	}
}

func TestVehicleDeleteByExternalID(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	_, _, err := svc.UpsertByExternalID(ctx, user.ID, driverEff, "ext-del", cmd.VehicleCreateInput{
		Name: "To delete",
		Kind: domain.VehicleKindWagon,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	deleted, err := svc.DeleteByExternalID(ctx, user.ID, driverEff, "ext-del")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted.Name != "To delete" {
		t.Fatalf("deleted name = %q", deleted.Name)
	}

	_, err = svc.DeleteByExternalID(ctx, user.ID, driverEff, "ext-del")
	if !errors.Is(err, svcerrors.ErrVehicleNotFound) {
		t.Fatalf("expected ErrVehicleNotFound, got %v", err)
	}

	_, err = svc.DeleteByExternalID(ctx, user.ID, driverEff, "nope")
	if !errors.Is(err, svcerrors.ErrVehicleNotFound) {
		t.Fatalf("expected ErrVehicleNotFound for missing id, got %v", err)
	}
}

func TestClearDCCAddressesOwnOnlyForDriver(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	userSvc := cmd.NewUser(bundle.Repo, bundle.Users, bundle.Vehicles, bundle.Trains, pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	alice, err := userSvc.Create(ctx, testAdminEff, cmd.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []cmd.PoolRange{{From: 100, To: 199}},
	})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := userSvc.Create(ctx, testAdminEff, cmd.UserCreateInput{
		Login: "bob", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []cmd.PoolRange{{From: 200, To: 299}},
	})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	addrA := uint16(110)
	addrB := uint16(210)
	va, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: alice.ID, Name: "Alice loco", Kind: domain.VehicleKindLoco, DCCAddress: &addrA,
	})
	if err != nil {
		t.Fatalf("create alice vehicle: %v", err)
	}
	vb, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: bob.ID, Name: "Bob loco", Kind: domain.VehicleKindLoco, DCCAddress: &addrB,
	})
	if err != nil {
		t.Fatalf("create bob vehicle: %v", err)
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	_, err = svc.ClearDCCAddresses(ctx, alice.ID, driverEff, 0, []domain.VehicleID{va.ID, vb.ID})
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned, got %v", err)
	}
	reloaded, err := svc.Get(ctx, va.ID)
	if err != nil {
		t.Fatalf("reload alice vehicle: %v", err)
	}
	if reloaded.DCCAddress == nil || *reloaded.DCCAddress != addrA {
		t.Fatalf("alice address changed after failed clear: %+v", reloaded.DCCAddress)
	}
	reloadedB, err := svc.Get(ctx, vb.ID)
	if err != nil {
		t.Fatalf("reload bob vehicle: %v", err)
	}
	if reloadedB.DCCAddress == nil || *reloadedB.DCCAddress != addrB {
		t.Fatalf("bob address changed after failed clear: %+v", reloadedB.DCCAddress)
	}

	cleared, err := svc.ClearDCCAddresses(ctx, alice.ID, driverEff, 0, []domain.VehicleID{va.ID})
	if err != nil {
		t.Fatalf("clear own: %v", err)
	}
	if len(cleared) != 1 || cleared[0].DCCAddress != nil {
		t.Fatalf("unexpected clear result: %+v", cleared)
	}
	reloaded, err = svc.Get(ctx, va.ID)
	if err != nil {
		t.Fatalf("reload after clear: %v", err)
	}
	if reloaded.DCCAddress != nil {
		t.Fatalf("expected nil DCC after clear, got %v", *reloaded.DCCAddress)
	}
}

func TestClearDCCAddressesAdminCanClearOthers(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	userSvc := cmd.NewUser(bundle.Repo, bundle.Users, bundle.Vehicles, bundle.Trains, pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	admin, err := userSvc.Create(ctx, testAdminEff, cmd.UserCreateInput{
		Login: "admin2", PIN: "123456", Role: domain.RoleAdmin,
		DCCPool: []cmd.PoolRange{{From: 1, To: 50}},
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	owner, err := userSvc.Create(ctx, testAdminEff, cmd.UserCreateInput{
		Login: "owner", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []cmd.PoolRange{{From: 100, To: 199}},
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	addr := uint16(150)
	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID, Name: "Owned", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	cleared, err := svc.ClearDCCAddresses(ctx, admin.ID, adminEff, 0, []domain.VehicleID{v.ID})
	if err != nil {
		t.Fatalf("admin clear: %v", err)
	}
	if len(cleared) != 1 || cleared[0].DCCAddress != nil {
		t.Fatalf("unexpected clear result: %+v", cleared)
	}
}

func TestClearDCCAddressesRequiresIDs(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	if _, err := svc.ClearDCCAddresses(ctx, 1, driverEff, 0, nil); !errors.Is(err, svcerrors.ErrVehicleIDsRequired) {
		t.Fatalf("expected ErrVehicleIDsRequired, got %v", err)
	}
}

type fakeAddressBrake struct {
	mu    sync.Mutex
	stops map[uint][]uint16
}

func (f *fakeAddressBrake) StopAddresses(_ context.Context, layoutID uint, addrs []uint16) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stops == nil {
		f.stops = make(map[uint][]uint16)
	}
	copied := append([]uint16(nil), addrs...)
	f.stops[layoutID] = append(f.stops[layoutID], copied...)
	return nil
}

func (f *fakeAddressBrake) layouts() []uint {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uint, 0, len(f.stops))
	for k := range f.stops {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func insertLayout(t *testing.T, ctx context.Context, layouts *repo.Layouts, name string) domain.Layout {
	t.Helper()
	now := time.Now().UTC()
	layout := domain.Layout{
		Name:             name,
		IsSystem:         false,
		Locked:           false,
		CreatedBy:        0,
		AdminPINHash:     "x",
		RadioChatEnabled: true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := layouts.Insert(ctx, &layout); err != nil {
		t.Fatalf("insert layout %q: %v", name, err)
	}
	return layout
}

func TestClearDCCAddressesStopsEveryHostingLayout(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := cmd.NewDCCPool(bundle.Repo, bundle.Pool)
	userSvc := cmd.NewUser(bundle.Repo, bundle.Users, bundle.Vehicles, bundle.Trains, pool)
	svc := cmd.NewVehicle(bundle.Repo, bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.DccFunctions, bundle.Users)

	brake := &fakeAddressBrake{}
	svc.SetAddressBrake(brake)

	owner, err := userSvc.Create(ctx, testAdminEff, cmd.UserCreateInput{
		Login: "owner", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []cmd.PoolRange{{From: 100, To: 199}},
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	sessionLayout := insertLayout(t, ctx, bundle.Layouts, "session")
	otherLayout := insertLayout(t, ctx, bundle.Layouts, "other")

	addr := uint16(150)
	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	now := time.Now().UTC()
	for _, layoutID := range []uint{sessionLayout.ID, otherLayout.ID} {
		row := domain.LayoutVehicle{
			LayoutID:      layoutID,
			VehicleID:     v.ID,
			AddedByUserID: owner.ID,
			AddedAt:       now,
		}
		if err := bundle.LayoutVehicles.Insert(ctx, &row); err != nil {
			t.Fatalf("roster vehicle on layout %d: %v", layoutID, err)
		}
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	cleared, err := svc.ClearDCCAddresses(ctx, owner.ID, driverEff, sessionLayout.ID, []domain.VehicleID{v.ID})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(cleared) != 1 || cleared[0].DCCAddress != nil {
		t.Fatalf("unexpected clear result: %+v", cleared)
	}

	got := brake.layouts()
	want := []uint{sessionLayout.ID, otherLayout.ID}
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if len(got) != len(want) {
		t.Fatalf("stopped layouts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stopped layouts = %v, want %v", got, want)
		}
	}
	brake.mu.Lock()
	defer brake.mu.Unlock()
	for _, layoutID := range want {
		addrs := brake.stops[layoutID]
		if len(addrs) != 1 || addrs[0] != addr {
			t.Fatalf("layout %d addrs = %v, want [%d]", layoutID, addrs, addr)
		}
	}
}
