// Package config
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	// AllowedOrigins restricts CORS. "*" is accepted but only sensible locally.
	AllowedOrigins []string
	// RateLimit is the number of requests one address may make per minute.
	// Zero turns the limiter off, for hosts where it would keep one count per
	// instance and cannot tell visitors apart.
	RateLimit int
	// MigrateOnStart applies the schema before listening. A serverless host
	// starts a process per cold start, and each run of the schema's ALTER
	// TABLEs takes an exclusive lock, so there the seeder applies it instead.
	MigrateOnStart bool
}

// loadDotEnv walks up from the working directory looking for a .env file, so
// commands and tests behave the same regardless of where they are run from.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func Load() *Config {
	loadDotEnv()

	return &Config{
		Port:           getEnv("PORT", "8080"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres"),
		DBName:         getEnv("DB_NAME", "dionysus"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		AllowedOrigins: splitList(getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:4173")),
		RateLimit:      getEnvInt("RATE_LIMIT", 240),
		MigrateOnStart: getEnvBool("MIGRATE_ON_START", true),
	}
}

// getEnvInt falls back rather than failing, because a typo in an operational
// knob should not stop the server from booting.
func getEnvInt(key string, fallback int) int {
	value, err := strconv.Atoi(getEnv(key, ""))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(getEnv(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
