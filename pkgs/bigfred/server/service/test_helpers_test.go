package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/repo/migrations"
)

var testAdminEff = domain.NewEffectiveRoles(domain.RoleAdmin)

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
