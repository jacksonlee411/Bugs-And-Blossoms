package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrgunitExtPayloadEngine_HasKernelHelpersAndStableErrors(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"CREATE OR REPLACE FUNCTION orgunit.merge_org_event_payload_with_correction(",
		"orgunit.merge_org_event_payload_with_correction(e.payload, lc.correction_payload)",
		"orgunit.merge_org_event_payload_with_correction(se.payload, lc.correction_payload)",
		"CREATE OR REPLACE FUNCTION orgunit.apply_org_event_ext_payload(",
		"MESSAGE = 'ORG_EXT_PAYLOAD_INVALID_SHAPE'",
		"MESSAGE = 'ORG_EXT_PAYLOAD_NOT_ALLOWED_FOR_EVENT'",
		"MESSAGE = 'ORG_EXT_FIELD_NOT_CONFIGURED'",
		"MESSAGE = 'ORG_EXT_FIELD_NOT_ENABLED_AS_OF'",
		"MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH'",
		"MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_REQUIRED'",
		"MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_NOT_ALLOWED'",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitExtPayloadEngine_SplitAndMoveCopyExtSlots(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"v_row.ext_str_01",
		"v_row.ext_str_70",
		"v_row.ext_int_01",
		"v_row.ext_int_15",
		"v_row.ext_uuid_01",
		"v_row.ext_uuid_10",
		"v_row.ext_bool_01",
		"v_row.ext_bool_15",
		"v_row.ext_date_01",
		"v_row.ext_date_15",
		"v_row.ext_num_01",
		"v_row.ext_num_10",
		"v_row.ext_labels_snapshot",
		"u.ext_str_01",
		"u.ext_str_70",
		"u.ext_int_01",
		"u.ext_int_15",
		"u.ext_uuid_01",
		"u.ext_uuid_10",
		"u.ext_bool_01",
		"u.ext_bool_15",
		"u.ext_date_01",
		"u.ext_date_15",
		"u.ext_num_01",
		"u.ext_num_10",
		"u.ext_labels_snapshot",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitExtPayloadEngine_SubmitAndReplayCallApplyExtPayload(t *testing.T) {
	root := repoRootFromCurrentFile(t)

	enginePath := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql")
	engineBytes, err := os.ReadFile(enginePath)
	if err != nil {
		t.Fatalf("read %s: %v", enginePath, err)
	}
	if !strings.Contains(string(engineBytes), "PERFORM orgunit.apply_org_event_ext_payload(") {
		t.Fatalf("missing apply_org_event_ext_payload call in %s", enginePath)
	}

	submitPath := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql")
	submitBytes, err := os.ReadFile(submitPath)
	if err != nil {
		t.Fatalf("read %s: %v", submitPath, err)
	}
	if !strings.Contains(string(submitBytes), "PERFORM orgunit.apply_org_event_ext_payload(") {
		t.Fatalf("missing apply_org_event_ext_payload call in %s", submitPath)
	}
}
