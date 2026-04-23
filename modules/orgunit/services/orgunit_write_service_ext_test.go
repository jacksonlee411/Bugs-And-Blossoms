package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestCorrectExtPatchDictAddsLabelSnapshot(t *testing.T) {
	var captured map[string]any
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{
				EventType:     types.OrgUnitEventRename,
				EffectiveDate: "2026-01-01",
			}, nil
		},
		listEnabledFieldCfgsFn: func(_ context.Context, _ string, _ string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{
				{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
				{FieldKey: "short_name", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)},
			}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, patch json.RawMessage, _ string, _ string) (string, error) {
			if err := json.Unmarshal(patch, &captured); err != nil {
				return "", err
			}
			return "corr", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			Name: new("New Name"),
			Ext: map[string]any{
				"org_type":   "10",
				"short_name": "R&D",
			},
		},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if captured["new_name"] != "New Name" {
		t.Fatalf("patch=%v", captured)
	}
	ext, _ := captured["ext"].(map[string]any)
	if ext["org_type"] != "10" || ext["short_name"] != "R&D" {
		t.Fatalf("ext=%v", ext)
	}
	labels, _ := captured["ext_labels_snapshot"].(map[string]any)
	if labels["org_type"] != "Department" {
		t.Fatalf("labels=%v", labels)
	}
	if _, ok := labels["short_name"]; ok {
		t.Fatalf("plain field should not have label snapshot: %v", labels)
	}
}

func TestCorrectExtPatchDictClearDoesNotGenerateLabelSnapshot(t *testing.T) {
	var captured map[string]any
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{
				EventType:     types.OrgUnitEventRename,
				EffectiveDate: "2026-01-01",
			}, nil
		},
		listEnabledFieldCfgsFn: func(_ context.Context, _ string, _ string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{
				{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
			}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, patch json.RawMessage, _ string, _ string) (string, error) {
			if err := json.Unmarshal(patch, &captured); err != nil {
				return "", err
			}
			return "corr", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			Ext: map[string]any{
				"org_type": nil,
			},
		},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	ext, _ := captured["ext"].(map[string]any)
	if value, ok := ext["org_type"]; !ok || value != nil {
		t.Fatalf("ext=%v", ext)
	}
	if _, ok := captured["ext_labels_snapshot"]; ok {
		t.Fatalf("expected no label snapshot when clearing dict field: %v", captured)
	}
}

func TestCorrectExtPatchValidationFailClosed(t *testing.T) {
	t.Run("effective_date correction can include ext (DEV-PLAN-108)", func(t *testing.T) {
		origResolve := resolveDictLabelInWrite
		resolveDictLabelInWrite = func(_ context.Context, _ string, asOf string, _ string, code string) (string, bool, error) {
			if strings.TrimSpace(asOf) != "2026-01-02" {
				return "", false, errors.New("as_of mismatch")
			}
			if strings.TrimSpace(code) != "10" {
				return "", false, errors.New("code mismatch")
			}
			return "Org Type 10", true, nil
		}
		t.Cleanup(func() { resolveDictLabelInWrite = origResolve })

		var captured map[string]any
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 10000001, nil
			},
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventRename,
					EffectiveDate: "2026-01-01",
				}, nil
			},
			listEnabledFieldCfgsFn: func(_ context.Context, _ string, _ string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{
					{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
				}, nil
			},
			submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, patch json.RawMessage, _ string, _ string) (string, error) {
				if err := json.Unmarshal(patch, &captured); err != nil {
					return "", err
				}
				return "c1", nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				EffectiveDate: new("2026-01-02"),
				Ext: map[string]any{
					"org_type": "10",
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if captured == nil {
			t.Fatalf("expected patch captured")
		}
		if captured["effective_date"] != "2026-01-02" {
			t.Fatalf("patch=%v", captured)
		}
		ext, _ := captured["ext"].(map[string]any)
		if ext == nil || ext["org_type"] != "10" {
			t.Fatalf("patch=%v", captured)
		}
		labels, _ := captured["ext_labels_snapshot"].(map[string]any)
		if labels == nil || labels["org_type"] != "Org Type 10" {
			t.Fatalf("patch=%v", captured)
		}
	})

	t.Run("ext field not enabled is rejected", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 10000001, nil
			},
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventRename,
					EffectiveDate: "2026-01-01",
				}, nil
			},
			listEnabledFieldCfgsFn: func(_ context.Context, _ string, _ string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				Ext: map[string]any{
					"org_type": "10",
				},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("dict invalid option returns ORG_INVALID_ARGUMENT", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 10000001, nil
			},
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventRename,
					EffectiveDate: "2026-01-01",
				}, nil
			},
			listEnabledFieldCfgsFn: func(_ context.Context, _ string, _ string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{
					{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
				}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				Ext: map[string]any{
					"org_type": "NO_SUCH_OPTION",
				},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("expected ORG_INVALID_ARGUMENT, got %v", err)
		}
	})
}

func TestListEnabledExtFieldConfigs_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("store error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		})
		if _, _, err := svc.listEnabledExtFieldConfigs(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("skip blank and trim", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{
					{FieldKey: " "},
					{FieldKey: " org_type "},
				}, nil
			},
		})
		cfgs, keys, err := svc.listEnabledExtFieldConfigs(ctx, "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(cfgs) != 1 || cfgs[0].FieldKey != "org_type" {
			t.Fatalf("cfgs=%v", cfgs)
		}
		if joinStrings(keys) != "org_type" {
			t.Fatalf("keys=%v", keys)
		}
	})

	t.Run("reserved keys are filtered", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{
					{FieldKey: "name"},
					{FieldKey: "ext"},
					{FieldKey: "org_type"},
				}, nil
			},
		})
		cfgs, keys, err := svc.listEnabledExtFieldConfigs(ctx, "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(cfgs) != 1 || cfgs[0].FieldKey != "org_type" {
			t.Fatalf("cfgs=%v", cfgs)
		}
		if joinStrings(keys) != "org_type" {
			t.Fatalf("keys=%v", keys)
		}
	})

	t.Run("custom d_ field keeps only strict dict mapping", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{
					{FieldKey: "d_org_type", ValueType: "int", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
					{FieldKey: "d_org_type", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
					{FieldKey: "d_org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"other"}`)},
					{FieldKey: "d_org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
					{FieldKey: "x_cost_center", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)},
				}, nil
			},
		})
		cfgs, keys, err := svc.listEnabledExtFieldConfigs(ctx, "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(cfgs) != 2 {
			t.Fatalf("cfgs=%v", cfgs)
		}
		if joinStrings(keys) != "d_org_type,x_cost_center" {
			t.Fatalf("keys=%v", keys)
		}
	})
}

func TestBuildExtPayload_Branches(t *testing.T) {
	fieldConfigs := []types.TenantFieldConfig{
		{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		{FieldKey: "short_name", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)},
	}

	t.Run("success with dict labels and plain field", func(t *testing.T) {
		ext, labels, err := buildExtPayload(map[string]any{
			"org_type":   "10",
			"short_name": "R&D",
		}, fieldConfigs)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ext["org_type"] != "10" || ext["short_name"] != "R&D" {
			t.Fatalf("ext=%v", ext)
		}
		if labels["org_type"] != "Department" {
			t.Fatalf("labels=%v", labels)
		}
	})

	t.Run("dict nil does not generate label", func(t *testing.T) {
		ext, labels, err := buildExtPayload(map[string]any{
			"org_type": nil,
		}, fieldConfigs)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if _, ok := labels["org_type"]; ok {
			t.Fatalf("labels=%v", labels)
		}
		if _, ok := ext["org_type"]; !ok {
			t.Fatalf("ext=%v", ext)
		}
	})

	t.Run("invalid key branches", func(t *testing.T) {
		cases := []map[string]any{
			{" ": "x"},
			{"name": "x"},
			{"unknown_field": "x"},
		}
		for _, payload := range cases {
			if _, _, err := buildExtPayload(payload, fieldConfigs); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
				t.Fatalf("payload=%v err=%v", payload, err)
			}
		}
	})

	t.Run("unknown field definition is rejected", func(t *testing.T) {
		cfgs := []types.TenantFieldConfig{
			{FieldKey: "unknown_field", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)},
		}
		if _, _, err := buildExtPayload(map[string]any{"unknown_field": "x"}, cfgs); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("custom plain field in x_ namespace is accepted", func(t *testing.T) {
		cfgs := []types.TenantFieldConfig{
			{FieldKey: "x_cost_center", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)},
		}
		ext, labels, err := buildExtPayload(map[string]any{"x_cost_center": "CC-001"}, cfgs)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ext["x_cost_center"] != "CC-001" {
			t.Fatalf("ext=%v", ext)
		}
		if len(labels) != 0 {
			t.Fatalf("labels=%v", labels)
		}
	})

	t.Run("dict validation branches", func(t *testing.T) {
		if _, _, err := buildExtPayload(map[string]any{"org_type": 1}, fieldConfigs); err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := buildExtPayload(map[string]any{"org_type": " "}, fieldConfigs); err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := buildExtPayload(map[string]any{"org_type": "NO_SUCH"}, fieldConfigs); err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
		badCfg := []types.TenantFieldConfig{
			{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{}`)},
		}
		if _, _, err := buildExtPayload(map[string]any{"org_type": "10"}, badCfg); err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestValidateExtFieldKeyEnabled_Branches(t *testing.T) {
	if err := validateExtFieldKeyEnabled("short_name", types.TenantFieldConfig{FieldKey: "short_name", ValueType: "text", DataSourceType: "PLAIN"}); err != nil {
		t.Fatalf("builtin key err=%v", err)
	}
	if err := validateExtFieldKeyEnabled("x_cost_center", types.TenantFieldConfig{FieldKey: "x_cost_center", ValueType: "text", DataSourceType: "PLAIN"}); err != nil {
		t.Fatalf("custom key err=%v", err)
	}
	if err := validateExtFieldKeyEnabled("d_org_type", types.TenantFieldConfig{FieldKey: "d_org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}); err != nil {
		t.Fatalf("dict key err=%v", err)
	}
	if err := validateExtFieldKeyEnabled("unknown_field", types.TenantFieldConfig{FieldKey: "unknown_field", ValueType: "text", DataSourceType: "PLAIN"}); err == nil {
		t.Fatal("expected unknown field rejected")
	}
	if err := validateExtFieldKeyEnabled("d_org_type", types.TenantFieldConfig{FieldKey: "d_org_type", ValueType: "int", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}); err == nil {
		t.Fatal("expected dict non-text rejected")
	}
	if err := validateExtFieldKeyEnabled("d_org_type", types.TenantFieldConfig{FieldKey: "d_org_type", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}); err == nil {
		t.Fatal("expected dict non-dict type rejected")
	}
	if err := validateExtFieldKeyEnabled("d_org_type", types.TenantFieldConfig{FieldKey: "d_org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"other"}`)}); err == nil {
		t.Fatal("expected dict key/config mismatch rejected")
	}
	if err := validateExtFieldKeyEnabled("x_cost_center", types.TenantFieldConfig{FieldKey: "x_cost_center", ValueType: "int", DataSourceType: "PLAIN"}); err != nil {
		t.Fatalf("custom int key err=%v", err)
	}
	if err := validateExtFieldKeyEnabled("x_cost_center", types.TenantFieldConfig{FieldKey: "x_cost_center", ValueType: "numeric", DataSourceType: "PLAIN"}); err != nil {
		t.Fatalf("custom numeric key err=%v", err)
	}
	if err := validateExtFieldKeyEnabled("x_cost_center", types.TenantFieldConfig{FieldKey: "x_cost_center", ValueType: "json", DataSourceType: "PLAIN"}); err == nil {
		t.Fatal("expected custom invalid value_type rejected")
	}
	if err := validateExtFieldKeyEnabled("x_cost_center", types.TenantFieldConfig{FieldKey: "x_cost_center", ValueType: "text", DataSourceType: "DICT"}); err == nil {
		t.Fatal("expected custom non-plain rejected")
	}
}

func TestAppendActions_ExtPayloadAndLabels(t *testing.T) {
	ctx := context.Background()
	cfgs := []types.TenantFieldConfig{
		{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
	}

	t.Run("create includes ext payload and labels", func(t *testing.T) {
		var payload map[string]any
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
				if orgCode == "A001" {
					return 0, orgunit.ErrOrgCodeNotFound
				}
				return 0, errors.New("unexpected org code")
			},
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return cfgs, nil
			},
			submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, raw json.RawMessage, _ string, _ string) (int64, error) {
				if err := json.Unmarshal(raw, &payload); err != nil {
					return 0, err
				}
				return 1, nil
			},
			findEventByUUIDFn: func(context.Context, string, string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{OrgNodeKey: "10000001"}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Name:          "Org A",
			Ext:           map[string]any{"org_type": "10"},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if payload["ext"] == nil || payload["ext_labels_snapshot"] == nil {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("rename/move/disable/enable/set business unit include ext payload", func(t *testing.T) {
		cases := []struct {
			name string
			call func(OrgUnitWriteService) error
		}{
			{
				name: "rename",
				call: func(svc OrgUnitWriteService) error {
					return svc.Rename(ctx, "t1", RenameOrgUnitRequest{
						EffectiveDate: "2026-01-01",
						OrgCode:       "A001",
						NewName:       "Org B",
						Ext:           map[string]any{"org_type": "10"},
					})
				},
			},
			{
				name: "move",
				call: func(svc OrgUnitWriteService) error {
					return svc.Move(ctx, "t1", MoveOrgUnitRequest{
						EffectiveDate:    "2026-01-01",
						OrgCode:          "A001",
						NewParentOrgCode: "P001",
						Ext:              map[string]any{"org_type": "10"},
					})
				},
			},
			{
				name: "disable",
				call: func(svc OrgUnitWriteService) error {
					return svc.Disable(ctx, "t1", DisableOrgUnitRequest{
						EffectiveDate: "2026-01-01",
						OrgCode:       "A001",
						Ext:           map[string]any{"org_type": "10"},
					})
				},
			},
			{
				name: "enable",
				call: func(svc OrgUnitWriteService) error {
					return svc.Enable(ctx, "t1", EnableOrgUnitRequest{
						EffectiveDate: "2026-01-01",
						OrgCode:       "A001",
						Ext:           map[string]any{"org_type": "10"},
					})
				},
			},
			{
				name: "set_business_unit",
				call: func(svc OrgUnitWriteService) error {
					return svc.SetBusinessUnit(ctx, "t1", SetBusinessUnitRequest{
						EffectiveDate:  "2026-01-01",
						OrgCode:        "A001",
						IsBusinessUnit: true,
						Ext:            map[string]any{"org_type": "10"},
					})
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var payload map[string]any
				store := orgUnitWriteStoreStub{
					resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
						switch orgCode {
						case "A001":
							return 10000001, nil
						case "P001":
							return 10000002, nil
						default:
							return 0, orgunit.ErrOrgCodeNotFound
						}
					},
					listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
						return cfgs, nil
					},
					submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, raw json.RawMessage, _ string, _ string) (int64, error) {
						if err := json.Unmarshal(raw, &payload); err != nil {
							return 0, err
						}
						return 1, nil
					},
				}
				svc := NewOrgUnitWriteService(store)
				if err := tc.call(svc); err != nil {
					t.Fatalf("err=%v", err)
				}
				if payload["ext"] == nil || payload["ext_labels_snapshot"] == nil {
					t.Fatalf("payload=%v", payload)
				}
			})
		}
	})
}

func TestAppendActions_PolicyFailClosedBranches(t *testing.T) {
	orig := resolveOrgUnitMutationPolicyInWrite
	resolveOrgUnitMutationPolicyInWrite = func(key OrgUnitMutationPolicyKey, _ OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
		if key.EmittedEventType == OrgUnitEmittedMove {
			return OrgUnitMutationPolicyDecision{Enabled: false, DenyReasons: []string{"ORG_ROOT_CANNOT_BE_MOVED"}}, nil
		}
		return OrgUnitMutationPolicyDecision{Enabled: false}, nil
	}
	t.Cleanup(func() { resolveOrgUnitMutationPolicyInWrite = orig })

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			switch orgCode {
			case "A001":
				return 10000001, nil
			case "P001":
				return 10000002, nil
			case "NEW1":
				return 0, orgunit.ErrOrgCodeNotFound
			default:
				return 0, orgunit.ErrOrgCodeNotFound
			}
		},
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		findEventByUUIDFn: func(context.Context, string, string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{OrgNodeKey: "10000001"}, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "NEW1", Name: "New"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected create fail-closed, got %v", err)
	}
	if err := svc.Rename(ctx, "t1", RenameOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewName: "N"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected rename fail-closed, got %v", err)
	}
	if err := svc.Move(ctx, "t1", MoveOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewParentOrgCode: "P001"}); err == nil || err.Error() != "ORG_ROOT_CANNOT_BE_MOVED" {
		t.Fatalf("expected move deny reason, got %v", err)
	}
	if err := svc.Disable(ctx, "t1", DisableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected disable fail-closed, got %v", err)
	}
	if err := svc.Enable(ctx, "t1", EnableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected enable fail-closed, got %v", err)
	}
	if err := svc.SetBusinessUnit(ctx, "t1", SetBusinessUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", IsBusinessUnit: true}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected set BU fail-closed, got %v", err)
	}
}

func TestCreate_AppendAdditionalErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("org already exists", func(t *testing.T) {
		svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
		})
		_, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Name:          "Org A",
		})
		if err == nil || err.Error() != "ORG_ALREADY_EXISTS" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parent resolve unexpected error", func(t *testing.T) {
		svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
				if orgCode == "A001" {
					return 0, orgunit.ErrOrgCodeNotFound
				}
				return 0, errors.New("boom")
			},
		})
		_, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Name:          "Org A",
			ParentOrgCode: "P001",
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("list config error", func(t *testing.T) {
		svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunit.ErrOrgCodeNotFound },
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		})
		_, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Name:          "Org A",
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("policy resolve error", func(t *testing.T) {
		orig := resolveOrgUnitMutationPolicyInWrite
		resolveOrgUnitMutationPolicyInWrite = func(OrgUnitMutationPolicyKey, OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
			return OrgUnitMutationPolicyDecision{}, errors.New("boom")
		}
		t.Cleanup(func() { resolveOrgUnitMutationPolicyInWrite = orig })

		svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunit.ErrOrgCodeNotFound },
		})
		_, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Name:          "Org A",
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("build ext payload error", func(t *testing.T) {
		svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunit.ErrOrgCodeNotFound },
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
			},
		})
		_, err := svc.Create(ctx, "t1", CreateOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Name:          "Org A",
			Ext:           map[string]any{"unknown_field": "x"},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestAppendActions_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	baseStore := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			switch orgCode {
			case "A001":
				return 10000001, nil
			case "P001":
				return 10000002, nil
			default:
				return 0, orgunit.ErrOrgCodeNotFound
			}
		},
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
		},
	}

	t.Run("list config errors", func(t *testing.T) {
		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return nil, errors.New("boom")
		}
		svc := NewOrgUnitWriteService(store)
		cases := []struct {
			name string
			call func() error
		}{
			{"rename", func() error {
				return svc.Rename(ctx, "t1", RenameOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewName: "X"})
			}},
			{"move", func() error {
				return svc.Move(ctx, "t1", MoveOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewParentOrgCode: "P001"})
			}},
			{"disable", func() error {
				return svc.Disable(ctx, "t1", DisableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001"})
			}},
			{"enable", func() error {
				return svc.Enable(ctx, "t1", EnableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001"})
			}},
			{"set_bu", func() error {
				return svc.SetBusinessUnit(ctx, "t1", SetBusinessUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", IsBusinessUnit: true})
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				if err == nil || err.Error() != "boom" {
					t.Fatalf("err=%v", err)
				}
			})
		}
	})

	t.Run("policy resolve errors", func(t *testing.T) {
		orig := resolveOrgUnitMutationPolicyInWrite
		resolveOrgUnitMutationPolicyInWrite = func(OrgUnitMutationPolicyKey, OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
			return OrgUnitMutationPolicyDecision{}, errors.New("boom")
		}
		t.Cleanup(func() { resolveOrgUnitMutationPolicyInWrite = orig })

		svc := NewOrgUnitWriteService(baseStore)
		cases := []struct {
			name string
			call func() error
		}{
			{"rename", func() error {
				return svc.Rename(ctx, "t1", RenameOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewName: "X"})
			}},
			{"move", func() error {
				return svc.Move(ctx, "t1", MoveOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewParentOrgCode: "P001"})
			}},
			{"disable", func() error {
				return svc.Disable(ctx, "t1", DisableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001"})
			}},
			{"enable", func() error {
				return svc.Enable(ctx, "t1", EnableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001"})
			}},
			{"set_bu", func() error {
				return svc.SetBusinessUnit(ctx, "t1", SetBusinessUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", IsBusinessUnit: true})
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				if err == nil || err.Error() != "boom" {
					t.Fatalf("err=%v", err)
				}
			})
		}
	})

	t.Run("build ext payload errors", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		cases := []struct {
			name string
			call func() error
		}{
			{"rename", func() error {
				return svc.Rename(ctx, "t1", RenameOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewName: "X", Ext: map[string]any{"unknown_field": "x"}})
			}},
			{"move", func() error {
				return svc.Move(ctx, "t1", MoveOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewParentOrgCode: "P001", Ext: map[string]any{"unknown_field": "x"}})
			}},
			{"disable", func() error {
				return svc.Disable(ctx, "t1", DisableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", Ext: map[string]any{"unknown_field": "x"}})
			}},
			{"enable", func() error {
				return svc.Enable(ctx, "t1", EnableOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", Ext: map[string]any{"unknown_field": "x"}})
			}},
			{"set_bu", func() error {
				return svc.SetBusinessUnit(ctx, "t1", SetBusinessUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", IsBusinessUnit: true, Ext: map[string]any{"unknown_field": "x"}})
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
					t.Fatalf("err=%v", err)
				}
			})
		}
	})

	t.Run("move disabled without deny reasons", func(t *testing.T) {
		orig := resolveOrgUnitMutationPolicyInWrite
		resolveOrgUnitMutationPolicyInWrite = func(key OrgUnitMutationPolicyKey, facts OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
			if key.ActionKind == OrgUnitActionEventUpdate && key.EmittedEventType == OrgUnitEmittedMove {
				return OrgUnitMutationPolicyDecision{Enabled: false, DenyReasons: []string{}}, nil
			}
			return orig(key, facts)
		}
		t.Cleanup(func() { resolveOrgUnitMutationPolicyInWrite = orig })
		svc := NewOrgUnitWriteService(baseStore)
		err := svc.Move(ctx, "t1", MoveOrgUnitRequest{EffectiveDate: "2026-01-01", OrgCode: "A001", NewParentOrgCode: "P001"})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("disable/enable ext marshal errors", func(t *testing.T) {
		withMarshalJSON(t, func(any) ([]byte, error) {
			return nil, errors.New("marshal")
		})
		svc := NewOrgUnitWriteService(baseStore)
		if err := svc.Disable(ctx, "t1", DisableOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Ext:           map[string]any{"org_type": "10"},
		}); err == nil || err.Error() != "marshal" {
			t.Fatalf("disable err=%v", err)
		}
		if err := svc.Enable(ctx, "t1", EnableOrgUnitRequest{
			EffectiveDate: "2026-01-01",
			OrgCode:       "A001",
			Ext:           map[string]any{"org_type": "10"},
		}); err == nil || err.Error() != "marshal" {
			t.Fatalf("enable err=%v", err)
		}
	})
}

func TestCorrectExtPatch_AdditionalErrorBranches(t *testing.T) {
	t.Run("list enabled field configs error", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{EventType: types.OrgUnitEventRename, EffectiveDate: "2026-01-01"}, nil
			},
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch:               OrgUnitCorrectionPatch{Name: new("New")},
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom, got %v", err)
		}
	})

	t.Run("dict missing dict_code returns ORG_INVALID_ARGUMENT", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{EventType: types.OrgUnitEventRename, EffectiveDate: "2026-01-01"}, nil
			},
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{}`)}}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				Ext: map[string]any{"org_type": "10"},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("expected ORG_INVALID_ARGUMENT, got %v", err)
		}
	})

	t.Run("dict value not string returns ORG_INVALID_ARGUMENT", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{EventType: types.OrgUnitEventRename, EffectiveDate: "2026-01-01"}, nil
			},
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				Ext: map[string]any{"org_type": 1},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("expected ORG_INVALID_ARGUMENT, got %v", err)
		}
	})

	t.Run("dict value empty returns ORG_INVALID_ARGUMENT", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{EventType: types.OrgUnitEventRename, EffectiveDate: "2026-01-01"}, nil
			},
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				Ext: map[string]any{"org_type": " "},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("expected ORG_INVALID_ARGUMENT, got %v", err)
		}
	})

	t.Run("unknown field definition is rejected", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
				return types.OrgUnitEvent{EventType: types.OrgUnitEventRename, EffectiveDate: "2026-01-01"}, nil
			},
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{{FieldKey: "unknown_field", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)}}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "req1",
			Patch: OrgUnitCorrectionPatch{
				Ext: map[string]any{"unknown_field": "x"},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})
}

func joinStrings(items []string) string {
	var out strings.Builder
	for i, item := range items {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(item)
	}
	return out.String()
}
