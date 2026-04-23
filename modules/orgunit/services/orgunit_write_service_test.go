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

//go:fix inline
func stringPtr(v string) *string {
	return new(v)
}

//go:fix inline
func boolPtr(v bool) *bool {
	return new(v)
}
