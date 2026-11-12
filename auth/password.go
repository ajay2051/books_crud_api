// Package auth contains password hashing and JWT access tokens.
// It has no HTTP code, so it is easy to test and reuse.
package auth

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost 12 takes roughly 250ms per hash on modern hardware: slow enough to make
// brute-forcing stolen hashes expensive, fast enough for interactive login.
const bcryptCost = 12

// ErrInvalidCredentials means the password did not match.
var ErrInvalidCredentials = errors.New("invalid credentials")

// HashPassword returns a salted bcrypt hash. Never store or log the plain password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword returns nil on a match, ErrInvalidCredentials on a mismatch,
// and a wrapped error for anything unexpected (e.g. a corrupt hash).
func CheckPassword(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	switch {
	case err == nil:
		return nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return ErrInvalidCredentials
	default:
		return fmt.Errorf("compare password: %w", err)
	}
}

var (
	dummyOnce sync.Once
	dummyHash []byte
)

// BurnPasswordCheck does a bcrypt comparison against a throwaway hash.
// Call it when the email doesn't exist, so "unknown email" and "wrong password"
// take about the same time and attackers can't discover which emails are registered.
func BurnPasswordCheck(password string) {
	dummyOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcryptCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
