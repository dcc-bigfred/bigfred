package repo

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// TakeoverRequestStore persists active takeover state machine rows.
type TakeoverRequestStore interface {
	Insert(ctx context.Context, row *domain.TakeoverRequest) error
	Update(ctx context.Context, row *domain.TakeoverRequest) error
	FindByID(ctx context.Context, id uint) (domain.TakeoverRequest, error)
	ListPending(ctx context.Context) ([]domain.TakeoverRequest, error)
	ListGrantedBySignalman(ctx context.Context, signalmanID uint) ([]domain.TakeoverRequest, error)
	FindPendingForTarget(ctx context.Context, target domain.TakeoverTarget, targetID string) (domain.TakeoverRequest, error)
	ListGranted(ctx context.Context) ([]domain.TakeoverRequest, error)
	RequiresJanitor() bool
}
