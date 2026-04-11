package main

import (
	"strings"
	"testing"
	"time"
)

func TestTargetStaffingBootstrapPaths(t *testing.T) {
	paths := targetStaffingBootstrapPaths("/tmp/staffing-schema")
	want := []string{
		"/tmp/staffing-schema/00001_staffing_schema.sql",
		"/tmp/staffing-schema/00002_staffing_tables.sql",
	}
	if len(paths) != len(want) {
		t.Fatalf("unexpected path count: got %d want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("unexpected path at %d: got %q want %q", i, paths[i], want[i])
		}
	}
}

func TestBuildOrgunitStoplineSpecsIncludesTargetRealStaffing(t *testing.T) {
	samples := orgunitStoplineSamples{AsOfDate: "2026-01-01"}
	samples.Heavy.Tenant.TenantUUID = "heavy-tenant"
	samples.Heavy.Root.OrgID = 10000001
	samples.Heavy.Root.OrgCode = "ROOT"
	samples.Heavy.Root.OrgNodeKey = "AAAAAAAB"
	samples.Heavy.DetailsTarget.OrgID = 10000002
	samples.Heavy.DetailsTarget.OrgCode = "NODE"
	samples.Heavy.DetailsTarget.OrgNodeKey = "AAAAAAAC"
	samples.Heavy.SubtreeFilter.OrgCode = "SUBTREE"
	samples.Heavy.SubtreeFilter.NodePath = "AAAAAAAB.AAAAAAAC"
	samples.Heavy.MoveTarget.OrgID = 10000003
	samples.Heavy.MoveTarget.OrgCode = "MOVE"
	samples.Heavy.MoveTarget.OrgNodeKey = "AAAAAAAD"
	samples.Heavy.MoveNewParent.OrgID = 10000004
	samples.Heavy.MoveNewParent.OrgCode = "PARENT"
	samples.Heavy.MoveNewParent.OrgNodeKey = "AAAAAAAE"
	samples.Heavy.SearchQuery = "foo"
	samples.Heavy.MoveEffectiveDate = "2026-01-02"
	samples.Chain.Tenant.TenantUUID = "chain-tenant"
	samples.Chain.BusinessUnit.OrgID = 10000005
	samples.Chain.BusinessUnit.OrgCode = "BU001"
	samples.Chain.BusinessUnit.OrgNodeKey = "AAAAAAAF"

	specs := buildOrgunitStoplineSpecs(samples)

	var targetRealFound bool
	for _, spec := range specs {
		if spec.Key != "staffing-by-org" {
			continue
		}
		if spec.Stage == "target-real" {
			targetRealFound = true
			if !strings.Contains(spec.SQL, "FROM staffing.position_versions") {
				t.Fatalf("target-real staffing query must use staffing.position_versions, got %q", spec.SQL)
			}
		}
		if spec.Stage == "target-shadow" && strings.Contains(spec.SQL, "FROM stopline.position_versions") {
			t.Fatalf("target-shadow staffing explain should not remain in spec list")
		}
	}
	if !targetRealFound {
		t.Fatalf("expected target-real staffing-by-org spec")
	}
}

func TestRenderOrgunitStoplineMarkdownNotes(t *testing.T) {
	report := orgunitStoplineReport{}
	report.CapturedAt = mustParseRFC3339(t, "2026-04-11T00:00:00Z")
	report.AsOfDate = "2026-01-01"
	report.Samples.Heavy.Tenant.TenantUUID = "heavy-tenant"
	report.Samples.Heavy.Tenant.Name = "Heavy Tenant"
	report.Samples.Heavy.Tenant.Hostname = "heavy.localhost"
	report.Samples.Heavy.Root.OrgCode = "ROOT"
	report.Samples.Heavy.Root.OrgNodeKey = "AAAAAAAB"
	report.Samples.Heavy.DetailsTarget.OrgCode = "NODE"
	report.Samples.Heavy.DetailsTarget.OrgNodeKey = "AAAAAAAC"
	report.Samples.Heavy.SubtreeFilter.OrgCode = "SUBTREE"
	report.Samples.Heavy.SubtreeFilter.NodePath = "AAAAAAAB.AAAAAAAC"
	report.Samples.Heavy.MoveTarget.OrgCode = "MOVE"
	report.Samples.Heavy.MoveNewParent.OrgCode = "PARENT"
	report.Samples.Heavy.MoveEffectiveDate = "2026-01-02"
	report.Samples.Chain.Tenant.TenantUUID = "chain-tenant"
	report.Samples.Chain.Tenant.Name = "Chain Tenant"
	report.Samples.Chain.Tenant.Hostname = "chain.localhost"
	report.Samples.Chain.BusinessUnit.OrgCode = "BU001"
	report.Samples.Chain.BusinessUnit.SetID = "S2601"
	report.Results = []orgunitStoplineExplainRecord{
		{
			Key:         "staffing-by-org",
			Stage:       "target-real",
			RawJSONFile: "target-real-staffing-by-org.explain.json",
		},
		{
			Key:         "setid-resolve",
			Stage:       "target-real",
			RawJSONFile: "target-real-setid-resolve.explain.json",
		},
	}

	md := renderOrgunitStoplineMarkdown(report)
	if !strings.Contains(md, "`target-real` 当前覆盖 `orgunit` 新 schema、`orgunit.setid_binding_versions` 与 committed `staffing.position_versions`") {
		t.Fatalf("markdown notes missing target-real staffing note: %s", md)
	}
	if !strings.Contains(md, "`SetID` 的 explain 已不再依赖 `stopline` shadow 表") {
		t.Fatalf("markdown notes missing target-real setid note: %s", md)
	}
}

func mustParseRFC3339(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", raw, err)
	}
	return parsed
}
