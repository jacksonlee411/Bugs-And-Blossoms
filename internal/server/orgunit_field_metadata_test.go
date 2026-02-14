package server

import (
	"testing"
)

func TestOrgUnitFieldMetadata_Definitions_ListAndLookup(t *testing.T) {
	defs := listOrgUnitFieldDefinitions()
	if len(defs) == 0 {
		t.Fatalf("expected field definitions")
	}
	for i := 1; i < len(defs); i++ {
		if defs[i-1].FieldKey > defs[i].FieldKey {
			t.Fatalf("expected sorted field definitions, got %q before %q", defs[i-1].FieldKey, defs[i].FieldKey)
		}
	}

	orig, ok := lookupOrgUnitFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type definition")
	}
	origDict, _ := orig.DataSourceConfig["dict_code"].(string)
	if origDict != "org_type" {
		t.Fatalf("dict_code=%q", origDict)
	}

	// Ensure lookup returns a defensive copy of the config map.
	orig.DataSourceConfig["dict_code"] = "mutated"
	again, ok := lookupOrgUnitFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type definition")
	}
	againDict, _ := again.DataSourceConfig["dict_code"].(string)
	if againDict != "org_type" {
		t.Fatalf("expected dict_code unchanged, got %q", againDict)
	}

	if _, ok := lookupOrgUnitFieldDefinition("not-exists"); ok {
		t.Fatalf("expected lookup to fail")
	}
}

func TestOrgUnitFieldMetadata_DictOptions_ListAndSort(t *testing.T) {
	if got := listOrgUnitDictOptions("no-such-dict", "", 10); len(got) != 0 {
		t.Fatalf("expected empty, got=%v", got)
	}

	// Keyword filtering.
	filtered := listOrgUnitDictOptions("org_type", "comp", 10)
	if len(filtered) == 0 {
		t.Fatalf("expected filtered results")
	}

	// limit < 0 should behave as "no limit" (after normalization).
	all := listOrgUnitDictOptions("org_type", "", -1)
	if len(all) == 0 {
		t.Fatalf("expected options")
	}

	// limit should be applied when > 0.
	one := listOrgUnitDictOptions("org_type", "", 1)
	if len(one) != 1 {
		t.Fatalf("expected 1 option, got=%d", len(one))
	}

	// Tie-breaker sorting: label then value.
	orig := orgUnitDictOptionsRegistry["tie"]
	t.Cleanup(func() {
		if orig == nil {
			delete(orgUnitDictOptionsRegistry, "tie")
			return
		}
		orgUnitDictOptionsRegistry["tie"] = orig
	})
	orgUnitDictOptionsRegistry["tie"] = []orgUnitFieldOption{
		{Value: "b", Label: "Same"},
		{Value: "a", Label: "Same"},
	}
	tie := listOrgUnitDictOptions("tie", "", 0)
	if len(tie) != 2 || tie[0].Value != "a" || tie[1].Value != "b" {
		t.Fatalf("unexpected tie sort: %v", tie)
	}
}

func TestOrgUnitFieldMetadata_DataSourceConfigJSON(t *testing.T) {
	t.Run("marshal ok", func(t *testing.T) {
		def, ok := lookupOrgUnitFieldDefinition("org_type")
		if !ok {
			t.Fatalf("expected org_type definition")
		}
		if got := string(orgUnitFieldDataSourceConfigJSON(def)); got == "" || got == "{}" {
			t.Fatalf("unexpected json=%q", got)
		}
	})

	t.Run("marshal error falls back to {}", func(t *testing.T) {
		def := orgUnitFieldDefinition{
			FieldKey:         "bad",
			DataSourceType:   "DICT",
			DataSourceConfig: map[string]any{"x": func() {}}, // json.Marshal should fail.
		}
		if got := string(orgUnitFieldDataSourceConfigJSON(def)); got != "{}" {
			t.Fatalf("json=%q", got)
		}
	})
}

func TestOrgUnitFieldMetadata_DictOptionsAndLookup(t *testing.T) {
	t.Run("unknown dict code is empty", func(t *testing.T) {
		if got := listOrgUnitDictOptions("nope", "", 10); len(got) != 0 {
			t.Fatalf("len=%d", len(got))
		}
	})

	t.Run("keyword filters and limit applies", func(t *testing.T) {
		got := listOrgUnitDictOptions("org_type", "comp", 1)
		if len(got) != 1 {
			t.Fatalf("len=%d", len(got))
		}
		if got[0].Value != "COMPANY" {
			t.Fatalf("value=%q", got[0].Value)
		}
	})

	t.Run("negative limit means no limit", func(t *testing.T) {
		all := listOrgUnitDictOptions("org_type", "", -1)
		if len(all) != len(orgUnitDictOptionsRegistry["org_type"]) {
			t.Fatalf("len=%d", len(all))
		}
	})

	t.Run("stable sort by label then value", func(t *testing.T) {
		// Create a dedicated dict registry entry to cover the comparator tie-breaker.
		orig := orgUnitDictOptionsRegistry["test_dup"]
		t.Cleanup(func() {
			if orig == nil {
				delete(orgUnitDictOptionsRegistry, "test_dup")
			} else {
				orgUnitDictOptionsRegistry["test_dup"] = orig
			}
		})
		orgUnitDictOptionsRegistry["test_dup"] = []orgUnitFieldOption{
			{Value: "B", Label: "Same"},
			{Value: "A", Label: "Same"},
		}

		got := listOrgUnitDictOptions("test_dup", "", 0)
		if len(got) != 2 {
			t.Fatalf("len=%d", len(got))
		}
		if got[0].Value != "A" || got[1].Value != "B" {
			t.Fatalf("unexpected order: %#v", got)
		}
	})

	t.Run("lookup label", func(t *testing.T) {
		if _, ok := lookupOrgUnitDictLabel("org_type", ""); ok {
			t.Fatalf("expected empty to fail")
		}
		if got, ok := lookupOrgUnitDictLabel("org_type", "department"); !ok || got != "Department" {
			t.Fatalf("got=%q ok=%v", got, ok)
		}
		if _, ok := lookupOrgUnitDictLabel("org_type", "missing"); ok {
			t.Fatalf("expected missing to fail")
		}
	})

	t.Run("lookup option", func(t *testing.T) {
		if _, ok := lookupOrgUnitDictOption("org_type", ""); ok {
			t.Fatalf("expected empty to fail")
		}
		opt, ok := lookupOrgUnitDictOption("org_type", "department")
		if !ok || opt.Label != "Department" || opt.Value != "DEPARTMENT" {
			t.Fatalf("opt=%#v ok=%v", opt, ok)
		}
		if _, ok := lookupOrgUnitDictOption("org_type", "missing"); ok {
			t.Fatalf("expected missing to fail")
		}
	})
}

func TestOrgUnitFieldMetadata_DataSourceConfigOptions(t *testing.T) {
	t.Run("plain returns nil", func(t *testing.T) {
		def, ok := lookupOrgUnitFieldDefinition("short_name")
		if !ok {
			t.Fatalf("expected short_name definition")
		}
		if got := orgUnitFieldDataSourceConfigOptions(def); got != nil {
			t.Fatalf("expected nil, got=%v", got)
		}
	})

	t.Run("dict uses configured options and returns defensive copies", func(t *testing.T) {
		def, ok := lookupOrgUnitFieldDefinition("org_type")
		if !ok {
			t.Fatalf("expected org_type definition")
		}
		got := orgUnitFieldDataSourceConfigOptions(def)
		if len(got) != 1 {
			t.Fatalf("len=%d", len(got))
		}
		if got[0]["dict_code"] != "org_type" {
			t.Fatalf("dict_code=%v", got[0]["dict_code"])
		}

		// Mutating the returned value must not affect subsequent lookups.
		got[0]["dict_code"] = "mutated"
		again, ok := lookupOrgUnitFieldDefinition("org_type")
		if !ok {
			t.Fatalf("expected org_type definition")
		}
		optsAgain := orgUnitFieldDataSourceConfigOptions(again)
		if len(optsAgain) != 1 || optsAgain[0]["dict_code"] != "org_type" {
			t.Fatalf("optsAgain=%v", optsAgain)
		}
	})

	t.Run("dict without options falls back to default config", func(t *testing.T) {
		def := orgUnitFieldDefinition{
			FieldKey:         "x",
			DataSourceType:   "DICT",
			DataSourceConfig: map[string]any{"dict_code": "x"},
		}
		got := orgUnitFieldDataSourceConfigOptions(def)
		if len(got) != 1 || got[0]["dict_code"] != "x" {
			t.Fatalf("got=%v", got)
		}
		// Returned config should be a copy.
		got[0]["dict_code"] = "mutated"
		if def.DataSourceConfig["dict_code"] != "x" {
			t.Fatalf("def mutated=%v", def.DataSourceConfig["dict_code"])
		}
	})
}
