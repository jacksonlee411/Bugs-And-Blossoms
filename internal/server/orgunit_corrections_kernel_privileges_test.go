package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRootFromCurrentFile080B(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
}

func TestOrgunitMigration080B_CorrectionsKernelPrivileges(t *testing.T) {
	root := repoRootFromCurrentFile080B(t)
	p := filepath.Join(root, "migrations/orgunit/20260210101000_orgunit_corrections_kernel_privileges.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	signatures := []string{
		"orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)",
		"orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)",
		"orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)",
		"orgunit.submit_org_rescind(uuid, int, text, text, uuid)",
	}
	for _, signature := range signatures {
		if !strings.Contains(s, "ALTER FUNCTION "+signature+"\n  OWNER TO orgunit_kernel;") {
			t.Fatalf("missing OWNER TO orgunit_kernel for %s", signature)
		}
		if !strings.Contains(s, "ALTER FUNCTION "+signature+"\n  SECURITY DEFINER;") {
			t.Fatalf("missing SECURITY DEFINER for %s", signature)
		}
		if !strings.Contains(s, "ALTER FUNCTION "+signature+"\n  SET search_path = pg_catalog, orgunit, public;") {
			t.Fatalf("missing search_path for %s", signature)
		}
	}
}
