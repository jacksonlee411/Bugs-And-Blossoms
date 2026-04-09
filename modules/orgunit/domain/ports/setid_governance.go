package ports

import "context"

type SetID struct {
	SetID    string
	Name     string
	Status   string
	IsShared bool
}

type SetIDBindingRow struct {
	OrgUnitID string
	SetID     string
	ValidFrom string
	ValidTo   string
}

type ScopeCode struct {
	ScopeCode   string
	OwnerModule string
	ShareMode   string
	IsStable    bool
}

type ScopePackage struct {
	PackageID     string
	ScopeCode     string
	PackageCode   string
	OwnerSetID    string
	Name          string
	Status        string
	EffectiveDate string `json:"-"`
	UpdatedAt     string `json:"-"`
}

type OwnedScopePackage struct {
	PackageID     string `json:"package_id"`
	ScopeCode     string `json:"scope_code"`
	PackageCode   string `json:"package_code"`
	OwnerSetID    string `json:"owner_setid"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	EffectiveDate string `json:"effective_date"`
}

type ScopeSubscription struct {
	SetID         string
	ScopeCode     string
	PackageID     string
	PackageOwner  string
	EffectiveDate string
	EndDate       string
}

type SetIDGovernanceStore interface {
	EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error
	ListSetIDs(ctx context.Context, tenantID string) ([]SetID, error)
	ListGlobalSetIDs(ctx context.Context) ([]SetID, error)
	CreateSetID(ctx context.Context, tenantID string, setID string, name string, effectiveDate string, requestID string, initiatorID string) error
	ListSetIDBindings(ctx context.Context, tenantID string, asOfDate string) ([]SetIDBindingRow, error)
	BindSetID(ctx context.Context, tenantID string, orgUnitID string, effectiveDate string, setID string, requestID string, initiatorID string) error
	ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error)
	CreateGlobalSetID(ctx context.Context, name string, requestID string, initiatorID string, actorScope string) error
	ListScopeCodes(ctx context.Context, tenantID string) ([]ScopeCode, error)
	CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error)
	DisableScopePackage(ctx context.Context, tenantID string, packageID string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error)
	ListScopePackages(ctx context.Context, tenantID string, scopeCode string) ([]ScopePackage, error)
	ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]OwnedScopePackage, error)
	CreateScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, requestID string, initiatorID string) (ScopeSubscription, error)
	GetScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, asOfDate string) (ScopeSubscription, error)
	CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ScopePackage, error)
	ListGlobalScopePackages(ctx context.Context, scopeCode string) ([]ScopePackage, error)
}
