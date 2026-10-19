package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env               string
	Port              string
	DatabaseURL       string
	PublicOrigin      string
	SessionCookieName string
	SessionTTL        time.Duration
}

func Load() (Config, error) {
	ttlMin, err := strconv.Atoi(getenv("SESSION_TTL_MINUTES", "60"))
	if err != nil || ttlMin <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL_MINUTES must be a positive integer")
	}

	c := Config{
		// Safe default: anything that is not explicitly "development" is treated as production.
		Env:               getenv("APP_ENV", "production"),
		Port:              getenv("API_PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		PublicOrigin:      getenv("PUBLIC_ORIGIN", "http://localhost:3000"),
		SessionCookieName: getenv("SESSION_COOKIE_NAME", "ph_session"),
		SessionTTL:        time.Duration(ttlMin) * time.Minute,
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func (c Config) IsDev() bool { return c.Env == "development" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
