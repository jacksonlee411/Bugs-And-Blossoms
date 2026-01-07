package superadmin

import "testing"

func TestDBDSNFromEnv(t *testing.T) {
	t.Setenv("SUPERADMIN_DATABASE_URL", "")
	if _, err := dbDSNFromEnv(); err == nil {
		t.Fatal("expected error")
	}

	t.Setenv("SUPERADMIN_DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	got, err := dbDSNFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected dsn")
	}
}

func TestDBDSNFromFallbackEnv_Defaults(t *testing.T) {
	got := dbDSNFromFallbackEnv()
	if got == "" {
		t.Fatal("expected dsn")
	}
}

func TestDBDSNFromFallbackEnv_Overrides(t *testing.T) {
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_PORT", "1111")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_NAME", "n")
	t.Setenv("DB_SSLMODE", "require")

	got := dbDSNFromFallbackEnv()
	if got != "postgres://u:p@h:1111/n?sslmode=require" {
		t.Fatalf("got=%q", got)
	}
}
