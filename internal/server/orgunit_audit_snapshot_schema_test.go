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

func TestOrgunitSchema_SubmitOrgEvent_UsesCanonicalSnapshots(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	insertStart := strings.Index(s, "INSERT INTO orgunit.org_events (")
	if insertStart < 0 {
		t.Fatalf("missing org_events insert in %s", p)
	}
	insertEnd := strings.Index(s[insertStart:], "ON CONFLICT (event_uuid) DO NOTHING")
	if insertEnd < 0 {
		t.Fatalf("missing ON CONFLICT block in %s", p)
	}
	insertBlock := s[insertStart : insertStart+insertEnd]

	// 防止回归：submit_org_event 不允许把 patch/payload 直接写进 before/after 快照。
	for _, forbidden := range []string{"before_snapshot", "after_snapshot"} {
		if strings.Contains(insertBlock, forbidden) {
			t.Fatalf("unexpected %s in submit_org_event insert block", forbidden)
		}
	}

	// 确保使用 canonical snapshot 抽取，再回写事件快照。
	for _, token := range []string{
		"v_before_snapshot := orgunit.extract_orgunit_snapshot",
		"v_after_snapshot := orgunit.extract_orgunit_snapshot",
		"PERFORM orgunit.assert_org_event_snapshots",
		"SET before_snapshot = v_before_snapshot",
		"after_snapshot = v_after_snapshot",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}
