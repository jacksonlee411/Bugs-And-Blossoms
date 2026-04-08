package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

type assignmentStoreStub struct {
	listFn    func(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error)
	upsertFn  func(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error)
	correctFn func(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error)
	rescindFn func(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error)
}

func (s assignmentStoreStub) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]types.Assignment, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, tenantID, asOfDate, personUUID)
}

func (s assignmentStoreStub) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error) {
	return s.upsertFn(ctx, tenantID, effectiveDate, personUUID, positionUUID, status, allocatedFte)
}

func (s assignmentStoreStub) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	return s.correctFn(ctx, tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
}

func (s assignmentStoreStub) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	return s.rescindFn(ctx, tenantID, assignmentUUID, targetEffectiveDate, payload)
}

func TestCanonicalizeJSONObjectRaw(t *testing.T) {
	t.Run("empty -> bad request", func(t *testing.T) {
		_, err := CanonicalizeJSONObjectRaw(nil)
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("invalid json -> bad request", func(t *testing.T) {
		_, err := CanonicalizeJSONObjectRaw(json.RawMessage("{bad"))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("non-object -> bad request", func(t *testing.T) {
		_, err := CanonicalizeJSONObjectRaw(json.RawMessage(`[1,2,3]`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("mixed types canonicalize", func(t *testing.T) {
		raw := json.RawMessage(`{"b":true,"a":null,"c":"x","d":1,"e":[2,3]}`)
		got, err := CanonicalizeJSONObjectRaw(raw)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if string(got) != `{"a":null,"b":true,"c":"x","d":1,"e":[2,3]}` {
			t.Fatalf("got=%s", got)
		}
	})
}

func TestCanonicalizeJSONObjectOrEmpty(t *testing.T) {
	t.Run("empty => {}", func(t *testing.T) {
		got, err := CanonicalizeJSONObjectOrEmpty(nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{}` {
			t.Fatalf("got=%s", got)
		}
	})

	t.Run("null => {}", func(t *testing.T) {
		got, err := CanonicalizeJSONObjectOrEmpty(json.RawMessage(`null`))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{}` {
			t.Fatalf("got=%s", got)
		}
	})

	t.Run("object => canonicalize", func(t *testing.T) {
		got, err := CanonicalizeJSONObjectOrEmpty(json.RawMessage(`{"b":1,"a":2}`))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{"a":2,"b":1}` {
			t.Fatalf("got=%s", got)
		}
	})
}

func TestCanonicalizeJSON_DefaultBranches(t *testing.T) {
	t.Run("default marshal ok", func(t *testing.T) {
		var b strings.Builder
		if err := canonicalizeJSON(&b, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("err=%v", err)
		}
		if b.String() == "" {
			t.Fatalf("expected non-empty output")
		}
	})

	t.Run("default marshal error", func(t *testing.T) {
		var b strings.Builder
		ch := make(chan int)
		if err := canonicalizeJSON(&b, ch); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("map propagates nested error", func(t *testing.T) {
		var b strings.Builder
		ch := make(chan int)
		if err := canonicalizeJSON(&b, map[string]any{"x": ch}); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("array propagates nested error", func(t *testing.T) {
		var b strings.Builder
		ch := make(chan int)
		if err := canonicalizeJSON(&b, []any{ch}); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestAssignmentsFacade_UpsertPrimaryAssignmentForPerson(t *testing.T) {
	t.Run("validates required fields before store", func(t *testing.T) {
		called := false
		facade := NewAssignmentsFacade(assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (types.Assignment, error) {
				called = true
				return types.Assignment{}, nil
			},
		})

		_, err := facade.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if called {
			t.Fatal("unexpected store call")
		}
	})

	t.Run("normalizes defaults before store", func(t *testing.T) {
		var got types.Assignment
		facade := NewAssignmentsFacade(assignmentStoreStub{
			upsertFn: func(_ context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (types.Assignment, error) {
				got = types.Assignment{
					AssignmentUUID: "as1",
					PersonUUID:     personUUID,
					PositionUUID:   positionUUID,
					Status:         status,
					EffectiveAt:    effectiveDate,
				}
				if tenantID != "t1" {
					t.Fatalf("tenantID=%q", tenantID)
				}
				if allocatedFte != "0.5" {
					t.Fatalf("allocatedFte=%q", allocatedFte)
				}
				return got, nil
			},
		})

		assignment, err := facade.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", " 2026-01-01 ", " p1 ", " pos1 ", " ", " 0.5 ")
		if err != nil {
			t.Fatal(err)
		}
		if got.EffectiveAt != "2026-01-01" || got.PersonUUID != "p1" || got.PositionUUID != "pos1" {
			t.Fatalf("store got=%+v", got)
		}
		if assignment.Status != "active" {
			t.Fatalf("status=%q", assignment.Status)
		}
	})
}

func TestAssignmentsFacade_CorrectAssignmentEvent(t *testing.T) {
	t.Run("validates before store", func(t *testing.T) {
		called := false
		facade := NewAssignmentsFacade(assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				called = true
				return "", nil
			},
		})

		_, err := facade.CorrectAssignmentEvent(context.Background(), "t1", "", "2026-01-01", json.RawMessage(`{}`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if called {
			t.Fatal("unexpected store call")
		}
	})

	t.Run("missing target effective date", func(t *testing.T) {
		facade := NewAssignmentsFacade(assignmentStoreStub{})
		_, err := facade.CorrectAssignmentEvent(context.Background(), "t1", "as1", "", json.RawMessage(`{}`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("invalid target effective date", func(t *testing.T) {
		facade := NewAssignmentsFacade(assignmentStoreStub{})
		_, err := facade.CorrectAssignmentEvent(context.Background(), "t1", "as1", "bad", json.RawMessage(`{}`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("canonicalizes payload before store", func(t *testing.T) {
		called := false
		facade := NewAssignmentsFacade(assignmentStoreStub{
			correctFn: func(_ context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
				called = true
				if tenantID != "t1" {
					t.Fatalf("tenantID=%q", tenantID)
				}
				if assignmentUUID != "as1" {
					t.Fatalf("assignmentUUID=%q", assignmentUUID)
				}
				if targetEffectiveDate != "2026-01-01" {
					t.Fatalf("targetEffectiveDate=%q", targetEffectiveDate)
				}
				if string(replacementPayload) != `{"a":2,"b":1}` {
					t.Fatalf("payload=%s", replacementPayload)
				}
				return "evt1", nil
			},
		})

		got, err := facade.CorrectAssignmentEvent(context.Background(), "t1", " as1 ", " 2026-01-01 ", json.RawMessage(`{"b":1,"a":2}`))
		if err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("expected store call")
		}
		if got != "evt1" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("invalid replacement payload rejects", func(t *testing.T) {
		facade := NewAssignmentsFacade(assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				t.Fatal("unexpected store call")
				return "", nil
			},
		})

		_, err := facade.CorrectAssignmentEvent(context.Background(), "t1", "as1", "2026-01-01", json.RawMessage(`"bad"`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})
}

func TestAssignmentsFacade_RescindAssignmentEvent(t *testing.T) {
	t.Run("validates before store", func(t *testing.T) {
		called := false
		facade := NewAssignmentsFacade(assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				called = true
				return "", nil
			},
		})

		_, err := facade.RescindAssignmentEvent(context.Background(), "t1", "as1", "bad", nil)
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if called {
			t.Fatal("unexpected store call")
		}
	})

	t.Run("missing target effective date", func(t *testing.T) {
		facade := NewAssignmentsFacade(assignmentStoreStub{})
		_, err := facade.RescindAssignmentEvent(context.Background(), "t1", "as1", "", json.RawMessage(`{}`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("invalid target effective date", func(t *testing.T) {
		facade := NewAssignmentsFacade(assignmentStoreStub{})
		_, err := facade.RescindAssignmentEvent(context.Background(), "t1", "as1", "bad", json.RawMessage(`{}`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("normalizes null payload to empty object", func(t *testing.T) {
		called := false
		facade := NewAssignmentsFacade(assignmentStoreStub{
			rescindFn: func(_ context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
				called = true
				if tenantID != "t1" {
					t.Fatalf("tenantID=%q", tenantID)
				}
				if assignmentUUID != "as1" {
					t.Fatalf("assignmentUUID=%q", assignmentUUID)
				}
				if targetEffectiveDate != "2026-01-01" {
					t.Fatalf("targetEffectiveDate=%q", targetEffectiveDate)
				}
				if string(payload) != `{}` {
					t.Fatalf("payload=%s", payload)
				}
				return "evt2", nil
			},
		})

		got, err := facade.RescindAssignmentEvent(context.Background(), "t1", " as1 ", " 2026-01-01 ", json.RawMessage(`null`))
		if err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("expected store call")
		}
		if got != "evt2" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("invalid payload rejects", func(t *testing.T) {
		facade := NewAssignmentsFacade(assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				t.Fatal("unexpected store call")
				return "", nil
			},
		})

		_, err := facade.RescindAssignmentEvent(context.Background(), "t1", "as1", "2026-01-01", json.RawMessage(`"bad"`))
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})
}
