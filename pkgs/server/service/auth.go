// Package service is the business-logic layer that sits between the
// HTTP / WebSocket transport and the persistence layer (REL
// repositories). Services know nothing about HTTP — they receive
// plain domain inputs and emit plain domain outputs / errors.
package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
)

// ErrInvalidCredentials is returned for any failed login attempt. The
// same error is returned for "unknown login" and "wrong PIN" on purpose
// — it prevents enumeration of valid logins.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrAccountDeactivated is returned by Login when the credentials are
// valid but the user account has been soft-disabled by an admin. The
// error is distinct from ErrInvalidCredentials so the UI can render
// a precise message — there is no enumeration risk because reaching
// this branch already requires a valid (login, pin) pair.
var ErrAccountDeactivated = errors.New("account_deactivated")

// argon2idParams are the OWASP-recommended baseline (RFC 9106) tuned
// for an interactive login flow (<100 ms on a modern desktop CPU). The
// numbers are encoded inside the stored hash, so they can be bumped
// for newly-issued credentials without invalidating existing rows.
var argon2idParams = struct {
	time, memory uint32
	parallelism  uint8
	saltLen, key uint32
}{
	time:        2,
	memory:      64 * 1024,
	parallelism: 2,
	saltLen:     16,
	key:         32,
}

// Identity carries the authenticated caller through HTTP and WS
// handlers. It is intentionally minimal at this milestone — effective
// roles will be added together with LayoutSignalman (§7a.2). For
// now it carries the User + the Layout the user picked on the login
// form (§7a.1).
type Identity struct {
	User   domain.User
	Layout domain.Layout
}

// HasRole returns true when the user's permanent role matches any of
// the supplied options. This is a convenience helper used by the
// RequireRole middleware.
func (i Identity) HasRole(roles ...domain.Role) bool {
	for _, r := range roles {
		if i.User.Role == r {
			return true
		}
	}
	return false
}

// AuthService implements the login/logout/me flow described in §7a.1.
//
// Rate-limiting (the Redis-backed exponential back-off mentioned in the
// spec) is intentionally deferred to a later milestone; this bootstrap
// trusts the network boundary and focuses on getting argon2id + JWT
// right.
type AuthService struct {
	users        *repo.Users
	layouts      *LayoutService
	signalmen    *repo.LayoutSignalmen
	elevations   *repo.SudoElevations
	jwtSecret    []byte
	sessionTTL   time.Duration
	cookieDomain string
}

// AuthConfig groups the few knobs the service exposes.
type AuthConfig struct {
	// JWTSecret signs the session JWT. MUST be at least 32 bytes in
	// production. Leaving it empty is a programmer error and panics.
	JWTSecret []byte
	// SessionTTL is the lifetime of a single login. Defaults to 24h
	// per §7a.1 when zero.
	SessionTTL time.Duration
}

// NewAuthService returns a ready-to-use AuthService. The LayoutService
// is used to validate the chosen layoutId at login time (§7a.1 steps
// 2-3) and to rehydrate the Layout side of Identity from a verified
// JWT (so a layout deleted out of band immediately invalidates
// outstanding cookies, mirroring the User branch).
//
// `elevations` plugs into Effective so sudo grants count toward the
// role hierarchy (§7a.7). It MAY be nil only in legacy tests that
// pre-date the sudo feature; in that case AuthService behaves as if
// no sudo grants exist.
func NewAuthService(users *repo.Users, layouts *LayoutService, signalmen *repo.LayoutSignalmen, elevations *repo.SudoElevations, cfg AuthConfig) *AuthService {
	if len(cfg.JWTSecret) == 0 {
		panic("service.NewAuthService: JWTSecret must not be empty")
	}
	if layouts == nil {
		panic("service.NewAuthService: LayoutService must not be nil")
	}
	if signalmen == nil {
		panic("service.NewAuthService: LayoutSignalmen must not be nil")
	}
	ttl := cfg.SessionTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	return &AuthService{
		users:      users,
		layouts:    layouts,
		signalmen:  signalmen,
		elevations: elevations,
		jwtSecret:  cfg.JWTSecret,
		sessionTTL: ttl,
	}
}

// SessionTTL exposes the configured session lifetime so the HTTP
// handler can match the JWT TTL with the cookie Max-Age.
func (s *AuthService) SessionTTL() time.Duration { return s.sessionTTL }

// HashPIN produces a PHC-formatted argon2id hash suitable for storage
// in users.pin_hash. Used by the bootstrap seeder and by any future
// "change PIN" endpoint.
func HashPIN(pin string) (string, error) {
	salt := make([]byte, argon2idParams.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}
	digest := argon2.IDKey([]byte(pin), salt,
		argon2idParams.time, argon2idParams.memory,
		argon2idParams.parallelism, argon2idParams.key)

	// Standard PHC encoding ("$argon2id$v=19$m=…,t=…,p=…$<salt>$<hash>")
	// keeps the parameters next to the hash, so verification doesn't
	// have to assume the current global cost.
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2idParams.memory, argon2idParams.time, argon2idParams.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

// verifyPIN compares the candidate PIN against a stored argon2id hash
// in constant time. Returns nil iff the PIN matches.
func verifyPIN(pin, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrInvalidCredentials
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return ErrInvalidCredentials
	}

	var memory, timeCost uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &parallelism); err != nil {
		return ErrInvalidCredentials
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrInvalidCredentials
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrInvalidCredentials
	}

	computed := argon2.IDKey([]byte(pin), salt, timeCost, memory, parallelism, uint32(len(expected)))
	if subtle.ConstantTimeCompare(expected, computed) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}

// Login exchanges a (login, pin, layoutId) triplet for an Identity.
// The caller (HTTP handler) is responsible for turning the returned
// Identity into a JWT via IssueToken and packaging it into a Secure
// HttpOnly cookie.
//
// The (login, pin) branch is checked first so an attacker that
// hand-crafts a request with a bad layoutId still can't enumerate
// valid logins via differing status codes. The layout branch only
// runs once credentials match.
func (s *AuthService) Login(ctx context.Context, login, pin string, layoutID uint) (Identity, error) {
	user, err := s.users.FindByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			// Burn ~the same CPU cycles as a real verify so timing
			// doesn't leak whether the login exists.
			_ = verifyPIN(pin, "$argon2id$v=19$m=65536,t=2,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
			return Identity{}, ErrInvalidCredentials
		}
		return Identity{}, err
	}

	if err := verifyPIN(pin, user.PINHash); err != nil {
		return Identity{}, ErrInvalidCredentials
	}

	// Soft-disabled accounts pass the credential check above (so the
	// admin can still re-activate them by login) but are rejected
	// here. Reaching this branch already requires a valid PIN, so
	// surfacing a distinct error code is not an enumeration risk.
	if !user.Active {
		return Identity{}, ErrAccountDeactivated
	}

	layout, err := s.layouts.ValidateForLogin(ctx, layoutID)
	if err != nil {
		return Identity{}, err
	}

	return Identity{User: user, Layout: layout}, nil
}

// Effective computes the flat role membership for (user, layout)
// at the present moment (§7a.2 / §7a.7). Permanent role, layout
// signalman grant and sudo admin elevation all collapse onto the
// same set — a sudo admin is indistinguishable from a permanent
// admin for every authority check (§7a.7.6).
func (s *AuthService) Effective(ctx context.Context, user domain.User, layoutID uint) (domain.EffectiveRoles, error) {
	roles := []domain.Role{user.Role}
	now := time.Now().UTC()

	if hasGrant, err := s.signalmen.HasActiveGrant(ctx, layoutID, user.ID, now); err != nil {
		return domain.EffectiveRoles{}, err
	} else if hasGrant {
		roles = append(roles, domain.RoleSignalman)
	}

	if s.elevations != nil {
		if _, err := s.elevations.FindActive(ctx, user.ID, layoutID, now); err == nil {
			roles = append(roles, domain.RoleAdmin)
		} else if !errors.Is(err, repo.ErrSudoElevationNotFound) {
			return domain.EffectiveRoles{}, err
		}
	}
	return domain.NewEffectiveRoles(roles...), nil
}

// EffectiveDisplayRole returns the single role label shown on the
// dashboard (§6.3c): admin beats signalman beats driver. Sudo
// elevations count — a sudo admin shows as "admin" until the
// elevation expires.
func (s *AuthService) EffectiveDisplayRole(ctx context.Context, user domain.User, layoutID uint) (domain.Role, error) {
	roles, err := s.Effective(ctx, user, layoutID)
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

// IsEffectiveSignalman reports whether the user may operate as a
// signalman inside the layout. Admins (sudo or permanent) and
// signalman grants both count.
func (s *AuthService) IsEffectiveSignalman(ctx context.Context, user domain.User, layoutID uint) (bool, error) {
	roles, err := s.Effective(ctx, user, layoutID)
	if err != nil {
		return false, err
	}
	return roles.Has(domain.RoleSignalman) || roles.Has(domain.RoleAdmin), nil
}

// sessionClaims is the wire shape of the JWT payload. Keep it small —
// every WebSocket frame carries the cookie, so each byte matters.
// `lid` is the immutable layout binding documented in §7a.1.
type sessionClaims struct {
	UID  uint   `json:"uid"`
	Role string `json:"role"`
	LID  uint   `json:"lid"`
	jwt.RegisteredClaims
}

// IssueToken signs a session JWT for the given identity. The TTL
// matches AuthService.sessionTTL.
func (s *AuthService) IssueToken(id Identity) (string, time.Time, error) {
	expiry := time.Now().Add(s.sessionTTL)
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
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiry, nil
}

// VerifyToken parses + cryptographically verifies a previously issued
// session token, and loads the corresponding User + Layout from the
// database (so a deleted account or a deleted layout immediately
// invalidates outstanding cookies).
//
// Tokens minted before the layout binding existed have LID = 0 — for
// those we fall back to the system layout so M2-era sessions keep
// working across the upgrade.
func (s *AuthService) VerifyToken(ctx context.Context, raw string) (Identity, error) {
	parsed, err := jwt.ParseWithClaims(raw, &sessionClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return Identity{}, ErrInvalidCredentials
	}
	claims, ok := parsed.Claims.(*sessionClaims)
	if !ok {
		return Identity{}, ErrInvalidCredentials
	}

	user, err := s.users.FindByID(ctx, claims.UID)
	if err != nil {
		return Identity{}, ErrInvalidCredentials
	}

	var layout domain.Layout
	if claims.LID == 0 {
		layout, err = s.layouts.GetSystem(ctx)
	} else {
		layout, err = s.layouts.Get(ctx, claims.LID)
	}
	if err != nil {
		return Identity{}, ErrInvalidCredentials
	}

	return Identity{User: user, Layout: layout}, nil
}
