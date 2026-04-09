package jobcatalog

import "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/infrastructure/persistence"

func NewMemoryStore() *persistence.MemoryStore {
	return persistence.NewMemoryStore()
}
