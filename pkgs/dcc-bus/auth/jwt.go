// Package auth verifies the same session JWT that loco-server signs,
// but without pulling in the full AuthService dependency chain
// (users repo, layouts repo, sudo elevations). The daemon only needs
// the (userID, login, layoutID) triplet at the trust boundary; richer
// authorization decisions are re-evaluated against domain objects
// loaded from read-only SQLite by `pkgs/dcc-bus/state`.
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned by Verify for any JWT that fails the
// HMAC check, has expired, or doesn't carry the expected claims.
var ErrInvalidToken = errors.New("invalid token")

// ErrLayoutMismatch is returned when the token's `lid` claim doesn't
// match the daemon's --layout-id flag. Two cookies for different
// layouts can never coexist on the data plane.
var ErrLayoutMismatch = errors.New("layout mismatch")

// Claims is the subset of pkgs/server/service.sessionClaims that the
// dcc-bus daemon cares about. Keep the field tags identical so the
// wire shape doesn't drift between processes.
type Claims struct {
	UID uint   `json:"uid"`
	LID uint   `json:"lid"`
	jwt.RegisteredClaims
}

// Identity is the verified caller. The Login is taken from the
// JWT's `sub` claim (loco-server sets it to user.Login on issue);
// dcc-bus uses it for audit fan-in.
type Identity struct {
	UserID   uint
	LayoutID uint
	Login    string
}

// Verifier validates JWTs against a shared HMAC secret and binds
// each verification to a specific layout. Two daemons that share
// the secret but watch different layouts will reject each other's
// tokens via ErrLayoutMismatch.
type Verifier struct {
	secret   []byte
	layoutID uint
}

// NewVerifier returns a Verifier ready to validate tokens. The
// caller MUST supply the same secret loco-server uses to sign;
// loco-server propagates it to the daemon via the `--jwt-secret`
// flag at spawn time.
func NewVerifier(secret []byte, layoutID uint) *Verifier {
	return &Verifier{secret: secret, layoutID: layoutID}
}

// Verify parses + cryptographically verifies a JWT, asserts the
// `lid` claim matches the daemon's bound layout, and returns the
// caller's Identity. Anything other than ErrInvalidToken or
// ErrLayoutMismatch is propagated as-is.
func (v *Verifier) Verify(raw string) (Identity, error) {
	if raw == "" {
		return Identity{}, ErrInvalidToken
	}
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil || !parsed.Valid {
		return Identity{}, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.UID == 0 {
		return Identity{}, ErrInvalidToken
	}
	if claims.LID != v.layoutID {
		return Identity{}, ErrLayoutMismatch
	}
	return Identity{
		UserID:   claims.UID,
		LayoutID: claims.LID,
		Login:    claims.Subject,
	}, nil
}
