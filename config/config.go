// Package config reads settings from environment variables (12-factor style).
// A local .env file is loaded if present; in product on, real env vars win.
//

package config

import (
	"github.com/joho/godotenv"
	"os"
	"strconv"
)

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

type Config struct {
	Port          string
	DatabaseURL   string
	GinMode       string // debug | release | test
	RunMigrations bool   // run migration automatically on startup

}

func Load() Config {
	_ = godotenv.Load() // .env is optional; ignore "file not found"
	return Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/booksdb?sslmode=disable"),
		GinMode:       getEnv("GIN_MODE", "debug"),
		RunMigrations: getBool("RUN_MIGRATIONS", true),
	}
}
