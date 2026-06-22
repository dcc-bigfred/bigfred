package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

// UserWithDCCPool is a catalogue row enriched with the user's DCC pool.
type UserWithDCCPool struct {
	User    domain.User
	DCCPool []domain.DCCAddressRange
}

// UserCreateInput is the validated payload of User.Create.
type UserCreateInput struct {
	Login        string
	PIN          string
	Organization string
	Role         domain.Role
	DCCPool      []PoolRange
}

// UserUpdateInput is the validated payload of User.Update.
type UserUpdateInput struct {
	Login        *string
	Organization *string
	Role         *domain.Role
	PIN          *string
	DCCPool      *[]PoolRange
}

// User implements the admin-only user catalogue described in §4.1 / §7a.5.
type User struct {
	users    *repo.Users
	vehicles *repo.Vehicles
	trains   *repo.Trains
	dccPool  DCCPoolManagerPort
	sec      security.UserSecurityContext
}

// NewUser constructs a User use-case handler.
func NewUser(users *repo.Users, vehicles *repo.Vehicles, trains *repo.Trains, dccPool DCCPoolManagerPort) *User {
	return &User{users: users, vehicles: vehicles, trains: trains, dccPool: dccPool}
}

// List returns every user in the catalogue.
func (u *User) List(ctx context.Context) ([]domain.User, error) {
	return u.users.ListAll(ctx)
}

// ListWithDCCPools returns every user together with their DCC pool rows.
func (u *User) ListWithDCCPools(ctx context.Context, eff domain.EffectiveRoles) ([]UserWithDCCPool, error) {
	if err := u.checkManageUsers(eff); err != nil {
		return nil, err
	}
	users, err := u.users.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	allRanges, err := u.dccPool.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint][]domain.DCCAddressRange, len(users))
	for _, r := range allRanges {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	out := make([]UserWithDCCPool, 0, len(users))
	for _, user := range users {
		pool := byUser[user.ID]
		if pool == nil {
			pool = []domain.DCCAddressRange{}
		}
		out = append(out, UserWithDCCPool{User: user, DCCPool: pool})
	}
	return out, nil
}

// Get loads a single user by primary key.
func (u *User) Get(ctx context.Context, id uint) (domain.User, error) {
	row, err := u.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return domain.User{}, svcerrors.ErrUserNotFound
		}
		return domain.User{}, err
	}
	return row, nil
}

// Create inserts a brand-new user.
func (u *User) Create(ctx context.Context, eff domain.EffectiveRoles, in UserCreateInput) (domain.User, error) {
	if err := u.checkManageUsers(eff); err != nil {
		return domain.User{}, err
	}
	login, err := validation.SanitiseLogin(in.Login)
	if err != nil {
		return domain.User{}, err
	}
	if err := validation.ValidateUserPIN(in.PIN); err != nil {
		return domain.User{}, err
	}
	if !validation.IsPermanentRole(in.Role) {
		return domain.User{}, svcerrors.ErrUserRoleInvalid
	}

	if _, err := u.users.FindByLogin(ctx, login); err == nil {
		return domain.User{}, svcerrors.ErrUserLoginTaken
	} else if !errors.Is(err, repo.ErrUserNotFound) {
		return domain.User{}, err
	}
	if err := u.dccPool.Validate(ctx, 0, in.DCCPool); err != nil {
		return domain.User{}, err
	}

	hash, err := helpers.HashPIN(in.PIN)
	if err != nil {
		return domain.User{}, err
	}

	now := time.Now().UTC()
	row := domain.User{
		Login:        login,
		PINHash:      hash,
		Organization: validation.SanitiseOrganization(in.Organization),
		Role:         in.Role,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := u.users.Insert(ctx, &row); err != nil {
		return domain.User{}, err
	}
	if _, err := u.dccPool.Replace(ctx, eff, row.ID, in.DCCPool); err != nil {
		return domain.User{}, err
	}
	return row, nil
}

// Update mutates an existing user in place.
func (u *User) Update(ctx context.Context, eff domain.EffectiveRoles, id uint, in UserUpdateInput) (domain.User, error) {
	if err := u.checkManageUsers(eff); err != nil {
		return domain.User{}, err
	}
	row, err := u.Get(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	if in.Login != nil {
		login, err := validation.SanitiseLogin(*in.Login)
		if err != nil {
			return domain.User{}, err
		}
		if login != row.Login {
			if other, err := u.users.FindByLogin(ctx, login); err == nil {
				if other.ID != row.ID {
					return domain.User{}, svcerrors.ErrUserLoginTaken
				}
			} else if !errors.Is(err, repo.ErrUserNotFound) {
				return domain.User{}, err
			}
			row.Login = login
		}
	}
	if in.Role != nil {
		if !validation.IsPermanentRole(*in.Role) {
			return domain.User{}, svcerrors.ErrUserRoleInvalid
		}
		row.Role = *in.Role
	}
	if in.Organization != nil {
		row.Organization = validation.SanitiseOrganization(*in.Organization)
	}
	if in.PIN != nil {
		if err := validation.ValidateUserPIN(*in.PIN); err != nil {
			return domain.User{}, err
		}
		hash, err := helpers.HashPIN(*in.PIN)
		if err != nil {
			return domain.User{}, err
		}
		row.PINHash = hash
	}
	if in.DCCPool != nil {
		if _, err := u.dccPool.Replace(ctx, eff, row.ID, *in.DCCPool); err != nil {
			return domain.User{}, err
		}
	}

	row.UpdatedAt = time.Now().UTC()
	if err := u.users.Update(ctx, &row); err != nil {
		return domain.User{}, err
	}
	return row, nil
}

// GetDCCPool loads the DCC address ranges assigned to a user.
func (u *User) GetDCCPool(ctx context.Context, userID uint) ([]domain.DCCAddressRange, error) {
	return u.dccPool.List(ctx, userID)
}

// SetActive flips the user's Active flag.
func (u *User) SetActive(ctx context.Context, eff domain.EffectiveRoles, actorID, id uint, active bool) (domain.User, error) {
	if err := u.checkManageUsers(eff); err != nil {
		return domain.User{}, err
	}
	row, err := u.Get(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if !active {
		actor, err := u.Get(ctx, actorID)
		if err != nil {
			return domain.User{}, err
		}
		if d := u.sec.CanDeactivateSelf(actor, row); !d.Allowed {
			return domain.User{}, svcerrors.ErrCannotDeactivateSelf
		}
	}
	if row.Active == active {
		return row, nil
	}
	row.Active = active
	row.UpdatedAt = time.Now().UTC()
	if err := u.users.Update(ctx, &row); err != nil {
		return domain.User{}, err
	}
	return row, nil
}

// Delete removes the user.
func (u *User) Delete(ctx context.Context, eff domain.EffectiveRoles, actorID, id uint) error {
	if err := u.checkManageUsers(eff); err != nil {
		return err
	}
	row, err := u.Get(ctx, id)
	if err != nil {
		return err
	}
	actor, err := u.Get(ctx, actorID)
	if err != nil {
		return err
	}
	if d := u.sec.CanDeleteSelf(actor, row); !d.Allowed {
		return svcerrors.ErrCannotDeleteSelf
	}
	nv, err := u.vehicles.CountByOwner(ctx, row.ID)
	if err != nil {
		return err
	}
	if nv > 0 {
		return svcerrors.ErrUserHasVehicles
	}
	nt, err := u.trains.CountByOwner(ctx, row.ID)
	if err != nil {
		return err
	}
	if nt > 0 {
		return svcerrors.ErrUserHasTrains
	}
	if err := u.dccPool.DeleteForUser(ctx, eff, row.ID); err != nil {
		return err
	}
	return u.users.Delete(ctx, &row)
}

func (u *User) checkManageUsers(eff domain.EffectiveRoles) error {
	decision := u.sec.CanManageUsers(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonForbidden:
		return svcerrors.ErrUserForbidden
	default:
		return errors.New(decision.Reason)
	}
}
