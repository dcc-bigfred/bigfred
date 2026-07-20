package main

import (
	"context"
	"net"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/redis/go-redis/v9"
)

func TestLoadProbeTargetsDedupePrefersPairedLogin(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	snap := contract.RemoteClientsSnapshotWire{
		LayoutID:         1,
		CommandStationID: 2,
		Clients: []contract.RemoteClientWire{
			{IP: "10.0.0.5", Protocol: "z21", UserLogin: "_", Paired: false},
			{IP: "10.0.0.5", Protocol: "withrottle", UserLogin: "alice", Paired: true},
			{IP: "10.0.0.6", Protocol: "z21", UserLogin: "bob", Paired: true},
		},
	}
	raw, err := contract.MarshalRemoteClientsSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(context.Background(), contract.RemoteClientsSnapshotKey(1, 2), raw, 0).Err(); err != nil {
		t.Fatal(err)
	}

	got, err := LoadProbeTargets(context.Background(), rdb)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("targets=%d want 2: %+v", len(got), got)
	}
	byIP := map[string]ProbeTarget{}
	for _, tgt := range got {
		byIP[tgt.IP.String()] = tgt
	}
	if byIP["10.0.0.5"].Login != "alice" {
		t.Fatalf("10.0.0.5 login=%q want alice", byIP["10.0.0.5"].Login)
	}
	if byIP["10.0.0.5"].Protocol != "withrottle" {
		t.Fatalf("protocol=%q", byIP["10.0.0.5"].Protocol)
	}
	if !byIP["10.0.0.6"].IP.Equal(net.ParseIP("10.0.0.6")) {
		t.Fatalf("missing 10.0.0.6")
	}
}
