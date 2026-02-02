package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOrgunitCodeCSV_NormalizeAndHeader(t *testing.T) {
	content := "org_id,org_code\n10000001,a_b-1\n"
	path := writeTempFile(t, content)

	rows, conflicts := readOrgunitCodeCSV(path)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d", len(conflicts))
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].normalized != "A_B-1" {
		t.Fatalf("normalized=%q", rows[0].normalized)
	}
}

func TestReadOrgunitCodeCSV_Invalids(t *testing.T) {
	content := "org_id,org_code\ninvalid,A1\n10000001, A1\n"
	path := writeTempFile(t, content)

	_, conflicts := readOrgunitCodeCSV(path)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}
	reasons := conflictReasons(conflicts)
	if reasons["org_id_invalid"] == 0 {
		t.Fatalf("expected org_id_invalid")
	}
	if reasons["org_code_invalid"] == 0 {
		t.Fatalf("expected org_code_invalid")
	}
}

func TestValidateOrgunitCodeRows_DuplicatesAndConflicts(t *testing.T) {
	rows := []orgunitCodeRow{
		{line: 1, orgID: 10000001, rawCode: "A1", normalized: "A1"},
		{line: 2, orgID: 10000002, rawCode: "A1", normalized: "A1"},
		{line: 3, orgID: 10000001, rawCode: "B1", normalized: "B1"},
	}
	existingOrgSet := map[int]struct{}{
		10000001: {},
		10000002: {},
	}
	existingCodes := map[string]int{
		"C1": 10000003,
	}
	existingOrgIDs := map[int]string{
		10000001: "A1",
	}

	_, conflicts := validateOrgunitCodeRows(rows, existingOrgSet, existingCodes, existingOrgIDs)
	reasons := conflictReasons(conflicts)
	if reasons["org_code_duplicate_input"] == 0 {
		t.Fatalf("expected org_code_duplicate_input")
	}
	if reasons["org_id_duplicate_input"] == 0 {
		t.Fatalf("expected org_id_duplicate_input")
	}
	if reasons["org_id_conflict_db"] == 0 {
		t.Fatalf("expected org_id_conflict_db")
	}
}

func TestValidateOrgunitCodeRows_MissingOrgID(t *testing.T) {
	rows := []orgunitCodeRow{{line: 1, orgID: 10000005, rawCode: "A5", normalized: "A5"}}
	_, conflicts := validateOrgunitCodeRows(rows, map[int]struct{}{}, map[string]int{}, map[int]string{})
	reasons := conflictReasons(conflicts)
	if reasons["org_id_missing_db"] == 0 {
		t.Fatalf("expected org_id_missing_db")
	}
}

func TestValidateOrgunitCodeRows_AlreadyMapped(t *testing.T) {
	rows := []orgunitCodeRow{{line: 1, orgID: 10000001, rawCode: "A1", normalized: "A1"}}
	existingOrgSet := map[int]struct{}{10000001: {}}
	existingCodes := map[string]int{"A1": 10000001}
	existingOrgIDs := map[int]string{10000001: "A1"}

	validRows, conflicts := validateOrgunitCodeRows(rows, existingOrgSet, existingCodes, existingOrgIDs)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d", len(conflicts))
	}
	if len(validRows) != 1 || !validRows[0].alreadyMapped {
		t.Fatalf("expected alreadyMapped")
	}
}

func conflictReasons(conflicts []orgunitCodeConflict) map[string]int {
	counts := make(map[string]int)
	for _, c := range conflicts {
		counts[c.reason]++
	}
	return counts
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.csv")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
