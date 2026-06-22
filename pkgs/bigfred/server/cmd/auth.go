package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

// Identity carries the authenticated caller through HTTP and WS handlers.
type Identity struct {
	User   domain.User
	Layout domain.Layout
}

// HasRole returns true when the user's permanent role matches any supplied role.
func (i Identity) HasRole(roles ...domain.Role) bool {
	for _, r := range roles {
		if i.User.Role == r {
			return true
		}
	}
	return false
}

// AuthConfig groups the few knobs the auth use-case exposes.
type AuthConfig struct {
	JWTSecret  []byte
	SessionTTL time.Duration
}

// Auth implements login/logout/me support described in §7a.1.
type Auth struct {
	users        *repo.Users
	layouts      *Layout
	signalmen    *repo.LayoutSignalmen
	elevations   repo.SudoElevationStore
	jwtSecret    []byte
	sessionTTL   time.Duration
	cookieDomain string
}

// NewAuth returns a ready-to-use Auth use-case handler.
func NewAuth(users *repo.Users, layouts *Layout, signalmen *repo.LayoutSignalmen, elevations repo.SudoElevationStore, cfg AuthConfig) *Auth {
	if len(cfg.JWTSecret) == 0 {
		panic("cmd.NewAuth: JWTSecret must not be empty")
	}
	if layouts == nil {
		panic("cmd.NewAuth: Layout must not be nil")
	}
	if signalmen == nil {
		panic("cmd.NewAuth: LayoutSignalmen must not be nil")
	}
	ttl := cfg.SessionTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	return &Auth{
		users:      users,
		layouts:    layouts,
		signalmen:  signalmen,
		elevations: elevations,
		jwtSecret:  cfg.JWTSecret,
		sessionTTL: ttl,
	}
}

// SessionTTL exposes the configured session lifetime.
func (a *Auth) SessionTTL() time.Duration { return a.sessionTTL }

// Login exchanges a (login, pin, layoutId) triplet for an Identity.
func (a *Auth) Login(ctx context.Context, login, pin string, layoutID uint) (Identity, error) {
	user, err := a.users.FindByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			_ = verifyPIN(pin, "$argon2id$v=19$m=65536,t=2,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
			return Identity{}, svcerrors.ErrInvalidCredentials
		}
		return Identity{}, err
	}

	if err := verifyPIN(pin, user.PINHash); err != nil {
		return Identity{}, svcerrors.ErrInvalidCredentials
	}
	if !user.Active {
		return Identity{}, svcerrors.ErrAccountDeactivated
	}

	layout, err := a.layouts.ValidateForLogin(ctx, layoutID)
	if err != nil {
		return Identity{}, err
	}
	return Identity{User: user, Layout: layout}, nil
}

// EffectiveSnapshot is the outcome of one layout-scoped role resolution
// pass. Sudo is non-nil when an active sudo_elevations row contributed
// admin authority (permanent admins do not populate it).
type EffectiveSnapshot struct {
	Roles domain.EffectiveRoles
	Sudo  *domain.SudoElevation
}

// DisplayRole returns the single role label shown on the dashboard.
func (s EffectiveSnapshot) DisplayRole() domain.Role {
	if s.Roles.Has(domain.RoleAdmin) {
		return domain.RoleAdmin
	}
	if s.Roles.Has(domain.RoleSignalman) {
		return domain.RoleSignalman
	}
	return domain.RoleDriver
}

// IsSignalman reports whether the user may operate as a signalman.
func (s EffectiveSnapshot) IsSignalman() bool {
	return s.Roles.Has(domain.RoleSignalman) || s.Roles.Has(domain.RoleAdmin)
}

// Effective computes flat role membership for (user, layout).
func (a *Auth) Effective(ctx context.Context, user domain.User, layoutID uint) (domain.EffectiveRoles, error) {
	snap, err := a.resolveEffective(ctx, user, layoutID)
	if err != nil {
		return domain.EffectiveRoles{}, err
	}
	return snap.Roles, nil
}

// EffectiveSnapshot resolves roles and the active sudo grant (if any)
// in one repository pass. Use it when the caller needs both effective
// membership and sudo timestamps (e.g. GET /auth/me).
func (a *Auth) EffectiveSnapshot(ctx context.Context, user domain.User, layoutID uint) (EffectiveSnapshot, error) {
	return a.resolveEffective(ctx, user, layoutID)
}

func (a *Auth) resolveEffective(ctx context.Context, user domain.User, layoutID uint) (EffectiveSnapshot, error) {
	roles := []domain.Role{user.Role}
	now := time.Now().UTC()
	var sudo *domain.SudoElevation

	if hasGrant, err := a.signalmen.HasActiveGrant(ctx, layoutID, user.ID, now); err != nil {
		return EffectiveSnapshot{}, err
	} else if hasGrant {
		roles = append(roles, domain.RoleSignalman)
	}
	if a.elevations != nil {
		row, err := a.elevations.FindActive(ctx, user.ID, layoutID, now)
		if err == nil {
			roles = append(roles, domain.RoleAdmin)
			sudo = &row
		} else if !errors.Is(err, repo.ErrSudoElevationNotFound) {
			return EffectiveSnapshot{}, err
		}
	}
	return EffectiveSnapshot{
		Roles: domain.NewEffectiveRoles(roles...),
		Sudo:  sudo,
	}, nil
}

// EffectiveForUserID loads the user and computes layout-scoped roles.
func (a *Auth) EffectiveForUserID(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error) {
	user, err := a.users.FindByID(ctx, userID)
	if err != nil {
		return domain.EffectiveRoles{}, err
	}
	return a.Effective(ctx, user, layoutID)
}

// EffectiveDisplayRole returns the single role label shown on the dashboard.
func (a *Auth) EffectiveDisplayRole(ctx context.Context, user domain.User, layoutID uint) (domain.Role, error) {
	snap, err := a.resolveEffective(ctx, user, layoutID)
	if err != nil {
		return "", err
	}
	return snap.DisplayRole(), nil
}

// IsEffectiveSignalman reports whether the user may operate as a signalman.
func (a *Auth) IsEffectiveSignalman(ctx context.Context, user domain.User, layoutID uint) (bool, error) {
	snap, err := a.resolveEffective(ctx, user, layoutID)
	if err != nil {
		return false, err
	}
	return snap.IsSignalman(), nil
}

type sessionClaims struct {
	UID  uint   `json:"uid"`
	Role string `json:"role"`
	LID  uint   `json:"lid"`
	jwt.RegisteredClaims
}

// IssueToken signs a session JWT for the given identity.
func (a *Auth) IssueToken(id Identity) (string, time.Time, error) {
	expiry := time.Now().Add(a.sessionTTL)
	claims := sessionClaims{
		UID:  id.User.ID,
		Role: string(id.User.Role),
		LID:  id.Layout.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.User.Login,
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiry, nil
}

// VerifyToken parses and verifies a previously issued session token.
func (a *Auth) VerifyToken(ctx context.Context, raw string) (Identity, error) {
	parsed, err := jwt.ParseWithClaims(raw, &sessionClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return Identity{}, svcerrors.ErrInvalidCredentials
	}
	claims, ok := parsed.Claims.(*sessionClaims)
	if !ok {
		return Identity{}, svcerrors.ErrInvalidCredentials
	}

	user, err := a.users.FindByID(ctx, claims.UID)
	if err != nil {
		return Identity{}, svcerrors.ErrInvalidCredentials
	}

	var layout domain.Layout
	if claims.LID == 0 {
		layout, err = a.layouts.GetSystem(ctx)
	} else {
		layout, err = a.layouts.Get(ctx, claims.LID)
	}
	if err != nil {
		return Identity{}, svcerrors.ErrInvalidCredentials
	}

	return Identity{User: user, Layout: layout}, nil
}

// ChangePIN rotates the caller's password after verifying the current one.
func (a *Auth) ChangePIN(ctx context.Context, userID uint, currentPIN, newPIN string) error {
	user, err := a.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return svcerrors.ErrUserNotFound
		}
		return err
	}
	if err := verifyPIN(currentPIN, user.PINHash); err != nil {
		return svcerrors.ErrInvalidCredentials
	}
	if err := validation.ValidateUserPIN(newPIN); err != nil {
		return err
	}
	hash, err := helpers.HashPIN(newPIN)
	if err != nil {
		return err
	}
	user.PINHash = hash
	user.UpdatedAt = time.Now().UTC()
	return a.users.Update(ctx, &user)
}

// UpdateProfile updates the caller's self-service profile fields.
func (a *Auth) UpdateProfile(ctx context.Context, userID uint, organization string) (domain.User, error) {
	user, err := a.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return domain.User{}, svcerrors.ErrUserNotFound
		}
		return domain.User{}, err
	}
	user.Organization = validation.SanitiseOrganization(organization)
	user.UpdatedAt = time.Now().UTC()
	if err := a.users.Update(ctx, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func verifyPIN(pin, encoded string) error {
	if err := helpers.VerifyPIN(pin, encoded); err != nil {
		return svcerrors.ErrInvalidCredentials
	}
	return nil
}
