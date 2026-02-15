package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMustNewHandler_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/bugs_and_blossoms?sslmode=disable")

	if MustNewHandler() == nil {
		t.Fatal("expected handler")
	}
}

func TestMustNewHandler_Panic(t *testing.T) {
	t.Setenv("ALLOWLIST_PATH", filepath.Join(t.TempDir(), "missing.yaml"))

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustNewHandler()
}
