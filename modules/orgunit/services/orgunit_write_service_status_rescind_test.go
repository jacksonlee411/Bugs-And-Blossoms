package services

import (
	"context"
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

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
	if key, ok := parentPatchKey(types.OrgUnitEventCreate); !ok || key != "parent_org_node_key" {
		t.Fatalf("expected parent_org_node_key key, got %v %v", key, ok)
	}
	if key, ok := parentPatchKey(types.OrgUnitEventMove); !ok || key != "new_parent_org_node_key" {
		t.Fatalf("expected new_parent_org_node_key key, got %v %v", key, ok)
	}
	if _, ok := parentPatchKey(types.OrgUnitEventRename); ok {
		t.Fatalf("expected rename to be unsupported")
	}
}

func TestResolveManagerInvalidPernr(t *testing.T) {
	svc := newWriteService(orgUnitWriteStoreStub{})
	if _, _, _, err := svc.resolveManager("ABC"); err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
		t.Fatalf("expected pernr invalid, got %v", err)
	}
}

func TestResolveManagerSuccess(t *testing.T) {
	svc := newWriteService(orgUnitWriteStoreStub{})
	pernr, uuid, name, err := svc.resolveManager("01001")
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if pernr != "1001" || uuid != "" || name != "" {
		t.Fatalf("unexpected manager data: %v %v %v", pernr, uuid, name)
	}
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
