package service

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
)

// SeedConfig describes the bootstrap admin account that is inserted
// into a freshly-created database. The seeder is a no-op once any
// user exists, so editing these values after the first run has no
// effect on existing installations.
type SeedConfig struct {
	Login string
	PIN   string
}

// SeedDefaults is the well-known bootstrap account. The PIN is
// intentionally short and printable so the first login on a new
// install can be performed from a phone. Users SHOULD change it
// immediately after first login (a "change PIN" endpoint is part of
// M2 follow-up work).
var SeedDefaults = SeedConfig{
	Login: "admin",
	PIN:   "123456",
}

// SeedAdmin inserts the default admin account if and only if the
// users table is empty. It returns true when a seed actually
// happened, so the caller can log a one-shot warning that the
// default PIN is in effect.
func SeedAdmin(ctx context.Context, users *repo.Users, cfg SeedConfig) (bool, error) {
	n, err := users.Count(ctx)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}

	hash, err := HashPIN(cfg.PIN)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC()
	user := domain.User{
		Login:     cfg.Login,
		PINHash:   hash,
		Role:      domain.RoleAdmin,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := users.Insert(ctx, &user); err != nil {
		return false, err
	}
	return true, nil
}
