package iam

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/iam/infrastructure/persistence"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type DictPGStore struct {
	pool PGBeginner
	core *persistence.PGStore
}

func NewDictPGStore(pool PGBeginner) *DictPGStore {
	return &DictPGStore{
		pool: pool,
		core: persistence.NewPGStore(pool),
	}
}

func (s *DictPGStore) ListDicts(ctx context.Context, tenantID string, asOf string) ([]persistence.DictItem, error) {
	return s.core.ListDicts(ctx, tenantID, asOf)
}

func (s *DictPGStore) CreateDict(ctx context.Context, tenantID string, req persistence.DictCreateRequest) (persistence.DictItem, bool, error) {
	return s.core.CreateDict(ctx, tenantID, req)
}

func (s *DictPGStore) DisableDict(ctx context.Context, tenantID string, req persistence.DictDisableRequest) (persistence.DictItem, bool, error) {
	return s.core.DisableDict(ctx, tenantID, req)
}

func (s *DictPGStore) ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]persistence.DictValueItem, error) {
	return s.core.ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, status)
}

func (s *DictPGStore) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	return s.core.ResolveValueLabel(ctx, tenantID, asOf, dictCode, code)
}

func (s *DictPGStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	return s.core.ListOptions(ctx, tenantID, asOf, dictCode, keyword, limit)
}

func (s *DictPGStore) CreateDictValue(ctx context.Context, tenantID string, req persistence.DictCreateValueRequest) (persistence.DictValueItem, bool, error) {
	return s.core.CreateDictValue(ctx, tenantID, req)
}

func (s *DictPGStore) DisableDictValue(ctx context.Context, tenantID string, req persistence.DictDisableValueRequest) (persistence.DictValueItem, bool, error) {
	return s.core.DisableDictValue(ctx, tenantID, req)
}

func (s *DictPGStore) CorrectDictValue(ctx context.Context, tenantID string, req persistence.DictCorrectValueRequest) (persistence.DictValueItem, bool, error) {
	return s.core.CorrectDictValue(ctx, tenantID, req)
}

func (s *DictPGStore) SubmitDictEvent(
	ctx context.Context,
	tenantID string,
	dictCode string,
	eventType string,
	day string,
	payload map[string]any,
	requestID string,
	initiator string,
) (persistence.DictItem, bool, error) {
	return persistence.SubmitDictEvent(ctx, s.pool, tenantID, dictCode, eventType, day, payload, requestID, initiator)
}

func (s *DictPGStore) SubmitValueEvent(
	ctx context.Context,
	tenantID string,
	dictCode string,
	code string,
	eventType string,
	day string,
	payload map[string]any,
	requestID string,
	initiator string,
) (persistence.DictValueItem, bool, error) {
	return persistence.SubmitValueEvent(ctx, s.pool, tenantID, dictCode, code, eventType, day, payload, requestID, initiator)
}

func (s *DictPGStore) ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]persistence.DictValueAuditItem, error) {
	return s.core.ListDictValueAudit(ctx, tenantID, dictCode, code, limit)
}

type DictMemoryStore struct {
	*persistence.MemoryStore
	Dicts  map[string]map[string]persistence.DictItem
	Values map[string][]persistence.DictValueItem
}

func NewDictMemoryStore() *DictMemoryStore {
	core := persistence.NewMemoryStore()
	return &DictMemoryStore{
		MemoryStore: core,
		Dicts:       core.Dicts,
		Values:      core.Values,
	}
}

func (s *DictMemoryStore) ResolveSourceTenant(tenantID string, dictCode string) (string, bool) {
	if _, ok := s.Dicts[tenantID][dictCode]; ok {
		return tenantID, true
	}
	return "", false
}

func (s *DictMemoryStore) ValuesForTenant(tenantID string) []persistence.DictValueItem {
	if items, ok := s.Values[tenantID]; ok {
		return items
	}
	return nil
}
