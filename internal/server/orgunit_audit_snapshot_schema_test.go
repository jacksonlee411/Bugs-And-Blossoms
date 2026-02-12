package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRootFromCurrentFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/server/xxx_test.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
}

func TestOrgunitSchema_SubmitOrgEvent_UsesInsertCompleteSnapshots(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	if !strings.Contains(s, "INSERT INTO orgunit.org_events (") {
		t.Fatalf("missing org_events insert in %s", p)
	}

	// 080C 收口：submit_org_event 必须 INSERT 即写齐 before/after 快照。
	for _, required := range []string{"before_snapshot", "after_snapshot"} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing %s in submit_org_event", required)
		}
	}
	if strings.Contains(s, "UPDATE orgunit.org_events") {
		t.Fatalf("unexpected post-insert org_events update in %s", p)
	}

	// 确保使用 canonical snapshot 抽取，并在单条 INSERT 中写齐。
	for _, token := range []string{
		"SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;",
		"v_before_snapshot := orgunit.extract_orgunit_snapshot",
		"v_after_snapshot := orgunit.extract_orgunit_snapshot",
		"PERFORM orgunit.assert_org_event_snapshots",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitEngine_NoPostInsertSnapshotUpdate(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	if strings.Contains(s, "UPDATE orgunit.org_events") {
		t.Fatalf("unexpected post-insert org_events update in %s", p)
	}

	for _, token := range []string{
		"CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(",
		"PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(",
		"rescind_outcome",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitSchema_PresencePredicateStrictRescindOutcome(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, forbidden := range []string{
		"p_before_snapshot IS NULL AND p_after_snapshot IS NULL",
		"p_rescind_outcome IS NULL",
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("unexpected transitional token %q in %s", forbidden, p)
		}
	}

	for _, required := range []string{
		"WHEN p_event_type = 'CREATE'",
		"WHEN p_event_type IN ('MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS')",
		"(p_rescind_outcome = 'ABSENT' AND p_after_snapshot IS NULL)",
		"(p_rescind_outcome = 'PRESENT' AND p_after_snapshot IS NOT NULL)",
		"AND rescind_outcome IN ('PRESENT','ABSENT')",
	} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing %q in %s", required, p)
		}
	}
}

func TestOrgunitSchema_RescindSnapshotContentConstraints(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"CREATE OR REPLACE FUNCTION orgunit.is_orgunit_snapshot_complete(p_snapshot jsonb)",
		"CREATE OR REPLACE FUNCTION orgunit.is_org_event_snapshot_content_valid(",
		"CONSTRAINT org_events_rescind_payload_required CHECK",
		"CONSTRAINT org_events_snapshot_content_check CHECK",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitMigration080D_BackfillsRescindSnapshotContent(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "migrations/orgunit/20260212113000_orgunit_rescind_snapshot_completeness.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"UPDATE orgunit.org_events e",
		"jsonb_build_object(",
		"'target_effective_date'",
		"WITH target AS (",
		"CONSTRAINT org_events_rescind_payload_required CHECK",
		"CONSTRAINT org_events_snapshot_content_check CHECK",
		"VALIDATE CONSTRAINT org_events_snapshot_content_check",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitMigration080C_IntroducesPendingReplayEngine(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "migrations/orgunit/20260210203000_orgunit_snapshot_insert_complete.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"CREATE OR REPLACE FUNCTION orgunit.org_events_effective_for_replay(",
		"CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(",
		"UPDATE orgunit.org_events",
		"SET rescind_outcome = CASE WHEN after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END",
		"SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitMigration080A_ReappliesKernelFunctionPrivileges(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "migrations/orgunit/20260210093000_orgunit_audit_snapshot_canonical.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	signatures := []string{
		"orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)",
		"orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)",
		"orgunit.submit_org_rescind(uuid, int, text, text, uuid)",
		"orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)",
		"orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)",
	}
	for _, signature := range signatures {
		if !strings.Contains(s, "ALTER FUNCTION "+signature+"\n  OWNER TO orgunit_kernel;") {
			t.Fatalf("missing OWNER TO orgunit_kernel for %s in %s", signature, p)
		}
		if !strings.Contains(s, "ALTER FUNCTION "+signature+"\n  SECURITY DEFINER;") {
			t.Fatalf("missing SECURITY DEFINER for %s in %s", signature, p)
		}
		if !strings.Contains(s, "ALTER FUNCTION "+signature+"\n  SET search_path = pg_catalog, orgunit, public;") {
			t.Fatalf("missing search_path for %s in %s", signature, p)
		}
	}
}
