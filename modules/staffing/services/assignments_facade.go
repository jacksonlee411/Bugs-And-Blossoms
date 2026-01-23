package services

import (
	"context"
	"encoding/json"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

type AssignmentsFacade struct {
	store ports.AssignmentStore
}

func NewAssignmentsFacade(store ports.AssignmentStore) AssignmentsFacade {
	return AssignmentsFacade{store: store}
}

func (f AssignmentsFacade) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error) {
	return f.store.ListAssignmentsForPerson(ctx, tenantID, asOfDate, personUUID)
}

func (f AssignmentsFacade) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, allocatedFte string) (types.Assignment, error) {
	return f.store.UpsertPrimaryAssignmentForPerson(ctx, tenantID, effectiveDate, personUUID, positionID, status, allocatedFte)
}

func (f AssignmentsFacade) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	return f.store.CorrectAssignmentEvent(ctx, tenantID, assignmentID, targetEffectiveDate, replacementPayload)
}

func (f AssignmentsFacade) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	return f.store.RescindAssignmentEvent(ctx, tenantID, assignmentID, targetEffectiveDate, payload)
}
