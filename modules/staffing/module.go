package staffing

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/infrastructure/persistence"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/services"
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewAssignmentPGStore(pool PGBeginner) ports.AssignmentStore {
	return persistence.NewAssignmentPGStore(pool)
}

func NewAssignmentsFacade(store ports.AssignmentStore) services.AssignmentsFacade {
	return services.NewAssignmentsFacade(store)
}

func NewAssignmentsFacadeWithPGStore(pool PGBeginner) services.AssignmentsFacade {
	return services.NewAssignmentsFacade(NewAssignmentPGStore(pool))
}
