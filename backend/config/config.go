package config

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all environment-driven application settings.
type Config struct {
	AppPort      string
	DatabaseURL  string // Replaced separate DB fields with a single Postgres URL
	JWTSecret    string
	JWTExpiryHrs string
	UploadDir    string
	AIServiceURL string
}

// Load reads .env (if present) and environment variables into a Config struct.
// Sensible defaults are provided for local development, but production
// deployments must always set these via environment variables / secrets.
func Load() *Config {
	loadedEnv := false
	for _, envPath := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(envPath); err == nil {
			loadedEnv = true
			break
		}
	}
	if !loadedEnv {
		log.Println("No .env file found, relying on system environment variables")
	}

	dbHost := getEnv("DB_HOST", "127.0.0.1")
	dbPort := getEnv("DB_PORT", "5433")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "coalguard")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	cfg := &Config{
		AppPort: getEnv("APP_PORT", "8080"),
		DatabaseURL: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(dbUser),
			url.QueryEscape(dbPass),
			dbHost,
			dbPort,
			dbName,
			dbSSLMode,
		),
		JWTSecret:    getEnv("JWT_SECRET", "CHANGE_ME_IN_PRODUCTION"),
		JWTExpiryHrs: getEnv("JWT_EXPIRY_HOURS", "12"),
		UploadDir:    getEnv("UPLOAD_DIR", "./uploads"),
		AIServiceURL: getEnv("AI_SERVICE_URL", "http://localhost:5000"),
	}

	if databaseURL := getEnv("DATABASE_URL", ""); databaseURL != "" {
		cfg.DatabaseURL = databaseURL
	}

	if cfg.JWTSecret == "CHANGE_ME_IN_PRODUCTION" {
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET in .env for production use.")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
