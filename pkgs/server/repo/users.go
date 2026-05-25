package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// ErrUserNotFound is returned by Users.FindByLogin / FindByID when no
// matching row exists. Callers should treat it as a soft "not found"
// condition (e.g. AuthService converts it into a generic "invalid
// credentials" response so the caller cannot enumerate logins).
var ErrUserNotFound = errors.New("user not found")

// Users is the persistence adapter for domain.User. Construct it with
// NewUsers; the zero value is unusable.
type Users struct {
	repo rel.Repository
}

// NewUsers returns a Users repository bound to the given REL instance.
func NewUsers(r rel.Repository) *Users {
	return &Users{repo: r}
}

// Count returns the total number of rows in `users`. Used by the
// bootstrap seeder to decide whether to insert the default admin
// account on a freshly created database.
func (u *Users) Count(ctx context.Context) (int, error) {
	return u.repo.Count(ctx, "users")
}

// FindByLogin looks up a user by their case-sensitive login.
func (u *Users) FindByLogin(ctx context.Context, login string) (domain.User, error) {
	var user domain.User
	err := u.repo.Find(ctx, &user, where.Eq("login", login))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

// FindByID looks up a user by their primary key.
func (u *Users) FindByID(ctx context.Context, id uint) (domain.User, error) {
	var user domain.User
	err := u.repo.Find(ctx, &user, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

// ListAll returns every user, ordered by login. Used by the admin
// user-management screen — sorted ASCII so the UI never reshuffles
// rows between renders.
func (u *Users) ListAll(ctx context.Context) ([]domain.User, error) {
	var rows []domain.User
	err := u.repo.FindAll(ctx, &rows, sort.Asc("login"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new user. The caller is responsible for hashing
// PIN before populating PINHash; this layer never touches plaintext
// secrets.
func (u *Users) Insert(ctx context.Context, user *domain.User) error {
	return u.repo.Insert(ctx, user)
}

// Update writes an existing user back. The caller bumps UpdatedAt.
func (u *Users) Update(ctx context.Context, user *domain.User) error {
	return u.repo.Update(ctx, user)
}

// Delete removes a user row. Service-layer guards (no owned vehicles /
// trains, not self) run first; this method is the final write.
func (u *Users) Delete(ctx context.Context, user *domain.User) error {
	return u.repo.Delete(ctx, user)
}
