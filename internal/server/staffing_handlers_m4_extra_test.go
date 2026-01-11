package server

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
)

func TestStaffingHandlers_M4_ExtraCoverage(t *testing.T) {
	t.Run("handlePositionsAPI as_of defaults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			listFn: func(_ context.Context, _ string, asOfDate string) ([]Position, error) {
				if _, err := time.Parse("2006-01-02", asOfDate); err != nil {
					t.Fatal(err)
				}
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI as_of defaults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			listFn: func(_ context.Context, _ string, asOfDate string, _ string) ([]Assignment, error) {
				if _, err := time.Parse("2006-01-02", asOfDate); err != nil {
					t.Fatal(err)
				}
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI get error bad request mapping", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
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

	t.Run("handleAssignmentEventsCorrectAPI missing assignment_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"target_effective_date":"2026-01-01","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI missing target_effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI error conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
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

	t.Run("handleAssignmentEventsRescindAPI missing assignment_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"target_effective_date":"2026-01-01","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI missing target_effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI invalid target date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"bad","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","payload":{"note":"x"}}`))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","payload":{}}`))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","payload":{}}`))
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

	t.Run("handleAssignments correct_event branches", func(t *testing.T) {
		positionStore := positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) {
			return []Position{{ID: "pos1", LifecycleStatus: "active"}}, nil
		}}

		t.Run("missing assignment_id/target_effective_date", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=correct_event&effective_date=2026-01-01&person_uuid=p1&position_id=pos1"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "assignment_id/target_effective_date is required") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("invalid target_effective_date", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=correct_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=bad&position_id=pos1"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "target_effective_date 无效") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("missing position_id", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=correct_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=2026-01-01"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "position_id is required") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("store error", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=correct_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=2026-01-01&position_id=pos1"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil },
				correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
					return "", &pgconn.PgError{Message: "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND"}
				},
			}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("ok (covers optional fields)", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=correct_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=2026-01-01&position_id=pos1&base_salary=100.00&allocated_fte=0.5&status=inactive"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil },
				correctFn: func(_ context.Context, _ string, _ string, _ string, raw json.RawMessage) (string, error) {
					var m map[string]any
					if err := json.Unmarshal(raw, &m); err != nil {
						return "", err
					}
					if m["base_salary"] != "100.00" || m["allocated_fte"] != "0.5" || m["status"] != "inactive" {
						return "", errors.New("unexpected payload")
					}
					return "e1", nil
				},
			}, newPersonMemoryStore())
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	})

	t.Run("handleAssignments rescind_event branches", func(t *testing.T) {
		positionStore := positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) {
			return []Position{{ID: "pos1", LifecycleStatus: "active"}}, nil
		}}

		t.Run("missing assignment_id/target_effective_date", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=rescind_event&effective_date=2026-01-01&person_uuid=p1"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "assignment_id/target_effective_date is required") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("invalid target_effective_date", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=rescind_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=bad"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "target_effective_date 无效") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("store error", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=rescind_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=2026-01-01"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil },
				rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
					return "", &pgconn.PgError{Message: "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND"}
				},
			}, newPersonMemoryStore())
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})

		t.Run("ok (covers note payload)", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=rescind_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=2026-01-01&note=hello"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleAssignments(rec, req, positionStore, assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil },
				rescindFn: func(_ context.Context, _ string, _ string, _ string, raw json.RawMessage) (string, error) {
					var m map[string]any
					if err := json.Unmarshal(raw, &m); err != nil {
						return "", err
					}
					if m["note"] != "hello" {
						return "", errors.New("expected note payload")
					}
					return "e1", nil
				},
			}, newPersonMemoryStore())
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	})

	t.Run("handleAssignments unknown action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("action=unknown&effective_date=2026-01-01&person_uuid=p1&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{ID: "pos1", LifecycleStatus: "active"}}, nil
			}},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "unknown action") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("handleAssignments correct_event uses assignmentStore.CorrectAssignmentEvent request format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", bytes.NewBufferString("action=correct_event&effective_date=2026-01-01&person_uuid=p1&assignment_id=as1&target_effective_date=2026-01-01&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{ID: "pos1", LifecycleStatus: "active"}}, nil
			}},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
					return "e1", nil
				},
			},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
