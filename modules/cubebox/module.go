package cubebox

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/persistence"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewPGStore(pool PGBeginner) *persistence.PGStore {
	return persistence.NewPGStore(pool)
}

func NewLocalFileService(rootDir string) *services.FileService {
	return services.NewFileService(infrastructure.NewLocalFileStore(rootDir))
}
