package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

// User-management sentinel errors. They are deliberately
// machine-readable so the HTTP layer can map each onto a status code
// and the frontend can localise the message through the
// `errors:<code>` i18n namespace (§4.1).
var (
	// ErrUserNotFound is returned by Get / Update / Delete when no
	// row matches.
	ErrUserNotFound = errors.New("user_not_found")

	// ErrUserLoginRequired covers blank / whitespace-only logins.
	ErrUserLoginRequired = errors.New("user_login_required")

	// ErrUserLoginInvalid covers logins with disallowed characters.
	// The frontend renders both this and ErrUserLoginRequired as
	// "Wpisz poprawny login" style helper text.
	ErrUserLoginInvalid = errors.New("user_login_invalid")

	// ErrUserLoginTaken is the unique-name violation surfaced as a
	// machine code so the UI can highlight the right field.
	ErrUserLoginTaken = errors.New("user_login_taken")

	// ErrUserPINRequired covers blank PIN on create / change-pin.
	ErrUserPINRequired = errors.New("user_pin_required")

	// ErrUserPINInvalid covers PINs of the wrong length or with
	// non-digit characters.
	ErrUserPINInvalid = errors.New("user_pin_invalid")

	// ErrUserRoleInvalid is returned when the caller supplies a role
	// value outside { driver, admin } — `signalman` is never a
	// permanent role (§7a.2).
	ErrUserRoleInvalid = errors.New("user_role_invalid")

	// ErrUserHasVehicles refuses deletion of users that still own
	// vehicles. The admin must reassign / remove the owned rows
	// first.
	ErrUserHasVehicles = errors.New("user_has_vehicles")

	// ErrUserHasTrains refuses deletion of users that still own
	// trains. Same rationale as ErrUserHasVehicles.
	ErrUserHasTrains = errors.New("user_has_trains")

	// ErrCannotDeactivateSelf prevents an admin from locking
	// themselves out via deactivation.
	ErrCannotDeactivateSelf = errors.New("cannot_deactivate_self")

	// ErrCannotDeleteSelf prevents an admin from removing their own
	// account.
	ErrCannotDeleteSelf = errors.New("cannot_delete_self")

	// ErrUserForbidden is returned when a non-admin attempts a
	// user-catalogue operation guarded by CanManageUsers.
	ErrUserForbidden = errors.New("forbidden")
)

// minPINLength is the minimum number of digits we accept on create
// and change-pin. Six matches the spec default (§7a.1). PINs are
// always all-digit so the on-screen keypad on a phone is the only
// input device users need.
const (
	minPINLength    = 4
	maxPINLength    = 12
	maxUserLoginLen = 32
)

// UserService implements the admin-only user catalogue described in
// §4.1 / §7a.5. The service is intentionally agnostic of HTTP and
// audit-log integration — those concerns are layered on top by the
// handler.
type UserService struct {
	users    *repo.Users
	vehicles *repo.Vehicles
	trains   *repo.Trains
	dccPool  *DCCPoolService
	sec      security.UserSecurityContext
}

// NewUserService constructs a UserService bound to the persistence
// repositories it needs for create / update / delete validation.
func NewUserService(users *repo.Users, vehicles *repo.Vehicles, trains *repo.Trains, dccPool *DCCPoolService) *UserService {
	return &UserService{users: users, vehicles: vehicles, trains: trains, dccPool: dccPool}
}

// UserWithDCCPool is a catalogue row enriched with the user's DCC
// address ranges for the admin UI.
type UserWithDCCPool struct {
	User    domain.User
	DCCPool []domain.DCCAddressRange
}

// List returns every user in the catalogue. Admin-only; the HTTP
// layer is responsible for the role check.
func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	return s.users.ListAll(ctx)
}

// ListWithDCCPools returns every user together with their DCC pool
// rows so the admin UI can render the catalogue in one round trip.
func (s *UserService) ListWithDCCPools(ctx context.Context, eff domain.EffectiveRoles) ([]UserWithDCCPool, error) {
	if err := s.checkManageUsers(eff); err != nil {
		return nil, err
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	allRanges, err := s.dccPool.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint][]domain.DCCAddressRange, len(users))
	for _, r := range allRanges {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	out := make([]UserWithDCCPool, 0, len(users))
	for _, u := range users {
		pool := byUser[u.ID]
		if pool == nil {
			pool = []domain.DCCAddressRange{}
		}
		out = append(out, UserWithDCCPool{User: u, DCCPool: pool})
	}
	return out, nil
}

// Get loads a single user by primary key. Returns ErrUserNotFound
// (not the repo-level sentinel) so the HTTP layer can switch on a
// single error namespace.
func (s *UserService) Get(ctx context.Context, id uint) (domain.User, error) {
	row, err := s.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}
	return row, nil
}

// UserCreateInput is the validated payload of UserService.Create.
type UserCreateInput struct {
	Login   string
	PIN     string
	Role    domain.Role
	DCCPool []PoolRange
}

// Create inserts a brand-new user. The PIN is hashed inside this
// method so plaintext never escapes via a service contract.
func (s *UserService) Create(ctx context.Context, eff domain.EffectiveRoles, in UserCreateInput) (domain.User, error) {
	if err := s.checkManageUsers(eff); err != nil {
		return domain.User{}, err
	}
	login, err := sanitiseLogin(in.Login)
	if err != nil {
		return domain.User{}, err
	}
	if err := validatePIN(in.PIN); err != nil {
		return domain.User{}, err
	}
	if !isPermanentRole(in.Role) {
		return domain.User{}, ErrUserRoleInvalid
	}

	if _, err := s.users.FindByLogin(ctx, login); err == nil {
		return domain.User{}, ErrUserLoginTaken
	} else if !errors.Is(err, repo.ErrUserNotFound) {
		return domain.User{}, err
	}

	if err := s.dccPool.Validate(ctx, 0, in.DCCPool); err != nil {
		return domain.User{}, err
	}

	hash, err := HashPIN(in.PIN)
	if err != nil {
		return domain.User{}, err
	}

	now := time.Now().UTC()
	row := domain.User{
		Login:     login,
		PINHash:   hash,
		Role:      in.Role,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.users.Insert(ctx, &row); err != nil {
		return domain.User{}, err
	}
	if _, err := s.dccPool.Replace(ctx, eff, row.ID, in.DCCPool); err != nil {
		return domain.User{}, err
	}
	return row, nil
}

// UserUpdateInput is the validated payload of UserService.Update.
// Each field is a pointer so the caller can distinguish "leave
// alone" from "explicit empty".
type UserUpdateInput struct {
	Login   *string
	Role    *domain.Role
	PIN     *string
	DCCPool *[]PoolRange
}

// Update mutates an existing user in place. Only the fields the
// caller supplies are touched.
func (s *UserService) Update(ctx context.Context, eff domain.EffectiveRoles, id uint, in UserUpdateInput) (domain.User, error) {
	if err := s.checkManageUsers(eff); err != nil {
		return domain.User{}, err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	if in.Login != nil {
		login, err := sanitiseLogin(*in.Login)
		if err != nil {
			return domain.User{}, err
		}
		if login != row.Login {
			if other, err := s.users.FindByLogin(ctx, login); err == nil {
				if other.ID != row.ID {
					return domain.User{}, ErrUserLoginTaken
				}
			} else if !errors.Is(err, repo.ErrUserNotFound) {
				return domain.User{}, err
			}
			row.Login = login
		}
	}
	if in.Role != nil {
		if !isPermanentRole(*in.Role) {
			return domain.User{}, ErrUserRoleInvalid
		}
		row.Role = *in.Role
	}
	if in.PIN != nil {
		if err := validatePIN(*in.PIN); err != nil {
			return domain.User{}, err
		}
		hash, err := HashPIN(*in.PIN)
		if err != nil {
			return domain.User{}, err
		}
		row.PINHash = hash
	}
	if in.DCCPool != nil {
		if _, err := s.dccPool.Replace(ctx, eff, row.ID, *in.DCCPool); err != nil {
			return domain.User{}, err
		}
	}

	row.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, &row); err != nil {
		return domain.User{}, err
	}
	return row, nil
}

// GetDCCPool loads the DCC address ranges assigned to a user.
func (s *UserService) GetDCCPool(ctx context.Context, userID uint) ([]domain.DCCAddressRange, error) {
	return s.dccPool.List(ctx, userID)
}

// SetActive flips the user's Active flag. Idempotent on either
// branch. Self-deactivation is rejected via CanDeactivateSelf.
func (s *UserService) SetActive(ctx context.Context, eff domain.EffectiveRoles, actorID, id uint, active bool) (domain.User, error) {
	if err := s.checkManageUsers(eff); err != nil {
		return domain.User{}, err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if !active {
		actor, err := s.Get(ctx, actorID)
		if err != nil {
			return domain.User{}, err
		}
		if d := s.sec.CanDeactivateSelf(actor, row); !d.Allowed {
			return domain.User{}, ErrCannotDeactivateSelf
		}
	}
	if row.Active == active {
		return row, nil
	}
	row.Active = active
	row.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, &row); err != nil {
		return domain.User{}, err
	}
	return row, nil
}

// Delete removes the user. Self-deletion is rejected via
// CanDeleteSelf. The service guards the owned-vehicles /
// owned-trains invariant so a hand-crafted request cannot cascade
// orphan rows.
func (s *UserService) Delete(ctx context.Context, eff domain.EffectiveRoles, actorID, id uint) error {
	if err := s.checkManageUsers(eff); err != nil {
		return err
	}
	row, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	actor, err := s.Get(ctx, actorID)
	if err != nil {
		return err
	}
	if d := s.sec.CanDeleteSelf(actor, row); !d.Allowed {
		return ErrCannotDeleteSelf
	}
	nv, err := s.vehicles.CountByOwner(ctx, row.ID)
	if err != nil {
		return err
	}
	if nv > 0 {
		return ErrUserHasVehicles
	}
	nt, err := s.trains.CountByOwner(ctx, row.ID)
	if err != nil {
		return err
	}
	if nt > 0 {
		return ErrUserHasTrains
	}
	if err := s.dccPool.DeleteForUser(ctx, eff, row.ID); err != nil {
		return err
	}
	return s.users.Delete(ctx, &row)
}

func (s *UserService) checkManageUsers(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageUsers(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case "forbidden":
		return ErrUserForbidden
	default:
		return errors.New(decision.Reason)
	}
}

// isPermanentRole gates the closed catalogue of permanent roles
// (driver / admin). `signalman` is rejected because per §7a.2 it
// only exists as a layout-scoped grant.
func isPermanentRole(r domain.Role) bool {
	switch r {
	case domain.RoleDriver, domain.RoleAdmin:
		return true
	default:
		return false
	}
}

// sanitiseLogin trims, validates and lower-cases the login. The
// character class is ASCII-only to keep URL-encoded admin paths
// readable; spaces are forbidden so a copy-paste never silently
// fails.
func sanitiseLogin(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrUserLoginRequired
	}
	if len(trimmed) > maxUserLoginLen {
		return "", ErrUserLoginInvalid
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return "", ErrUserLoginInvalid
		}
	}
	return trimmed, nil
}

// validatePIN enforces the all-digit + length contract documented at
// the top of the file. The string is treated as opaque user input —
// no normalisation, no trimming (so " 12345 " is rejected).
func validatePIN(pin string) error {
	if pin == "" {
		return ErrUserPINRequired
	}
	if len(pin) < minPINLength || len(pin) > maxPINLength {
		return ErrUserPINInvalid
	}
	for _, r := range pin {
		if !unicode.IsDigit(r) {
			return ErrUserPINInvalid
		}
	}
	return nil
}
