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
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
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

func controllerWithStore(store assignmentsStoreStub) AssignmentsController {
	return AssignmentsController{
		TenantID: func(context.Context) (string, bool) { return "t1", true },
		NowUTC: func() time.Time {
			return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		},
		Facade: services.NewAssignmentsFacade(store),
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_TenantMissing(t *testing.T) {
	c := newAssignmentsController()
	c.TenantID = func(context.Context) (string, bool) { return "", false }
	req := httptest.NewRequest(http.MethodGet, "/api/assignments?person_uuid=p1", nil)
	rec := httptest.NewRecorder()
	c.HandleAssignmentsAPI(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_InvalidAsOf(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodGet, "/api/assignments?as_of=bad&person_uuid=p1", nil)
	rec := httptest.NewRecorder()
	c.HandleAssignmentsAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_AsOfRequired(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodGet, "/api/assignments?person_uuid=p1", nil)
	rec := httptest.NewRecorder()
	c.HandleAssignmentsAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_MethodNotAllowed(t *testing.T) {
	c := newAssignmentsController()
	req := httptest.NewRequest(http.MethodPut, "/api/assignments?as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()
	c.HandleAssignmentsAPI(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAssignmentsController_HandleAssignmentsAPI_GetBranches(t *testing.T) {
	t.Run("missing person_uuid", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodGet, "/api/assignments?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list error stable code => 422", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			listFn: func(context.Context, string, string, string) ([]types.Assignment, error) {
				return nil, &pgconn.PgError{Message: "STAFFING_LIST_FAILED"}
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list error bad request => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			listFn: func(context.Context, string, string, string) ([]types.Assignment, error) {
				return nil, httperr.NewBadRequest("bad")
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list error invalid input => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			listFn: func(context.Context, string, string, string) ([]types.Assignment, error) {
				return nil, &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list ok assigns=nil => []", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			listFn: func(context.Context, string, string, string) ([]types.Assignment, error) {
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
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

func TestAssignmentsController_HandleAssignmentsAPI_PostBranches(t *testing.T) {
	t.Run("bad json raw map", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader("{bad"))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("deprecated position_id rejected", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"position_id":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid effective_date", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"bad","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("effective_date required", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1","status":"weird"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("upsert conflict", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				return types.Assignment{}, &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("upsert invalid input => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				return types.Assignment{}, &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("upsert bad request => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				return types.Assignment{}, httperr.NewBadRequest("bad")
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("upsert unprocessable stable code", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				return types.Assignment{}, &pgconn.PgError{Message: "STAFFING_UPSERT_FAILED"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("upsert ok", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				return types.Assignment{AssignmentUUID: "a1"}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1","status":"active","allocated_fte":"1.0"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("upsert ok with inactive status", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				return types.Assignment{AssignmentUUID: "a1", Status: "inactive"}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1","status":"inactive"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST without as_of returns bad request", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments", strings.NewReader(`{"effective_date":"2026-01-01","person_uuid":"p1","position_uuid":"pos1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentsAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
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

func TestAssignmentsController_HandleAssignmentEventsCorrectAPI_Branches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		c := newAssignmentsController()
		c.TenantID = func(context.Context) (string, bool) { return "", false }
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodGet, "/api/assignments/events/correct", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("deprecated assignment_id rejected", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_id":"1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing assignment_uuid", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"target_effective_date":"2026-01-01"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing target_effective_date", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid target_effective_date", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"bad"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("replacement_payload valid with new key", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "e1", nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{"position_uuid":"pos1"}}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json raw map", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader("{bad"))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("replacement_payload invalid json is ignored", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "e1", nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":"bad"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("correct conflict", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("correct unprocessable stable code", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_CORRECT_FAILED"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("correct invalid input => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("correct bad request => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", httperr.NewBadRequest("bad")
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsCorrectAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
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

func TestAssignmentsController_HandleAssignmentEventsRescindAPI_Branches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		c := newAssignmentsController()
		c.TenantID = func(context.Context) (string, bool) { return "", false }
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodGet, "/api/assignments/events/rescind", nil)
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("deprecated assignment_id rejected", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_id":"1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing target_effective_date", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing assignment_uuid", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"target_effective_date":"2026-01-01"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json raw map", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader("{bad"))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid target date", func(t *testing.T) {
		c := newAssignmentsController()
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"bad"}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("rescind conflict", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("rescind invalid input => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("rescind bad request => 400", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", httperr.NewBadRequest("bad")
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("rescind ok", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "e1", nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{"note":"x"}}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("rescind unprocessable stable code", func(t *testing.T) {
		c := controllerWithStore(assignmentsStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_RESCIND_FAILED"}
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/assignments/events/rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		rec := httptest.NewRecorder()
		c.HandleAssignmentEventsRescindAPI(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})
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

func TestPgErrorCode_FallbackEmpty(t *testing.T) {
	if got := pgErrorCode(errors.New("boom")); got != "" {
		t.Fatalf("got=%q", got)
	}
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

func TestTraceIDFromRequest(t *testing.T) {
	cases := []struct {
		name        string
		traceparent string
		want        string
	}{
		{name: "empty", traceparent: "", want: ""},
		{name: "bad format", traceparent: "00-abc", want: ""},
		{name: "bad chars", traceparent: "00-0123456789abcdef0123456789abcdeg-0123456789abcdef-01", want: ""},
		{name: "zero trace", traceparent: "00-00000000000000000000000000000000-0123456789abcdef-01", want: ""},
		{name: "ok", traceparent: "00-ABCDEFABCDEFABCDEFABCDEFABCDEFAB-0123456789abcdef-01", want: "abcdefabcdefabcdefabcdefabcdefab"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}
			if got := traceIDFromRequest(req); got != tc.want {
				t.Fatalf("traceIDFromRequest()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestWriteError_TraceID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	rec := httptest.NewRecorder()

	writeError(rec, req, http.StatusBadRequest, "bad", "bad")

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, _ := body["trace_id"].(string); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("trace_id=%q", got)
	}
}
