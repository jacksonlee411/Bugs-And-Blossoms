package iam

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/iam/infrastructure/persistence"
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewDictPGStore(pool PGBeginner) *persistence.PGStore {
	return persistence.NewPGStore(pool)
}

func NewDictMemoryStore() *persistence.MemoryStore {
	return persistence.NewMemoryStore()
}
