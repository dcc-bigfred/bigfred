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
	elevations   *repo.SudoElevations
	jwtSecret    []byte
	sessionTTL   time.Duration
	cookieDomain string
}

// NewAuth returns a ready-to-use Auth use-case handler.
func NewAuth(users *repo.Users, layouts *Layout, signalmen *repo.LayoutSignalmen, elevations *repo.SudoElevations, cfg AuthConfig) *Auth {
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

// Effective computes flat role membership for (user, layout).
func (a *Auth) Effective(ctx context.Context, user domain.User, layoutID uint) (domain.EffectiveRoles, error) {
	roles := []domain.Role{user.Role}
	now := time.Now().UTC()

	if hasGrant, err := a.signalmen.HasActiveGrant(ctx, layoutID, user.ID, now); err != nil {
		return domain.EffectiveRoles{}, err
	} else if hasGrant {
		roles = append(roles, domain.RoleSignalman)
	}
	if a.elevations != nil {
		if _, err := a.elevations.FindActive(ctx, user.ID, layoutID, now); err == nil {
			roles = append(roles, domain.RoleAdmin)
		} else if !errors.Is(err, repo.ErrSudoElevationNotFound) {
			return domain.EffectiveRoles{}, err
		}
	}
	return domain.NewEffectiveRoles(roles...), nil
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
	roles, err := a.Effective(ctx, user, layoutID)
	if err != nil {
		return "", err
	}
	if roles.Has(domain.RoleAdmin) {
		return domain.RoleAdmin, nil
	}
	if roles.Has(domain.RoleSignalman) {
		return domain.RoleSignalman, nil
	}
	return domain.RoleDriver, nil
}

// IsEffectiveSignalman reports whether the user may operate as a signalman.
func (a *Auth) IsEffectiveSignalman(ctx context.Context, user domain.User, layoutID uint) (bool, error) {
	roles, err := a.Effective(ctx, user, layoutID)
	if err != nil {
		return false, err
	}
	return roles.Has(domain.RoleSignalman) || roles.Has(domain.RoleAdmin), nil
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

func verifyPIN(pin, encoded string) error {
	if err := helpers.VerifyPIN(pin, encoded); err != nil {
		return svcerrors.ErrInvalidCredentials
	}
	return nil
}
