package jobcatalog

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/infrastructure/persistence"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/services"
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewPGStore(pool PGBeginner) *persistence.PGStore {
	return persistence.NewPGStore(pool)
}

func NewMemoryStore() *persistence.MemoryStore {
	return persistence.NewMemoryStore()
}

type Principal = services.Principal
type SetIDStore = services.SetIDStore
type OwnedScopePackage = services.OwnedScopePackage
type JobCatalogView = services.JobCatalogView
type JobCatalogPackage = types.JobCatalogPackage

type storeAdapter struct {
	store ports.JobCatalogStore
}

func (a storeAdapter) ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (services.JobCatalogPackage, error) {
	pkg, err := a.store.ResolveJobCatalogPackageByCode(ctx, tenantID, packageCode, asOfDate)
	if err != nil {
		return services.JobCatalogPackage{}, err
	}
	return services.JobCatalogPackage{
		PackageUUID: pkg.PackageUUID,
		PackageCode: pkg.PackageCode,
		OwnerSetID:  pkg.OwnerSetID,
	}, nil
}

func (a storeAdapter) ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error) {
	return a.store.ResolveJobCatalogPackageBySetID(ctx, tenantID, setID, asOfDate)
}

func ResolveView(ctx context.Context, principal Principal, store ports.JobCatalogStore, setidStore services.SetIDStore, tenantID string, asOf string, packageCode string, setID string) (services.JobCatalogView, string) {
	return services.ResolveJobCatalogView(ctx, principal, storeAdapter{store: store}, setidStore, tenantID, asOf, packageCode, setID)
}

func CanEditDefltPackage(principal Principal) bool {
	return services.CanEditDefltPackage(principal)
}

func LoadOwnedPackages(ctx context.Context, principal Principal, setidStore services.SetIDStore, tenantID string, asOf string) ([]services.OwnedScopePackage, error) {
	return services.LoadOwnedJobCatalogPackages(ctx, principal, setidStore, tenantID, asOf)
}
