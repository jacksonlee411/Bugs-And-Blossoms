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
	opts := DataSourceConfigOptions(dict)
	if len(opts) != 1 || opts[0]["dict_code"] != "org_type" {
		t.Fatalf("opts=%v", opts)
	}

	fallback := FieldDefinition{
		FieldKey:         "x",
		DataSourceType:   "DICT",
		DataSourceConfig: map[string]any{"dict_code": "x"},
	}
	fallbackOpts := DataSourceConfigOptions(fallback)
	if len(fallbackOpts) != 1 || fallbackOpts[0]["dict_code"] != "x" {
		t.Fatalf("fallback=%v", fallbackOpts)
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
