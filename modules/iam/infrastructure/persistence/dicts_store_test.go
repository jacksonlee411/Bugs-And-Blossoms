package persistence

import "testing"

func TestMemoryStoreValuesForTenant(t *testing.T) {
	store := NewMemoryStore()

	got := store.valuesForTenant("t1")
	if len(got) == 0 {
		t.Fatalf("expected seed values, got=%v", got)
	}

	if got := store.valuesForTenant("missing"); got != nil {
		t.Fatalf("expected nil for missing tenant, got=%v", got)
	}
}
