package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

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
		OrgCode:       " \t ",
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
