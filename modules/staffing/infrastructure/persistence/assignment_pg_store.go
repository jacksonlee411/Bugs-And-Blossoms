package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	staffingservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/services"
)

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type AssignmentPGStore struct {
	pool pgBeginner
}

func NewAssignmentPGStore(pool pgBeginner) ports.AssignmentStore {
	return &AssignmentPGStore{pool: pool}
}

var assignmentEventNamespace = uuid.Must(uuid.Parse("6d73e345-ae88-4e9d-a2ed-89f292e94f7b"))

func deterministicAssignmentEventID(tenantID string, assignmentID string, effectiveDate string, assignmentType string) string {
	// Stable, payload-independent event_uuid for rerunnable upsert (DEV-PLAN-031 M3-A).
	name := fmt.Sprintf("staffing.assignment_event:%s:%s:%s:%s", tenantID, assignmentID, assignmentType, effectiveDate)
	return uuid.NewSHA1(assignmentEventNamespace, []byte(name)).String()
}

func (s *AssignmentPGStore) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  assignment_uuid::text,
	  person_uuid::text,
	  position_uuid::text,
	  status,
	  effective_date::text AS effective_date
	FROM staffing.get_assignment_snapshot($1::uuid, $2::uuid, $3::date)
	ORDER BY effective_date DESC, assignment_uuid::text ASC
	`, tenantID, personUUID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Assignment
	for rows.Next() {
		var a types.Assignment
		if err := rows.Scan(&a.AssignmentUUID, &a.PersonUUID, &a.PositionUUID, &a.Status, &a.EffectiveAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AssignmentPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error) {
	prepared, err := staffingservices.PrepareUpsertPrimaryAssignment(effectiveDate, personUUID, positionUUID, status, allocatedFte)
	if err != nil {
		return types.Assignment{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.Assignment{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.Assignment{}, err
	}

	effectiveDate = prepared.EffectiveDate
	personUUID = prepared.PersonUUID
	positionUUID = prepared.PositionUUID
	status = prepared.Status
	allocatedFte = prepared.AllocatedFTE

	assignmentType := "primary"

	var assignmentID string
	err = tx.QueryRow(ctx, `
	SELECT assignment_uuid::text
	FROM staffing.assignments
	WHERE tenant_uuid = $1::uuid AND person_uuid = $2::uuid AND assignment_type = $3::text
	`, tenantID, personUUID, assignmentType).Scan(&assignmentID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return types.Assignment{}, err
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&assignmentID); err != nil {
			return types.Assignment{}, err
		}
	}

	var eventsCount int
	if err := tx.QueryRow(ctx, `
	SELECT count(*)
	FROM staffing.assignment_events
	WHERE tenant_uuid = $1::uuid AND assignment_uuid = $2::uuid
	`, tenantID, assignmentID).Scan(&eventsCount); err != nil {
		return types.Assignment{}, err
	}

	eventType := "UPDATE"
	if eventsCount == 0 {
		eventType = "CREATE"
	}

	eventID := deterministicAssignmentEventID(tenantID, assignmentID, effectiveDate, assignmentType)
	requestID := eventID
	initiatorID := tenantID

	// Rerunnable upsert (DEV-PLAN-031 M3-A):
	// - If the effective_date already exists, reuse existing (event_uuid, request_id, initiator_uuid, event_type)
	//   so the Kernel hits the idempotency path instead of violating (tenant_uuid, assignment_uuid, effective_date) unique.
	{
		var existingEventType string
		var existingRequestID string
		var existingInitiatorID string
		err := tx.QueryRow(ctx, `
	SELECT
	  event_uuid::text,
	  event_type,
	  request_id,
	  initiator_uuid::text
	FROM staffing.assignment_events
	WHERE tenant_uuid = $1::uuid
	  AND assignment_uuid = $2::uuid
	  AND effective_date = $3::date
	LIMIT 1
	`, tenantID, assignmentID, effectiveDate).Scan(&eventID, &existingEventType, &existingRequestID, &existingInitiatorID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return types.Assignment{}, err
		}
		if err == nil {
			eventType = existingEventType
			requestID = existingRequestID
			initiatorID = existingInitiatorID
		}
	}

	payload := `{"position_uuid":` + strconv.Quote(positionUUID)
	if allocatedFte != "" {
		payload += `,"allocated_fte":` + strconv.Quote(allocatedFte)
	}
	if status != "" {
		payload += `,"status":` + strconv.Quote(status)
	}
	payload += `}`

	if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_assignment_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::uuid,
	  $5::text,
	  $6::text,
	  $7::date,
	  $8::jsonb,
	  $9::text,
	  $10::uuid
	)
	`, eventID, tenantID, assignmentID, personUUID, assignmentType, eventType, effectiveDate, []byte(payload), requestID, initiatorID); err != nil {
		return types.Assignment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return types.Assignment{}, err
	}

	return types.Assignment{
		AssignmentUUID: assignmentID,
		PersonUUID:     personUUID,
		PositionUUID:   positionUUID,
		Status:         status,
		EffectiveAt:    effectiveDate,
	}, nil
}

func (s *AssignmentPGStore) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	prepared, err := staffingservices.PrepareCorrectAssignmentEvent(tenantID, assignmentID, targetEffectiveDate, replacementPayload)
	if err != nil {
		return "", err
	}

	assignmentID = prepared.AssignmentUUID
	targetEffectiveDate = prepared.TargetEffectiveDate
	canonicalPayload := prepared.CanonicalPayload
	eventID := prepared.EventID
	requestID := eventID
	initiatorID := tenantID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_assignment_event_correction(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::date,
	  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
	`, eventID, tenantID, assignmentID, targetEffectiveDate, canonicalPayload, requestID, initiatorID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return eventID, nil
}

func (s *AssignmentPGStore) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	prepared, err := staffingservices.PrepareRescindAssignmentEvent(tenantID, assignmentID, targetEffectiveDate, payload)
	if err != nil {
		return "", err
	}

	assignmentID = prepared.AssignmentUUID
	targetEffectiveDate = prepared.TargetEffectiveDate
	canonicalPayload := prepared.CanonicalPayload
	eventID := prepared.EventID
	requestID := eventID
	initiatorID := tenantID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_assignment_event_rescind(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::date,
	  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
	`, eventID, tenantID, assignmentID, targetEffectiveDate, canonicalPayload, requestID, initiatorID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return eventID, nil
}
