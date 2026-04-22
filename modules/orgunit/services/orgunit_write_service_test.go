package services

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
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
	submitEventFn            func(ctx context.Context, tenantID string, eventUUID string, orgID *int, eventType string, effectiveDate string, payload json.RawMessage, requestID string, initiatorUUID string) (int64, error)
	submitCorrectionFn       func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error)
	submitStatusCorrectionFn func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error)
	submitRescindEventFn     func(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error)
	submitRescindOrgFn       func(ctx context.Context, tenantID string, orgID int, reason string, requestID string, initiatorUUID string) (int, error)
	findEventByUUIDFn        func(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error)
	findEventByEffectiveFn   func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (types.OrgUnitEvent, error)
	listEnabledFieldCfgsFn   func(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error)
	resolveOrgIDFn           func(ctx context.Context, tenantID string, orgCode string) (int, error)
	resolveOrgNodeKeyFn      func(ctx context.Context, tenantID string, orgCode string) (string, error)
	resolveOrgCodeFn         func(ctx context.Context, tenantID string, orgID int) (string, error)
	isOrgTreeInitializedFn   func(ctx context.Context, tenantID string) (bool, error)
}

func (s orgUnitWriteStoreStub) SubmitEvent(ctx context.Context, tenantID string, eventUUID string, orgNodeKey *string, eventType string, effectiveDate string, payload json.RawMessage, requestID string, initiatorUUID string) (int64, error) {
	if s.submitEventFn == nil {
		return 0, errors.New("SubmitEvent not mocked")
	}
	var orgID *int
	if orgNodeKey != nil {
		value, err := parseTestOrgNodeKey(*orgNodeKey)
		if err != nil {
			return 0, err
		}
		orgID = &value
	}
	return s.submitEventFn(ctx, tenantID, eventUUID, orgID, eventType, effectiveDate, payload, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitCorrection(ctx context.Context, tenantID string, orgNodeKey string, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error) {
	if s.submitCorrectionFn == nil {
		return "", errors.New("SubmitCorrection not mocked")
	}
	orgID, err := parseTestOrgNodeKey(orgNodeKey)
	if err != nil {
		return "", err
	}
	return s.submitCorrectionFn(ctx, tenantID, orgID, targetEffectiveDate, patch, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitStatusCorrection(ctx context.Context, tenantID string, orgNodeKey string, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error) {
	if s.submitStatusCorrectionFn == nil {
		return "", errors.New("SubmitStatusCorrection not mocked")
	}
	orgID, err := parseTestOrgNodeKey(orgNodeKey)
	if err != nil {
		return "", err
	}
	return s.submitStatusCorrectionFn(ctx, tenantID, orgID, targetEffectiveDate, targetStatus, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitRescindEvent(ctx context.Context, tenantID string, orgNodeKey string, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error) {
	if s.submitRescindEventFn == nil {
		return "", errors.New("SubmitRescindEvent not mocked")
	}
	orgID, err := parseTestOrgNodeKey(orgNodeKey)
	if err != nil {
		return "", err
	}
	return s.submitRescindEventFn(ctx, tenantID, orgID, targetEffectiveDate, reason, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) SubmitRescindOrg(ctx context.Context, tenantID string, orgNodeKey string, reason string, requestID string, initiatorUUID string) (int, error) {
	if s.submitRescindOrgFn == nil {
		return 0, errors.New("SubmitRescindOrg not mocked")
	}
	orgID, err := parseTestOrgNodeKey(orgNodeKey)
	if err != nil {
		return 0, err
	}
	return s.submitRescindOrgFn(ctx, tenantID, orgID, reason, requestID, initiatorUUID)
}

func (s orgUnitWriteStoreStub) FindEventByUUID(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error) {
	if s.findEventByUUIDFn == nil {
		return types.OrgUnitEvent{}, errors.New("FindEventByUUID not mocked")
	}
	return s.findEventByUUIDFn(ctx, tenantID, eventUUID)
}

func (s orgUnitWriteStoreStub) FindEventByEffectiveDate(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (types.OrgUnitEvent, error) {
	if s.findEventByEffectiveFn == nil {
		return types.OrgUnitEvent{}, errors.New("FindEventByEffectiveDate not mocked")
	}
	orgID, err := parseTestOrgNodeKey(orgNodeKey)
	if err != nil {
		return types.OrgUnitEvent{}, err
	}
	return s.findEventByEffectiveFn(ctx, tenantID, orgID, effectiveDate)
}

func (s orgUnitWriteStoreStub) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error) {
	if s.listEnabledFieldCfgsFn != nil {
		return s.listEnabledFieldCfgsFn(ctx, tenantID, asOf)
	}
	return []types.TenantFieldConfig{}, nil
}

func (s orgUnitWriteStoreStub) ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error) {
	if s.resolveOrgNodeKeyFn != nil {
		return s.resolveOrgNodeKeyFn(ctx, tenantID, orgCode)
	}
	if s.resolveOrgIDFn == nil {
		return "", errors.New("ResolveOrgNodeKey not mocked")
	}
	orgID, err := s.resolveOrgIDFn(ctx, tenantID, orgCode)
	if err != nil {
		return "", err
	}
	if orgID < 10000000 || orgID > 99999999 {
		return strconv.Itoa(orgID), nil
	}
	return mustEncodeTestOrgNodeKey(orgID), nil
}

func (s orgUnitWriteStoreStub) ResolveOrgCodeByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) (string, error) {
	if s.resolveOrgCodeFn == nil {
		return "", errors.New("ResolveOrgCodeByNodeKey not mocked")
	}
	orgID, err := parseTestOrgNodeKey(orgNodeKey)
	if err != nil {
		return "", err
	}
	return s.resolveOrgCodeFn(ctx, tenantID, orgID)
}

func (s orgUnitWriteStoreStub) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	if s.isOrgTreeInitializedFn != nil {
		return s.isOrgTreeInitializedFn(ctx, tenantID)
	}
	return s.resolveOrgIDFn != nil || s.resolveOrgNodeKeyFn != nil, nil
}

func parseTestOrgNodeKey(input string) (int, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, errors.New("org_node_key is required")
	}
	allDigits := true
	for _, r := range value {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return strconv.Atoi(value)
	}
	decoded, err := orgunitpkg.DecodeOrgNodeKey(value)
	if err != nil {
		return 0, err
	}
	return int(decoded), nil
}

func mustEncodeTestOrgNodeKey(orgID int) string {
	key, err := orgunitpkg.EncodeOrgNodeKey(int64(orgID))
	if err != nil {
		panic(err)
	}
	return key
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

func withDefaultCreateFieldDecisions(store orgUnitWriteStoreStub) orgUnitWriteStoreStub {
	if store.resolveOrgNodeKeyFn == nil {
		store.resolveOrgNodeKeyFn = func(context.Context, string, string) (string, error) {
			return "", orgunitpkg.ErrOrgCodeNotFound
		}
	}
	if store.isOrgTreeInitializedFn == nil {
		store.isOrgTreeInitializedFn = func(context.Context, string) (bool, error) {
			return false, nil
		}
	}
	return store
}

func newWriteService(store ports.OrgUnitWriteStore) *orgUnitWriteService {
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
			ParentOrgCode: new("parent"),
		},
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if captured["new_parent_org_node_key"] != mustEncodeTestOrgNodeKey(20000002) {
		t.Fatalf("expected new_parent_org_node_key mapped, got %#v", captured)
	}
	if _, ok := captured["parent_org_node_key"]; ok {
		t.Fatalf("unexpected parent_org_node_key in patch: %#v", captured)
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
			Name: new("Rename"),
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
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ json.RawMessage, _ string, _ string) (string, error) {
			return "evt-1", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			ManagerPernr: new("1001"),
		},
	})
	if err != nil {
		t.Fatalf("expected manager pernr to normalize without person lookup, got %v", err)
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
			Name: new("Name"),
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
			Name: new("Name"),
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
			Name: new("Name"),
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
			Name: new("Name"),
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
			EffectiveDate: new("2026-01-01"),
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
			EffectiveDate: new("2026-02-01"),
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
			EffectiveDate: new("2026-13-01"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
			t.Fatalf("expected effective date invalid, got %v", err)
		}
	})

	t.Run("name_empty", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: new(" "),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != "name is required" {
			t.Fatalf("expected name required, got %v", err)
		}
	})

	t.Run("name_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: new("Name"),
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
			Name: new("Name"),
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
			Name: new("Name"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_empty_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new(" "),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["parent_org_node_key"] != "" || fields["parent_org_code"] != "" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("parent_empty_move", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, OrgUnitCorrectionPatch{
			ParentOrgCode: new(" "),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("PARENT"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("A\n1"),
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
			ParentOrgCode: new("PARENT"),
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
			ParentOrgCode: new("PARENT"),
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
			ParentOrgCode: new("PARENT"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["parent_org_node_key"] != mustEncodeTestOrgNodeKey(20000002) || fields["parent_org_code"] != "PARENT" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("is_business_unit_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			IsBusinessUnit: new(true),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("is_business_unit_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventSetBusinessUnit}, OrgUnitCorrectionPatch{
			IsBusinessUnit: new(true),
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
			ManagerPernr: new("1001"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("manager_resolve_error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ManagerPernr: new("bad"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
			t.Fatalf("expected invalid pernr, got %v", err)
		}
	})

	t.Run("manager_success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ManagerPernr: new("1001"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["manager_pernr"] != "1001" {
			t.Fatalf("unexpected patch map: %#v", patchMap)
		}
		if fields["manager_pernr"] != "1001" {
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

	t.Run("ext custom plain field is accepted", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"x_cost_center": "CC-001"},
		}, []types.TenantFieldConfig{{FieldKey: "x_cost_center", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)}})
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		gotExt, ok := patchMap["ext"].(map[string]any)
		if !ok || gotExt["x_cost_center"] != "CC-001" {
			t.Fatalf("patchMap ext=%v", patchMap["ext"])
		}
		gotFields, ok := fields["ext"].(map[string]any)
		if !ok || gotFields["x_cost_center"] != "CC-001" {
			t.Fatalf("fields ext=%v", fields["ext"])
		}
	})

	t.Run("ext config exists but invalid field_key rejects", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"unknown_field": "x"},
		}, []types.TenantFieldConfig{{FieldKey: "unknown_field", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)}})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})
}

//go:fix inline
func stringPtr(v string) *string {
	return new(v)
}

//go:fix inline
func boolPtr(v bool) *bool {
	return new(v)
}
