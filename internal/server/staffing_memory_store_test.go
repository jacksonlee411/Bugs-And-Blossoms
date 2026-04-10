package server

import (
	"context"
	"encoding/json"

	staffingmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing"
)

type staffingMemoryStore struct {
	positions       map[string][]Position
	assigns         map[string]map[string][]Assignment
	positions0      []Position
	positionStore   PositionStore
	assignmentStore AssignmentStore
}

func newStaffingMemoryStore() *staffingMemoryStore {
	assigns := make(map[string]map[string][]Assignment)
	positions := make(map[string][]Position)
	return &staffingMemoryStore{
		positions:       positions,
		assigns:         assigns,
		positionStore:   staffingmodule.NewPositionMemoryStoreWithState(positions),
		assignmentStore: staffingmodule.NewAssignmentMemoryStoreWithState(assigns),
	}
}

func (s *staffingMemoryStore) positionDelegate() PositionStore {
	if s.positions == nil {
		s.positions = make(map[string][]Position)
	}
	if s.positionStore == nil {
		s.positionStore = staffingmodule.NewPositionMemoryStoreWithState(s.positions)
	}
	return s.positionStore
}

func (s *staffingMemoryStore) assignmentDelegate() AssignmentStore {
	if s.assigns == nil {
		s.assigns = make(map[string]map[string][]Assignment)
	}
	if s.assignmentStore == nil {
		s.assignmentStore = staffingmodule.NewAssignmentMemoryStoreWithState(s.assigns)
	}
	return s.assignmentStore
}

func (s *staffingMemoryStore) ListPositionsCurrent(_ context.Context, tenantID string, _ string) ([]Position, error) {
	return s.positionDelegate().ListPositionsCurrent(context.Background(), tenantID, "")
}

func (s *staffingMemoryStore) CreatePositionCurrent(_ context.Context, tenantID string, effectiveDate string, orgNodeKey string, jobProfileUUID string, capacityFTE string, name string) (Position, error) {
	return s.positionDelegate().CreatePositionCurrent(context.Background(), tenantID, effectiveDate, orgNodeKey, jobProfileUUID, capacityFTE, name)
}

func (s *staffingMemoryStore) UpdatePositionCurrent(_ context.Context, tenantID string, positionUUID string, effectiveDate string, orgNodeKey string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
	return s.positionDelegate().UpdatePositionCurrent(context.Background(), tenantID, positionUUID, effectiveDate, orgNodeKey, reportsToPositionUUID, jobProfileUUID, capacityFTE, name, lifecycleStatus)
}

func (s *staffingMemoryStore) ListAssignmentsForPerson(_ context.Context, tenantID string, _ string, personUUID string) ([]Assignment, error) {
	return s.assignmentDelegate().ListAssignmentsForPerson(context.Background(), tenantID, "", personUUID)
}

func (s *staffingMemoryStore) UpsertPrimaryAssignmentForPerson(_ context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, _ string) (Assignment, error) {
	return s.assignmentDelegate().UpsertPrimaryAssignmentForPerson(context.Background(), tenantID, effectiveDate, personUUID, positionUUID, status, "")
}

func (s *staffingMemoryStore) CorrectAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	return s.assignmentDelegate().CorrectAssignmentEvent(context.Background(), tenantID, assignmentID, targetEffectiveDate, replacementPayload)
}

func (s *staffingMemoryStore) RescindAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, _ json.RawMessage) (string, error) {
	return s.assignmentDelegate().RescindAssignmentEvent(context.Background(), tenantID, assignmentID, targetEffectiveDate, nil)
}
