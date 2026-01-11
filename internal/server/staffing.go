package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var staffingAssignmentEventNamespace = uuid.Must(uuid.Parse("6d73e345-ae88-4e9d-a2ed-89f292e94f7b"))
var staffingAssignmentEventCorrectionNamespace = uuid.Must(uuid.Parse("28ed309c-cec7-406c-a442-eef4ef9034ce"))
var staffingAssignmentEventRescindNamespace = uuid.Must(uuid.Parse("fd58b41a-6ccc-451c-b9b4-cb924810fb2d"))

func deterministicStaffingAssignmentEventID(tenantID string, assignmentID string, effectiveDate string, assignmentType string) string {
	// Stable, payload-independent event_id for rerunnable upsert (DEV-PLAN-031 M3-A).
	name := fmt.Sprintf("staffing.assignment_event:%s:%s:%s:%s", tenantID, assignmentID, assignmentType, effectiveDate)
	return uuid.NewSHA1(staffingAssignmentEventNamespace, []byte(name)).String()
}

func canonicalizeJSON(b *strings.Builder, v any) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sortStrings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			ks, _ := json.Marshal(k)
			b.Write(ks)
			b.WriteByte(':')
			if err := canonicalizeJSON(b, t[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
		return nil
	case []any:
		b.WriteByte('[')
		for i := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := canonicalizeJSON(b, t[i]); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	case json.Number:
		b.WriteString(t.String())
		return nil
	case string, bool, nil:
		bb, _ := json.Marshal(t)
		b.Write(bb)
		return nil
	default:
		bb, err := json.Marshal(t)
		if err != nil {
			return err
		}
		b.Write(bb)
		return nil
	}
}

func sortStrings(ss []string) {
	for i := 0; i < len(ss); i++ {
		for j := i + 1; j < len(ss); j++ {
			if ss[j] < ss[i] {
				ss[i], ss[j] = ss[j], ss[i]
			}
		}
	}
}

func canonicalizeJSONObjectRaw(raw json.RawMessage) ([]byte, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, newBadRequestError("json object is required")
	}

	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, newBadRequestError("invalid json")
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, newBadRequestError("json object is required")
	}

	var b strings.Builder
	_ = canonicalizeJSON(&b, v)
	return []byte(b.String()), nil
}

func canonicalizeJSONObjectOrEmpty(raw json.RawMessage) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return []byte(`{}`), nil
	}
	return canonicalizeJSONObjectRaw(raw)
}

func deterministicStaffingAssignmentCorrectionEventID(tenantID string, assignmentID string, targetEffectiveDate string, canonicalReplacementPayload []byte) string {
	sum := sha256.Sum256(canonicalReplacementPayload)
	name := fmt.Sprintf("staffing.assignment_event_correction:%s:%s:%s:%x", tenantID, assignmentID, targetEffectiveDate, sum[:])
	return uuid.NewSHA1(staffingAssignmentEventCorrectionNamespace, []byte(name)).String()
}

func deterministicStaffingAssignmentRescindEventID(tenantID string, assignmentID string, targetEffectiveDate string) string {
	name := fmt.Sprintf("staffing.assignment_event_rescind:%s:%s:%s", tenantID, assignmentID, targetEffectiveDate)
	return uuid.NewSHA1(staffingAssignmentEventRescindNamespace, []byte(name)).String()
}

type Position struct {
	ID              string
	OrgUnitID       string
	ReportsToID     string
	BusinessUnitID  string
	JobCatalogSetID string
	JobProfileID    string
	JobProfileCode  string
	Name            string
	LifecycleStatus string
	CapacityFTE     string
	EffectiveAt     string
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
	CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, businessUnitID string, jobProfileID string, capacityFTE string, name string) (Position, error)
	UpdatePositionCurrent(ctx context.Context, tenantID string, positionID string, effectiveDate string, orgUnitID string, businessUnitID string, reportsToPositionID string, jobProfileID string, capacityFTE string, name string, lifecycleStatus string) (Position, error)
}

type AssignmentStore interface {
	ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error)
	UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, baseSalary string, allocatedFte string) (Assignment, error)
	CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error)
	RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error)
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
	  COALESCE(reports_to_position_id::text, '') AS reports_to_position_id,
	  COALESCE(business_unit_id, '') AS business_unit_id,
	  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
	  COALESCE(job_profile_id::text, '') AS job_profile_id,
	  COALESCE(job_profile_code, '') AS job_profile_code,
	  COALESCE(name, '') AS name,
	  lifecycle_status,
	  capacity_fte::text AS capacity_fte,
	  effective_date::text AS effective_date
	FROM staffing.get_position_snapshot($1::uuid, $2::date)
	ORDER BY effective_date DESC, position_id::text ASC
	`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(
			&p.ID,
			&p.OrgUnitID,
			&p.ReportsToID,
			&p.BusinessUnitID,
			&p.JobCatalogSetID,
			&p.JobProfileID,
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

func (s *staffingPGStore) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, businessUnitID string, jobProfileID string, capacityFTE string, name string) (Position, error) {
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
	businessUnitID = strings.TrimSpace(businessUnitID)
	jobProfileID = strings.TrimSpace(jobProfileID)
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

	payload := `{"org_unit_id":` + strconv.Quote(orgUnitID)
	if businessUnitID != "" {
		payload += `,"business_unit_id":` + strconv.Quote(businessUnitID)
	}
	if jobProfileID != "" {
		payload += `,"job_profile_id":` + strconv.Quote(jobProfileID)
	}
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

	if err := tx.Commit(ctx); err != nil {
		return Position{}, err
	}

	if capacityFTE == "" {
		capacityFTE = "1.0"
	}
	return Position{
		ID:              positionID,
		OrgUnitID:       orgUnitID,
		ReportsToID:     "",
		BusinessUnitID:  businessUnitID,
		JobCatalogSetID: "",
		JobProfileID:    jobProfileID,
		JobProfileCode:  "",
		Name:            name,
		LifecycleStatus: "active",
		CapacityFTE:     capacityFTE,
		EffectiveAt:     effectiveDate,
	}, nil
}

func (s *staffingPGStore) UpdatePositionCurrent(ctx context.Context, tenantID string, positionID string, effectiveDate string, orgUnitID string, businessUnitID string, reportsToPositionID string, jobProfileID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
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
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return Position{}, newBadRequestError("position_id is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	businessUnitID = strings.TrimSpace(businessUnitID)
	reportsToPositionID = strings.TrimSpace(reportsToPositionID)
	jobProfileID = strings.TrimSpace(jobProfileID)
	capacityFTE = strings.TrimSpace(capacityFTE)
	name = strings.TrimSpace(name)
	lifecycleStatus = strings.TrimSpace(lifecycleStatus)

	payloadParts := make([]string, 0, 6)
	if orgUnitID != "" {
		payloadParts = append(payloadParts, `"org_unit_id":`+strconv.Quote(orgUnitID))
	}
	if businessUnitID != "" {
		payloadParts = append(payloadParts, `"business_unit_id":`+strconv.Quote(businessUnitID))
	}
	if reportsToPositionID != "" {
		payloadParts = append(payloadParts, `"reports_to_position_id":`+strconv.Quote(reportsToPositionID))
	}
	if jobProfileID != "" {
		if jobProfileID == "__CLEAR__" {
			payloadParts = append(payloadParts, `"job_profile_id":null`)
		} else {
			payloadParts = append(payloadParts, `"job_profile_id":`+strconv.Quote(jobProfileID))
		}
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
	`, eventID, tenantID, positionID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return Position{}, err
	}

	var out Position
	if err := tx.QueryRow(ctx, `
		SELECT
		  position_id::text,
		  org_unit_id::text,
		  COALESCE(reports_to_position_id::text, '') AS reports_to_position_id,
		  COALESCE(business_unit_id, '') AS business_unit_id,
		  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
		  COALESCE(job_profile_id::text, '') AS job_profile_id,
		  COALESCE(job_profile_code, '') AS job_profile_code,
		  COALESCE(name, '') AS name,
		  lifecycle_status,
		  capacity_fte::text AS capacity_fte,
		  effective_date::text AS effective_date
		FROM staffing.get_position_snapshot($1::uuid, $3::date)
		WHERE position_id = $2::uuid
		LIMIT 1
	`, tenantID, positionID, effectiveDate).Scan(
		&out.ID,
		&out.OrgUnitID,
		&out.ReportsToID,
		&out.BusinessUnitID,
		&out.JobCatalogSetID,
		&out.JobProfileID,
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
	  effective_date::text AS effective_date
	FROM staffing.get_assignment_snapshot($1::uuid, $2::uuid, $3::date)
	ORDER BY effective_date DESC, assignment_id::text ASC
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

func (s *staffingPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, baseSalary string, allocatedFte string) (Assignment, error) {
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
	status = strings.TrimSpace(status)
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

	eventID := deterministicStaffingAssignmentEventID(tenantID, assignmentID, effectiveDate, assignmentType)
	requestID := eventID
	initiatorID := tenantID

	// Rerunnable upsert (DEV-PLAN-031 M3-A):
	// - If the effective_date already exists, reuse existing (event_id, request_id, initiator_id, event_type)
	//   so the Kernel hits the idempotency path instead of violating (tenant_id, assignment_id, effective_date) unique.
	{
		var existingEventType string
		var existingRequestID string
		var existingInitiatorID string
		err := tx.QueryRow(ctx, `
	SELECT
	  event_id::text,
	  event_type,
	  request_id,
	  initiator_id::text
	FROM staffing.assignment_events
	WHERE tenant_id = $1::uuid
	  AND assignment_id = $2::uuid
	  AND effective_date = $3::date
	LIMIT 1
	`, tenantID, assignmentID, effectiveDate).Scan(&eventID, &existingEventType, &existingRequestID, &existingInitiatorID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Assignment{}, err
		}
		if err == nil {
			eventType = existingEventType
			requestID = existingRequestID
			initiatorID = existingInitiatorID
		}
	}

	payload := `{"position_id":` + strconv.Quote(positionID)
	if baseSalary != "" {
		payload += `,"base_salary":` + strconv.Quote(baseSalary)
	}
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
		return Assignment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, err
	}

	if status == "" {
		status = "active"
	}
	return Assignment{
		AssignmentID: assignmentID,
		PersonUUID:   personUUID,
		PositionID:   positionID,
		Status:       status,
		EffectiveAt:  effectiveDate,
	}, nil
}

func (s *staffingPGStore) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	assignmentID = strings.TrimSpace(assignmentID)
	if assignmentID == "" {
		return "", newBadRequestError("assignment_id is required")
	}
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if targetEffectiveDate == "" {
		return "", newBadRequestError("target_effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
		return "", newBadRequestError("invalid target_effective_date")
	}

	canonicalPayload, err := canonicalizeJSONObjectRaw(replacementPayload)
	if err != nil {
		return "", err
	}

	eventID := deterministicStaffingAssignmentCorrectionEventID(tenantID, assignmentID, targetEffectiveDate, canonicalPayload)
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

func (s *staffingPGStore) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	assignmentID = strings.TrimSpace(assignmentID)
	if assignmentID == "" {
		return "", newBadRequestError("assignment_id is required")
	}
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if targetEffectiveDate == "" {
		return "", newBadRequestError("target_effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
		return "", newBadRequestError("invalid target_effective_date")
	}

	canonicalPayload, err := canonicalizeJSONObjectOrEmpty(payload)
	if err != nil {
		return "", err
	}

	eventID := deterministicStaffingAssignmentRescindEventID(tenantID, assignmentID, targetEffectiveDate)
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

func (s *staffingMemoryStore) CreatePositionCurrent(_ context.Context, tenantID string, effectiveDate string, orgUnitID string, businessUnitID string, jobProfileID string, capacityFTE string, name string) (Position, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Position{}, newBadRequestError("effective_date is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	if orgUnitID == "" {
		return Position{}, newBadRequestError("org_unit_id is required")
	}
	businessUnitID = strings.TrimSpace(businessUnitID)
	jobProfileID = strings.TrimSpace(jobProfileID)
	capacityFTE = strings.TrimSpace(capacityFTE)
	if capacityFTE == "" {
		capacityFTE = "1.0"
	}
	name = strings.TrimSpace(name)

	id := "pos-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	p := Position{
		ID:              id,
		OrgUnitID:       orgUnitID,
		ReportsToID:     "",
		BusinessUnitID:  businessUnitID,
		JobCatalogSetID: "",
		JobProfileID:    jobProfileID,
		JobProfileCode:  "",
		Name:            name,
		LifecycleStatus: "active",
		CapacityFTE:     capacityFTE,
		EffectiveAt:     effectiveDate,
	}
	s.positions[tenantID] = append(s.positions[tenantID], p)
	return p, nil
}

func (s *staffingMemoryStore) UpdatePositionCurrent(_ context.Context, tenantID string, positionID string, effectiveDate string, orgUnitID string, businessUnitID string, reportsToPositionID string, jobProfileID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return Position{}, newBadRequestError("effective_date is required")
	}
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return Position{}, newBadRequestError("position_id is required")
	}
	orgUnitID = strings.TrimSpace(orgUnitID)
	businessUnitID = strings.TrimSpace(businessUnitID)
	reportsToPositionID = strings.TrimSpace(reportsToPositionID)
	jobProfileID = strings.TrimSpace(jobProfileID)
	capacityFTE = strings.TrimSpace(capacityFTE)
	name = strings.TrimSpace(name)
	lifecycleStatus = strings.TrimSpace(lifecycleStatus)
	if orgUnitID == "" && businessUnitID == "" && reportsToPositionID == "" && jobProfileID == "" && capacityFTE == "" && name == "" && lifecycleStatus == "" {
		return Position{}, newBadRequestError("at least one patch field is required")
	}

	for i := range s.positions[tenantID] {
		if s.positions[tenantID][i].ID != positionID {
			continue
		}
		if orgUnitID != "" {
			s.positions[tenantID][i].OrgUnitID = orgUnitID
		}
		if businessUnitID != "" {
			s.positions[tenantID][i].BusinessUnitID = businessUnitID
		}
		if reportsToPositionID != "" {
			s.positions[tenantID][i].ReportsToID = reportsToPositionID
		}
		if jobProfileID != "" {
			if jobProfileID == "__CLEAR__" {
				s.positions[tenantID][i].JobProfileID = ""
			} else {
				s.positions[tenantID][i].JobProfileID = jobProfileID
			}
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

func (s *staffingMemoryStore) UpsertPrimaryAssignmentForPerson(_ context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, _ string, _ string) (Assignment, error) {
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
	status = strings.TrimSpace(status)
	if status == "" {
		status = "active"
	}

	if s.assigns[tenantID] == nil {
		s.assigns[tenantID] = make(map[string][]Assignment)
	}

	a := Assignment{
		AssignmentID: "as-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		PersonUUID:   personUUID,
		PositionID:   positionID,
		Status:       status,
		EffectiveAt:  effectiveDate,
	}
	s.assigns[tenantID][personUUID] = append(s.assigns[tenantID][personUUID], a)
	return a, nil
}

func (s *staffingMemoryStore) CorrectAssignmentEvent(_ context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	assignmentID = strings.TrimSpace(assignmentID)
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if assignmentID == "" || targetEffectiveDate == "" {
		return "", newBadRequestError("assignment_id and target_effective_date are required")
	}

	canonicalPayload, err := canonicalizeJSONObjectRaw(replacementPayload)
	if err != nil {
		return "", err
	}

	eventID := deterministicStaffingAssignmentCorrectionEventID(tenantID, assignmentID, targetEffectiveDate, canonicalPayload)

	var payload map[string]any
	if err := json.Unmarshal(canonicalPayload, &payload); err != nil {
		return "", err
	}

	byPerson := s.assigns[tenantID]
	for personUUID := range byPerson {
		for i := range byPerson[personUUID] {
			a := &byPerson[personUUID][i]
			if a.AssignmentID != assignmentID || a.EffectiveAt != targetEffectiveDate {
				continue
			}
			if v, ok := payload["position_id"]; ok {
				a.PositionID = toString(v)
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
	assignmentID = strings.TrimSpace(assignmentID)
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if assignmentID == "" || targetEffectiveDate == "" {
		return "", newBadRequestError("assignment_id and target_effective_date are required")
	}

	eventID := deterministicStaffingAssignmentRescindEventID(tenantID, assignmentID, targetEffectiveDate)

	byPerson := s.assigns[tenantID]
	for personUUID := range byPerson {
		events := byPerson[personUUID]
		earliest := ""
		for _, a := range events {
			if a.AssignmentID != assignmentID {
				continue
			}
			if earliest == "" || a.EffectiveAt < earliest {
				earliest = a.EffectiveAt
			}
		}
		for i := range events {
			if events[i].AssignmentID != assignmentID || events[i].EffectiveAt != targetEffectiveDate {
				continue
			}
			if targetEffectiveDate == earliest {
				hasLater := false
				for _, a := range events {
					if a.AssignmentID == assignmentID && a.EffectiveAt > targetEffectiveDate {
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
