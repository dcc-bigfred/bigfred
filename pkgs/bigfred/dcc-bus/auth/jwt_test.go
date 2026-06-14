package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// helper: mint a test JWT signed with HS256.
func mintToken(t *testing.T, secret []byte, uid, lid uint, sub string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UID: uid,
		LID: lid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

func TestVerifierAcceptsValidToken(t *testing.T) {
	secret := []byte("test-secret-1234567890abcdef")
	v := NewVerifier(secret, 5)

	id, err := v.Verify(mintToken(t, secret, 42, 5, "alice"))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id.UserID != 42 || id.LayoutID != 5 || id.Login != "alice" {
		t.Fatalf("unexpected identity: %#v", id)
	}
}

func TestVerifierRejectsLayoutMismatch(t *testing.T) {
	secret := []byte("test-secret-1234567890abcdef")
	v := NewVerifier(secret, 5)

	_, err := v.Verify(mintToken(t, secret, 42, 6, "alice"))
	if !errors.Is(err, ErrLayoutMismatch) {
		t.Fatalf("expected ErrLayoutMismatch, got %v", err)
	}
}

func TestVerifierRejectsBadSecret(t *testing.T) {
	v := NewVerifier([]byte("secret-A"), 5)

	_, err := v.Verify(mintToken(t, []byte("secret-B"), 42, 5, "alice"))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifierRejectsEmptyToken(t *testing.T) {
	v := NewVerifier([]byte("x"), 1)
	if _, err := v.Verify(""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
