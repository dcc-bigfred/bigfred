package cmd_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

func newPairingStore(t *testing.T) (*remotepairing.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return remotepairing.NewStore(client), mr
}

func attachCS(t *testing.T, ctx context.Context, bundle repo.UsersBundle, layoutID, csID uint) {
	t.Helper()
	now := time.Now().UTC()
	if err := bundle.LayoutCommandStations.Attach(ctx, &domain.LayoutCommandStation{
		LayoutID:         layoutID,
		CommandStationID: csID,
		AddedAt:          now,
	}); err != nil {
		t.Fatalf("attach cs: %v", err)
	}
}

func freshRemoteEnv(t *testing.T) (context.Context, repo.UsersBundle, *remotepairing.Store, *miniredis.Miniredis) {
	t.Helper()
	bundle, cleanup := freshRepo(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	store, mr := newPairingStore(t)
	return ctx, bundle, store, mr
}

func insertRemoteCS(t *testing.T, ctx context.Context, stations *repo.CommandStations, name string, z21, wt bool) domain.CommandStation {
	t.Helper()
	now := time.Now().UTC()
	cs := domain.CommandStation{
		Name:                    name,
		Kind:                    domain.CommandStationKindLocoNetTCP,
		ConnectionURI:           "tcp://127.0.0.1:1235",
		SpeedSteps:              128,
		Z21ServerEnabled:        z21,
		WithrottleServerEnabled: wt,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := stations.Insert(ctx, &cs); err != nil {
		t.Fatalf("insert command station: %v", err)
	}
	return cs
}

func TestRemoteListClientsWorksWhenOnlyWithrottle(t *testing.T) {
	ctx, bundle, store, mr := freshRemoteEnv(t)
	cs := insertRemoteCS(t, ctx, bundle.CommandStations, "Main", false, true)
	const layoutID uint = 1
	attachCS(t, ctx, bundle, layoutID, cs.ID)

	snap := contract.RemoteClientsSnapshotWire{
		LayoutID:         layoutID,
		CommandStationID: cs.ID,
		Clients: []contract.RemoteClientWire{
			{
				ClientKey:   "withrottle:device1",
				Protocol:    contract.RemoteProtocolWithrottle,
				IP:          "10.0.0.2",
				Port:        12090,
				Paired:      true,
				LastSeenAt:  contract.NowMS(),
				ConnectedAt: contract.NowMS(),
			},
		},
	}
	raw, err := contract.MarshalRemoteClientsSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	mr.Set(contract.RemoteClientsSnapshotKey(layoutID, cs.ID), string(raw))

	remote := cmd.NewRemote(nil, cmd.NewWithrottleRemote(
		store, bundle.CommandStations, bundle.LayoutCommandStations, nil, nil,
	), store, bundle.Users)

	got, err := remote.ListClients(ctx, layoutID, cs.ID)
	if err != nil {
		t.Fatalf("ListClients: %v", err)
	}
	if len(got.Clients) != 1 || got.Clients[0].Protocol != contract.RemoteProtocolWithrottle {
		t.Fatalf("clients: %+v", got.Clients)
	}
}

func TestRemoteListClientsRejectedWhenNoServerEnabled(t *testing.T) {
	ctx, bundle, store, _ := freshRemoteEnv(t)
	cs := insertRemoteCS(t, ctx, bundle.CommandStations, "Off", false, false)
	const layoutID uint = 1
	attachCS(t, ctx, bundle, layoutID, cs.ID)

	remote := cmd.NewRemote(nil, cmd.NewWithrottleRemote(
		store, bundle.CommandStations, bundle.LayoutCommandStations, nil, nil,
	), store, nil)

	_, err := remote.ListClients(ctx, layoutID, cs.ID)
	if !errors.Is(err, svcerrors.ErrRemoteServerDisabled) {
		t.Fatalf("expected ErrRemoteServerDisabled, got %v", err)
	}
}

func TestRemoteUnpairDispatchesToWithrottle(t *testing.T) {
	ctx, bundle, store, _ := freshRemoteEnv(t)
	cs := insertRemoteCS(t, ctx, bundle.CommandStations, "Main", false, true)
	const layoutID uint = 1
	const userID uint = 7
	attachCS(t, ctx, bundle, layoutID, cs.ID)

	req, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         layoutID,
		CommandStationID: cs.ID,
		UserID:           userID,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "withrottle:HUtest"
	if _, ok, _, err := store.PairViaWithrottleCode(ctx, layoutID, cs.ID, req.PairingCode, clientKey, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}

	remote := cmd.NewRemote(nil, cmd.NewWithrottleRemote(
		store, bundle.CommandStations, bundle.LayoutCommandStations, nil, nil,
	), store, nil)

	if err := remote.Unpair(ctx, layoutID, cs.ID, userID, clientKey); err != nil {
		t.Fatalf("Unpair: %v", err)
	}
	_, ok, err := store.GetActiveByClientKey(ctx, layoutID, cs.ID, clientKey)
	if err != nil || ok {
		t.Fatalf("session should be gone: ok=%v err=%v", ok, err)
	}
}

func TestRemoteCancelPairingIsProtocolAgnostic(t *testing.T) {
	ctx, bundle, store, _ := freshRemoteEnv(t)
	cs := insertRemoteCS(t, ctx, bundle.CommandStations, "Main", false, true)
	const layoutID uint = 1
	const userID uint = 3
	attachCS(t, ctx, bundle, layoutID, cs.ID)

	if _, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         layoutID,
		CommandStationID: cs.ID,
		UserID:           userID,
		AllowAllVehicles: true,
	}); err != nil {
		t.Fatal(err)
	}

	remote := cmd.NewRemote(nil, cmd.NewWithrottleRemote(
		store, bundle.CommandStations, bundle.LayoutCommandStations, nil, nil,
	), store, nil)

	if err := remote.CancelPairing(ctx, layoutID, cs.ID, userID); err != nil {
		t.Fatalf("CancelPairing: %v", err)
	}
	_, ok, err := store.GetPendingByUser(ctx, layoutID, cs.ID, userID)
	if err != nil || ok {
		t.Fatalf("pending should be gone: ok=%v err=%v", ok, err)
	}
}
