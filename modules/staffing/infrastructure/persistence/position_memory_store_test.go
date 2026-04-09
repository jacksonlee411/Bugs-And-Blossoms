package persistence

import (
	"context"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

func TestPositionMemoryStore(t *testing.T) {
	store := NewPositionMemoryStore()

	t.Run("create validates required fields", func(t *testing.T) {
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "", "10000001", "jp1", "", "A")
		if err == nil || !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("create and update", func(t *testing.T) {
		p, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
		if err != nil {
			t.Fatal(err)
		}
		if p.CapacityFTE != "1.0" {
			t.Fatalf("expected default capacity, got %q", p.CapacityFTE)
		}

		updated, err := store.UpdatePositionCurrent(context.Background(), "t1", p.PositionUUID, "2026-02-01", "10000002", "mgr1", "jp2", "2.5", "B", "disabled")
		if err != nil {
			t.Fatal(err)
		}
		if updated.OrgUnitID != "10000002" || updated.LifecycleStatus != "disabled" || updated.EffectiveAt != "2026-02-01" {
			t.Fatalf("updated=%#v", updated)
		}
	})
}
