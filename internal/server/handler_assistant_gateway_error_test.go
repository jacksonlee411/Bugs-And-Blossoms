package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHandlerWithOptions_AssistantGatewayError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")
	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", "{")
	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{},
		OrgUnitStore:     newOrgUnitMemoryStore(),
		SetIDStore:       newSetIDMemoryStore(),
		JobCatalogStore:  newJobCatalogMemoryStore(),
		PersonStore:      newPersonMemoryStore(),
		PositionStore:    newStaffingMemoryStore(),
		AssignmentStore:  newStaffingMemoryStore(),
		DictStore:        newDictMemoryStore(),
	})
	if !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("expected runtime config invalid, got=%v", err)
	}
}
