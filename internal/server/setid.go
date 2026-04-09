package server

import (
	"context"
	orgunitmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit"
	orgunitports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
)

type SetID = orgunitports.SetID
type SetIDBindingRow = orgunitports.SetIDBindingRow
type ScopeCode = orgunitports.ScopeCode
type ScopePackage = orgunitports.ScopePackage
type OwnedScopePackage = orgunitports.OwnedScopePackage
type ScopeSubscription = orgunitports.ScopeSubscription
type SetIDGovernanceStore = orgunitports.SetIDGovernanceStore

type businessUnitLister interface {
	ListBusinessUnitsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
}

type setidPGStore struct {
	*orgunitmodule.SetIDPGStore
	pool pgBeginner
}

func newSetIDPGStore(pool pgBeginner) SetIDGovernanceStore {
	return &setidPGStore{
		SetIDPGStore: orgunitmodule.NewSetIDPGStore(pool),
		pool:         pool,
	}
}

func (s *setidPGStore) delegate() *orgunitmodule.SetIDPGStore {
	if s.SetIDPGStore == nil {
		s.SetIDPGStore = orgunitmodule.NewSetIDPGStore(s.pool)
	}
	return s.SetIDPGStore
}

func (s *setidPGStore) EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error {
	return s.delegate().EnsureBootstrap(ctx, tenantID, initiatorID)
}

func (s *setidPGStore) ListSetIDs(ctx context.Context, tenantID string) ([]SetID, error) {
	return s.delegate().ListSetIDs(ctx, tenantID)
}

func (s *setidPGStore) CreateSetID(ctx context.Context, tenantID string, setID string, name string, effectiveDate string, requestID string, initiatorID string) error {
	return s.delegate().CreateSetID(ctx, tenantID, setID, name, effectiveDate, requestID, initiatorID)
}

func (s *setidPGStore) ListSetIDBindings(ctx context.Context, tenantID string, asOfDate string) ([]SetIDBindingRow, error) {
	return s.delegate().ListSetIDBindings(ctx, tenantID, asOfDate)
}

func (s *setidPGStore) BindSetID(ctx context.Context, tenantID string, orgUnitID string, effectiveDate string, setID string, requestID string, initiatorID string) error {
	return s.delegate().BindSetID(ctx, tenantID, orgUnitID, effectiveDate, setID, requestID, initiatorID)
}

func (s *setidPGStore) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	return s.delegate().ResolveSetID(ctx, tenantID, orgUnitID, asOfDate)
}

func (s *setidPGStore) ensureGlobalShareSetID(ctx context.Context, initiatorID string) error {
	return s.delegate().EnsureGlobalShareSetID(ctx, initiatorID)
}

func (s *setidPGStore) ListGlobalSetIDs(ctx context.Context) ([]SetID, error) {
	return s.delegate().ListGlobalSetIDs(ctx)
}

func (s *setidPGStore) listGlobalSetIDs(ctx context.Context) ([]SetID, error) {
	return s.delegate().ListGlobalSetIDs(ctx)
}

func (s *setidPGStore) CreateGlobalSetID(ctx context.Context, name string, requestID string, initiatorID string, actorScope string) error {
	return s.delegate().CreateGlobalSetID(ctx, name, requestID, initiatorID, actorScope)
}

func (s *setidPGStore) ListScopeCodes(ctx context.Context, tenantID string) ([]ScopeCode, error) {
	return s.delegate().ListScopeCodes(ctx, tenantID)
}

func (s *setidPGStore) CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error) {
	return s.delegate().CreateScopePackage(ctx, tenantID, scopeCode, packageCode, ownerSetID, name, effectiveDate, requestID, initiatorID)
}

func (s *setidPGStore) DisableScopePackage(ctx context.Context, tenantID string, packageID string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error) {
	return s.delegate().DisableScopePackage(ctx, tenantID, packageID, effectiveDate, requestID, initiatorID)
}

func (s *setidPGStore) ListScopePackages(ctx context.Context, tenantID string, scopeCode string) ([]ScopePackage, error) {
	return s.delegate().ListScopePackages(ctx, tenantID, scopeCode)
}

func (s *setidPGStore) ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]OwnedScopePackage, error) {
	return s.delegate().ListOwnedScopePackages(ctx, tenantID, scopeCode, asOfDate)
}

func (s *setidPGStore) CreateScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, requestID string, initiatorID string) (ScopeSubscription, error) {
	return s.delegate().CreateScopeSubscription(ctx, tenantID, setID, scopeCode, packageID, packageOwner, effectiveDate, requestID, initiatorID)
}

func (s *setidPGStore) GetScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, asOfDate string) (ScopeSubscription, error) {
	return s.delegate().GetScopeSubscription(ctx, tenantID, setID, scopeCode, asOfDate)
}

func (s *setidPGStore) CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ScopePackage, error) {
	return s.delegate().CreateGlobalScopePackage(ctx, scopeCode, packageCode, name, effectiveDate, requestID, initiatorID, actorScope)
}

func (s *setidPGStore) ListGlobalScopePackages(ctx context.Context, scopeCode string) ([]ScopePackage, error) {
	return s.delegate().ListGlobalScopePackages(ctx, scopeCode)
}

type setidMemoryStore struct {
	*orgunitmodule.SetIDMemoryStore
	setids              map[string]map[string]SetID
	bindings            map[string]map[string]SetIDBindingRow
	scopePackages       map[string]map[string]map[string]ScopePackage
	scopeSubscriptions  map[string]map[string]map[string]ScopeSubscription
	globalScopePackages map[string]map[string]ScopePackage
	globalSetIDName     string
	seq                 int
}

func newSetIDMemoryStore() SetIDGovernanceStore {
	core := orgunitmodule.NewSetIDMemoryStore()
	return &setidMemoryStore{
		SetIDMemoryStore:    core,
		setids:              core.SetIDs,
		bindings:            core.Bindings,
		scopePackages:       core.ScopePackages,
		scopeSubscriptions:  core.ScopeSubscriptions,
		globalScopePackages: core.GlobalScopePackages,
		globalSetIDName:     core.GlobalSetIDName,
		seq:                 core.Seq,
	}
}

func (s *setidMemoryStore) EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error {
	err := s.SetIDMemoryStore.EnsureBootstrap(ctx, tenantID, initiatorID)
	s.globalSetIDName = s.SetIDMemoryStore.GlobalSetIDName
	s.seq = s.SetIDMemoryStore.Seq
	return err
}

func (s *setidMemoryStore) CreateGlobalSetID(ctx context.Context, name string, requestID string, initiatorID string, actorScope string) error {
	if err := s.SetIDMemoryStore.CreateGlobalSetID(ctx, name, requestID, initiatorID, actorScope); err != nil {
		return err
	}
	s.globalSetIDName = s.SetIDMemoryStore.GlobalSetIDName
	return nil
}

func (s *setidMemoryStore) CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error) {
	pkg, err := s.SetIDMemoryStore.CreateScopePackage(ctx, tenantID, scopeCode, packageCode, ownerSetID, name, effectiveDate, requestID, initiatorID)
	s.seq = s.SetIDMemoryStore.Seq
	return pkg, err
}

func (s *setidMemoryStore) CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ScopePackage, error) {
	pkg, err := s.SetIDMemoryStore.CreateGlobalScopePackage(ctx, scopeCode, packageCode, name, effectiveDate, requestID, initiatorID, actorScope)
	s.seq = s.SetIDMemoryStore.Seq
	return pkg, err
}

func listBusinessUnitsCurrent(ctx context.Context, orgStore OrgUnitStore, tenantID string, asOf string) ([]OrgUnitNode, error) {
	if lister, ok := orgStore.(businessUnitLister); ok {
		return lister.ListBusinessUnitsCurrent(ctx, tenantID, asOf)
	}
	nodes, err := orgStore.ListNodesCurrent(ctx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	businessUnits := make([]OrgUnitNode, 0, len(nodes))
	for _, n := range nodes {
		if n.IsBusinessUnit {
			businessUnits = append(businessUnits, n)
		}
	}
	return businessUnits, nil
}
