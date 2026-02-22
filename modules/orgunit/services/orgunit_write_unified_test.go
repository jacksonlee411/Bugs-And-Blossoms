package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

func TestWriteUnified_CreateOrg_SubmitCreate(t *testing.T) {
	var capturedType string
	var capturedPayload map[string]any
	store := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, orgID *int, eventType string, _ string, payload json.RawMessage, requestID string, _ string) (int64, error) {
			if orgID != nil {
				t.Fatalf("orgID should be nil for create")
			}
			if requestID != "req-1" {
				t.Fatalf("requestID=%s", requestID)
			}
			capturedType = eventType
			if err := json.Unmarshal(payload, &capturedPayload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return 1, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	name := "Root A"
	_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
		Intent:        "create_org",
		OrgCode:       "ROOT",
		EffectiveDate: "2026-01-01",
		RequestID:     "req-1",
		Patch: OrgUnitWritePatch{
			Name: &name,
		},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if capturedType != "CREATE" {
		t.Fatalf("eventType=%s", capturedType)
	}
	if capturedPayload["org_code"] != "ROOT" || capturedPayload["name"] != "Root A" {
		t.Fatalf("payload=%v", capturedPayload)
	}
}

func TestWriteUnified_AddVersion_SubmitUpdate(t *testing.T) {
	var capturedType string
	var capturedOrgID int
	store := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode != "A001" {
				t.Fatalf("orgCode=%s", orgCode)
			}
			return 10000001, nil
		},
		submitEventFn: func(_ context.Context, _ string, _ string, orgID *int, eventType string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			capturedType = eventType
			if orgID == nil {
				t.Fatalf("orgID nil")
			}
			capturedOrgID = *orgID
			return 2, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	name := "Org New Name"
	_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
		Intent:        "add_version",
		OrgCode:       "A001",
		EffectiveDate: "2026-01-02",
		RequestID:     "req-2",
		Patch: OrgUnitWritePatch{
			Name: &name,
		},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if capturedType != "UPDATE" {
		t.Fatalf("eventType=%s", capturedType)
	}
	if capturedOrgID != 10000001 {
		t.Fatalf("orgID=%d", capturedOrgID)
	}
}

func TestWriteUnified_Correct_UsesTargetEffectiveDate(t *testing.T) {
	var capturedTargetDate string
	var capturedPatch map[string]any
	store := orgUnitWriteStoreStub{
		listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
			return []types.TenantFieldConfig{}, nil
		},
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, targetEffectiveDate string, patch json.RawMessage, _ string, _ string) (string, error) {
			capturedTargetDate = targetEffectiveDate
			if err := json.Unmarshal(patch, &capturedPatch); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return "evt-c1", nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	status := "disabled"
	_, err := svc.Write(context.Background(), "t1", WriteOrgUnitRequest{
		Intent:              "correct",
		OrgCode:             "A001",
		EffectiveDate:       "2026-01-03",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req-3",
		Patch: OrgUnitWritePatch{
			Status: &status,
		},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if capturedTargetDate != "2026-01-01" {
		t.Fatalf("target=%s", capturedTargetDate)
	}
	if capturedPatch["effective_date"] != "2026-01-03" || capturedPatch["status"] != "disabled" {
		t.Fatalf("patch=%v", capturedPatch)
	}
}
