package types

type Assignment struct {
	AssignmentUUID string `json:"assignment_uuid"`
	PersonUUID     string `json:"person_uuid"`
	PositionUUID   string `json:"position_uuid"`
	Status         string `json:"status"`
	EffectiveAt    string `json:"effective_date"`
}
