package main

import (
	"net/url"
	"os"
	"time"
)

type config struct {
	Port          string
	DatabaseURL   string
	NtfyBaseURL   string
	TimeZone      string
	SessionTTL    time.Duration
	SecureCookies bool
	AdminUsername string
	AdminPassword string
}

func loadConfig() config {
	return config{
		Port:          envOr("CAKENIFY_PORT", "8080"),
		DatabaseURL:   databaseURL(),
		NtfyBaseURL:   envOr("CAKENIFY_NTFY_BASE_URL", "https://ntfy.bikasolutions.co.za"),
		TimeZone:      envOr("CAKENIFY_TIMEZONE", "Africa/Johannesburg"),
		SessionTTL:    envDurationOr("CAKENIFY_SESSION_TTL", 30*24*time.Hour),
		SecureCookies: envBoolOr("CAKENIFY_SECURE_COOKIES", true),
		AdminUsername: os.Getenv("CAKENIFY_ADMIN_USERNAME"),
		AdminPassword: os.Getenv("CAKENIFY_ADMIN_PASSWORD"),
	}
}

// databaseURL prefers a full CAKENIFY_DATABASE_URL override (e.g. for
// pointing at an external DB), but otherwise builds the DSN from separate
// host/user/password/etc. env vars via net/url so the password is
// percent-encoded correctly — a plain "postgres://user:pass@host" string
// built by hand (e.g. in docker-compose.yml) breaks the moment the
// password contains an unescaped reserved character like @, :, or %.
func databaseURL() string {
	if v := os.Getenv("CAKENIFY_DATABASE_URL"); v != "" {
		return v
	}

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(envOr("CAKENIFY_DB_USER", "cakenify"), envOr("CAKENIFY_DB_PASSWORD", "cakenify")),
		Host:   envOr("CAKENIFY_DB_HOST", "localhost") + ":" + envOr("CAKENIFY_DB_PORT", "5432"),
		Path:   "/" + envOr("CAKENIFY_DB_NAME", "cakenify"),
	}

	q := u.Query()
	q.Set("sslmode", envOr("CAKENIFY_DB_SSLMODE", "disable"))
	u.RawQuery = q.Encode()

	return u.String()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func envBoolOr(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	return v == "true" || v == "1"
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}

	return d
}
