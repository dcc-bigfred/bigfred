package repo

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// SudoElevationStore persists short-lived layout-scoped sudo grants (§7a.7).
// Production uses Redis (ephemeral TTL); tests may use the SQLite adapter.
type SudoElevationStore interface {
	FindActive(ctx context.Context, userID, layoutID uint, now time.Time) (domain.SudoElevation, error)
	Upsert(ctx context.Context, row *domain.SudoElevation) error
	Delete(ctx context.Context, userID, layoutID uint) error
	// ReapExpired removes grants past their expiry. Ephemeral backends
	// may return (nil, nil) because TTL already dropped the keys.
	ReapExpired(ctx context.Context, now time.Time) ([]domain.SudoElevation, error)
	// RequiresJanitor reports whether a periodic reap loop is needed
	// to purge expired rows (SQLite) or emit missed expiry side-effects.
	RequiresJanitor() bool
}
