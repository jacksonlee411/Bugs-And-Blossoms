package server

import (
	"context"
	"encoding/json"

	staffingmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing"
)

type staffingAssignmentPGStore struct {
	*staffingPGStore
}

func newStaffingAssignmentPGStore(pool pgBeginner) *staffingAssignmentPGStore {
	return &staffingAssignmentPGStore{staffingPGStore: newStaffingPGStore(pool)}
}

func (s *staffingAssignmentPGStore) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.ListAssignmentsForPerson(ctx, tenantID, asOfDate, personUUID)
}

func (s *staffingAssignmentPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (Assignment, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.UpsertPrimaryAssignmentForPerson(ctx, tenantID, effectiveDate, personUUID, positionUUID, status, allocatedFte)
}

func (s *staffingAssignmentPGStore) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.CorrectAssignmentEvent(ctx, tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
}

func (s *staffingAssignmentPGStore) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.RescindAssignmentEvent(ctx, tenantID, assignmentUUID, targetEffectiveDate, payload)
}
