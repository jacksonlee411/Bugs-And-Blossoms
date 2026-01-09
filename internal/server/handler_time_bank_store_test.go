package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHandlerWithOptions_TimeBankStoreNilCheckCovered(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	staffingStore := newStaffingMemoryStore()

	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:             localTenancyResolver(),
		OrgUnitStore:                newOrgUnitMemoryStore(),
		PositionStore:               staffingStore,
		AssignmentStore:             staffingStore,
		PayrollStore:                stubPayrollStore{},
		AttendanceStore:             staffingStore,
		AttendanceDailyResultsStore: staffingStore,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}
