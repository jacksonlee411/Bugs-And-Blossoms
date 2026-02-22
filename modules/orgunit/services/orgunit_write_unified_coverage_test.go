package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestWriteUnified_CoversValidationAndErrorBranches(t *testing.T) {
	store := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		resolveOrgIDFn: func(context.Context, string, string) (int, error) {
			return 10000001, nil
		},
		submitEventFn: func(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
			return 1, nil
		},
		submitCorrectionFn: func(context.Context, string, int, string, json.RawMessage, string, string) (string, error) {
			return "c1", nil
		},
	}
	svc := NewOrgUnitWriteService(store)

	t.Run("missing intent", func(t *testing.T) {
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("missing request_id", func(t *testing.T) {
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestID:     " ",
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("unsupported intent", func(t *testing.T) {
		name := "X"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "nope",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestWriteUnified_CoversUUIDAndMarshalFailure(t *testing.T) {
	store := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		submitEventFn: func(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	name := "Root A"

	t.Run("newUUID error", func(t *testing.T) {
		withNewUUID(t, func() (string, error) { return "", errors.New("uuid fail") })
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			OrgCode:       "ROOT",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "uuid fail") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("marshalJSON error", func(t *testing.T) {
		withMarshalJSON(t, func(any) ([]byte, error) { return nil, errors.New("marshal fail") })
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			OrgCode:       "ROOT",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || !strings.Contains(err.Error(), "marshal fail") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestWriteUnified_CoversParentAndManagerBranches(t *testing.T) {
	var captured map[string]any

	store := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{
				{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
			}, nil
		},
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			switch strings.TrimSpace(orgCode) {
			case "A001":
				return 10000001, nil
			case "P001":
				return 20000002, nil
			default:
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
		},
		findPersonByPernrFn: func(context.Context, string, string) (types.Person, error) {
			return types.Person{}, errors.New("unexpected FindPersonByPernr")
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, payload json.RawMessage, _ string, _ string) (int64, error) {
			if err := json.Unmarshal(payload, &captured); err != nil {
				return 0, err
			}
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)

	t.Run("parent empty rejected", func(t *testing.T) {
		name := "X"
		parent := " "
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-02",
			RequestID:     "r1",
			Patch: OrgUnitWritePatch{
				Name:          &name,
				ParentOrgCode: &parent,
			},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parent not found rejected", func(t *testing.T) {
		name := "X"
		parent := "P999"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-02",
			RequestID:     "r1",
			Patch: OrgUnitWritePatch{
				Name:          &name,
				ParentOrgCode: &parent,
			},
		})
		if err == nil || err.Error() != errParentNotFoundAsOf {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("manager clear + ext dict snapshot", func(t *testing.T) {
		captured = nil
		name := "New Name"
		manager := ""
		status := "disabled"
		isBU := true

		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-02",
			RequestID:     "r1",
			Patch: OrgUnitWritePatch{
				Name:           &name,
				Status:         &status,
				IsBusinessUnit: &isBU,
				ManagerPernr:   &manager,
				Ext:            map[string]any{"org_type": "10"},
			},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if captured == nil {
			t.Fatalf("expected payload captured")
		}
		if captured["manager_uuid"] != "" || captured["manager_pernr"] != "" {
			t.Fatalf("payload=%v", captured)
		}
		if captured["status"] != "disabled" || captured["is_business_unit"] != true {
			t.Fatalf("payload=%v", captured)
		}
		ext, _ := captured["ext"].(map[string]any)
		if ext == nil || ext["org_type"] != "10" {
			t.Fatalf("payload=%v", captured)
		}
		labels, _ := captured["ext_labels_snapshot"].(map[string]any)
		if labels == nil || labels["org_type"] == nil {
			t.Fatalf("payload=%v", captured)
		}
	})

	t.Run("status empty rejected", func(t *testing.T) {
		name := "X"
		emptyStatus := " "
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-02",
			RequestID:     "r1",
			Patch: OrgUnitWritePatch{
				Name:   &name,
				Status: &emptyStatus,
			},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("correct requires target_effective_date", func(t *testing.T) {
		status := "active"
		_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
			Intent:        "correct",
			OrgCode:       "A001",
			EffectiveDate: "2026-01-02",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Status: &status},
		})
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("err=%v", err)
		}
	})
}
