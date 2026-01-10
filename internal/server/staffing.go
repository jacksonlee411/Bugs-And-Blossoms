package server

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Position struct {
	ID          string
	OrgUnitID   string
	Name        string
	EffectiveAt string
}

type Assignment struct {
	AssignmentID string
	PersonUUID   string
	PositionID   string
	Status       string
	EffectiveAt  string
}

type PositionStore interface {
	ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]Position, error)
	CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, name string) (Position, error)
}

type AssignmentStore interface {
	ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error)
	UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, baseSalary string, allocatedFte string) (Assignment, error)
}

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
  position_id::text,
  org_unit_id::text,
  COALESCE(name, '') AS name,
  lower(validity)::text AS effective_date
FROM staffing.position_versions
WHERE tenant_id = $1::uuid
  AND lifecycle_status = 'active'
  AND validity @> $2::date
ORDER BY lower(validity) DESC, position_id::text ASC
`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.OrgUnitID, &p.Name, &p.EffectiveAt); err != nil {
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

func (s *staffingPGStore) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, name string) (Position, error) {
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
		return Position{}, errors.New("effective_date is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	if orgUnitID == "" {
		return Position{}, errors.New("org_unit_id is required")
	}
	name = strings.TrimSpace(name)

	var positionID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionID); err != nil {
		return Position{}, err
	}
	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return Position{}, err
	}

	payload := `{"org_unit_id":` + strconv.Quote(orgUnitID)
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

	if err := tx.Commit(ctx); err != nil {
		return Position{}, err
	}

	return Position{
		ID:          positionID,
		OrgUnitID:   orgUnitID,
		Name:        name,
		EffectiveAt: effectiveDate,
	}, nil
}

func (s *staffingPGStore) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error) {
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
  assignment_id::text,
  person_uuid::text,
  position_id::text,
  status,
  lower(validity)::text AS effective_date
FROM staffing.assignment_versions
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND validity @> $3::date
ORDER BY lower(validity) DESC, assignment_id::text ASC
`, tenantID, personUUID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.AssignmentID, &a.PersonUUID, &a.PositionID, &a.Status, &a.EffectiveAt); err != nil {
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

func (s *staffingPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, baseSalary string, allocatedFte string) (Assignment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return Assignment{}, err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Assignment{}, errors.New("effective_date is required")
	}
	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return Assignment{}, errors.New("person_uuid is required")
	}
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return Assignment{}, errors.New("position_id is required")
	}
	baseSalary = strings.TrimSpace(baseSalary)
	allocatedFte = strings.TrimSpace(allocatedFte)

	assignmentType := "primary"

	var assignmentID string
	err = tx.QueryRow(ctx, `
SELECT id::text
FROM staffing.assignments
WHERE tenant_id = $1::uuid AND person_uuid = $2::uuid AND assignment_type = $3::text
`, tenantID, personUUID, assignmentType).Scan(&assignmentID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return Assignment{}, err
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&assignmentID); err != nil {
			return Assignment{}, err
		}
	}

	var eventsCount int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM staffing.assignment_events
WHERE tenant_id = $1::uuid AND assignment_id = $2::uuid
`, tenantID, assignmentID).Scan(&eventsCount); err != nil {
		return Assignment{}, err
	}

	eventType := "UPDATE"
	if eventsCount == 0 {
		eventType = "CREATE"
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return Assignment{}, err
	}

	payload := `{"position_id":` + strconv.Quote(positionID)
	if baseSalary != "" {
		payload += `,"base_salary":` + strconv.Quote(baseSalary)
	}
	if allocatedFte != "" {
		payload += `,"allocated_fte":` + strconv.Quote(allocatedFte)
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
`, eventID, tenantID, assignmentID, personUUID, assignmentType, eventType, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return Assignment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, err
	}

	return Assignment{
		AssignmentID: assignmentID,
		PersonUUID:   personUUID,
		PositionID:   positionID,
		Status:       "active",
		EffectiveAt:  effectiveDate,
	}, nil
}

type staffingMemoryStore struct {
	positions  map[string][]Position
	assigns    map[string]map[string][]Assignment
	punches    map[string]map[string][]TimePunch
	positions0 []Position
}

func newStaffingMemoryStore() *staffingMemoryStore {
	return &staffingMemoryStore{
		positions: make(map[string][]Position),
		assigns:   make(map[string]map[string][]Assignment),
		punches:   make(map[string]map[string][]TimePunch),
	}
}

func (s *staffingMemoryStore) ListPositionsCurrent(_ context.Context, tenantID string, _ string) ([]Position, error) {
	return append([]Position(nil), s.positions[tenantID]...), nil
}

func (s *staffingMemoryStore) CreatePositionCurrent(_ context.Context, tenantID string, effectiveDate string, orgUnitID string, name string) (Position, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Position{}, errors.New("effective_date is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	if orgUnitID == "" {
		return Position{}, errors.New("org_unit_id is required")
	}
	name = strings.TrimSpace(name)

	id := "pos-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	p := Position{ID: id, OrgUnitID: orgUnitID, Name: name, EffectiveAt: effectiveDate}
	s.positions[tenantID] = append(s.positions[tenantID], p)
	return p, nil
}

func (s *staffingMemoryStore) ListAssignmentsForPerson(_ context.Context, tenantID string, _ string, personUUID string) ([]Assignment, error) {
	byPerson := s.assigns[tenantID]
	if byPerson == nil {
		return nil, nil
	}
	return append([]Assignment(nil), byPerson[personUUID]...), nil
}

func (s *staffingMemoryStore) UpsertPrimaryAssignmentForPerson(_ context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, _ string, _ string) (Assignment, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Assignment{}, errors.New("effective_date is required")
	}
	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return Assignment{}, errors.New("person_uuid is required")
	}
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return Assignment{}, errors.New("position_id is required")
	}

	if s.assigns[tenantID] == nil {
		s.assigns[tenantID] = make(map[string][]Assignment)
	}

	a := Assignment{
		AssignmentID: "as-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		PersonUUID:   personUUID,
		PositionID:   positionID,
		Status:       "active",
		EffectiveAt:  effectiveDate,
	}
	s.assigns[tenantID][personUUID] = append(s.assigns[tenantID][personUUID], a)
	return a, nil
}
