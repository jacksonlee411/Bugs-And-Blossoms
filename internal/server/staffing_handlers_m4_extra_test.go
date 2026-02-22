package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestStaffingHandlers_M4_ExtraCoverage(t *testing.T) {
	t.Run("handlePositionsAPI as_of required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI as_of required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI get error bad request mapping", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return nil, newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI get error bad request mapping", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			listFn: func(context.Context, string, string, string) ([]Assignment, error) {
				return nil, newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignment-events:correct", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI missing assignment_uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"target_effective_date":"2026-01-01","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI missing target_effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_uuid":"a1","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI error conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI error bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI error invalid input", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI error unprocessable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignment-events:rescind", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader("{bad"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI missing assignment_uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"target_effective_date":"2026-01-01","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI missing target_effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_uuid":"a1","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI invalid target date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"bad","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{"note":"x"}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "e1", nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI error bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI error invalid input", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
