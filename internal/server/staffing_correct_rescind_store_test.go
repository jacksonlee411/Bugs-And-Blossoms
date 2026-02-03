package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestStaffingPGStore_CorrectRescindAssignmentEvent(t *testing.T) {
	t.Run("correct validates inputs before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "", "2026-01-01", []byte(`{}`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("correct rejects missing target_effective_date before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "", []byte(`{}`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("correct rejects invalid target_effective_date before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "bad", []byte(`{}`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("correct rejects invalid payload before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`[]`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("correct begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("correct set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec"), execErrAt: 1}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("correct submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec"), execErrAt: 2}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("correct commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("correct ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		got, err := store.CorrectAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`{"position_uuid":"p1"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got == "" {
			t.Fatal("expected event id")
		}
	})

	t.Run("rescind validates inputs before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "", "2026-01-01", nil)
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("rescind rejects missing target_effective_date before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "", nil)
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("rescind rejects invalid target_effective_date before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "bad", nil)
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("rescind rejects invalid payload before tx", func(t *testing.T) {
		beginCalled := false
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			beginCalled = true
			return &stubTx{}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`[]`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
		if beginCalled {
			t.Fatal("unexpected Begin")
		}
	})

	t.Run("rescind begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rescind set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec"), execErrAt: 1}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rescind submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec"), execErrAt: 2}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rescind commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rescind ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		got, err := store.RescindAssignmentEvent(context.Background(), "t1", "a1", "2026-01-01", []byte(`{"note":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got == "" {
			t.Fatal("expected event id")
		}
	})
}

func TestStaffingMemoryStore_CorrectRescindAssignmentEvent(t *testing.T) {
	t.Run("correct validates required fields", func(t *testing.T) {
		s := newStaffingMemoryStore()
		_, err := s.CorrectAssignmentEvent(context.Background(), "t1", "", "2026-01-01", []byte(`{}`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("correct updates matching slice", func(t *testing.T) {
		s := newStaffingMemoryStore()
		tenantID := "t1"
		personUUID := "p1"
		assignmentID := "as1"
		s.assigns[tenantID] = map[string][]Assignment{
			personUUID: {{
				AssignmentUUID: assignmentID,
				PersonUUID:     personUUID,
				PositionUUID:   "pos1",
				Status:         "active",
				EffectiveAt:    "2026-01-01",
			}},
		}

		_, err := s.CorrectAssignmentEvent(context.Background(), tenantID, assignmentID, "2026-01-01", []byte(`{"position_uuid":"pos2","status":"inactive"}`))
		if err != nil {
			t.Fatal(err)
		}
		got := s.assigns[tenantID][personUUID][0]
		if got.PositionUUID != "pos2" || got.Status != "inactive" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("correct rejects invalid payload", func(t *testing.T) {
		s := newStaffingMemoryStore()
		_, err := s.CorrectAssignmentEvent(context.Background(), "t1", "as1", "2026-01-01", []byte(`[]`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("correct skips non-matching events", func(t *testing.T) {
		s := newStaffingMemoryStore()
		tenantID := "t1"
		personUUID := "p1"
		s.assigns[tenantID] = map[string][]Assignment{
			personUUID: {
				{AssignmentUUID: "other", PersonUUID: personUUID, PositionUUID: "pos1", Status: "active", EffectiveAt: "2026-01-01"},
				{AssignmentUUID: "as1", PersonUUID: personUUID, PositionUUID: "pos1", Status: "active", EffectiveAt: "2026-01-01"},
			},
		}
		_, err := s.CorrectAssignmentEvent(context.Background(), tenantID, "as1", "2026-01-01", []byte(`{"status":"inactive"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.assigns[tenantID][personUUID][1].Status; got != "inactive" {
			t.Fatalf("got=%s", got)
		}
	})

	t.Run("correct can hit json.Unmarshal error branch", func(t *testing.T) {
		s := newStaffingMemoryStore()
		tenantID := "t1"
		personUUID := "p1"
		assignmentID := "as1"
		s.assigns[tenantID] = map[string][]Assignment{
			personUUID: {{
				AssignmentUUID: assignmentID,
				PersonUUID:     personUUID,
				PositionUUID:   "pos1",
				Status:         "active",
				EffectiveAt:    "2026-01-01",
			}},
		}

		_, err := s.CorrectAssignmentEvent(context.Background(), tenantID, assignmentID, "2026-01-01", []byte(`{"position_uuid":1e309}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("correct not found", func(t *testing.T) {
		s := newStaffingMemoryStore()
		s.assigns["t1"] = map[string][]Assignment{"p1": {}}
		_, err := s.CorrectAssignmentEvent(context.Background(), "t1", "as1", "2026-01-01", []byte(`{"position_uuid":"pos2"}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rescind cannot rescind create when later slices exist", func(t *testing.T) {
		s := newStaffingMemoryStore()
		tenantID := "t1"
		personUUID := "p1"
		assignmentID := "as1"
		s.assigns[tenantID] = map[string][]Assignment{
			personUUID: {
				{AssignmentUUID: assignmentID, PersonUUID: personUUID, PositionUUID: "pos1", Status: "active", EffectiveAt: "2026-01-01"},
				{AssignmentUUID: assignmentID, PersonUUID: personUUID, PositionUUID: "pos2", Status: "active", EffectiveAt: "2026-02-01"},
			},
		}
		_, err := s.RescindAssignmentEvent(context.Background(), tenantID, assignmentID, "2026-01-01", nil)
		if err == nil || !strings.Contains(err.Error(), "STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND") {
			t.Fatalf("expected create cannot rescind, got %v", err)
		}
	})

	t.Run("rescind validates required fields", func(t *testing.T) {
		s := newStaffingMemoryStore()
		_, err := s.RescindAssignmentEvent(context.Background(), "t1", "", "2026-01-01", nil)
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("rescind can rescind create when no later slices", func(t *testing.T) {
		s := newStaffingMemoryStore()
		tenantID := "t1"
		personUUID := "p1"
		assignmentID := "as1"
		s.assigns[tenantID] = map[string][]Assignment{
			personUUID: {
				{AssignmentUUID: assignmentID, PersonUUID: personUUID, PositionUUID: "pos1", Status: "active", EffectiveAt: "2026-01-01"},
			},
		}
		_, err := s.RescindAssignmentEvent(context.Background(), tenantID, assignmentID, "2026-01-01", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.assigns[tenantID][personUUID]; len(got) != 0 {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("rescind removes target slice", func(t *testing.T) {
		s := newStaffingMemoryStore()
		tenantID := "t1"
		personUUID := "p1"
		assignmentID := "as1"
		s.assigns[tenantID] = map[string][]Assignment{
			personUUID: {
				{AssignmentUUID: "other", PersonUUID: personUUID, PositionUUID: "pos0", Status: "active", EffectiveAt: "2026-01-01"},
				{AssignmentUUID: assignmentID, PersonUUID: personUUID, PositionUUID: "pos1", Status: "active", EffectiveAt: "2026-01-01"},
				{AssignmentUUID: assignmentID, PersonUUID: personUUID, PositionUUID: "pos2", Status: "active", EffectiveAt: "2026-02-01"},
			},
		}
		_, err := s.RescindAssignmentEvent(context.Background(), tenantID, assignmentID, "2026-02-01", json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.assigns[tenantID][personUUID]; len(got) != 2 || got[1].EffectiveAt != "2026-01-01" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("rescind not found", func(t *testing.T) {
		s := newStaffingMemoryStore()
		s.assigns["t1"] = map[string][]Assignment{"p1": {}}
		_, err := s.RescindAssignmentEvent(context.Background(), "t1", "as1", "2026-01-01", nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
