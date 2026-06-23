package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type cleanupStation struct {
	commandstation.StubStation
	cleanups int
}

func (s *cleanupStation) CleanUp() error {
	s.cleanups++
	return nil
}

func TestRouterShutdown_estopsAllRosterLocosAndCleansUp(t *testing.T) {
	t.Parallel()

	const (
		addrA uint16 = 3
		addrB uint16 = 7
	)
	var speedCalls int
	st := &cleanupStation{
		StubStation: commandstation.StubStation{
			SetSpeedFn: func(_ commandstation.LocoAddr, speed uint8, _ bool, _ uint8) error {
				if speed != 1 {
					t.Fatalf("estop SetSpeed step = %d, want 1", speed)
				}
				speedCalls++
				return nil
			},
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()

	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{
				{Addr: addrA},
				{Addr: addrB},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	r.Shutdown()
	if speedCalls != 4 {
		t.Fatalf("SetSpeed calls = %d, want 4 (2 boot stop + 2 shutdown)", speedCalls)
	}
	if st.cleanups != 1 {
		t.Fatalf("CleanUp calls = %d, want 1", st.cleanups)
	}

	speedCalls = 0
	st.cleanups = 0
	r.Shutdown()
	if speedCalls != 0 {
		t.Fatalf("second Shutdown SetSpeed calls = %d, want 0 (idempotent)", speedCalls)
	}
	if st.cleanups != 0 {
		t.Fatalf("second Shutdown CleanUp calls = %d, want 0 (idempotent)", st.cleanups)
	}
}
