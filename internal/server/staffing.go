package server

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	staffingmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing"
	staffingports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	staffingtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

type Position = staffingtypes.Position
type Assignment = staffingtypes.Assignment

type PositionStore = staffingports.PositionStore
type AssignmentStore = staffingports.AssignmentStore

type staffingPGStore struct {
	pool pgBeginner
}

func newStaffingPGStore(pool pgBeginner) *staffingPGStore {
	return &staffingPGStore{pool: pool}
}

func (s *staffingPGStore) ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]Position, error) {
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
		  position_uuid::text,
		  org_unit_id::text,
		  COALESCE(reports_to_position_uuid::text, '') AS reports_to_position_uuid,
		  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
		  COALESCE(jobcatalog_setid_as_of::text, '') AS jobcatalog_setid_as_of,
		  COALESCE(job_profile_uuid::text, '') AS job_profile_uuid,
		  COALESCE(job_profile_code, '') AS job_profile_code,
		  COALESCE(name, '') AS name,
		  lifecycle_status,
	  capacity_fte::text AS capacity_fte,
	  effective_date::text AS effective_date
	FROM staffing.get_position_snapshot($1::uuid, $2::date)
	ORDER BY effective_date DESC, position_uuid::text ASC
	`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(
			&p.PositionUUID,
			&p.OrgUnitID,
			&p.ReportsToPositionUUID,
			&p.JobCatalogSetID,
			&p.JobCatalogSetIDAsOf,
			&p.JobProfileUUID,
			&p.JobProfileCode,
			&p.Name,
			&p.LifecycleStatus,
			&p.CapacityFTE,
			&p.EffectiveAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileUUID string, capacityFTE string, name string) (Position, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Position{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return Position{}, err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Position{}, newBadRequestError("effective_date is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	if orgUnitID == "" {
		return Position{}, newBadRequestError("org_unit_id is required")
	}
	if _, err := parseOrgID8(orgUnitID); err != nil {
		return Position{}, newBadRequestError("org_unit_id must be 8 digits")
	}
	jobProfileUUID = strings.TrimSpace(jobProfileUUID)
	if jobProfileUUID == "" {
		return Position{}, newBadRequestError("job_profile_uuid is required")
	}
	capacityFTE = strings.TrimSpace(capacityFTE)
	name = strings.TrimSpace(name)

	var positionID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionID); err != nil {
		return Position{}, err
	}
	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return Position{}, err
	}

	payload := `{"org_unit_id":` + strconv.Quote(orgUnitID) +
		`,"job_profile_uuid":` + strconv.Quote(jobProfileUUID)
	if capacityFTE != "" {
		payload += `,"capacity_fte":` + strconv.Quote(capacityFTE)
	}
	if name != "" {
		payload += `,"name":` + strconv.Quote(name)
	}
	payload += `}`

	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_position_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  'CREATE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, positionID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return Position{}, err
	}

	var out Position
	if err := tx.QueryRow(ctx, `
		SELECT
		  position_uuid::text,
		  org_unit_id::text,
		  COALESCE(reports_to_position_uuid::text, '') AS reports_to_position_uuid,
		  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
		  COALESCE(jobcatalog_setid_as_of::text, '') AS jobcatalog_setid_as_of,
		  COALESCE(job_profile_uuid::text, '') AS job_profile_uuid,
		  COALESCE(job_profile_code, '') AS job_profile_code,
		  COALESCE(name, '') AS name,
		  lifecycle_status,
		  capacity_fte::text AS capacity_fte,
		  effective_date::text AS effective_date
		FROM staffing.get_position_snapshot($1::uuid, $3::date)
		WHERE position_uuid = $2::uuid
		LIMIT 1
	`, tenantID, positionID, effectiveDate).Scan(
		&out.PositionUUID,
		&out.OrgUnitID,
		&out.ReportsToPositionUUID,
		&out.JobCatalogSetID,
		&out.JobCatalogSetIDAsOf,
		&out.JobProfileUUID,
		&out.JobProfileCode,
		&out.Name,
		&out.LifecycleStatus,
		&out.CapacityFTE,
		&out.EffectiveAt,
	); err != nil {
		return Position{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Position{}, err
	}

	return out, nil
}

func (s *staffingPGStore) UpdatePositionCurrent(ctx context.Context, tenantID string, positionUUID string, effectiveDate string, orgUnitID string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Position{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return Position{}, err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Position{}, newBadRequestError("effective_date is required")
	}
	positionUUID = strings.TrimSpace(positionUUID)
	if positionUUID == "" {
		return Position{}, newBadRequestError("position_uuid is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	reportsToPositionUUID = strings.TrimSpace(reportsToPositionUUID)
	jobProfileUUID = strings.TrimSpace(jobProfileUUID)
	capacityFTE = strings.TrimSpace(capacityFTE)
	name = strings.TrimSpace(name)
	lifecycleStatus = strings.TrimSpace(lifecycleStatus)

	if orgUnitID != "" {
		if _, err := parseOrgID8(orgUnitID); err != nil {
			return Position{}, newBadRequestError("org_unit_id must be 8 digits")
		}
	}

	payloadParts := make([]string, 0, 6)
	if orgUnitID != "" {
		payloadParts = append(payloadParts, `"org_unit_id":`+strconv.Quote(orgUnitID))
	}
	if reportsToPositionUUID != "" {
		payloadParts = append(payloadParts, `"reports_to_position_uuid":`+strconv.Quote(reportsToPositionUUID))
	}
	if jobProfileUUID != "" {
		payloadParts = append(payloadParts, `"job_profile_uuid":`+strconv.Quote(jobProfileUUID))
	}
	if capacityFTE != "" {
		payloadParts = append(payloadParts, `"capacity_fte":`+strconv.Quote(capacityFTE))
	}
	if name != "" {
		payloadParts = append(payloadParts, `"name":`+strconv.Quote(name))
	}
	if lifecycleStatus != "" {
		payloadParts = append(payloadParts, `"lifecycle_status":`+strconv.Quote(lifecycleStatus))
	}
	if len(payloadParts) == 0 {
		return Position{}, newBadRequestError("at least one patch field is required")
	}
	payload := `{` + strings.Join(payloadParts, ",") + `}`

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return Position{}, err
	}

	if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_position_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  'UPDATE',
	  $4::date,
	  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
	`, eventID, tenantID, positionUUID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return Position{}, err
	}

	var out Position
	if err := tx.QueryRow(ctx, `
			SELECT
			  position_uuid::text,
			  org_unit_id::text,
			  COALESCE(reports_to_position_uuid::text, '') AS reports_to_position_uuid,
			  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
			  COALESCE(jobcatalog_setid_as_of::text, '') AS jobcatalog_setid_as_of,
			  COALESCE(job_profile_uuid::text, '') AS job_profile_uuid,
			  COALESCE(job_profile_code, '') AS job_profile_code,
			  COALESCE(name, '') AS name,
			  lifecycle_status,
			  capacity_fte::text AS capacity_fte,
			  effective_date::text AS effective_date
			FROM staffing.get_position_snapshot($1::uuid, $3::date)
			WHERE position_uuid = $2::uuid
			LIMIT 1
		`, tenantID, positionUUID, effectiveDate).Scan(
		&out.PositionUUID,
		&out.OrgUnitID,
		&out.ReportsToPositionUUID,
		&out.JobCatalogSetID,
		&out.JobCatalogSetIDAsOf,
		&out.JobProfileUUID,
		&out.JobProfileCode,
		&out.Name,
		&out.LifecycleStatus,
		&out.CapacityFTE,
		&out.EffectiveAt,
	); err != nil {
		return Position{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Position{}, err
	}
	return out, nil
}

func (s *staffingPGStore) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.ListAssignmentsForPerson(ctx, tenantID, asOfDate, personUUID)
}

func (s *staffingPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (Assignment, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.UpsertPrimaryAssignmentForPerson(ctx, tenantID, effectiveDate, personUUID, positionUUID, status, allocatedFte)
}

func (s *staffingPGStore) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.CorrectAssignmentEvent(ctx, tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
}

func (s *staffingPGStore) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	facade := staffingmodule.NewAssignmentsFacadeWithPGStore(s.pool)
	return facade.RescindAssignmentEvent(ctx, tenantID, assignmentUUID, targetEffectiveDate, payload)
}

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

func (s *staffingMemoryStore) CreatePositionCurrent(_ context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileUUID string, capacityFTE string, name string) (Position, error) {
	return s.positionDelegate().CreatePositionCurrent(context.Background(), tenantID, effectiveDate, orgUnitID, jobProfileUUID, capacityFTE, name)
}

func (s *staffingMemoryStore) UpdatePositionCurrent(_ context.Context, tenantID string, positionUUID string, effectiveDate string, orgUnitID string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
	return s.positionDelegate().UpdatePositionCurrent(context.Background(), tenantID, positionUUID, effectiveDate, orgUnitID, reportsToPositionUUID, jobProfileUUID, capacityFTE, name, lifecycleStatus)
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
