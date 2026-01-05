package server

import (
	"net/url"
	"os"
	"testing"
)

func TestDBDSNFromEnv_DatabaseURL(t *testing.T) {
	if err := os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db?sslmode=disable"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DATABASE_URL") })

	if got := dbDSNFromEnv(); got != "postgres://u:p@h:5432/db?sslmode=disable" {
		t.Fatalf("got=%q", got)
	}
}

func TestDBDSNFromEnv_Defaults(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("DATABASE_URL") })
	t.Cleanup(func() { _ = os.Unsetenv("DB_HOST") })
	t.Cleanup(func() { _ = os.Unsetenv("DB_PORT") })
	t.Cleanup(func() { _ = os.Unsetenv("DB_USER") })
	t.Cleanup(func() { _ = os.Unsetenv("DB_PASSWORD") })
	t.Cleanup(func() { _ = os.Unsetenv("DB_NAME") })
	t.Cleanup(func() { _ = os.Unsetenv("DB_SSLMODE") })

	got := dbDSNFromEnv()
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "postgres" {
		t.Fatalf("scheme=%q", u.Scheme)
	}
	if u.Host == "" || u.Path == "" {
		t.Fatalf("host=%q path=%q", u.Host, u.Path)
	}
	if u.Query().Get("sslmode") == "" {
		t.Fatal("expected sslmode")
	}
}

func TestGetenvDefault(t *testing.T) {
	if err := os.Setenv("X_TEST_ENV", "v"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("X_TEST_ENV") })

	if got := getenvDefault("X_TEST_ENV", "d"); got != "v" {
		t.Fatalf("got=%q", got)
	}
	if got := getenvDefault("X_NO_SUCH_ENV", "d"); got != "d" {
		t.Fatalf("got=%q", got)
	}
}
