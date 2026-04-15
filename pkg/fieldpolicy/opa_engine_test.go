package fieldpolicy

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

func TestOPAEngineAuxiliaryBranches(t *testing.T) {
	t.Run("prepare query returns cached error", func(t *testing.T) {
		resetSetIDStrategyPreparedQueryForTest(t)

		setIDStrategyPreparedQueryErr = errors.New("cached query error")
		setIDStrategyPreparedQueryOnce.Do(func() {})

		query, err := prepareSetIDStrategyOPAQuery()
		if query != nil || err == nil || !strings.Contains(err.Error(), "cached query error") {
			t.Fatalf("query=%v err=%v", query, err)
		}

		_, err = resolveWithOPA(PolicyContext{}, "", nil)
		if err == nil || !strings.Contains(err.Error(), "prepare setid strategy opa query") {
			t.Fatalf("expected prepare error, got %v", err)
		}
	})

	t.Run("resolve with opa handles multiple result rows", func(t *testing.T) {
		installPreparedQueryForTest(t, "data.test.result[_]", `package test
result := [{"ok": true}, {"ok": true}]`)

		_, err := resolveWithOPA(PolicyContext{}, "", nil)
		if err == nil || !strings.Contains(err.Error(), "unexpected opa result cardinality") {
			t.Fatalf("expected cardinality error, got %v", err)
		}
	})

	t.Run("resolve with opa handles decode error", func(t *testing.T) {
		installPreparedQueryForTest(t, "data.test.result", `package test
result := "bad"`)

		_, err := resolveWithOPA(PolicyContext{}, "", nil)
		if err == nil || !strings.Contains(err.Error(), "decode setid strategy opa result") {
			t.Fatalf("expected decode error, got %v", err)
		}
	})

	t.Run("resolve with opa maps empty policy error to conflict", func(t *testing.T) {
		installPreparedQueryForTest(t, "data.test.result", `package test
result := {"ok": false}`)

		_, err := resolveWithOPA(PolicyContext{}, "", nil)
		if !errors.Is(err, errors.New(ErrorPolicyConflict)) && (err == nil || err.Error() != ErrorPolicyConflict) {
			t.Fatalf("expected policy conflict, got %v", err)
		}
	})

	t.Run("resolve with opa requires decision payload", func(t *testing.T) {
		installPreparedQueryForTest(t, "data.test.result", `package test
result := {"ok": true}`)

		_, err := resolveWithOPA(PolicyContext{}, "", nil)
		if err == nil || err.Error() != ErrorPolicyConflict {
			t.Fatalf("expected policy conflict, got %v", err)
		}
	})

	t.Run("remarshal opa value returns marshal error", func(t *testing.T) {
		var target map[string]any
		err := remarshalOPAValue(map[string]any{"bad": func() {}}, &target)
		if err == nil {
			t.Fatal("expected remarshal error")
		}
	})

	t.Run("normalize resolution trace trims blanks", func(t *testing.T) {
		if got := normalizeResolutionTrace(nil); got != nil {
			t.Fatalf("expected nil trace, got %v", got)
		}
		if got := normalizeResolutionTrace([]string{" ", "\t"}); got != nil {
			t.Fatalf("expected nil blank trace, got %v", got)
		}
		got := normalizeResolutionTrace([]string{" first ", "", "second "})
		want := []string{"first", "second"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("trace=%v want=%v", got, want)
		}
	})
}

func TestSetIDStrategyHelperBranches(t *testing.T) {
	t.Run("orgunit baseline capability key variants", func(t *testing.T) {
		cases := []string{
			OrgUnitCreateFieldPolicyCapabilityKey,
			OrgUnitAddVersionFieldPolicyCapabilityKey,
			OrgUnitInsertVersionFieldPolicyCapabilityKey,
			OrgUnitCorrectFieldPolicyCapabilityKey,
			OrgUnitWriteFieldPolicyCapabilityKey,
		}
		for _, capabilityKey := range cases {
			got, ok := OrgUnitBaselineCapabilityKey(capabilityKey)
			if !ok || got != OrgUnitWriteFieldPolicyCapabilityKey {
				t.Fatalf("capability=%q got=%q ok=%v", capabilityKey, got, ok)
			}
		}
		if got, ok := OrgUnitBaselineCapabilityKey("unknown.capability"); ok || got != "" {
			t.Fatalf("expected unknown capability to miss, got=%q ok=%v", got, ok)
		}
	})

	t.Run("sort records prefers priority effective date created at then policy id", func(t *testing.T) {
		records := []Record{
			{PolicyID: "z-last", Priority: 1, EffectiveDate: "2026-01-01", CreatedAt: "2026-01-01T00:00:00Z"},
			{PolicyID: "policy-b", Priority: 2, EffectiveDate: "2026-01-01", CreatedAt: "2026-01-01T00:00:00Z"},
			{PolicyID: "policy-a", Priority: 2, EffectiveDate: "2026-01-01", CreatedAt: "2026-01-01T00:00:00Z"},
			{PolicyID: "newer-effective", Priority: 2, EffectiveDate: "2026-02-01", CreatedAt: "2026-01-01T00:00:00Z"},
			{PolicyID: "newer-created", Priority: 2, EffectiveDate: "2026-02-01", CreatedAt: "2026-01-02T00:00:00Z"},
		}

		sortRecords(records)

		got := []string{records[0].PolicyID, records[1].PolicyID, records[2].PolicyID, records[3].PolicyID, records[4].PolicyID}
		want := []string{"newer-created", "newer-effective", "policy-a", "policy-b", "z-last"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("order=%v want=%v", got, want)
		}
	})

	t.Run("normalize records fills synthetic policy id", func(t *testing.T) {
		records := normalizeRecords([]Record{{
			CapabilityKey:       " Org.Capability ",
			FieldKey:            " FIELD_X ",
			OrgApplicability:    " TENANT ",
			ResolvedSetID:       " b1000 ",
			BusinessUnitNodeKey: " 10000001 ",
			DefaultRuleRef:      " rule.ref ",
			DefaultValue:        " value ",
			AllowedValueCodes:   []string{" A ", "A", "", " B "},
			PriorityMode:        "",
			LocalOverrideMode:   "",
			EffectiveDate:       " 2026-01-01 ",
			CreatedAt:           " 2026-01-01T00:00:00Z ",
		}})

		if len(records) != 1 {
			t.Fatalf("records=%v", records)
		}
		record := records[0]
		if record.PolicyID == "" || !strings.Contains(record.PolicyID, "org.capability") {
			t.Fatalf("expected synthetic policy id, got %q", record.PolicyID)
		}
		if record.CapabilityKey != "org.capability" || record.FieldKey != "field_x" || record.ResolvedSetID != "B1000" {
			t.Fatalf("normalized record=%+v", record)
		}
		if !reflect.DeepEqual(record.AllowedValueCodes, []string{"A", "B"}) {
			t.Fatalf("allowed value codes=%v", record.AllowedValueCodes)
		}
		if record.PriorityMode != PriorityModeBlendCustomFirst || record.LocalOverrideMode != LocalOverrideModeAllow {
			t.Fatalf("mode normalization=%+v", record)
		}
	})

	t.Run("normalize allowed value codes handles nil and blanks", func(t *testing.T) {
		if got := normalizeAllowedValueCodes(nil); got != nil {
			t.Fatalf("expected nil codes, got %v", got)
		}
		if got := normalizeAllowedValueCodes([]string{" ", "\t"}); got != nil {
			t.Fatalf("expected nil blank codes, got %v", got)
		}
		got := normalizeAllowedValueCodes([]string{" A ", "A", "B"})
		want := []string{"A", "B"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("codes=%v want=%v", got, want)
		}
	})

	t.Run("normalize policy ids handles nil blanks and duplicates", func(t *testing.T) {
		if got := normalizePolicyIDs(nil); got != nil {
			t.Fatalf("expected nil ids, got %v", got)
		}
		if got := normalizePolicyIDs([]string{" ", "\t"}); got != nil {
			t.Fatalf("expected nil blank ids, got %v", got)
		}
		got := normalizePolicyIDs([]string{" policy-a ", "policy-a", "policy-b"})
		want := []string{"policy-a", "policy-b"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ids=%v want=%v", got, want)
		}
	})
}

func resetSetIDStrategyPreparedQueryForTest(t *testing.T) {
	t.Helper()
	setIDStrategyPreparedQueryOnce = sync.Once{}
	setIDStrategyPreparedQuery = nil
	setIDStrategyPreparedQueryErr = nil
	t.Cleanup(func() {
		setIDStrategyPreparedQueryOnce = sync.Once{}
		setIDStrategyPreparedQuery = nil
		setIDStrategyPreparedQueryErr = nil
	})
}

func installPreparedQueryForTest(t *testing.T, query string, module string) {
	t.Helper()
	resetSetIDStrategyPreparedQueryForTest(t)

	prepared, err := rego.New(
		rego.Query(query),
		rego.Module("test.rego", module),
	).PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("prepare rego query: %v", err)
	}

	setIDStrategyPreparedQuery = &prepared
	setIDStrategyPreparedQueryOnce.Do(func() {})
}
