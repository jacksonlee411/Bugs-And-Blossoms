package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCubeboxBackfillCountIssues(t *testing.T) {
	ok := cubeboxBackfillSummary{
		TenantID:                  "t1",
		AssistantConversations:    1,
		CubeboxConversations:      1,
		AssistantTurns:            2,
		CubeboxTurns:              2,
		AssistantTasks:            3,
		CubeboxTasks:              3,
		AssistantIdempotency:      4,
		CubeboxIdempotency:        4,
		AssistantStateTransitions: 5,
		CubeboxStateTransitions:   5,
		AssistantTaskEvents:       6,
		CubeboxTaskEvents:         6,
		AssistantDispatchOutbox:   7,
		CubeboxDispatchOutbox:     7,
	}
	if issues := cubeboxBackfillCountIssues(ok); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}

	bad := ok
	bad.CubeboxTasks = 2
	issues := cubeboxBackfillCountIssues(bad)
	if len(issues) != 1 || !strings.Contains(issues[0], "tasks count mismatch") {
		t.Fatalf("expected tasks mismatch issue, got %v", issues)
	}
}

func TestCubeboxValidateTenantFileRecords(t *testing.T) {
	items := []cubeboxLocalFileRecord{
		{
			FileID:         "file_11111111-1111-1111-1111-111111111111",
			TenantID:       "00000000-0000-0000-0000-000000000001",
			ConversationID: "conv_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			FileName:       "a.txt",
			MediaType:      "text/plain",
			SizeBytes:      3,
			SHA256:         strings.Repeat("a", 64),
			StorageKey:     "00000000-0000-0000-0000-000000000001/file-a/a.txt",
			UploadedBy:     "user-1",
			UploadedAt:     "2026-04-15T01:02:03Z",
		},
		{
			FileID:     "file_22222222-2222-2222-2222-222222222222",
			TenantID:   "00000000-0000-0000-0000-000000000001",
			FileName:   "b.txt",
			MediaType:  "text/plain",
			SizeBytes:  4,
			SHA256:     strings.Repeat("b", 64),
			StorageKey: "00000000-0000-0000-0000-000000000001/file-b/b.txt",
			UploadedBy: "user-2",
			UploadedAt: "2026-04-15T02:02:03Z",
		},
	}

	filtered, issues := cubeboxValidateTenantFileRecords(items, "00000000-0000-0000-0000-000000000001")
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 items, got %d", len(filtered))
	}
	if filtered[0].FileID != items[0].FileID {
		t.Fatalf("expected uploaded_at ordering to keep first record first, got %q", filtered[0].FileID)
	}

	badItems := append([]cubeboxLocalFileRecord(nil), items...)
	badItems = append(badItems, cubeboxLocalFileRecord{
		FileID:     "bad",
		TenantID:   "00000000-0000-0000-0000-000000000001",
		FileName:   "",
		MediaType:  "",
		SizeBytes:  0,
		SHA256:     "nope",
		StorageKey: items[0].StorageKey,
		UploadedBy: "",
		UploadedAt: "bad-time",
	})
	_, issues = cubeboxValidateTenantFileRecords(badItems, "00000000-0000-0000-0000-000000000001")
	if len(issues) < 6 {
		t.Fatalf("expected multiple issues, got %v", issues)
	}
}

func TestCubeboxValidateStorageObjects(t *testing.T) {
	root := t.TempDir()
	objectDir := filepath.Join(root, "objects", "tenant-a", "file-a")
	if err := os.MkdirAll(objectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	filePath := filepath.Join(objectDir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sum, err := cubeboxComputeSHA256(filePath)
	if err != nil {
		t.Fatalf("compute sha256: %v", err)
	}

	records := []cubeboxLocalFileRecord{
		{
			FileID:     "file_33333333-3333-3333-3333-333333333333",
			TenantID:   "00000000-0000-0000-0000-000000000001",
			FileName:   "hello.txt",
			MediaType:  "text/plain",
			SizeBytes:  5,
			SHA256:     sum,
			StorageKey: "tenant-a/file-a/hello.txt",
			UploadedBy: "user-1",
			UploadedAt: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}

	validated, err := cubeboxValidateStorageObjects(records, root)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(validated) != 1 {
		t.Fatalf("expected 1 validated record, got %d", len(validated))
	}
	if validated[0].StorageSHA256 != sum {
		t.Fatalf("unexpected sha256: got %s want %s", validated[0].StorageSHA256, sum)
	}

	records[0].SizeBytes = 6
	if _, err := cubeboxValidateStorageObjects(records, root); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("expected size mismatch error, got %v", err)
	}
}

func TestParseRFC3339Flexible(t *testing.T) {
	for _, value := range []string{
		"2026-04-15T08:09:10Z",
		"2026-04-15T08:09:10.123456789Z",
	} {
		if _, err := parseRFC3339Flexible(value); err != nil {
			t.Fatalf("parseRFC3339Flexible(%q) error = %v", value, err)
		}
	}
	if _, err := parseRFC3339Flexible("bad-time"); err == nil {
		t.Fatalf("expected invalid time error")
	}
}

func TestCubeboxResolveTenantIDsFromIndex(t *testing.T) {
	items := []cubeboxLocalFileRecord{
		{TenantID: "00000000-0000-0000-0000-000000000002"},
		{TenantID: "00000000-0000-0000-0000-000000000001"},
		{TenantID: "00000000-0000-0000-0000-000000000002"},
	}

	got, err := cubeboxResolveTenantIDsFromIndex(items, "")
	if err != nil {
		t.Fatalf("cubeboxResolveTenantIDsFromIndex() error = %v", err)
	}
	want := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected tenant count: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	got, err = cubeboxResolveTenantIDsFromIndex(items, "00000000-0000-0000-0000-000000000009")
	if err != nil {
		t.Fatalf("explicit tenant should pass, got err=%v", err)
	}
	if len(got) != 1 || got[0] != "00000000-0000-0000-0000-000000000009" {
		t.Fatalf("unexpected explicit tenant result: %v", got)
	}
}
