package server

import (
	"context"
	"encoding/json"
	"testing"
)

type detailsExtStoreStub struct {
	cfgs    []orgUnitTenantFieldConfig
	cfgErr  error
	snap    orgUnitVersionExtSnapshot
	snapErr error
}

func (s detailsExtStoreStub) ListEnabledTenantFieldConfigsAsOf(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
	if s.cfgErr != nil {
		return nil, s.cfgErr
	}
	return append([]orgUnitTenantFieldConfig(nil), s.cfgs...), nil
}

func (s detailsExtStoreStub) GetOrgUnitVersionExtSnapshot(_ context.Context, _ string, _ int, _ string) (orgUnitVersionExtSnapshot, error) {
	if s.snapErr != nil {
		return orgUnitVersionExtSnapshot{}, s.snapErr
	}
	return s.snap, nil
}

func TestBuildOrgUnitDetailsExtFields_EmptyConfigs(t *testing.T) {
	items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{}, "t1", 10000001, "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty, got %d", len(items))
	}
}

func TestBuildOrgUnitDetailsExtFields_PlainAndDict_DisplayValueSources(t *testing.T) {
	baseCfgs := []orgUnitTenantFieldConfig{
		{FieldKey: "short_name", ValueType: "text", DataSourceType: "PLAIN", PhysicalCol: "ext_str_02"},
		{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`), PhysicalCol: "ext_str_01"},
	}

	t.Run("versions_snapshot", func(t *testing.T) {
		items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: baseCfgs,
			snap: orgUnitVersionExtSnapshot{
				VersionValues: map[string]any{
					"ext_str_01": "DEPARTMENT",
					"ext_str_02": "R&D",
				},
				VersionLabels: map[string]string{"org_type": "Department (v)"},
				EventLabels:   map[string]string{},
			},
		}, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Fatalf("len=%d", len(items))
		}
		if items[0].FieldKey != "org_type" || items[1].FieldKey != "short_name" {
			t.Fatalf("unexpected order: %q then %q", items[0].FieldKey, items[1].FieldKey)
		}
		if items[0].DisplayValueSource != "versions_snapshot" || items[0].DisplayValue == nil || *items[0].DisplayValue != "Department (v)" {
			t.Fatalf("dict display=%v source=%q", items[0].DisplayValue, items[0].DisplayValueSource)
		}
		if items[1].DisplayValueSource != "plain" || items[1].DisplayValue == nil || *items[1].DisplayValue != "R&D" {
			t.Fatalf("plain display=%v source=%q", items[1].DisplayValue, items[1].DisplayValueSource)
		}
	})

	t.Run("events_snapshot", func(t *testing.T) {
		items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: baseCfgs,
			snap: orgUnitVersionExtSnapshot{
				VersionValues: map[string]any{
					"ext_str_01": "DEPARTMENT",
					"ext_str_02": "R&D",
				},
				VersionLabels: map[string]string{},
				EventLabels:   map[string]string{"org_type": "Department (e)"},
			},
		}, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if items[0].DisplayValueSource != "events_snapshot" || items[0].DisplayValue == nil || *items[0].DisplayValue != "Department (e)" {
			t.Fatalf("dict display=%v source=%q", items[0].DisplayValue, items[0].DisplayValueSource)
		}
	})

	t.Run("dict_fallback", func(t *testing.T) {
		items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: baseCfgs,
			snap: orgUnitVersionExtSnapshot{
				VersionValues: map[string]any{
					"ext_str_01": "DEPARTMENT",
					"ext_str_02": nil,
				},
				VersionLabels: map[string]string{},
				EventLabels:   map[string]string{},
			},
		}, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if items[0].DisplayValueSource != "dict_fallback" || items[0].DisplayValue == nil || *items[0].DisplayValue != "Department" {
			t.Fatalf("dict display=%v source=%q", items[0].DisplayValue, items[0].DisplayValueSource)
		}
		if items[1].DisplayValueSource != "plain" || items[1].DisplayValue != nil {
			t.Fatalf("plain display=%v source=%q", items[1].DisplayValue, items[1].DisplayValueSource)
		}
	})

	t.Run("unresolved", func(t *testing.T) {
		items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: baseCfgs,
			snap: orgUnitVersionExtSnapshot{
				VersionValues: map[string]any{
					"ext_str_01": "UNKNOWN",
					"ext_str_02": "R&D",
				},
				VersionLabels: map[string]string{},
				EventLabels:   map[string]string{},
			},
		}, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if items[0].DisplayValueSource != "unresolved" || items[0].DisplayValue != nil {
			t.Fatalf("dict display=%v source=%q", items[0].DisplayValue, items[0].DisplayValueSource)
		}
	})
}

func TestBuildOrgUnitDetailsExtFields_ErrorAndEdgeBranches(t *testing.T) {
	t.Run("cfg error", func(t *testing.T) {
		_, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{cfgErr: errBoom{}}, "t1", 10000001, "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("snapshot error", func(t *testing.T) {
		_, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs:    []orgUnitTenantFieldConfig{{FieldKey: "short_name", PhysicalCol: "ext_str_01"}},
			snapErr: errBoom{},
		}, "t1", 10000001, "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("blank field key is skipped", func(t *testing.T) {
		items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: []orgUnitTenantFieldConfig{
				{FieldKey: "", PhysicalCol: "ext_str_01"},
				{FieldKey: "short_name", PhysicalCol: "ext_str_02"},
			},
			snap: orgUnitVersionExtSnapshot{
				VersionValues: map[string]any{"ext_str_02": "X"},
				VersionLabels: map[string]string{},
				EventLabels:   map[string]string{},
			},
		}, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].FieldKey != "short_name" {
			t.Fatalf("items=%v", items)
		}
	})

	t.Run("unknown definition fails closed", func(t *testing.T) {
		_, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: []orgUnitTenantFieldConfig{{FieldKey: "missing", PhysicalCol: "ext_str_01"}},
			snap: orgUnitVersionExtSnapshot{},
		}, "t1", 10000001, "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nil VersionValues does not panic", func(t *testing.T) {
		items, err := buildOrgUnitDetailsExtFields(context.Background(), detailsExtStoreStub{
			cfgs: []orgUnitTenantFieldConfig{{FieldKey: "short_name", PhysicalCol: "ext_str_01"}},
			snap: orgUnitVersionExtSnapshot{
				VersionValues: nil,
				VersionLabels: map[string]string{},
				EventLabels:   map[string]string{},
			},
		}, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Value != nil {
			t.Fatalf("items=%v", items)
		}
	})
}

func TestResolveOrgUnitExtDisplayValue_CustomDefinitions(t *testing.T) {
	t.Run("dict missing dict_code", func(t *testing.T) {
		def := orgUnitFieldDefinition{FieldKey: "x"}
		cfg := orgUnitTenantFieldConfig{DataSourceConfig: json.RawMessage(`{}`)}
		_, source := resolveOrgUnitExtDisplayValue(def, cfg, "text", "DICT", "ANY", orgUnitVersionExtSnapshot{})
		if source != "unresolved" {
			t.Fatalf("source=%q", source)
		}
	})

	t.Run("dict value nil", func(t *testing.T) {
		def := orgUnitFieldDefinition{FieldKey: "x"}
		cfg := orgUnitTenantFieldConfig{DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}
		_, source := resolveOrgUnitExtDisplayValue(def, cfg, "text", "DICT", nil, orgUnitVersionExtSnapshot{})
		if source != "unresolved" {
			t.Fatalf("source=%q", source)
		}
	})

	t.Run("dict non-string value uses fmt.Sprint", func(t *testing.T) {
		def := orgUnitFieldDefinition{
			FieldKey: "x",
		}
		cfg := orgUnitTenantFieldConfig{DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}
		got, source := resolveOrgUnitExtDisplayValue(def, cfg, "text", "DICT", json.Number("DEPARTMENT"), orgUnitVersionExtSnapshot{})
		if source != "dict_fallback" || got == nil || *got != "Department" {
			t.Fatalf("display=%v source=%q", got, source)
		}
	})

	t.Run("entity is unresolved", func(t *testing.T) {
		def := orgUnitFieldDefinition{FieldKey: "x"}
		_, source := resolveOrgUnitExtDisplayValue(def, orgUnitTenantFieldConfig{}, "text", "ENTITY", "ANY", orgUnitVersionExtSnapshot{})
		if source != "unresolved" {
			t.Fatalf("source=%q", source)
		}
	})
}

func TestFormatOrgUnitPlainDisplayValue_AllTypes(t *testing.T) {
	if got := formatOrgUnitPlainDisplayValue("text", "x"); got != "x" {
		t.Fatalf("text=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("text", 1); got != "1" {
		t.Fatalf("text-nonstring=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("uuid", "u1"); got != "u1" {
		t.Fatalf("uuid=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("date", "2026-01-01"); got != "2026-01-01" {
		t.Fatalf("date=%q", got)
	}

	if got := formatOrgUnitPlainDisplayValue("int", 12); got != "12" {
		t.Fatalf("int=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("int", int32(15)); got != "15" {
		t.Fatalf("int32=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("int", int64(13)); got != "13" {
		t.Fatalf("int64=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("int", float64(14)); got != "14" {
		t.Fatalf("float64-int=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("int", float64(1.5)); got != "1.5" {
		t.Fatalf("float64=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("int", "16"); got != "16" {
		t.Fatalf("int-default=%q", got)
	}

	if got := formatOrgUnitPlainDisplayValue("bool", true); got != "true" {
		t.Fatalf("bool-true=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("bool", false); got != "false" {
		t.Fatalf("bool-false=%q", got)
	}
	if got := formatOrgUnitPlainDisplayValue("bool", "true"); got != "true" {
		t.Fatalf("bool-default=%q", got)
	}

	if got := formatOrgUnitPlainDisplayValue("unknown", "x"); got != "x" {
		t.Fatalf("unknown=%q", got)
	}
}
