package main

import (
	"strings"
	"testing"
	"time"
)

func TestValidateOrgunitSnapshot_Success(t *testing.T) {
	snapshot := orgunitSnapshotFile{
		Version:    orgunitSnapshotVersion,
		AsOfDate:   "2026-04-09",
		ExportedAt: time.Now().UTC(),
		Tenants: []orgunitSnapshotTenant{
			{
				TenantUUID: "00000000-0000-0000-0000-000000000001",
				NodeCount:  2,
				RootCount:  1,
				Nodes: []orgunitSnapshotNode{
					{OrgCode: "ROOT", Name: "Root", Status: "active", IsBusinessUnit: true},
					{OrgCode: "CHILD", ParentOrgCode: "ROOT", Name: "Child", Status: "disabled"},
				},
			},
		},
	}

	if err := validateOrgunitSnapshot(snapshot); err != nil {
		t.Fatalf("validateOrgunitSnapshot() error = %v", err)
	}
}

func TestValidateOrgunitSnapshot_Failures(t *testing.T) {
	base := orgunitSnapshotFile{
		Version:    orgunitSnapshotVersion,
		AsOfDate:   "2026-04-09",
		ExportedAt: time.Now().UTC(),
		Tenants: []orgunitSnapshotTenant{
			{
				TenantUUID: "00000000-0000-0000-0000-000000000001",
				NodeCount:  2,
				RootCount:  1,
				Nodes: []orgunitSnapshotNode{
					{OrgCode: "ROOT", Name: "Root", Status: "active", IsBusinessUnit: true},
					{OrgCode: "CHILD", ParentOrgCode: "ROOT", Name: "Child", Status: "active"},
				},
			},
		},
	}

	t.Run("missing parent", func(t *testing.T) {
		snapshot := cloneOrgunitSnapshotFile(base)
		snapshot.Tenants[0].Nodes[1].ParentOrgCode = "MISSING"
		err := validateOrgunitSnapshot(snapshot)
		if err == nil || !strings.Contains(err.Error(), "missing parent") {
			t.Fatalf("expected missing parent error, got %v", err)
		}
	})

	t.Run("duplicate org code", func(t *testing.T) {
		snapshot := cloneOrgunitSnapshotFile(base)
		snapshot.Tenants[0].Nodes[1].OrgCode = "ROOT"
		err := validateOrgunitSnapshot(snapshot)
		if err == nil || !strings.Contains(err.Error(), "duplicate org_code") {
			t.Fatalf("expected duplicate org_code error, got %v", err)
		}
	})

	t.Run("multiple roots", func(t *testing.T) {
		snapshot := cloneOrgunitSnapshotFile(base)
		snapshot.Tenants[0].Nodes[1].ParentOrgCode = ""
		snapshot.Tenants[0].RootCount = 2
		err := validateOrgunitSnapshot(snapshot)
		if err == nil || !strings.Contains(err.Error(), "root_count must be exactly 1") {
			t.Fatalf("expected root_count error, got %v", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		snapshot := cloneOrgunitSnapshotFile(base)
		snapshot.Tenants[0].Nodes[1].Status = "oops"
		err := validateOrgunitSnapshot(snapshot)
		if err == nil || !strings.Contains(err.Error(), "invalid status") {
			t.Fatalf("expected invalid status error, got %v", err)
		}
	})
}

func TestOrderOrgunitSnapshotNodes(t *testing.T) {
	nodes := []orgunitSnapshotNode{
		{OrgCode: "CHILD-B", ParentOrgCode: "ROOT", Name: "B", Status: "active"},
		{OrgCode: "ROOT", Name: "Root", Status: "active"},
		{OrgCode: "CHILD-A", ParentOrgCode: "ROOT", Name: "A", Status: "active"},
	}

	ordered, err := orderOrgunitSnapshotNodes(nodes)
	if err != nil {
		t.Fatalf("orderOrgunitSnapshotNodes() error = %v", err)
	}

	got := []string{ordered[0].OrgCode, ordered[1].OrgCode, ordered[2].OrgCode}
	want := []string{"ROOT", "CHILD-A", "CHILD-B"}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("ordered[%d] = %q, want %q (all=%v)", idx, got[idx], want[idx], got)
		}
	}
}

func TestOrderOrgunitSnapshotNodes_Cycle(t *testing.T) {
	nodes := []orgunitSnapshotNode{
		{OrgCode: "A", ParentOrgCode: "B", Name: "A", Status: "active"},
		{OrgCode: "B", ParentOrgCode: "A", Name: "B", Status: "active"},
	}

	if _, err := orderOrgunitSnapshotNodes(nodes); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestSplitNodePath(t *testing.T) {
	got := splitNodePath("AAAAAAAB.AAAAAAAC.AAAAAAAD")
	want := []string{"AAAAAAAB", "AAAAAAAC", "AAAAAAAD"}
	if !equalStringSlices(got, want) {
		t.Fatalf("splitNodePath() = %v, want %v", got, want)
	}
}

func cloneOrgunitSnapshotFile(input orgunitSnapshotFile) orgunitSnapshotFile {
	out := input
	out.Tenants = append([]orgunitSnapshotTenant(nil), input.Tenants...)
	for idx := range out.Tenants {
		out.Tenants[idx].Nodes = append([]orgunitSnapshotNode(nil), input.Tenants[idx].Nodes...)
	}
	return out
}
