package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// layoutRosterPublisher pushes full roster snapshots to Redis for dcc-bus.
type layoutRosterPublisher interface {
	PublishLayoutAllowedVehicles(ctx context.Context, snap contract.AllowedVehicles) error
	PublishLayoutDefinedTrains(ctx context.Context, snap contract.DefinedTrains) error
}
