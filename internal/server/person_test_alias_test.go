package server

import personmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/person"

func newPersonPGStore(pool pgBeginner) PersonStore {
	return personmodule.NewPGStore(pool)
}

func newPersonMemoryStore() PersonStore {
	return personmodule.NewMemoryStore()
}
