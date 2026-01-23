package routing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAllowlistYAML_Errors(t *testing.T) {
	t.Parallel()

	_, err := ParseAllowlistYAML([]byte{0xff})
	if err == nil {
		t.Fatal("expected yaml error")
	}

	_, err = ParseAllowlistYAML([]byte("version: 2\nentrypoints: {}"))
	if err == nil {
		t.Fatal("expected version error")
	}

	_, err = ParseAllowlistYAML([]byte("version: 1"))
	if err == nil {
		t.Fatal("expected entrypoints error")
	}
}

func TestLoadAllowlist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.yaml")
	if err := os.WriteFile(path, []byte(`version: 1
entrypoints:
  server:
    routes:
      - path: /health
        methods: [GET]
        route_class: ops
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if a.Version != 1 {
		t.Fatalf("version=%d", a.Version)
	}
	if _, ok := a.Entrypoints["server"]; !ok {
		t.Fatal("missing entrypoint")
	}
}

func TestLoadAllowlist_ReadError(t *testing.T) {
	t.Parallel()

	if _, err := LoadAllowlist(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected error")
	}
}
