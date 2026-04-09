package server

import staffingmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing"

func newStaffingPGStore(pool pgBeginner) PositionStore {
	return staffingmodule.NewPositionPGStore(pool)
}
