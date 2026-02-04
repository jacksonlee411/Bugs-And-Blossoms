package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitWriteStoreStub struct {
	submitEventFn          func(ctx context.Context, tenantID string, eventUUID string, orgID *int, eventType string, effectiveDate string, payload json.RawMessage, requestCode string, initiatorUUID string) (int64, error)
	submitCorrectionFn     func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error)
	findEventByUUIDFn      func(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error)
	findEventByEffectiveFn func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (types.OrgUnitEvent, error)
	resolveOrgIDFn         func(ctx context.Context, tenantID string, orgCode string) (int, error)
	resolveOrgCodeFn       func(ctx context.Context, tenantID string, orgID int) (string, error)
	findPersonByPernrFn    func(ctx context.Context, tenantID string, pernr string) (types.Person, error)
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
			return 10000001, nil
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
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
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
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
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
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
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
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
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
			if orgCode == "ROOT" {
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
			if orgCode == "ROOT" {
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
		OrgCode:          "ROOT",
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
			if orgCode == "ROOT" {
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
		OrgCode:          "ROOT",
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
			if orgCode == "ROOT" {
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
		OrgCode:          "ROOT",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "uuid" {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestMoveSubmitEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "ROOT" {
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
		OrgCode:          "ROOT",
		NewParentOrgCode: "PARENT",
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestMoveSuccess(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "ROOT" {
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
		OrgCode:          "ROOT",
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

	t.Run("effective_date_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			EffectiveDate: stringPtr("2026-13-01"),
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
			t.Fatalf("expected effective date invalid, got %v", err)
		}
	})

	t.Run("name_empty", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: stringPtr(" "),
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != "name is required" {
			t.Fatalf("expected name required, got %v", err)
		}
	})

	t.Run("name_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: stringPtr("Name"),
		})
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
		})
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["new_name"] != "Name" {
			t.Fatalf("unexpected patch map: %#v", patchMap)
		}
	})

	t.Run("parent_empty_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr(" "),
		})
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
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("PARENT"),
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: stringPtr("A\n1"),
		})
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
		})
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
		})
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
		})
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
		})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("is_business_unit_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventSetBusinessUnit}, OrgUnitCorrectionPatch{
			IsBusinessUnit: boolPtr(true),
		})
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
		})
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
		})
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
		})
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
}

func stringPtr(v string) *string {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
