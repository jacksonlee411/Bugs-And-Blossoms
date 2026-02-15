package server

import (
	"context"
	"errors"
	"testing"
)

type buListerStub struct {
	orgUnitStoreStub
	bus []OrgUnitNode
	err error
}

func (s buListerStub) ListBusinessUnitsCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]OrgUnitNode(nil), s.bus...), nil
}

type nodesCurrentStub struct {
	orgUnitStoreStub
	nodes []OrgUnitNode
	err   error
}

func (s nodesCurrentStub) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]OrgUnitNode(nil), s.nodes...), nil
}

func TestListBusinessUnitsCurrent_UsesListerWhenAvailable(t *testing.T) {
	ctx := context.Background()
	want := []OrgUnitNode{{ID: "1", OrgCode: "A001", IsBusinessUnit: true}}
	got, err := listBusinessUnitsCurrent(ctx, buListerStub{bus: want}, "t1", "2026-01-01")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "A001" {
		t.Fatalf("got=%v", got)
	}
}

func TestListBusinessUnitsCurrent_FiltersWhenListerMissing(t *testing.T) {
	ctx := context.Background()
	nodes := []OrgUnitNode{
		{OrgCode: "A001", IsBusinessUnit: true},
		{OrgCode: "A002", IsBusinessUnit: false},
		{OrgCode: "A003", IsBusinessUnit: true},
	}
	got, err := listBusinessUnitsCurrent(ctx, nodesCurrentStub{nodes: nodes}, "t1", "2026-01-01")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 2 || got[0].OrgCode != "A001" || got[1].OrgCode != "A003" {
		t.Fatalf("got=%v", got)
	}
}

func TestListBusinessUnitsCurrent_PropagatesError(t *testing.T) {
	ctx := context.Background()
	if _, err := listBusinessUnitsCurrent(ctx, nodesCurrentStub{err: errors.New("boom")}, "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
}
