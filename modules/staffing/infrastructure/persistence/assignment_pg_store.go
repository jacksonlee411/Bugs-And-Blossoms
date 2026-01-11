package persistence

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
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
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
var assignmentEventCorrectionNamespace = uuid.Must(uuid.Parse("28ed309c-cec7-406c-a442-eef4ef9034ce"))
var assignmentEventRescindNamespace = uuid.Must(uuid.Parse("fd58b41a-6ccc-451c-b9b4-cb924810fb2d"))

func deterministicAssignmentEventID(tenantID string, assignmentID string, effectiveDate string, assignmentType string) string {
	// Stable, payload-independent event_id for rerunnable upsert (DEV-PLAN-031 M3-A).
	name := fmt.Sprintf("staffing.assignment_event:%s:%s:%s:%s", tenantID, assignmentID, assignmentType, effectiveDate)
	return uuid.NewSHA1(assignmentEventNamespace, []byte(name)).String()
}

func deterministicAssignmentCorrectionEventID(tenantID string, assignmentID string, targetEffectiveDate string, canonicalReplacementPayload []byte) string {
	sum := sha256.Sum256(canonicalReplacementPayload)
	name := fmt.Sprintf("staffing.assignment_event_correction:%s:%s:%s:%x", tenantID, assignmentID, targetEffectiveDate, sum[:])
	return uuid.NewSHA1(assignmentEventCorrectionNamespace, []byte(name)).String()
}

func deterministicAssignmentRescindEventID(tenantID string, assignmentID string, targetEffectiveDate string) string {
	name := fmt.Sprintf("staffing.assignment_event_rescind:%s:%s:%s", tenantID, assignmentID, targetEffectiveDate)
	return uuid.NewSHA1(assignmentEventRescindNamespace, []byte(name)).String()
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
		return nil, httperr.NewBadRequest("json object is required")
	}

	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, httperr.NewBadRequest("invalid json")
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, httperr.NewBadRequest("json object is required")
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

	var out []types.Assignment
	for rows.Next() {
		var a types.Assignment
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

func (s *AssignmentPGStore) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, baseSalary string, allocatedFte string) (types.Assignment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.Assignment{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.Assignment{}, err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return types.Assignment{}, errors.New("effective_date is required")
	}
	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return types.Assignment{}, errors.New("person_uuid is required")
	}
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return types.Assignment{}, errors.New("position_id is required")
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
	WHERE tenant_id = $1::uuid AND assignment_id = $2::uuid
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
			return types.Assignment{}, err
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
		return types.Assignment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return types.Assignment{}, err
	}

	if status == "" {
		status = "active"
	}
	return types.Assignment{
		AssignmentID: assignmentID,
		PersonUUID:   personUUID,
		PositionID:   positionID,
		Status:       status,
		EffectiveAt:  effectiveDate,
	}, nil
}

func (s *AssignmentPGStore) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	assignmentID = strings.TrimSpace(assignmentID)
	if assignmentID == "" {
		return "", httperr.NewBadRequest("assignment_id is required")
	}
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if targetEffectiveDate == "" {
		return "", httperr.NewBadRequest("target_effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
		return "", httperr.NewBadRequest("invalid target_effective_date")
	}

	canonicalPayload, err := canonicalizeJSONObjectRaw(replacementPayload)
	if err != nil {
		return "", err
	}

	eventID := deterministicAssignmentCorrectionEventID(tenantID, assignmentID, targetEffectiveDate, canonicalPayload)
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
	assignmentID = strings.TrimSpace(assignmentID)
	if assignmentID == "" {
		return "", httperr.NewBadRequest("assignment_id is required")
	}
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if targetEffectiveDate == "" {
		return "", httperr.NewBadRequest("target_effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
		return "", httperr.NewBadRequest("invalid target_effective_date")
	}

	canonicalPayload, err := canonicalizeJSONObjectOrEmpty(payload)
	if err != nil {
		return "", err
	}

	eventID := deterministicAssignmentRescindEventID(tenantID, assignmentID, targetEffectiveDate)
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
