package cubebox

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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
	return services.NewFileService(nil, infrastructure.NewLocalFileStore(rootDir))
}

func DefaultLocalFileRoot() string {
	if raw := strings.TrimSpace(os.Getenv("CUBEBOX_FILE_ROOT")); raw != "" {
		return raw
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(".local/cubebox/files")
	}
	return filepath.Join(wd, ".local", "cubebox", "files")
}

func NewDefaultLocalFileService() *services.FileService {
	return NewLocalFileService(DefaultLocalFileRoot())
}

func NewPGFileService(pool PGBeginner, rootDir string) *services.FileService {
	if pool == nil {
		return NewLocalFileService(rootDir)
	}
	return services.NewFileService(NewPGStore(pool), infrastructure.NewLocalFileStore(rootDir))
}

func NewFacade(store *persistence.PGStore, runtime services.RuntimeProbe, fileSvc *services.FileService, legacy services.LegacyFacade) *services.Facade {
	if store == nil {
		return services.NewFacade(nil, runtime, fileSvc, legacy)
	}
	return services.NewFacade(store, runtime, fileSvc, legacy)
}
