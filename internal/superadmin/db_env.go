package superadmin

import (
	"errors"
	"net/url"
	"os"
)

func dbDSNFromEnv() (string, error) {
	if v := os.Getenv("SUPERADMIN_DATABASE_URL"); v != "" {
		return v, nil
	}

	return "", errors.New("superadmin: SUPERADMIN_DATABASE_URL is required")
}

func dbDSNFromFallbackEnv() string {
	host := getenvDefault("DB_HOST", "127.0.0.1")
	port := getenvDefault("DB_PORT", "5438")
	user := getenvDefault("DB_USER", "app")
	pass := getenvDefault("DB_PASSWORD", "app")
	name := getenvDefault("DB_NAME", "bugs_and_blossoms")
	sslmode := getenvDefault("DB_SSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   host + ":" + port,
		Path:   "/" + name,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
