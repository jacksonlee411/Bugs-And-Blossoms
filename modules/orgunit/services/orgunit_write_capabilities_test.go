package services

import (
	"reflect"
	"testing"
)

func TestResolveWriteCapabilities_InvalidIntent(t *testing.T) {
	if _, err := ResolveWriteCapabilities(OrgUnitWriteIntent("nope"), nil, OrgUnitWriteCapabilitiesFacts{}); err == nil {
		t.Fatalf("expected err")
	}
}

func TestResolveWriteCapabilities_DenyReasonsSortedAndDeduped(t *testing.T) {
	decision, err := ResolveWriteCapabilities(
		OrgUnitWriteIntentAddVersion,
		[]string{" org_type ", "name", "org_type", "ext_labels_snapshot", ""},
		OrgUnitWriteCapabilitiesFacts{
			CanAdmin:         false,
			TreeInitialized:  false,
			TargetExistsAsOf: false,
			OrgCode:          "A001",
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.Enabled {
		t.Fatalf("expected disabled")
	}
	want := []string{"FORBIDDEN", "ORG_TREE_NOT_INITIALIZED", "ORG_NOT_FOUND_AS_OF"}
	if !reflect.DeepEqual(decision.DenyReasons, want) {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}
}

func TestResolveWriteCapabilities_CreateRootAlreadyExists(t *testing.T) {
	decision, err := ResolveWriteCapabilities(
		OrgUnitWriteIntentCreateOrg,
		nil,
		OrgUnitWriteCapabilitiesFacts{
			CanAdmin:         true,
			TreeInitialized:  true,
			OrgAlreadyExists: true,
			OrgCode:          "root",
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.Enabled {
		t.Fatalf("expected disabled")
	}
	want := []string{"ORG_ALREADY_EXISTS", "ORG_ROOT_ALREADY_EXISTS"}
	if !reflect.DeepEqual(decision.DenyReasons, want) {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}
}

func TestResolveWriteCapabilities_EnabledPayloadKeys(t *testing.T) {
	decision, err := ResolveWriteCapabilities(
		OrgUnitWriteIntentCorrect,
		[]string{"org_type", "x_custom_text", "name", "parent_org_code"},
		OrgUnitWriteCapabilitiesFacts{
			CanAdmin:         true,
			TreeInitialized:  true,
			TargetExistsAsOf: true,
			OrgCode:          "A001",
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !decision.Enabled {
		t.Fatalf("expected enabled")
	}
	if decision.FieldPayloadKeys["parent_org_code"] != "parent_id" {
		t.Fatalf("field_payload_keys=%v", decision.FieldPayloadKeys)
	}
	if decision.FieldPayloadKeys["org_type"] != "ext.org_type" {
		t.Fatalf("field_payload_keys=%v", decision.FieldPayloadKeys)
	}
	if decision.FieldPayloadKeys["x_custom_text"] != "ext.x_custom_text" {
		t.Fatalf("field_payload_keys=%v", decision.FieldPayloadKeys)
	}
}

func TestResolveWriteCapabilities_CoversMoreDenyReasons(t *testing.T) {
	t.Run("create non-root requires tree initialized", func(t *testing.T) {
		decision, err := ResolveWriteCapabilities(
			OrgUnitWriteIntentCreateOrg,
			nil,
			OrgUnitWriteCapabilitiesFacts{
				CanAdmin:        true,
				TreeInitialized: false,
				OrgCode:         "A001",
			},
		)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if decision.Enabled {
			t.Fatalf("expected disabled")
		}
		if !reflect.DeepEqual(decision.DenyReasons, []string{"ORG_TREE_NOT_INITIALIZED"}) {
			t.Fatalf("deny=%v", decision.DenyReasons)
		}
	})

	t.Run("correct event not found", func(t *testing.T) {
		decision, err := ResolveWriteCapabilities(
			OrgUnitWriteIntentCorrect,
			nil,
			OrgUnitWriteCapabilitiesFacts{
				CanAdmin:            true,
				TreeInitialized:     true,
				TargetExistsAsOf:    true,
				OrgCode:             "A001",
				TargetEventNotFound: true,
			},
		)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if decision.Enabled {
			t.Fatalf("expected disabled")
		}
		if !reflect.DeepEqual(decision.DenyReasons, []string{"ORG_EVENT_NOT_FOUND"}) {
			t.Fatalf("deny=%v", decision.DenyReasons)
		}
	})

	t.Run("correct event rescinded", func(t *testing.T) {
		decision, err := ResolveWriteCapabilities(
			OrgUnitWriteIntentCorrect,
			nil,
			OrgUnitWriteCapabilitiesFacts{
				CanAdmin:             true,
				TreeInitialized:      true,
				TargetExistsAsOf:     true,
				OrgCode:              "A001",
				TargetEventRescinded: true,
			},
		)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if decision.Enabled {
			t.Fatalf("expected disabled")
		}
		if !reflect.DeepEqual(decision.DenyReasons, []string{"ORG_EVENT_RESCINDED"}) {
			t.Fatalf("deny=%v", decision.DenyReasons)
		}
	})

	t.Run("correct tree not initialized and target missing", func(t *testing.T) {
		decision, err := ResolveWriteCapabilities(
			OrgUnitWriteIntentCorrect,
			nil,
			OrgUnitWriteCapabilitiesFacts{
				CanAdmin:         true,
				TreeInitialized:  false,
				TargetExistsAsOf: false,
				OrgCode:          "A001",
			},
		)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if decision.Enabled {
			t.Fatalf("expected disabled")
		}
		if !reflect.DeepEqual(decision.DenyReasons, []string{"ORG_TREE_NOT_INITIALIZED", "ORG_NOT_FOUND_AS_OF"}) {
			t.Fatalf("deny=%v", decision.DenyReasons)
		}
	})
}

func TestWriteCapabilitiesHelpers_CoverBranches(t *testing.T) {
	if got := normalizeWriteExtFieldKeys(nil); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
	keys := normalizeWriteExtFieldKeys([]string{"", " name ", "org_type", "org_type", "ext", "x_custom"})
	if !reflect.DeepEqual(keys, []string{"org_type", "x_custom"}) {
		t.Fatalf("keys=%v", keys)
	}

	if !isReservedWriteExtFieldKey(" name ") {
		t.Fatalf("expected reserved")
	}
	if !isReservedWriteExtFieldKey("ext_labels_snapshot") {
		t.Fatalf("expected reserved")
	}
	if isReservedWriteExtFieldKey("org_type") {
		t.Fatalf("expected not reserved")
	}

	merged := mergeAndSortWriteKeys([]string{"a", " a ", "", "b"}, []string{"b", "c", " "})
	if !reflect.DeepEqual(merged, []string{"a", "b", "c"}) {
		t.Fatalf("merged=%v", merged)
	}

	if got := dedupAndSortWriteDenyReasons([]string{}); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
	got := dedupAndSortWriteDenyReasons([]string{" X ", "FORBIDDEN", "X", "", "ORG_TREE_NOT_INITIALIZED"})
	if !reflect.DeepEqual(got, []string{"FORBIDDEN", "ORG_TREE_NOT_INITIALIZED", "X"}) {
		t.Fatalf("got=%v", got)
	}

	if writeDenyReasonPriority("FORBIDDEN") != 10 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("FORBIDDEN"))
	}
	if writeDenyReasonPriority("ORG_TREE_NOT_INITIALIZED") != 20 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("ORG_TREE_NOT_INITIALIZED"))
	}
	if writeDenyReasonPriority("ORG_NOT_FOUND_AS_OF") != 30 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("ORG_NOT_FOUND_AS_OF"))
	}
	if writeDenyReasonPriority("ORG_ALREADY_EXISTS") != 40 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("ORG_ALREADY_EXISTS"))
	}
	if writeDenyReasonPriority("ORG_ROOT_ALREADY_EXISTS") != 50 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("ORG_ROOT_ALREADY_EXISTS"))
	}
	if writeDenyReasonPriority("ORG_EVENT_NOT_FOUND") != 60 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("ORG_EVENT_NOT_FOUND"))
	}
	if writeDenyReasonPriority("ORG_EVENT_RESCINDED") != 70 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("ORG_EVENT_RESCINDED"))
	}
	if writeDenyReasonPriority("X") != 100 {
		t.Fatalf("priority=%d", writeDenyReasonPriority("X"))
	}
}
