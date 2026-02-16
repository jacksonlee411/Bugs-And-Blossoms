package services

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitWriteDictResolverStub struct{}

func (orgUnitWriteDictResolverStub) ResolveValueLabel(_ context.Context, _ string, _ string, dictCode string, code string) (string, bool, error) {
	if strings.TrimSpace(dictCode) != "org_type" {
		return "", false, nil
	}
	switch strings.TrimSpace(code) {
	case "10":
		return "Department", true, nil
	case "20":
		return "Company", true, nil
	default:
		return "", false, nil
	}
}

func (orgUnitWriteDictResolverStub) ListOptions(_ context.Context, _ string, _ string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	if strings.TrimSpace(dictCode) != "org_type" {
		return []dictpkg.Option{}, nil
	}
	options := []dictpkg.Option{
		{Code: "10", Label: "Department", Status: "active", EnabledOn: "1970-01-01"},
		{Code: "20", Label: "Company", Status: "active", EnabledOn: "1970-01-01"},
	}
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle != "" {
		filtered := make([]dictpkg.Option, 0, len(options))
		for _, option := range options {
			if strings.Contains(strings.ToLower(option.Code), needle) || strings.Contains(strings.ToLower(option.Label), needle) {
				filtered = append(filtered, option)
			}
		}
		options = filtered
	}
	if limit > 0 && len(options) > limit {
		options = options[:limit]
	}
	return options, nil
}

func TestMain(m *testing.M) {
	_ = dictpkg.RegisterResolver(orgUnitWriteDictResolverStub{})
	os.Exit(m.Run())
}

type orgUnitWriteStoreStub struct {
	submitEventFn            func(ctx context.Context, tenantID string, eventUUID string, orgID *int, eventType string, effectiveDate string, payload json.RawMessage, requestCode string, initiatorUUID string) (int64, error)
	submitCorrectionFn       func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error)
	submitStatusCorrectionFn func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error)
	submitRescindEventFn     func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error)
	submitRescindOrgFn       func(ctx context.Context, tenantID string, orgID int, reason string, requestID string, initiatorUUID string) (int, error)
	findEventByUUIDFn        func(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error)
	findEventByEffectiveFn   func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (types.OrgUnitEvent, error)
	listEnabledFieldCfgsFn   func(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error)
	resolveOrgIDFn           func(ctx context.Context, tenantID string, orgCode string) (int, error)
	resolveOrgCodeFn         func(ctx context.Context, tenantID string, orgID int) (string, error)
	findPersonByPernrFn      func(ctx context.Context, tenantID string, pernr string) (types.Person, error)
}

func (s orgUnitWriteStoreStub) SubmitEvent(ctx context.Context, tenantID string, eventUUID string, orgID *int, eventType string, effectiveDate string, payload json.RawMessage, requestCode string, initiatorUUID string) (int64, error) {
	if s.submitEventFn == nil {
		return 0, errors.New("SubmitEvent not mocked")
	}
	return s.submitEventFn(ctx, tenantID, eventUUID, orgID, eventType, effectiveDate, payload, requestCode, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitCorrection(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error) {
	if s.submitCorrectionFn == nil {
		return "", errors.New("SubmitCorrection not mocked")
	}
	return s.submitCorrectionFn(ctx, tenantID, orgID, targetEffectiveDate, patch, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitStatusCorrection(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error) {
	if s.submitStatusCorrectionFn == nil {
		return "", errors.New("SubmitStatusCorrection not mocked")
	}
	return s.submitStatusCorrectionFn(ctx, tenantID, orgID, targetEffectiveDate, targetStatus, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitRescindEvent(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error) {
	if s.submitRescindEventFn == nil {
		return "", errors.New("SubmitRescindEvent not mocked")
	}
	return s.submitRescindEventFn(ctx, tenantID, orgID, targetEffectiveDate, reason, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitRescindOrg(ctx context.Context, tenantID string, orgID int, reason string, requestID string, initiatorUUID string) (int, error) {
	if s.submitRescindOrgFn == nil {
		return 0, errors.New("SubmitRescindOrg not mocked")
	}
	return s.submitRescindOrgFn(ctx, tenantID, orgID, reason, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) FindEventByUUID(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error) {
	if s.findEventByUUIDFn == nil {
		return types.OrgUnitEvent{}, errors.New("FindEventByUUID not mocked")
	}
	return s.findEventByUUIDFn(ctx, tenantID, eventUUID)
}

func (s orgUnitWriteStoreStub) FindEventByEffectiveDate(ctx context.Context, tenantID string, orgID int, effectiveDate string) (types.OrgUnitEvent, error) {
	if s.findEventByEffectiveFn == nil {
		return types.OrgUnitEvent{}, errors.New("FindEventByEffectiveDate not mocked")
	}
	return s.findEventByEffectiveFn(ctx, tenantID, orgID, effectiveDate)
}

func (s orgUnitWriteStoreStub) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error) {
	if s.listEnabledFieldCfgsFn != nil {
		return s.listEnabledFieldCfgsFn(ctx, tenantID, asOf)
	}
	return []types.TenantFieldConfig{}, nil
}

func (s orgUnitWriteStoreStub) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveOrgIDFn == nil {
		return 0, errors.New("ResolveOrgID not mocked")
	}
	return s.resolveOrgIDFn(ctx, tenantID, orgCode)
}

func (s orgUnitWriteStoreStub) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	if s.resolveOrgCodeFn == nil {
		return "", errors.New("ResolveOrgCode not mocked")
	}
	return s.resolveOrgCodeFn(ctx, tenantID, orgID)
}

func (s orgUnitWriteStoreStub) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (types.Person, error) {
	if s.findPersonByPernrFn == nil {
		return types.Person{}, errors.New("FindPersonByPernr not mocked")
	}
	return s.findPersonByPernrFn(ctx, tenantID, pernr)
}

func withNewUUID(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := newUUID
	newUUID = fn
	t.Cleanup(func() { newUUID = orig })
}

func withMarshalJSON(t *testing.T, fn func(any) ([]byte, error)) {
	t.Helper()
	orig := marshalJSON
	marshalJSON = fn
	t.Cleanup(func() { marshalJSON = orig })
}

func newWriteService(store orgUnitWriteStoreStub) *orgUnitWriteService {
	return &orgUnitWriteService{store: store}
}

func TestCreateRejectsInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})

	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       " ",
		Name:          "Root",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org_code invalid error, got: %v", err)
	}
}

func TestCorrectMapsParentForMove(t *testing.T) {
	var captured map[string]any
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			switch orgCode {
			case "ROOT":
				return 10000001, nil
			case "PARENT":
				return 20000002, nil
			default:
				return 0, errors.New("unexpected org code")
			}
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, nil
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
		OrgCode:             "root",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("parent"),
		},
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if captured["new_parent_id"] != float64(20000002) && captured["new_parent_id"] != 20000002 {
		t.Fatalf("expected new_parent_id mapped, got %#v", captured)
	}
	if _, ok := captured["parent_id"]; ok {
		t.Fatalf("unexpected parent_id in patch: %#v", captured)
	}
}

func TestCorrectRejectsNameOnMove(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			Name: stringPtr("Rename"),
		},
	})
	if err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad request, got %v", err)
	}
}

func TestCorrectManagerPernrNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
		findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
			return types.Person{}, ports.ErrPersonNotFound
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			ManagerPernr: stringPtr("1001"),
		},
	})
	if err == nil || err.Error() != errManagerPernrNotFound {
		t.Fatalf("expected manager pernr not found, got %v", err)
	}
}

func TestCreateManagerPernrInactive(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
		findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
			return types.Person{UUID: "p1", Pernr: "1001", Status: "inactive"}, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
		findEventByUUIDFn: func(_ context.Context, _ string, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{OrgID: 10000001}, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
		ManagerPernr:  "1001",
	})
	if err == nil || err.Error() != errManagerPernrInactive {
		t.Fatalf("expected manager pernr inactive, got %v", err)
	}
}

func TestCorrectRequiresPatch(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchRequired {
		t.Fatalf("expected patch required, got %v", err)
	}
}

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
			Name: stringPtr("New Name"),
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
	t.Run("effective_date correction mode rejects ext", func(t *testing.T) {
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
				EffectiveDate: stringPtr("2026-01-02"),
				Ext: map[string]any{
					"org_type": "10",
				},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
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
					return 0, orgunitpkg.ErrOrgCodeNotFound
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
				return types.OrgUnitEvent{OrgID: 10000001}, nil
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
							return 0, orgunitpkg.ErrOrgCodeNotFound
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
				return 0, orgunitpkg.ErrOrgCodeNotFound
			default:
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
		},
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		findEventByUUIDFn: func(context.Context, string, string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{OrgID: 10000001}, nil
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
					return 0, orgunitpkg.ErrOrgCodeNotFound
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
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound },
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
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound },
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
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound },
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
				return 0, orgunitpkg.ErrOrgCodeNotFound
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
			Patch:               OrgUnitCorrectionPatch{Name: stringPtr("New")},
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
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ","
		}
		out += item
	}
	return out
}

func TestCorrectStatusSuccess(t *testing.T) {
	called := false
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode != "ROOT" {
				t.Fatalf("orgCode=%s", orgCode)
			}
			return 10000001, nil
		},
		submitStatusCorrectionFn: func(_ context.Context, tenantID string, orgID int, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error) {
			called = true
			if tenantID != "t1" || orgID != 10000001 || targetEffectiveDate != "2026-01-01" || targetStatus != "disabled" || requestID != "req-1" || initiatorUUID != "t1" {
				t.Fatalf("unexpected args: tenant=%s orgID=%d target=%s status=%s request=%s initiator=%s", tenantID, orgID, targetEffectiveDate, targetStatus, requestID, initiatorUUID)
			}
			return "corr", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	got, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{
		OrgCode:             "root",
		TargetEffectiveDate: "2026-01-01",
		TargetStatus:        "inactive",
		RequestID:           "req-1",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatalf("expected status correction call")
	}
	if got.OrgCode != "ROOT" || got.EffectiveDate != "2026-01-01" {
		t.Fatalf("result=%+v", got)
	}
	if got.Fields["target_status"] != "disabled" {
		t.Fatalf("fields=%+v", got.Fields)
	}
}

func TestCorrectStatusValidationAndErrors(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	if _, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{OrgCode: "ROOT", TargetStatus: "active", RequestID: "r1"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad date, got %v", err)
	}
	if _, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", RequestID: "r1"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad target status, got %v", err)
	}
	if _, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", TargetStatus: "active"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected request_id bad request, got %v", err)
	}

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", TargetStatus: "active", RequestID: "r1"}); err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org not found, got %v", err)
	}

	store = orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitStatusCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ string, _ string, _ string) (string, error) {
			return "", errors.New("submit")
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", TargetStatus: "active", RequestID: "r1"}); err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestRescindRecordSuccess(t *testing.T) {
	called := false
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode != "ROOT" {
				t.Fatalf("orgCode=%s", orgCode)
			}
			return 10000001, nil
		},
		submitRescindEventFn: func(_ context.Context, tenantID string, orgID int, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error) {
			called = true
			if tenantID != "t1" || orgID != 10000001 || targetEffectiveDate != "2026-01-01" || reason != "bad-data" || requestID != "req-1" || initiatorUUID != "t1" {
				t.Fatalf("unexpected args: tenant=%s orgID=%d target=%s reason=%s request=%s initiator=%s", tenantID, orgID, targetEffectiveDate, reason, requestID, initiatorUUID)
			}
			return "corr", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	got, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{
		OrgCode:             "root",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req-1",
		Reason:              "bad-data",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatalf("expected rescind call")
	}
	if got.OrgCode != "ROOT" || got.EffectiveDate != "2026-01-01" {
		t.Fatalf("result=%+v", got)
	}
}

func TestRescindRecordValidationAndNotFound(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", Reason: "x"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected request_id bad request, got %v", err)
	}
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", RequestID: "r1"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected reason bad request, got %v", err)
	}

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", RequestID: "r1", Reason: "x"}); err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org not found, got %v", err)
	}
}

func TestRescindOrgSuccessAndValidation(t *testing.T) {
	called := false
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode != "ROOT" {
				t.Fatalf("orgCode=%s", orgCode)
			}
			return 10000001, nil
		},
		submitRescindOrgFn: func(_ context.Context, tenantID string, orgID int, reason string, requestID string, initiatorUUID string) (int, error) {
			called = true
			if tenantID != "t1" || orgID != 10000001 || reason != "bad-org" || requestID != "req-2" || initiatorUUID != "t1" {
				t.Fatalf("unexpected args")
			}
			return 3, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	got, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "root", RequestID: "req-2", Reason: "bad-org"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatalf("expected rescind org call")
	}
	if got.Fields["rescinded_events"] != 3 {
		t.Fatalf("fields=%+v", got.Fields)
	}

	svc = NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	if _, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "ROOT", Reason: "x"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected request_id bad request, got %v", err)
	}
	if _, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "ROOT", RequestID: "r1"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected reason bad request, got %v", err)
	}
}

func TestRescindRecordStoreErrorPaths(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "bad", RequestID: "r1", Reason: "x"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad date, got %v", err)
	}
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "bad\n", TargetEffectiveDate: "2026-01-01", RequestID: "r1", Reason: "x"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad org code, got %v", err)
	}

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", RequestID: "r1", Reason: "x"}); err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}

	store = orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitRescindEventFn: func(_ context.Context, _ string, _ int, _ string, _ string, _ string, _ string) (string, error) {
			return "", errors.New("submit")
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.RescindRecord(context.Background(), "t1", RescindRecordOrgUnitRequest{OrgCode: "ROOT", TargetEffectiveDate: "2026-01-01", RequestID: "r1", Reason: "x"}); err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestRescindOrgStoreErrorPaths(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	if _, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "bad\n", RequestID: "r1", Reason: "x"}); err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad org code, got %v", err)
	}

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "ROOT", RequestID: "r1", Reason: "x"}); err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}

	store = orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitRescindOrgFn: func(_ context.Context, _ string, _ int, _ string, _ string, _ string) (int, error) {
			return 0, errors.New("submit")
		},
	}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "ROOT", RequestID: "r1", Reason: "x"}); err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}

	store = orgUnitWriteStoreStub{resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound }}
	svc = NewOrgUnitWriteService(store)
	if _, err := svc.RescindOrg(context.Background(), "t1", RescindOrgUnitRequest{OrgCode: "ROOT", RequestID: "r1", Reason: "x"}); err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org not found, got %v", err)
	}
}

func TestValidateDate(t *testing.T) {
	if _, err := validateDate(""); err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
	if _, err := validateDate("2026-13-01"); err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
	got, err := validateDate("2026-01-02")
	if err != nil || got != "2026-01-02" {
		t.Fatalf("expected valid date, got %v (%v)", got, err)
	}
}

func TestNormalizePernr(t *testing.T) {
	if _, err := normalizePernr(""); err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
		t.Fatalf("expected pernr invalid, got %v", err)
	}
	if _, err := normalizePernr("A100"); err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
		t.Fatalf("expected pernr invalid, got %v", err)
	}
	got, err := normalizePernr("000123")
	if err != nil || got != "123" {
		t.Fatalf("expected trimmed pernr, got %v (%v)", got, err)
	}
	got, err = normalizePernr("00000000")
	if err != nil || got != "0" {
		t.Fatalf("expected zero pernr, got %v (%v)", got, err)
	}
}

func TestNamePatchKey(t *testing.T) {
	if key, ok := namePatchKey(types.OrgUnitEventCreate); !ok || key != "name" {
		t.Fatalf("expected name key, got %v %v", key, ok)
	}
	if key, ok := namePatchKey(types.OrgUnitEventRename); !ok || key != "new_name" {
		t.Fatalf("expected new_name key, got %v %v", key, ok)
	}
	if _, ok := namePatchKey(types.OrgUnitEventMove); ok {
		t.Fatalf("expected move to be unsupported")
	}
}

func TestParentPatchKey(t *testing.T) {
	if key, ok := parentPatchKey(types.OrgUnitEventCreate); !ok || key != "parent_id" {
		t.Fatalf("expected parent_id key, got %v %v", key, ok)
	}
	if key, ok := parentPatchKey(types.OrgUnitEventMove); !ok || key != "new_parent_id" {
		t.Fatalf("expected new_parent_id key, got %v %v", key, ok)
	}
	if _, ok := parentPatchKey(types.OrgUnitEventRename); ok {
		t.Fatalf("expected rename to be unsupported")
	}
}

func TestResolveManagerInvalidPernr(t *testing.T) {
	svc := newWriteService(orgUnitWriteStoreStub{})
	if _, _, _, err := svc.resolveManager(context.Background(), "t1", "ABC"); err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
		t.Fatalf("expected pernr invalid, got %v", err)
	}
}

func TestResolveManagerStoreError(t *testing.T) {
	svc := newWriteService(orgUnitWriteStoreStub{
		findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
			return types.Person{}, errors.New("find")
		},
	})
	if _, _, _, err := svc.resolveManager(context.Background(), "t1", "1001"); err == nil || err.Error() != "find" {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestResolveManagerSuccess(t *testing.T) {
	svc := newWriteService(orgUnitWriteStoreStub{
		findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
			return types.Person{UUID: "p1", Pernr: "1001", DisplayName: "Manager", Status: "active"}, nil
		},
	})
	pernr, uuid, name, err := svc.resolveManager(context.Background(), "t1", "01001")
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if pernr != "1001" || uuid != "p1" || name != "Manager" {
		t.Fatalf("unexpected manager data: %v %v %v", pernr, uuid, name)
	}
}

func TestCreateInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-13-01",
		OrgCode:       "ROOT",
		Name:          "Root",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestCreateRequiresName(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          " ",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != "name is required" {
		t.Fatalf("expected name required, got %v", err)
	}
}

func TestCreateParentNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
		ParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != errParentNotFoundAsOf {
		t.Fatalf("expected parent not found, got %v", err)
	}
}

func TestCreateParentResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
		ParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestCreateParentInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	})
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
		ParentOrgCode: "A\n1",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestCreateManagerInvalid(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	})
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
		ManagerPernr:  "ABC",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
		t.Fatalf("expected pernr invalid, got %v", err)
	}
}

func TestCreateMarshalError(t *testing.T) {
	withMarshalJSON(t, func(any) ([]byte, error) {
		return nil, errors.New("marshal")
	})
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	})
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
	})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestCreateUUIDError(t *testing.T) {
	withNewUUID(t, func() (string, error) {
		return "", errors.New("uuid")
	})
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	})
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestCreateSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 0, errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestCreateFindEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
		findEventByUUIDFn: func(_ context.Context, _ string, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{}, errors.New("find")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
	})
	if err == nil || err.Error() != "find" {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestCreateSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "ROOT" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			if orgCode != "PARENT" {
				return 0, errors.New("unexpected org code")
			}
			return 20000002, nil
		},
		findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
			return types.Person{UUID: "p1", Pernr: "1001", DisplayName: "Manager", Status: "active"}, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
		findEventByUUIDFn: func(_ context.Context, _ string, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{OrgID: 10000001}, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	res, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		Name:           "Root",
		ParentOrgCode:  "PARENT",
		IsBusinessUnit: true,
		ManagerPernr:   "1001",
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if res.OrgID != "10000001" || res.OrgCode != "ROOT" || res.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.Fields["parent_org_code"] != "PARENT" || res.Fields["manager_pernr"] != "1001" || res.Fields["manager_name"] != "Manager" {
		t.Fatalf("unexpected fields: %#v", res.Fields)
	}
}

func TestRenameInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-13-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestRenameInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       " \t ",
		NewName:       "Name",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestRenameRequiresName(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       " ",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != "new_name is required" {
		t.Fatalf("expected new_name required, got %v", err)
	}
}

func TestRenameOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestRenameResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestRenameMarshalError(t *testing.T) {
	withMarshalJSON(t, func(any) ([]byte, error) {
		return nil, errors.New("marshal")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestRenameUUIDError(t *testing.T) {
	withNewUUID(t, func() (string, error) {
		return "", errors.New("uuid")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestRenameSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 0, errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestRenameSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Rename(context.Background(), "t1", RenameOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		NewName:       "Name",
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestMoveInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-13-01",
		OrgCode:          "ROOT",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestMoveInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          " \t ",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestMoveRequiresParent(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != "new_parent_org_code is required" {
		t.Fatalf("expected new_parent_org_code required, got %v", err)
	}
}

func TestMoveInvalidParentOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "ROOT",
		NewParentOrgCode: "A\n1",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected parent org code invalid, got %v", err)
	}
}

func TestMoveOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			return 0, errors.New("unexpected org code")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "A001",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestMoveResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "ROOT" {
				return 0, errors.New("resolve")
			}
			return 0, errors.New("unexpected org code")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "ROOT",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestMoveParentNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "ROOT" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			return 0, errors.New("unexpected org code")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "ROOT",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != errParentNotFoundAsOf {
		t.Fatalf("expected parent not found, got %v", err)
	}
}

func TestMoveParentResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 0, errors.New("resolve-parent")
			}
			return 0, errors.New("unexpected org code")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "A001",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "resolve-parent" {
		t.Fatalf("expected parent resolve error, got %v", err)
	}
}

func TestMoveMarshalError(t *testing.T) {
	withMarshalJSON(t, func(any) ([]byte, error) {
		return nil, errors.New("marshal")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 20000002, nil
			}
			return 0, errors.New("unexpected org code")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "A001",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestMoveUUIDError(t *testing.T) {
	withNewUUID(t, func() (string, error) {
		return "", errors.New("uuid")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 20000002, nil
			}
			return 0, errors.New("unexpected org code")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "A001",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestMoveSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 20000002, nil
			}
			return 0, errors.New("unexpected org code")
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 0, errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "A001",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestMoveSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 20000002, nil
			}
			return 0, errors.New("unexpected org code")
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Move(context.Background(), "t1", MoveOrgUnitRequest{
		EffectiveDate:    "2026-01-01",
		OrgCode:          "A001",
		NewParentOrgCode: "PARENT",
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestDisableInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-13-01",
		OrgCode:       "ROOT",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestDisableInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       " \t ",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestDisableOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestDisableResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestDisableUUIDError(t *testing.T) {
	withNewUUID(t, func() (string, error) {
		return "", errors.New("uuid")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestDisableSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 0, errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestDisableSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Disable(context.Background(), "t1", DisableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestEnableInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-13-01",
		OrgCode:       "ROOT",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestEnableInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       " 	 ",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestEnableOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestEnableResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestEnableUUIDError(t *testing.T) {
	withNewUUID(t, func() (string, error) {
		return "", errors.New("uuid")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestEnableSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 0, errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestEnableSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, eventType string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			if eventType != "ENABLE" {
				t.Fatalf("event_type=%q", eventType)
			}
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.Enable(context.Background(), "t1", EnableOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestSetBusinessUnitInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-13-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestSetBusinessUnitInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        " \t ",
		IsBusinessUnit: true,
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestSetBusinessUnitOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestSetBusinessUnitResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestSetBusinessUnitMarshalError(t *testing.T) {
	withMarshalJSON(t, func(any) ([]byte, error) {
		return nil, errors.New("marshal")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestSetBusinessUnitUUIDError(t *testing.T) {
	withNewUUID(t, func() (string, error) {
		return "", errors.New("uuid")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestSetBusinessUnitSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 0, errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestSetBusinessUnitSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	err := svc.SetBusinessUnit(context.Background(), "t1", SetBusinessUnitRequest{
		EffectiveDate:  "2026-01-01",
		OrgCode:        "ROOT",
		IsBusinessUnit: true,
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCorrectInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-13-01",
		RequestID:           "req",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestCorrectInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             " \t ",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestCorrectRequestIDRequired(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != "request_id is required" {
		t.Fatalf("expected request_id required, got %v", err)
	}
}

func TestCorrectOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestCorrectResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestCorrectEventNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{}, ports.ErrOrgEventNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		},
	})
	if err == nil || err.Error() != errOrgEventNotFound {
		t.Fatalf("expected org event not found, got %v", err)
	}
}

func TestCorrectEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{}, errors.New("find")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		},
	})
	if err == nil || err.Error() != "find" {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestCorrectMarshalError(t *testing.T) {
	withMarshalJSON(t, func(any) ([]byte, error) {
		return nil, errors.New("marshal")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		},
	})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestCorrectSubmitError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ json.RawMessage, _ string, _ string) (string, error) {
			return "", errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		},
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestCorrectPolicyResolveError(t *testing.T) {
	orig := resolveOrgUnitMutationPolicyInWrite
	resolveOrgUnitMutationPolicyInWrite = func(OrgUnitMutationPolicyKey, OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
		return OrgUnitMutationPolicyDecision{}, errors.New("boom")
	}
	t.Cleanup(func() { resolveOrgUnitMutationPolicyInWrite = orig })

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate, EffectiveDate: "2026-01-01"}, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			EffectiveDate: stringPtr("2026-01-01"),
		},
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestCorrectUsesPatchedEffectiveDate(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ json.RawMessage, _ string, _ string) (string, error) {
			return "corr", nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	res, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			EffectiveDate: stringPtr("2026-02-01"),
		},
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if res.EffectiveDate != "2026-02-01" {
		t.Fatalf("expected patched effective date, got %v", res.EffectiveDate)
	}
}

func TestBuildCorrectionPatch(t *testing.T) {
	ctx := context.Background()
	emptyCfgs := []types.TenantFieldConfig{}

	t.Run("effective_date_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			EffectiveDate: stringPtr("2026-13-01"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
			t.Fatalf("expected effective date invalid, got %v", err)
		}
	})

	t.Run("name_empty", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: stringPtr(" "),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != "name is required" {
			t.Fatalf("expected name required, got %v", err)
		}
	})

	t.Run("name_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["name"] != "Name" || fields["name"] != "Name" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("name_rename", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["new_name"] != "Name" {
			t.Fatalf("unexpected patch map: %#v", patchMap)
		}
	})

	t.Run("name_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_empty_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr(" "),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["parent_id"] != "" || fields["parent_org_code"] != "" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("parent_empty_move", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr(" "),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("PARENT"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("A\n1"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
			t.Fatalf("expected org code invalid, got %v", err)
		}
	})

	t.Run("parent_not_found", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
		})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("PARENT"),
		}, emptyCfgs)
		if err == nil || err.Error() != errParentNotFoundAsOf {
			t.Fatalf("expected parent not found, got %v", err)
		}
	})

	t.Run("parent_resolve_error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, errors.New("resolve")
			},
		})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("PARENT"),
		}, emptyCfgs)
		if err == nil || err.Error() != "resolve" {
			t.Fatalf("expected resolve error, got %v", err)
		}
	})

	t.Run("parent_success_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 20000002, nil
			},
		})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("PARENT"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["parent_id"] != 20000002 || fields["parent_org_code"] != "PARENT" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("is_business_unit_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			IsBusinessUnit: boolPtr(true),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("is_business_unit_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventSetBusinessUnit}, OrgUnitCorrectionPatch{
			IsBusinessUnit: boolPtr(true),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["is_business_unit"] != true || fields["is_business_unit"] != true {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("manager_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			ManagerPernr: stringPtr("1001"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("manager_resolve_error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
				return types.Person{}, errors.New("find")
			},
		})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ManagerPernr: stringPtr("1001"),
		}, emptyCfgs)
		if err == nil || err.Error() != "find" {
			t.Fatalf("expected find error, got %v", err)
		}
	})

	t.Run("manager_success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			findPersonByPernrFn: func(_ context.Context, _ string, _ string) (types.Person, error) {
				return types.Person{UUID: "p1", Pernr: "1001", DisplayName: "Manager", Status: "active"}, nil
			},
		})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ManagerPernr: stringPtr("1001"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["manager_uuid"] != "p1" || patchMap["manager_pernr"] != "1001" {
			t.Fatalf("unexpected patch map: %#v", patchMap)
		}
		if fields["manager_pernr"] != "1001" || fields["manager_name"] != "Manager" {
			t.Fatalf("unexpected fields: %#v", fields)
		}
	})

	t.Run("ext blank field key rejects", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{" ": "x"},
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("ext missing config rejects", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"org_type": "10"},
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("ext config blank key is skipped", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"org_type": "10"},
		}, []types.TenantFieldConfig{{FieldKey: " "}})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})
}

func stringPtr(v string) *string {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func TestCorrectStatusAdditionalBranches(t *testing.T) {
	t.Run("invalid org code", func(t *testing.T) {
		svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
		_, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{
			OrgCode:             " ",
			TargetEffectiveDate: "2026-01-01",
			TargetStatus:        "active",
			RequestID:           "r1",
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
			t.Fatalf("expected org code invalid, got %v", err)
		}
	})

	t.Run("resolve org unexpected error", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, errors.New("resolve")
			},
		}
		svc := NewOrgUnitWriteService(store)
		_, err := svc.CorrectStatus(context.Background(), "t1", CorrectStatusOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			TargetStatus:        "active",
			RequestID:           "r1",
		})
		if err == nil || err.Error() != "resolve" {
			t.Fatalf("expected resolve error, got %v", err)
		}
	})
}

func TestNormalizeTargetStatusAdditionalAliases(t *testing.T) {
	cases := map[string]string{
		"enabled":  "active",
		"有效":       "active",
		"inactive": "disabled",
		"无效":       "disabled",
		"active":   "active",
		"disabled": "disabled",
	}
	for input, expected := range cases {
		got, err := normalizeTargetStatus(input)
		if err != nil {
			t.Fatalf("input=%q err=%v", input, err)
		}
		if got != expected {
			t.Fatalf("input=%q got=%q want=%q", input, got, expected)
		}
	}
}

func TestResolveInitiatorUUID(t *testing.T) {
	if got := resolveInitiatorUUID(" user-1 ", "tenant-1"); got != "user-1" {
		t.Fatalf("got=%q", got)
	}
	if got := resolveInitiatorUUID(" ", " tenant-1 "); got != "tenant-1" {
		t.Fatalf("got=%q", got)
	}
}
