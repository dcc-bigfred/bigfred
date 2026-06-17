package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

var ErrTakeoverRequestNotFound = errors.New("takeover_request_not_found")

// TakeoverRequests persists takeover state machine rows.
type TakeoverRequests struct {
	repo rel.Repository
}

// NewTakeoverRequests returns a TakeoverRequests repository.
func NewTakeoverRequests(r rel.Repository) *TakeoverRequests {
	return &TakeoverRequests{repo: r}
}

var _ TakeoverRequestStore = (*TakeoverRequests)(nil)

// RequiresJanitor implements TakeoverRequestStore.
func (t *TakeoverRequests) RequiresJanitor() bool { return true }

func (t *TakeoverRequests) Insert(ctx context.Context, row *domain.TakeoverRequest) error {
	return t.repo.Insert(ctx, row)
}

func (t *TakeoverRequests) Update(ctx context.Context, row *domain.TakeoverRequest) error {
	return t.repo.Update(ctx, row)
}

func (t *TakeoverRequests) FindByID(ctx context.Context, id uint) (domain.TakeoverRequest, error) {
	var row domain.TakeoverRequest
	if err := t.repo.Find(ctx, &row, where.Eq("id", id)); err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.TakeoverRequest{}, ErrTakeoverRequestNotFound
		}
		return domain.TakeoverRequest{}, err
	}
	return row, nil
}

// ListPending returns every request still awaiting driver action or
// auto-grant. Used on server restart to reschedule timers.
func (t *TakeoverRequests) ListPending(ctx context.Context) ([]domain.TakeoverRequest, error) {
	var rows []domain.TakeoverRequest
	if err := t.repo.FindAll(ctx, &rows, where.Eq("state", domain.TakeoverStatePending)); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListGrantedBySignalman returns active granted rows for one signalman.
func (t *TakeoverRequests) ListGrantedBySignalman(ctx context.Context, signalmanID uint) ([]domain.TakeoverRequest, error) {
	var rows []domain.TakeoverRequest
	if err := t.repo.FindAll(ctx, &rows,
		where.Eq("signalman_user_id", signalmanID),
		where.Eq("state", domain.TakeoverStateGranted),
	); err != nil {
	 return nil, err
	}
	return rows, nil
}

// FindPendingForTarget returns a pending request for the same target, if any.
func (t *TakeoverRequests) FindPendingForTarget(
	ctx context.Context,
	target domain.TakeoverTarget,
	targetID uint,
) (domain.TakeoverRequest, error) {
	var row domain.TakeoverRequest
	if err := t.repo.Find(ctx, &row,
		where.Eq("target", string(target)),
		where.Eq("target_id", targetID),
		where.Eq("state", domain.TakeoverStatePending),
	); err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.TakeoverRequest{}, ErrTakeoverRequestNotFound
		}
		return domain.TakeoverRequest{}, err
	}
	return row, nil
}

// ListGrantedExpired returns granted rows whose lease should have ended.
// Caller checks lease expiry separately; this lists all granted for janitor.
func (t *TakeoverRequests) ListGranted(ctx context.Context) ([]domain.TakeoverRequest, error) {
	var rows []domain.TakeoverRequest
	if err := t.repo.FindAll(ctx, &rows, where.Eq("state", domain.TakeoverStateGranted)); err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkDecision sets terminal metadata on a row at `now`.
func MarkTakeoverDecision(row *domain.TakeoverRequest, state domain.TakeoverState, now time.Time) {
	row.State = state
	row.DecisionAt = &now
}
