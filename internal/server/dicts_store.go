package server

import (
	"context"

	"github.com/jackc/pgx/v5"
	iammodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/iam"
	iampersistence "github.com/jacksonlee411/Bugs-And-Blossoms/modules/iam/infrastructure/persistence"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

const (
	dictCodeOrgType              = iampersistence.DictCodeOrgType
	globalTenantID               = iampersistence.GlobalTenantID
	dictEventCreated             = iampersistence.DictEventCreated
	dictEventLabelCorrected      = iampersistence.DictEventLabelCorrected
	dictEventDisabled            = iampersistence.DictEventDisabled
	dictValueEventCreated        = iampersistence.DictValueEventCreated
	dictValueEventLabelCorrected = iampersistence.DictValueEventLabelCorrected
	dictValueEventDisabled       = iampersistence.DictValueEventDisabled
	dictRegistryEventCreated     = iampersistence.DictRegistryEventCreated
	dictRegistryEventDisabled    = iampersistence.DictRegistryEventDisabled
	dictOptionSetIDDeflt         = iampersistence.DictOptionSetIDDeflt
	dictOptionSetIDSourceDeflt   = iampersistence.DictOptionSetIDSourceDeflt
)

var (
	errDictCodeRequired          = iampersistence.ErrDictCodeRequired
	errDictCodeInvalid           = iampersistence.ErrDictCodeInvalid
	errDictNotFound              = iampersistence.ErrDictNotFound
	errDictNameRequired          = iampersistence.ErrDictNameRequired
	errDictCodeConflict          = iampersistence.ErrDictCodeConflict
	errDictDisabled              = iampersistence.ErrDictDisabled
	errDictDisabledOnRequired    = iampersistence.ErrDictDisabledOnRequired
	errDictValueCodeRequired     = iampersistence.ErrDictValueCodeRequired
	errDictValueLabelRequired    = iampersistence.ErrDictValueLabelRequired
	errDictValueNotFoundAsOf     = iampersistence.ErrDictValueNotFoundAsOf
	errDictValueConflict         = iampersistence.ErrDictValueConflict
	errDictValueDictDisabled     = iampersistence.ErrDictValueDictDisabled
	errDictBaselineNotReady      = iampersistence.ErrDictBaselineNotReady
	errDictRequestIDRequired     = iampersistence.ErrDictRequestIDRequired
	errDictEffectiveDayRequired  = iampersistence.ErrDictEffectiveDayRequired
	errDictDisabledOnInvalidDate = iampersistence.ErrDictDisabledOnInvalidDate
)

type DictStore = iampersistence.DictStore
type DictItem = iampersistence.DictItem
type DictValueItem = iampersistence.DictValueItem
type DictValueAuditItem = iampersistence.DictValueAuditItem
type DictCreateRequest = iampersistence.DictCreateRequest
type DictDisableRequest = iampersistence.DictDisableRequest
type DictCreateValueRequest = iampersistence.DictCreateValueRequest
type DictDisableValueRequest = iampersistence.DictDisableValueRequest
type DictCorrectValueRequest = iampersistence.DictCorrectValueRequest

type dictPGStore struct {
	*iammodule.DictPGStore
	pool pgBeginner
}

func newDictPGStore(pool pgBeginner) DictStore {
	return &dictPGStore{
		DictPGStore: iammodule.NewDictPGStore(pool),
		pool:        pool,
	}
}

func (s *dictPGStore) delegate() *iammodule.DictPGStore {
	if s.DictPGStore == nil {
		s.DictPGStore = iammodule.NewDictPGStore(s.pool)
	}
	return s.DictPGStore
}

func (s *dictPGStore) ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error) {
	return s.delegate().ListDicts(ctx, tenantID, asOf)
}

func (s *dictPGStore) CreateDict(ctx context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error) {
	return s.delegate().CreateDict(ctx, tenantID, req)
}

func (s *dictPGStore) DisableDict(ctx context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error) {
	return s.delegate().DisableDict(ctx, tenantID, req)
}

func (s *dictPGStore) submitDictEvent(
	ctx context.Context,
	tenantID string,
	dictCode string,
	eventType string,
	day string,
	payload map[string]any,
	requestID string,
	initiator string,
) (DictItem, bool, error) {
	return s.delegate().SubmitDictEvent(ctx, tenantID, dictCode, eventType, day, payload, requestID, initiator)
}

func (s *dictPGStore) ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	return s.delegate().ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, status)
}

func (s *dictPGStore) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	return s.delegate().ResolveValueLabel(ctx, tenantID, asOf, dictCode, code)
}

func (s *dictPGStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	return s.delegate().ListOptions(ctx, tenantID, asOf, dictCode, keyword, limit)
}

func (s *dictPGStore) CreateDictValue(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error) {
	return s.delegate().CreateDictValue(ctx, tenantID, req)
}

func (s *dictPGStore) DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error) {
	return s.delegate().DisableDictValue(ctx, tenantID, req)
}

func (s *dictPGStore) CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error) {
	return s.delegate().CorrectDictValue(ctx, tenantID, req)
}

func (s *dictPGStore) submitValueEvent(ctx context.Context, tenantID string, dictCode string, code string, eventType string, day string, payload map[string]any, requestID string, initiator string) (DictValueItem, bool, error) {
	return s.delegate().SubmitValueEvent(ctx, tenantID, dictCode, code, eventType, day, payload, requestID, initiator)
}

func (s *dictPGStore) ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error) {
	return s.delegate().ListDictValueAudit(ctx, tenantID, dictCode, code, limit)
}

type dictMemoryStore struct {
	*iammodule.DictMemoryStore
	dicts  map[string]map[string]DictItem
	values map[string][]DictValueItem
}

func newDictMemoryStore() DictStore {
	core := iammodule.NewDictMemoryStore()
	return &dictMemoryStore{
		DictMemoryStore: core,
		dicts:           core.Dicts,
		values:          core.Values,
	}
}

func (s *dictMemoryStore) resolveSourceTenant(tenantID string, dictCode string) (string, bool) {
	return s.DictMemoryStore.ResolveSourceTenant(tenantID, dictCode)
}

func (s *dictMemoryStore) valuesForTenant(tenantID string) []DictValueItem {
	return s.DictMemoryStore.ValuesForTenant(tenantID)
}

func resolveDictSourceTenantAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string) (string, error) {
	return iampersistence.ResolveDictSourceTenantAsOfTx(ctx, tx, tenantID, dictCode, asOf)
}

func resolveDictSourceTenantTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string) (string, error) {
	return iampersistence.ResolveDictSourceTenantTx(ctx, tx, tenantID, dictCode)
}

func getDictFromEventTx(ctx context.Context, tx pgx.Tx, tenantID string, eventID int64) (DictItem, error) {
	return iampersistence.GetDictFromEventTx(ctx, tx, tenantID, eventID)
}

func getDictValueFromEventTx(ctx context.Context, tx pgx.Tx, tenantID string, eventID int64) (DictValueItem, error) {
	return iampersistence.GetDictValueFromEventTx(ctx, tx, tenantID, eventID)
}

func assertTenantBaselineReadyTx(ctx context.Context, tx pgx.Tx, tenantID string) error {
	return iampersistence.AssertTenantBaselineReadyTx(ctx, tx, tenantID)
}

func assertTenantDictActiveAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string) error {
	return iampersistence.AssertTenantDictActiveAsOfTx(ctx, tx, tenantID, dictCode, asOf)
}

func resolveValueLabelByTenant(ctx context.Context, tx pgx.Tx, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	return iampersistence.ResolveValueLabelByTenant(ctx, tx, tenantID, asOf, dictCode, code)
}

func cloneDictItem(item DictItem) DictItem {
	return iampersistence.CloneDictItem(item)
}

func dictActiveAsOf(item DictItem, asOf string) bool {
	return iampersistence.DictActiveAsOf(item, asOf)
}

func dictStatusAsOf(item DictItem, asOf string) string {
	return iampersistence.DictStatusAsOf(item, asOf)
}

func valueStatusAsOf(item DictValueItem, asOf string) string {
	return iampersistence.ValueStatusAsOf(item, asOf)
}

func dictDisplayName(dictCode string) string {
	return iampersistence.DictDisplayName(dictCode)
}
