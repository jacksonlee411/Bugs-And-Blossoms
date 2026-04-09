package iam

import "testing"

func TestDictMemoryStoreHelpers(t *testing.T) {
	store := NewDictMemoryStore()

	if tenantID, ok := store.ResolveSourceTenant("t1", "org_type"); !ok || tenantID != "t1" {
		t.Fatalf("tenantID=%q ok=%v", tenantID, ok)
	}
	if tenantID, ok := store.ResolveSourceTenant("t1", "missing"); ok || tenantID != "" {
		t.Fatalf("tenantID=%q ok=%v", tenantID, ok)
	}

	if got := store.ValuesForTenant("t1"); len(got) == 0 {
		t.Fatal("expected values")
	}
	if got := store.ValuesForTenant("missing"); got != nil {
		t.Fatalf("expected nil, got=%v", got)
	}
}
