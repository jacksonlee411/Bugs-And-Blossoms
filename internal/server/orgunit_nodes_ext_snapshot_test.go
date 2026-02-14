package server

import (
	"context"
	"testing"
)

func TestOrgUnitMemoryStore_GetOrgUnitVersionExtSnapshot(t *testing.T) {
	store := newOrgUnitMemoryStore()
	snap, err := store.GetOrgUnitVersionExtSnapshot(context.Background(), "t1", 10000001, "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if !snap.HasVersionData {
		t.Fatalf("expected HasVersionData=true")
	}
	if snap.VersionValues == nil || snap.VersionLabels == nil || snap.EventLabels == nil {
		t.Fatalf("expected maps to be initialized")
	}
}
