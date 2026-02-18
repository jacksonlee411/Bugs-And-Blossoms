package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestWriteUnified_CoversMoreWriteBranches(t *testing.T) {
	baseStore := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		resolveOrgIDFn: func(context.Context, string, string) (int, error) {
			return 10000001, nil
		},
		findPersonByPernrFn: func(context.Context, string, string) (types.Person, error) {
			return types.Person{}, ports.ErrPersonNotFound
		},
		submitEventFn: func(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
			return 1, nil
		},
		submitCorrectionFn: func(context.Context, string, int, string, json.RawMessage, string, string) (string, error) {
			return "c1", nil
		},
	}

	t.Run("effective_date invalid", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "bad",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "bad\x7f",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("list enabled field configs error bubbles", func(t *testing.T) {
		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return nil, errors.New("boom")
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parent_org_code normalize error", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		name := "X"
		parent := "bad\x7f"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:          &name,
				ParentOrgCode: &parent,
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parent_org_code resolve unexpected error bubbles", func(t *testing.T) {
		store := baseStore
		store.resolveOrgIDFn = func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "P001" {
				return 0, errors.New("boom")
			}
			return 10000001, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		parent := "P001"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:          &name,
				ParentOrgCode: &parent,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parent_org_code success sets parent_id and parent_org_code field", func(t *testing.T) {
		var captured map[string]any
		store := baseStore
		store.resolveOrgIDFn = func(_ context.Context, _ string, orgCode string) (int, error) {
			switch orgCode {
			case "P001":
				return 20000002, nil
			case "A001":
				return 10000001, nil
			default:
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
		}
		store.submitEventFn = func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, payload json.RawMessage, _ string, _ string) (int64, error) {
			if err := json.Unmarshal(payload, &captured); err != nil {
				return 0, err
			}
			return 1, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		parent := "P001"
		result, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:          &name,
				ParentOrgCode: &parent,
			},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if captured["parent_id"] != float64(20000002) { // json.Unmarshal uses float64 for numbers.
			t.Fatalf("payload=%v", captured)
		}
		if result.Fields["parent_org_code"] != "P001" {
			t.Fatalf("fields=%v", result.Fields)
		}
	})

	t.Run("name empty rejected", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		empty := " "
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &empty},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("create_org missing name rejected (late check)", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			OrgCode:       "ROOT",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("create_org submit error bubbles", func(t *testing.T) {
		store := baseStore
		store.submitEventFn = func(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
			return 0, errors.New("submit fail")
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root A"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			OrgCode:       "ROOT",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "submit fail") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("add_version requires patch", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("add_version org not found maps to ORG_CODE_NOT_FOUND", func(t *testing.T) {
		store := baseStore
		store.resolveOrgIDFn = func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "A001" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			return 0, errors.New("unexpected")
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errOrgCodeNotFound {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("add_version resolve org unexpected error bubbles", func(t *testing.T) {
		store := baseStore
		store.resolveOrgIDFn = func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("boom")
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("add_version marshal error bubbles", func(t *testing.T) {
		store := baseStore
		svc := NewOrgUnitWriteService(store)
		name := "X"
		withMarshalJSON(t, func(any) ([]byte, error) { return nil, errors.New("marshal fail") })
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "marshal fail") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("add_version submit error bubbles", func(t *testing.T) {
		store := baseStore
		store.submitEventFn = func(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
			return 0, errors.New("submit fail")
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "insert_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "submit fail") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct target_effective_date invalid", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		status := "active"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:              "correct",
			OrgCode:             "A001",
			EffectiveDate:       "2026-01-01",
			TargetEffectiveDate: "bad",
			RequestCode:         "r1",
			Patch:               OrgUnitWritePatch{Status: &status},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct resolve org not found maps to ORG_CODE_NOT_FOUND", func(t *testing.T) {
		store := baseStore
		store.resolveOrgIDFn = func(context.Context, string, string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound }
		svc := NewOrgUnitWriteService(store)
		status := "active"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:              "correct",
			OrgCode:             "A001",
			EffectiveDate:       "2026-01-01",
			TargetEffectiveDate: "2026-01-01",
			RequestCode:         "r1",
			Patch:               OrgUnitWritePatch{Status: &status},
		})
		if err == nil || err.Error() != errOrgCodeNotFound {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct resolve org unexpected error bubbles", func(t *testing.T) {
		store := baseStore
		store.resolveOrgIDFn = func(context.Context, string, string) (int, error) { return 0, errors.New("boom") }
		svc := NewOrgUnitWriteService(store)
		status := "active"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:              "correct",
			OrgCode:             "A001",
			EffectiveDate:       "2026-01-01",
			TargetEffectiveDate: "2026-01-01",
			RequestCode:         "r1",
			Patch:               OrgUnitWritePatch{Status: &status},
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct requires patch when effective_date unchanged and no other fields", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:              "correct",
			OrgCode:             "A001",
			EffectiveDate:       "2026-01-01",
			TargetEffectiveDate: "2026-01-01",
			RequestCode:         "r1",
			Patch:               OrgUnitWritePatch{},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct marshal error bubbles", func(t *testing.T) {
		svc := NewOrgUnitWriteService(baseStore)
		status := "disabled"
		withMarshalJSON(t, func(any) ([]byte, error) { return nil, errors.New("marshal fail") })
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:              "correct",
			OrgCode:             "A001",
			EffectiveDate:       "2026-01-02",
			TargetEffectiveDate: "2026-01-01",
			RequestCode:         "r1",
			Patch:               OrgUnitWritePatch{Status: &status},
		})
		if err == nil || !strings.Contains(err.Error(), "marshal fail") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct submit error bubbles", func(t *testing.T) {
		store := baseStore
		store.submitCorrectionFn = func(context.Context, string, int, string, json.RawMessage, string, string) (string, error) {
			return "", errors.New("submit fail")
		}
		svc := NewOrgUnitWriteService(store)
		status := "disabled"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:              "correct",
			OrgCode:             "A001",
			EffectiveDate:       "2026-01-02",
			TargetEffectiveDate: "2026-01-01",
			RequestCode:         "r1",
			Patch:               OrgUnitWritePatch{Status: &status},
		})
		if err == nil || !strings.Contains(err.Error(), "submit fail") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("manager pernr invalid", func(t *testing.T) {
		store := baseStore
		store.findPersonByPernrFn = func(context.Context, string, string) (types.Person, error) {
			return types.Person{}, errors.New("unexpected")
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		pernr := "abc"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:         &name,
				ManagerPernr: &pernr,
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("manager pernr not found mapped", func(t *testing.T) {
		store := baseStore
		store.findPersonByPernrFn = func(context.Context, string, string) (types.Person, error) {
			return types.Person{}, ports.ErrPersonNotFound
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		pernr := "123"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:         &name,
				ManagerPernr: &pernr,
			},
		})
		if err == nil || err.Error() != errManagerPernrNotFound {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("manager pernr inactive mapped", func(t *testing.T) {
		store := baseStore
		store.findPersonByPernrFn = func(context.Context, string, string) (types.Person, error) {
			return types.Person{Status: "disabled"}, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		pernr := "123"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:         &name,
				ManagerPernr: &pernr,
			},
		})
		if err == nil || err.Error() != errManagerPernrInactive {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("manager pernr store error bubbles", func(t *testing.T) {
		store := baseStore
		store.findPersonByPernrFn = func(context.Context, string, string) (types.Person, error) {
			return types.Person{}, errors.New("boom")
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		pernr := "123"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:         &name,
				ManagerPernr: &pernr,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("manager pernr success sets manager fields", func(t *testing.T) {
		var captured map[string]any
		store := baseStore
		store.findPersonByPernrFn = func(context.Context, string, string) (types.Person, error) {
			return types.Person{UUID: "u1", DisplayName: "Alice", Status: "active"}, nil
		}
		store.submitEventFn = func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, payload json.RawMessage, _ string, _ string) (int64, error) {
			if err := json.Unmarshal(payload, &captured); err != nil {
				return 0, err
			}
			return 1, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		pernr := "0000123"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name:         &name,
				ManagerPernr: &pernr,
			},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if captured["manager_uuid"] != "u1" || captured["manager_pernr"] != "123" {
			t.Fatalf("payload=%v", captured)
		}
	})

	t.Run("ext invalid field key rejected", func(t *testing.T) {
		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name: &name,
				Ext:  map[string]any{"": "x"},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext dict wrong value type rejected", func(t *testing.T) {
		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name: &name,
				Ext:  map[string]any{"org_type": 1},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext dict label resolution failure rejected", func(t *testing.T) {
		orig := resolveDictLabelInWrite
		resolveDictLabelInWrite = func(context.Context, string, string, string, string) (string, bool, error) {
			return "", false, errors.New("boom")
		}
		t.Cleanup(func() { resolveDictLabelInWrite = orig })

		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name: &name,
				Ext:  map[string]any{"org_type": "10"},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgInvalidArgument {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext dict nil value does not require labels snapshot", func(t *testing.T) {
		var captured map[string]any
		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}, nil
		}
		store.submitEventFn = func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, payload json.RawMessage, _ string, _ string) (int64, error) {
			if err := json.Unmarshal(payload, &captured); err != nil {
				return 0, err
			}
			return 1, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name: &name,
				Ext:  map[string]any{"org_type": nil},
			},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if _, ok := captured["ext_labels_snapshot"]; ok {
			t.Fatalf("payload=%v", captured)
		}
	})

	t.Run("ext field config missing rejected", func(t *testing.T) {
		store := baseStore
		store.listEnabledFieldCfgsFn = func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		}
		svc := NewOrgUnitWriteService(store)
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch: OrgUnitWritePatch{
				Name: &name,
				Ext:  map[string]any{"org_type": "10"},
			},
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})
}
