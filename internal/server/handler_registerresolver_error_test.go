package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHandlerWithOptions_DictResolverTypedNilError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	var typedNil *dictMemoryStore
	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
		DictStore:       typedNil,
	})
	if err == nil || !strings.Contains(err.Error(), "dict: resolver is nil") {
		t.Fatalf("err=%v", err)
	}
}
