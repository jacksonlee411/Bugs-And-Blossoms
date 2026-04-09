package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/assignmentrules"
	staffingports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	staffingtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	staffingpersistence "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/infrastructure/persistence"
	staffingservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/services"
)

type Position struct {
	PositionUUID          string
	OrgUnitID             string
	ReportsToPositionUUID string
	JobCatalogSetID       string
	JobCatalogSetIDAsOf   string
	JobProfileUUID        string
	JobProfileCode        string
	Name                  string
	LifecycleStatus       string
	CapacityFTE           string
	EffectiveAt           string
}

type Assignment = staffingtypes.Assignment

type PositionStore interface {
	ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]Position, error)
	CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileUUID string, capacityFTE string, name string) (Position, error)
	UpdatePositionCurrent(ctx context.Context, tenantID string, positionUUID string, effectiveDate string, orgUnitID string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error)
}

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
	store := staffingpersistence.NewAssignmentPGStore(s.pool)
	facade := staffingservices.NewAssignmentsFacade(store)
	return facade.ListAssignmentsForPerson(ctx, tenantID, asOfDate, personUUID)
}

func (s *staffingPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (Assignment, error) {
	store := staffingpersistence.NewAssignmentPGStore(s.pool)
	facade := staffingservices.NewAssignmentsFacade(store)
	return facade.UpsertPrimaryAssignmentForPerson(ctx, tenantID, effectiveDate, personUUID, positionUUID, status, allocatedFte)
}

func (s *staffingPGStore) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	store := staffingpersistence.NewAssignmentPGStore(s.pool)
	facade := staffingservices.NewAssignmentsFacade(store)
	return facade.CorrectAssignmentEvent(ctx, tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
}

func (s *staffingPGStore) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	store := staffingpersistence.NewAssignmentPGStore(s.pool)
	facade := staffingservices.NewAssignmentsFacade(store)
	return facade.RescindAssignmentEvent(ctx, tenantID, assignmentUUID, targetEffectiveDate, payload)
}

type staffingMemoryStore struct {
	positions  map[string][]Position
	assigns    map[string]map[string][]Assignment
	positions0 []Position
}

func newStaffingMemoryStore() *staffingMemoryStore {
	return &staffingMemoryStore{
		positions: make(map[string][]Position),
		assigns:   make(map[string]map[string][]Assignment),
	}
}

func (s *staffingMemoryStore) ListPositionsCurrent(_ context.Context, tenantID string, _ string) ([]Position, error) {
	return append([]Position(nil), s.positions[tenantID]...), nil
}

func (s *staffingMemoryStore) CreatePositionCurrent(_ context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileUUID string, capacityFTE string, name string) (Position, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Position{}, newBadRequestError("effective_date is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	if orgUnitID == "" {
		return Position{}, newBadRequestError("org_unit_id is required")
	}
	jobProfileUUID = strings.TrimSpace(jobProfileUUID)
	if jobProfileUUID == "" {
		return Position{}, newBadRequestError("job_profile_uuid is required")
	}
	capacityFTE = strings.TrimSpace(capacityFTE)
	if capacityFTE == "" {
		capacityFTE = "1.0"
	}
	name = strings.TrimSpace(name)

	id := "pos-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	p := Position{
		PositionUUID:          id,
		OrgUnitID:             orgUnitID,
		ReportsToPositionUUID: "",
		JobCatalogSetID:       "",
		JobCatalogSetIDAsOf:   "",
		JobProfileUUID:        jobProfileUUID,
		JobProfileCode:        "",
		Name:                  name,
		LifecycleStatus:       "active",
		CapacityFTE:           capacityFTE,
		EffectiveAt:           effectiveDate,
	}
	s.positions[tenantID] = append(s.positions[tenantID], p)
	return p, nil
}

func (s *staffingMemoryStore) UpdatePositionCurrent(_ context.Context, tenantID string, positionUUID string, effectiveDate string, orgUnitID string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
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
	if orgUnitID == "" && reportsToPositionUUID == "" && jobProfileUUID == "" && capacityFTE == "" && name == "" && lifecycleStatus == "" {
		return Position{}, newBadRequestError("at least one patch field is required")
	}

	for i := range s.positions[tenantID] {
		if s.positions[tenantID][i].PositionUUID != positionUUID {
			continue
		}
		if orgUnitID != "" {
			s.positions[tenantID][i].OrgUnitID = orgUnitID
		}
		if reportsToPositionUUID != "" {
			s.positions[tenantID][i].ReportsToPositionUUID = reportsToPositionUUID
		}
		if jobProfileUUID != "" {
			s.positions[tenantID][i].JobProfileUUID = jobProfileUUID
		}
		if capacityFTE != "" {
			s.positions[tenantID][i].CapacityFTE = capacityFTE
		}
		if name != "" {
			s.positions[tenantID][i].Name = name
		}
		if lifecycleStatus != "" {
			s.positions[tenantID][i].LifecycleStatus = lifecycleStatus
		}
		s.positions[tenantID][i].EffectiveAt = effectiveDate
		return s.positions[tenantID][i], nil
	}
	return Position{}, errors.New("position not found")
}

func (s *staffingMemoryStore) ListAssignmentsForPerson(_ context.Context, tenantID string, _ string, personUUID string) ([]Assignment, error) {
	byPerson := s.assigns[tenantID]
	if byPerson == nil {
		return nil, nil
	}
	return append([]Assignment(nil), byPerson[personUUID]...), nil
}

func (s *staffingMemoryStore) UpsertPrimaryAssignmentForPerson(_ context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, _ string) (Assignment, error) {
	prepared, err := assignmentrules.PrepareUpsertPrimaryAssignment(effectiveDate, personUUID, positionUUID, status, "")
	if err != nil {
		return Assignment{}, err
	}
	effectiveDate = prepared.EffectiveDate
	personUUID = prepared.PersonUUID
	positionUUID = prepared.PositionUUID
	status = prepared.Status

	if s.assigns[tenantID] == nil {
		s.assigns[tenantID] = make(map[string][]Assignment)
	}

	a := Assignment{
		AssignmentUUID: "as-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		PersonUUID:     personUUID,
		PositionUUID:   positionUUID,
		Status:         status,
		EffectiveAt:    effectiveDate,
	}
	s.assigns[tenantID][personUUID] = append(s.assigns[tenantID][personUUID], a)
	return a, nil
}

func (s *staffingMemoryStore) CorrectAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
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
				a.PositionUUID = toString(v)
			}
			if v, ok := payload["status"]; ok {
				a.Status = toString(v)
			}
			return eventID, nil
		}
	}
	return "", errors.New("STAFFING_ASSIGNMENT_EVENT_NOT_FOUND")
}

func (s *staffingMemoryStore) RescindAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, _ json.RawMessage) (string, error) {
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
