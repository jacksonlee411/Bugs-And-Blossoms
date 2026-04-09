package server

import (
	staffingports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	staffingtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

type Position = staffingtypes.Position
type Assignment = staffingtypes.Assignment

type PositionStore = staffingports.PositionStore
type AssignmentStore = staffingports.AssignmentStore
