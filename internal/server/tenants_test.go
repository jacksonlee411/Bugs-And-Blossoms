package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostWithoutPort(t *testing.T) {
	if got := hostWithoutPort("localhost:8080"); got != "localhost" {
		t.Fatalf("got=%q", got)
	}
	if got := hostWithoutPort("localhost"); got != "localhost" {
		t.Fatalf("got=%q", got)
	}
}

func TestLoadTenants_Errors(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	tmp := t.TempDir()

	pMissing := filepath.Join(tmp, "missing.yaml")
	if err := os.Setenv("TENANTS_PATH", pMissing); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenants(); err == nil {
		t.Fatal("expected missing file error")
	}

	pBad := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(pBad, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TENANTS_PATH", pBad); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenants(); err == nil {
		t.Fatal("expected yaml error")
	}

	pVer := filepath.Join(tmp, "ver.yaml")
	if err := os.WriteFile(pVer, []byte("version: 2\ntenants: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TENANTS_PATH", pVer); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenants(); err == nil {
		t.Fatal("expected version error")
	}

	pEmpty := filepath.Join(tmp, "empty.yaml")
	if err := os.WriteFile(pEmpty, []byte("version: 1\ntenants: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TENANTS_PATH", pEmpty); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenants(); err == nil {
		t.Fatal("expected empty error")
	}

	pInvalid := filepath.Join(tmp, "invalid.yaml")
	if err := os.WriteFile(pInvalid, []byte("version: 1\ntenants:\n  - id: \"\"\n    domain: \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TENANTS_PATH", pInvalid); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenants(); err == nil {
		t.Fatal("expected invalid tenant error")
	}
}

func TestDefaultTenantsPath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	p, err := defaultTenantsPath()
	if err != nil {
		t.Fatal(err)
	}
	if p == "" {
		t.Fatal("empty path")
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = defaultTenantsPath()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadTenants_DefaultPathError(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = loadTenants()
	if err == nil {
		t.Fatal("expected error")
	}
}
