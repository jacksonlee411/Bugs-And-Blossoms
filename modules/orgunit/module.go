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

func NewPGStore(pool PGBeginner) ports.OrgUnitWriteStore {
	return persistence.NewOrgUnitPGStore(pool)
}

func NewWriteService(store ports.OrgUnitWriteStore) services.OrgUnitWriteService {
	return services.NewOrgUnitWriteService(store)
}

func NewWriteServiceWithPGStore(pool PGBeginner) services.OrgUnitWriteService {
	return services.NewOrgUnitWriteService(NewPGStore(pool))
}

func NewSetIDMemoryStore() ports.SetIDGovernanceStore {
	return persistence.NewSetIDMemoryStore()
}
