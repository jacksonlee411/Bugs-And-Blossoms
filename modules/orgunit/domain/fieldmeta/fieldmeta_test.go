package fieldmeta

import (
	"encoding/json"
	"testing"
)

func TestFieldMetadata_Definitions_ListAndLookup(t *testing.T) {
	defs := ListFieldDefinitions()
	if len(defs) == 0 {
		t.Fatalf("expected definitions")
	}
	for i := 1; i < len(defs); i++ {
		if defs[i-1].FieldKey > defs[i].FieldKey {
			t.Fatalf("not sorted: %q > %q", defs[i-1].FieldKey, defs[i].FieldKey)
		}
	}

	def, ok := LookupFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type")
	}
	if got, _ := def.DataSourceConfig["dict_code"].(string); got != "org_type" {
		t.Fatalf("dict_code=%q", got)
	}

	def.DataSourceConfig["dict_code"] = "mutated"
	again, ok := LookupFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type")
	}
	if got, _ := again.DataSourceConfig["dict_code"].(string); got != "org_type" {
		t.Fatalf("dict_code mutated=%q", got)
	}
}

func TestFieldMetadata_DataSourceConfigJSON(t *testing.T) {
	def, ok := LookupFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type")
	}
	if got := string(DataSourceConfigJSON(def)); got == "" || got == "{}" {
		t.Fatalf("unexpected json=%q", got)
	}

	bad := FieldDefinition{DataSourceConfig: map[string]any{"x": func() {}}}
	if got := string(DataSourceConfigJSON(bad)); got != "{}" {
		t.Fatalf("json=%q", got)
	}
}

func TestFieldMetadata_DataSourceConfigOptions(t *testing.T) {
	plain, ok := LookupFieldDefinition("short_name")
	if !ok {
		t.Fatalf("expected short_name")
	}
	if got := DataSourceConfigOptions(plain); got != nil {
		t.Fatalf("expected nil, got=%v", got)
	}

	dict, ok := LookupFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type")
	}
	if opts := DataSourceConfigOptions(dict); opts != nil {
		t.Fatalf("expected nil for dict, got=%v", opts)
	}

	fallback := FieldDefinition{
		FieldKey:         "x",
		DataSourceType:   "ENTITY",
		DataSourceConfig: map[string]any{"entity": "person", "id_kind": "uuid"},
	}
	fallbackOpts := DataSourceConfigOptions(fallback)
	if len(fallbackOpts) != 1 || fallbackOpts[0]["entity"] != "person" {
		t.Fatalf("fallback=%v", fallbackOpts)
	}
}

func TestFieldMetadata_cloneFieldDefinition_DeepCopyOptions(t *testing.T) {
	def := FieldDefinition{
		FieldKey:       "x",
		DataSourceType: "ENTITY",
		DataSourceConfig: map[string]any{
			"entity":  "person",
			"id_kind": "uuid",
		},
		DataSourceConfigOptions: []map[string]any{
			{"entity": "person", "id_kind": "uuid"},
		},
	}

	cloned := cloneFieldDefinition(def)
	def.DataSourceConfig["entity"] = "mutated"
	def.DataSourceConfigOptions[0]["entity"] = "mutated"

	if got, _ := cloned.DataSourceConfig["entity"].(string); got != "person" {
		t.Fatalf("config entity=%q", got)
	}
	if got, _ := cloned.DataSourceConfigOptions[0]["entity"].(string); got != "person" {
		t.Fatalf("option entity=%q", got)
	}

	empty := cloneFieldDefinition(FieldDefinition{DataSourceConfig: map[string]any{}, DataSourceConfigOptions: nil})
	if empty.DataSourceConfigOptions != nil {
		t.Fatalf("expected nil options, got=%v", empty.DataSourceConfigOptions)
	}
}

func TestFieldMetadata_IsCustomPlainFieldKey(t *testing.T) {
	if !IsCustomPlainFieldKey("x_cost_center") {
		t.Fatal("expected x_cost_center to be valid")
	}
	invalid := []string{"x_", "x-COST", "X_cost", "short_name"}
	for _, key := range invalid {
		if IsCustomPlainFieldKey(key) {
			t.Fatalf("expected %q invalid", key)
		}
	}
}

func TestFieldMetadata_IsCustomDictFieldKey(t *testing.T) {
	if !IsCustomDictFieldKey("d_org_type") {
		t.Fatal("expected d_org_type to be valid")
	}
	if IsCustomDictFieldKey("org_type") {
		t.Fatal("expected org_type to be invalid as dict field key")
	}
	invalid := []string{"d_", "d-ORG", "D_org", "d_orgType"}
	for _, key := range invalid {
		if IsCustomDictFieldKey(key) {
			t.Fatalf("expected %q invalid", key)
		}
	}

	got, ok := DictCodeFromDictFieldKey("d_org_type")
	if !ok || got != "org_type" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
	if _, ok := DictCodeFromDictFieldKey("org_type"); ok {
		t.Fatal("expected non-dict field key to fail")
	}
}

func TestFieldMetadata_DictCodeFromDataSourceConfig(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"dict_code": " org_type "})
	got, ok := DictCodeFromDataSourceConfig(raw)
	if !ok || got != "org_type" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}

	if _, ok := DictCodeFromDataSourceConfig(nil); ok {
		t.Fatalf("expected nil raw to fail")
	}
	if _, ok := DictCodeFromDataSourceConfig(json.RawMessage(`{"dict_code":""}`)); ok {
		t.Fatalf("expected empty dict_code to fail")
	}
	if _, ok := DictCodeFromDataSourceConfig(json.RawMessage(`{`)); ok {
		t.Fatalf("expected bad json to fail")
	}
}
