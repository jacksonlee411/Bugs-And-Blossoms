package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

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

func TestCreateManagerPernrNormalized(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
		findEventByUUIDFn: func(_ context.Context, _ string, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{OrgNodeKey: "10000001"}, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Create(context.Background(), "t1", CreateOrgUnitRequest{
		EffectiveDate: "2026-01-01",
		OrgCode:       "ROOT",
		Name:          "Root",
		ManagerPernr:  "01001",
	})
	if err != nil {
		t.Fatalf("expected normalized manager pernr, got %v", err)
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
		submitEventFn: func(_ context.Context, _ string, _ string, _ *int, _ string, _ string, _ json.RawMessage, _ string, _ string) (int64, error) {
			return 1, nil
		},
		findEventByUUIDFn: func(_ context.Context, _ string, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{OrgNodeKey: "10000001"}, nil
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
	if res.OrgCode != "ROOT" || res.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.Fields["parent_org_code"] != "PARENT" || res.Fields["manager_pernr"] != "1001" {
		t.Fatalf("unexpected fields: %#v", res.Fields)
	}
}
