package services

import (
	"context"
	"encoding/json"
	"strings"

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

func (f AssignmentsFacade) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error) {
	prepared, err := PrepareUpsertPrimaryAssignment(effectiveDate, personUUID, positionUUID, status, allocatedFte)
	if err != nil {
		return types.Assignment{}, err
	}

	assignment, err := f.store.UpsertPrimaryAssignmentForPerson(
		ctx,
		tenantID,
		prepared.EffectiveDate,
		prepared.PersonUUID,
		prepared.PositionUUID,
		prepared.Status,
		prepared.AllocatedFTE,
	)
	if err != nil {
		return types.Assignment{}, err
	}
	if strings.TrimSpace(assignment.Status) == "" {
		assignment.Status = prepared.Status
	}
	return assignment, nil
}

func (f AssignmentsFacade) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	prepared, err := PrepareCorrectAssignmentEvent(tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
	if err != nil {
		return "", err
	}
	return f.store.CorrectAssignmentEvent(ctx, tenantID, prepared.AssignmentUUID, prepared.TargetEffectiveDate, prepared.CanonicalPayload)
}

func (f AssignmentsFacade) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	prepared, err := PrepareRescindAssignmentEvent(tenantID, assignmentUUID, targetEffectiveDate, payload)
	if err != nil {
		return "", err
	}
	return f.store.RescindAssignmentEvent(ctx, tenantID, prepared.AssignmentUUID, prepared.TargetEffectiveDate, prepared.CanonicalPayload)
}
