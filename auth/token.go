package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrTokenExpired: the token was valid but is past its expiry time.
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidToken: bad signature, wrong algorithm, wrong issuer, malformed, etc.
	ErrInvalidToken = errors.New("invalid token")
)

// Claims are the data stored inside the access token.
// The user ID goes in the standard "sub" (Subject) claim.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// TokenManager signs and verifies HS256 access tokens.
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenManager(secret, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// Issue creates a signed access token and reports how long it is valid for.
func (m *TokenManager) Issue(userID int64, email string) (string, time.Duration, error) {
	now := time.Now()

	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", 0, fmt.Errorf("generate token id: %w", err)
	}

	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			ID:        hex.EncodeToString(jti), // unique per token; lets you add revocation later
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign token: %w", err)
	}
	return signed, m.ttl, nil
}

// Parse verifies the signature, algorithm, issuer and time-based claims.
// It returns ErrTokenExpired or ErrInvalidToken (possibly wrapped), never a raw library error.
func (m *TokenManager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Pinning the algorithm blocks the classic "alg: none" and algorithm-swap attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5*time.Second), // tolerate small clock differences between servers
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return claims, nil
}
