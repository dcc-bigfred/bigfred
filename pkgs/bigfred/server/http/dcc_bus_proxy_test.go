package httpapi

import (
	"errors"
	"testing"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func TestDccBusEnsureErrorCode(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{service.ErrCommandStationNotAttached, "command_station_not_attached"},
		{svcerrors.ErrNoDCCBusPortsAvailable, svcerrors.CodeNoDCCBusPortsAvailable},
		{service.ErrDccBusUnavailable, "dcc_bus_unavailable"},
		{errors.New("upsert failed"), "dcc_bus_unavailable"},
	}
	for _, tc := range cases {
		if got := dccBusEnsureErrorCode(tc.err); got != tc.want {
			t.Errorf("dccBusEnsureErrorCode(%v)=%q want %q", tc.err, got, tc.want)
		}
	}
}
