package server

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	jobcatalogmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog"
	jobcatalogpersistence "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/infrastructure/persistence"
	jobcatalogservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/services"
)

type jobcatalogMemoryStore = jobcatalogpersistence.MemoryStore
type jobcatalogPGStore = jobcatalogpersistence.PGStore

func newJobCatalogPGStore(pool pgBeginner) JobCatalogStore {
	return jobcatalogmodule.NewPGStore(pool)
}

func newJobCatalogMemoryStore() JobCatalogStore {
	return jobcatalogmodule.NewMemoryStore()
}

func normalizeSetID(input string) string {
	return jobcatalogservices.NormalizeSetID(input)
}

func normalizePackageCode(input string) string {
	return jobcatalogservices.NormalizePackageCode(input)
}

func canEditDefltPackage(ctx context.Context) bool {
	return jobcatalogservices.CanEditDefltPackage(jobCatalogPrincipalFromContext(ctx))
}

func ownerSetIDEditable(ctx context.Context, setidStore jobCatalogSetIDStore, tenantID string, ownerSetID string) bool {
	return jobcatalogservices.OwnerSetIDEditable(ctx, jobCatalogPrincipalFromContext(ctx), adaptJobCatalogSetIDStore(setidStore), tenantID, ownerSetID)
}

func loadOwnedJobCatalogPackages(ctx context.Context, setidStore jobCatalogSetIDStore, tenantID string, asOf string) ([]OwnedScopePackage, error) {
	rows, err := jobcatalogservices.LoadOwnedJobCatalogPackages(ctx, jobCatalogPrincipalFromContext(ctx), adaptJobCatalogSetIDStore(setidStore), tenantID, asOf)
	if err != nil {
		return nil, err
	}
	return toServerOwnedScopePackages(rows), nil
}

func resolveJobCatalogPackageByCode(ctx context.Context, tx pgx.Tx, tenantID string, packageCode string, asOfDate string) (JobCatalogPackage, error) {
	return jobcatalogpersistence.ResolveJobCatalogPackageByCode(ctx, tx, tenantID, packageCode, asOfDate)
}

func stampJobProfileSetID(ctx context.Context, tx pgx.Tx, tenantID string, packageUUID string, profileUUID string, setID string) error {
	return jobcatalogpersistence.StampJobProfileSetID(ctx, tx, tenantID, packageUUID, profileUUID, setID)
}

func quoteAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, strconv.Quote(v))
	}
	return out
}
