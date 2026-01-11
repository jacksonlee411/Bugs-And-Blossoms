package ports

import (
	"context"
	"encoding/json"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

type AssignmentStore interface {
	ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error)
	UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, baseSalary string, allocatedFte string) (types.Assignment, error)
	CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error)
	RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error)
}
