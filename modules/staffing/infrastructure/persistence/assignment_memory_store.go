package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/assignmentrules"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

type AssignmentMemoryStore struct {
	assigns map[string]map[string][]types.Assignment
}

func NewAssignmentMemoryStore() ports.AssignmentStore {
	return NewAssignmentMemoryStoreWithState(nil)
}

func NewAssignmentMemoryStoreWithState(assigns map[string]map[string][]types.Assignment) *AssignmentMemoryStore {
	if assigns == nil {
		assigns = make(map[string]map[string][]types.Assignment)
	}
	return &AssignmentMemoryStore{
		assigns: assigns,
	}
}

func (s *AssignmentMemoryStore) ListAssignmentsForPerson(_ context.Context, tenantID string, _ string, personUUID string) ([]types.Assignment, error) {
	byPerson := s.assigns[tenantID]
	if byPerson == nil {
		return nil, nil
	}
	return append([]types.Assignment(nil), byPerson[personUUID]...), nil
}

func (s *AssignmentMemoryStore) UpsertPrimaryAssignmentForPerson(_ context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, _ string) (types.Assignment, error) {
	prepared, err := assignmentrules.PrepareUpsertPrimaryAssignment(effectiveDate, personUUID, positionUUID, status, "")
	if err != nil {
		return types.Assignment{}, err
	}
	effectiveDate = prepared.EffectiveDate
	personUUID = prepared.PersonUUID
	positionUUID = prepared.PositionUUID
	status = prepared.Status

	if s.assigns[tenantID] == nil {
		s.assigns[tenantID] = make(map[string][]types.Assignment)
	}

	a := types.Assignment{
		AssignmentUUID: "as-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		PersonUUID:     personUUID,
		PositionUUID:   positionUUID,
		Status:         status,
		EffectiveAt:    effectiveDate,
	}
	s.assigns[tenantID][personUUID] = append(s.assigns[tenantID][personUUID], a)
	return a, nil
}

func (s *AssignmentMemoryStore) CorrectAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	prepared, err := assignmentrules.PrepareCorrectAssignmentEvent(tenantID, assignmentID, targetEffectiveDate, replacementPayload)
	if err != nil {
		return "", err
	}
	assignmentID = prepared.AssignmentUUID
	targetEffectiveDate = prepared.TargetEffectiveDate
	canonicalPayload := prepared.CanonicalPayload
	eventID := prepared.EventID

	var payload map[string]any
	if err := json.Unmarshal(canonicalPayload, &payload); err != nil {
		return "", err
	}

	byPerson := s.assigns[tenantID]
	for personUUID := range byPerson {
		for i := range byPerson[personUUID] {
			a := &byPerson[personUUID][i]
			if a.AssignmentUUID != assignmentID || a.EffectiveAt != targetEffectiveDate {
				continue
			}
			if v, ok := payload["position_uuid"]; ok {
				a.PositionUUID = fmt.Sprint(v)
			}
			if v, ok := payload["status"]; ok {
				a.Status = fmt.Sprint(v)
			}
			return eventID, nil
		}
	}
	return "", errors.New("STAFFING_ASSIGNMENT_EVENT_NOT_FOUND")
}

func (s *AssignmentMemoryStore) RescindAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, _ json.RawMessage) (string, error) {
	prepared, err := assignmentrules.PrepareRescindAssignmentEvent(tenantID, assignmentID, targetEffectiveDate, nil)
	if err != nil {
		return "", err
	}
	assignmentID = prepared.AssignmentUUID
	targetEffectiveDate = prepared.TargetEffectiveDate
	eventID := prepared.EventID

	byPerson := s.assigns[tenantID]
	for personUUID := range byPerson {
		events := byPerson[personUUID]
		earliest := ""
		for _, a := range events {
			if a.AssignmentUUID != assignmentID {
				continue
			}
			if earliest == "" || a.EffectiveAt < earliest {
				earliest = a.EffectiveAt
			}
		}
		for i := range events {
			if events[i].AssignmentUUID != assignmentID || events[i].EffectiveAt != targetEffectiveDate {
				continue
			}
			if targetEffectiveDate == earliest {
				hasLater := false
				for _, a := range events {
					if a.AssignmentUUID == assignmentID && a.EffectiveAt > targetEffectiveDate {
						hasLater = true
						break
					}
				}
				if hasLater {
					return "", errors.New("STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND")
				}
			}
			byPerson[personUUID] = append(events[:i], events[i+1:]...)
			return eventID, nil
		}
	}
	return "", errors.New("STAFFING_ASSIGNMENT_EVENT_NOT_FOUND")
}
