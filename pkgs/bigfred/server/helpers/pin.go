package helpers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrPINMismatch is returned when a candidate PIN does not match a stored hash.
var ErrPINMismatch = errors.New("pin mismatch")

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

// HashPIN produces a PHC-formatted argon2id hash suitable for storage.
func HashPIN(pin string) (string, error) {
	salt := make([]byte, argon2idParams.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}
	digest := argon2.IDKey([]byte(pin), salt,
		argon2idParams.time, argon2idParams.memory,
		argon2idParams.parallelism, argon2idParams.key)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2idParams.memory, argon2idParams.time, argon2idParams.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

// VerifyPIN compares a candidate PIN against a stored argon2id hash.
func VerifyPIN(pin, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrPINMismatch
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return ErrPINMismatch
	}

	var memory, timeCost uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &parallelism); err != nil {
		return ErrPINMismatch
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrPINMismatch
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrPINMismatch
	}

	computed := argon2.IDKey([]byte(pin), salt, timeCost, memory, parallelism, uint32(len(expected)))
	if subtle.ConstantTimeCompare(expected, computed) != 1 {
		return ErrPINMismatch
	}
	return nil
}
