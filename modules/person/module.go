package person

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/person/infrastructure/persistence"
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
