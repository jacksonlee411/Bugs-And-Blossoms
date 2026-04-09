package persistence

import (
	"context"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

func TestAssignmentMemoryStore_WithStateSharesBackingMap(t *testing.T) {
	assigns := map[string]map[string][]types.Assignment{
		"t1": {
			"p1": {{
				AssignmentUUID: "as1",
				PersonUUID:     "p1",
				PositionUUID:   "pos1",
				Status:         "active",
				EffectiveAt:    "2026-01-01",
			}},
		},
	}

	store := NewAssignmentMemoryStoreWithState(assigns)
	listed, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].AssignmentUUID != "as1" {
		t.Fatalf("listed=%#v", listed)
	}

	_, err = store.CorrectAssignmentEvent(context.Background(), "t1", "as1", "2026-01-01", []byte(`{"status":"inactive"}`))
	if err != nil {
		t.Fatal(err)
	}

	if got := assigns["t1"]["p1"][0].Status; got != "inactive" {
		t.Fatalf("expected backing map updated, got %q", got)
	}
}
