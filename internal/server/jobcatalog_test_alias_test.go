package server

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	jobcatalogmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog"
	jobcatalogpersistence "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/infrastructure/persistence"
)

type jobcatalogMemoryStore = jobcatalogpersistence.MemoryStore
type jobcatalogPGStore = jobcatalogpersistence.PGStore

func newJobCatalogPGStore(pool pgBeginner) JobCatalogStore {
	return jobcatalogmodule.NewPGStore(pool)
}

func newJobCatalogMemoryStore() JobCatalogStore {
	return jobcatalogmodule.NewMemoryStore()
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
