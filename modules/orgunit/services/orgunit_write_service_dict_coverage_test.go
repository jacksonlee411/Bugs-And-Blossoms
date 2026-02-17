package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

func TestGlobalDictResolver_ListOptions(t *testing.T) {
	if err := dictpkg.RegisterResolver(orgUnitWriteDictResolverStub{}); err != nil {
		t.Fatalf("register err=%v", err)
	}

	opts, err := globalDictResolver{}.ListOptions(context.Background(), "t1", "2026-01-01", "org_type", "dep", 1)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opts) != 1 || opts[0].Code != "10" {
		t.Fatalf("opts=%+v", opts)
	}
}

func TestBuildCorrectionPatch_UsesCorrectedDateForDictLookup(t *testing.T) {
	svc := newWriteService(orgUnitWriteStoreStub{})
	orig := resolveDictLabelInWrite
	defer func() { resolveDictLabelInWrite = orig }()

	var gotAsOf string
	resolveDictLabelInWrite = func(_ context.Context, _ string, asOf string, dictCode string, code string) (string, bool, error) {
		gotAsOf = asOf
		if dictCode != "org_type" || code != "10" {
			t.Fatalf("dictCode=%q code=%q", dictCode, code)
		}
		return "Department", true, nil
	}

	newDate := "2026-03-01"
	patchMap, fields, correctedDate, err := svc.buildCorrectionPatch(
		context.Background(),
		"t1",
		types.OrgUnitEvent{EventType: types.OrgUnitEventCreate, EffectiveDate: "2026-01-01"},
		OrgUnitCorrectionPatch{
			EffectiveDate: &newDate,
			Ext:           map[string]any{"org_type": "10"},
		},
		[]types.TenantFieldConfig{{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if correctedDate != newDate || gotAsOf != newDate {
		t.Fatalf("correctedDate=%q gotAsOf=%q", correctedDate, gotAsOf)
	}
	if _, ok := patchMap["ext_labels_snapshot"]; !ok {
		t.Fatalf("patchMap=%v", patchMap)
	}
	if _, ok := fields["ext"]; !ok {
		t.Fatalf("fields=%v", fields)
	}
}

func TestBuildExtPayloadWithContext_BlankConfigKeySkipped(t *testing.T) {
	orig := resolveDictLabelInWrite
	defer func() { resolveDictLabelInWrite = orig }()

	resolveDictLabelInWrite = func(_ context.Context, _ string, asOf string, dictCode string, code string) (string, bool, error) {
		if asOf != "2026-01-01" || dictCode != "org_type" || code != "10" {
			t.Fatalf("asOf=%q dictCode=%q code=%q", asOf, dictCode, code)
		}
		return "Department", true, nil
	}

	extPayload, extLabels, err := buildExtPayloadWithContext(
		context.Background(),
		"t1",
		"2026-01-01",
		map[string]any{"org_type": "10"},
		[]types.TenantFieldConfig{
			{FieldKey: " ", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)},
			{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(extPayload) != 1 || extPayload["org_type"] != "10" {
		t.Fatalf("extPayload=%v", extPayload)
	}
	if extLabels["org_type"] != "Department" {
		t.Fatalf("extLabels=%v", extLabels)
	}
}
