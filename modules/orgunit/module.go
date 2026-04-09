package orgunit

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type SetIDPGStore struct {
	*persistence.SetIDPGStore
}

func NewPGStore(pool PGBeginner) ports.OrgUnitWriteStore {
	return persistence.NewOrgUnitPGStore(pool)
}

func NewWriteService(store ports.OrgUnitWriteStore) services.OrgUnitWriteService {
	return services.NewOrgUnitWriteService(store)
}

func NewWriteServiceWithPGStore(pool PGBeginner) services.OrgUnitWriteService {
	return services.NewOrgUnitWriteService(NewPGStore(pool))
}

func NewSetIDMemoryStore() *SetIDMemoryStore {
	core := persistence.NewSetIDMemoryStore()
	return &SetIDMemoryStore{
		SetIDMemoryStore:    core,
		SetIDs:              core.SetIDs,
		Bindings:            core.Bindings,
		ScopePackages:       core.ScopePackages,
		ScopeSubscriptions:  core.ScopeSubscriptions,
		GlobalScopePackages: core.GlobalScopePackages,
		GlobalSetIDName:     core.GlobalSetIDName,
		Seq:                 core.Seq,
	}
}

func NewSetIDPGStore(pool PGBeginner) *SetIDPGStore {
	return &SetIDPGStore{SetIDPGStore: persistence.NewSetIDPGStore(pool)}
}

func (s *SetIDPGStore) EnsureGlobalShareSetID(ctx context.Context, initiatorID string) error {
	return s.SetIDPGStore.EnsureGlobalShareSetID(ctx, initiatorID)
}

type SetIDMemoryStore struct {
	*persistence.SetIDMemoryStore
	SetIDs              map[string]map[string]ports.SetID
	Bindings            map[string]map[string]ports.SetIDBindingRow
	ScopePackages       map[string]map[string]map[string]ports.ScopePackage
	ScopeSubscriptions  map[string]map[string]map[string]ports.ScopeSubscription
	GlobalScopePackages map[string]map[string]ports.ScopePackage
	GlobalSetIDName     string
	Seq                 int
}

func (s *SetIDMemoryStore) EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error {
	if err := s.SetIDMemoryStore.EnsureBootstrap(ctx, tenantID, initiatorID); err != nil {
		return err
	}
	s.GlobalSetIDName = s.SetIDMemoryStore.GlobalSetIDName
	s.Seq = s.SetIDMemoryStore.Seq
	return nil
}

func (s *SetIDMemoryStore) CreateGlobalSetID(ctx context.Context, name string, requestID string, initiatorID string, actorScope string) error {
	if err := s.SetIDMemoryStore.CreateGlobalSetID(ctx, name, requestID, initiatorID, actorScope); err != nil {
		return err
	}
	s.GlobalSetIDName = s.SetIDMemoryStore.GlobalSetIDName
	return nil
}

func (s *SetIDMemoryStore) CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, requestID string, initiatorID string) (ports.ScopePackage, error) {
	pkg, err := s.SetIDMemoryStore.CreateScopePackage(ctx, tenantID, scopeCode, packageCode, ownerSetID, name, effectiveDate, requestID, initiatorID)
	s.Seq = s.SetIDMemoryStore.Seq
	return pkg, err
}

func (s *SetIDMemoryStore) CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ports.ScopePackage, error) {
	pkg, err := s.SetIDMemoryStore.CreateGlobalScopePackage(ctx, scopeCode, packageCode, name, effectiveDate, requestID, initiatorID, actorScope)
	s.Seq = s.SetIDMemoryStore.Seq
	return pkg, err
}
