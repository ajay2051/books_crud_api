// Package config reads settings from environment variables (12-factor style).
// A local .env file is loaded if present; in production, real env vars win.
// Load fails fast on missing or unsafe settings, so a misconfigured app never starts.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const minJWTSecretLength = 32

type Config struct {
	Port          string
	DatabaseURL   string
	GinMode       string // debug | release | test
	RunMigrations bool   // run migrations automatically on startup

	JWTSecret      string        // HMAC signing key, at least 32 characters
	JWTIssuer      string        // "iss" claim; tokens from other issuers are rejected
	AccessTokenTTL time.Duration // how long an access token stays valid
}

func Load() (Config, error) {
	_ = godotenv.Load() // .env is optional; ignore "file not found"

	cfg := Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		GinMode:       getEnv("GIN_MODE", "debug"),
		RunMigrations: getBool("RUN_MIGRATIONS", true),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTIssuer:     getEnv("JWT_ISSUER", "golang_crud"),
	}

	var problems []string

	if cfg.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	}

	if len(cfg.JWTSecret) < minJWTSecretLength {
		problems = append(problems,
			"JWT_SECRET is required and must be at least 32 characters (generate one with: openssl rand -base64 48)")
	}

	ttl, err := time.ParseDuration(getEnv("ACCESS_TOKEN_TTL", "1h"))
	switch {
	case err != nil:
		problems = append(problems, "ACCESS_TOKEN_TTL is not a valid duration (examples: 15m, 1h, 24h)")
	case ttl <= 0 || ttl > 24*time.Hour:
		problems = append(problems, "ACCESS_TOKEN_TTL must be greater than 0 and at most 24h")
	default:
		cfg.AccessTokenTTL = ttl
	}

	if len(problems) > 0 {
		return Config{}, errors.New("invalid configuration:\n - " + strings.Join(problems, "\n - "))
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
