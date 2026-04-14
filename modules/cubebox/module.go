package cubebox

import "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"

func NewLocalFileService(rootDir string) *services.FileService {
	return services.NewFileService(rootDir)
}
