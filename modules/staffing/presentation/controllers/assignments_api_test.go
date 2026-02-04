package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/services"
)

type assignmentsStoreStub struct {
	listFn    func(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error)
	upsertFn  func(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error)
	correctFn func(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error)
	rescindFn func(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error)
}

func (s assignmentsStoreStub) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, asOfDate, personUUID)
	}
	return nil, errors.New("not implemented")
}

func (s assignmentsStoreStub) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, tenantID, effectiveDate, personUUID, positionUUID, status, allocatedFte)
	}
	return types.Assignment{}, errors.New("not implemented")
}

func (s assignmentsStoreStub) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	if s.correctFn != nil {
		return s.correctFn(ctx, tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
	}
	return "", errors.New("not implemented")
}

func (s assignmentsStoreStub) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	if s.rescindFn != nil {
		return s.rescindFn(ctx, tenantID, assignmentUUID, targetEffectiveDate, payload)
	}
	return "", errors.New("not implemented")
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("read") }
func (errReadCloser) Close() error             { return nil }

func newAssignmentsController() AssignmentsController {
	return AssignmentsController{
		TenantID: func(context.Context) (string, bool) { return "t1", true },
		NowUTC: func() time.Time {
			return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		},
		Facade: services.NewAssignmentsFacade(assignmentsStoreStub{}),
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_ReadError(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", nil)
	req.Body = errReadCloser{}
	rec := httptest.NewRecorder()
	c.HandleAssignmentsAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_DecodeError(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":123}`))
	rec := httptest.NewRecorder()
	c.HandleAssignmentsAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentEventsCorrectAPI_ReadError(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", nil)
	req.Body = errReadCloser{}
	rec := httptest.NewRecorder()
	c.HandleAssignmentEventsCorrectAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentEventsCorrectAPI_DecodeError(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":123}`))
	rec := httptest.NewRecorder()
	c.HandleAssignmentEventsCorrectAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentEventsCorrectAPI_LegacyPayload(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", bytes.NewReader([]byte(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{"position_id":"pos1"}}`)))
	rec := httptest.NewRecorder()
	c.HandleAssignmentEventsCorrectAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentEventsRescindAPI_ReadError(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", nil)
	req.Body = errReadCloser{}
	rec := httptest.NewRecorder()
	c.HandleAssignmentEventsRescindAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentEventsRescindAPI_DecodeError(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":123}`))
	rec := httptest.NewRecorder()
	c.HandleAssignmentEventsRescindAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStablePgMessage(t *testing.T) {
	t.Run("stable message passthrough", func(t *testing.T) {
		err := &pgconn.PgError{Message: "STAFFING_OK"}
		if got := stablePgMessage(err); got != "STAFFING_OK" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("constraint position overlap", func(t *testing.T) {
		err := &pgconn.PgError{ConstraintName: "assignment_versions_position_no_overlap"}
		if got := stablePgMessage(err); got != "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("constraint one per day", func(t *testing.T) {
		err := &pgconn.PgError{ConstraintName: "assignment_events_one_per_day_unique"}
		if got := stablePgMessage(err); got != "STAFFING_ASSIGNMENT_ONE_PER_DAY" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("fallback error", func(t *testing.T) {
		err := errors.New("boom")
		if got := stablePgMessage(err); got != "boom" {
			t.Fatalf("got=%q", got)
		}
	})
}

func TestIsStableDBCode(t *testing.T) {
	cases := []struct {
		code  string
		valid bool
	}{
		{"", false},
		{"UNKNOWN", false},
		{"1BAD", false},
		{"bad_code", false},
		{"BAD-CODE", false},
		{"STAFFING_OK_1", true},
	}
	for _, c := range cases {
		if got := isStableDBCode(c.code); got != c.valid {
			t.Fatalf("code=%q got=%v", c.code, got)
		}
	}
}
