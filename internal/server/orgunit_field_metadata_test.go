package server

import (
	"context"
	"errors"
	"testing"

	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

type orgunitDictResolverStub struct {
	resolveFn func(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error)
	listFn    func(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error)
}

func (s orgunitDictResolverStub) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, tenantID, asOf, dictCode, code)
	}
	return "", false, nil
}

func (s orgunitDictResolverStub) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, asOf, dictCode, keyword, limit)
	}
	return nil, nil
}

func TestOrgUnitFieldMetadata_Definitions_ListAndLookup(t *testing.T) {
	defs := listOrgUnitFieldDefinitions()
	if len(defs) == 0 {
		t.Fatalf("expected field definitions")
	}
	for i := 1; i < len(defs); i++ {
		if defs[i-1].FieldKey > defs[i].FieldKey {
			t.Fatalf("not sorted")
		}
	}
	def, ok := lookupOrgUnitFieldDefinition("org_type")
	if !ok {
		t.Fatalf("expected org_type")
	}
	def.DataSourceConfig["dict_code"] = "changed"
	again, ok := lookupOrgUnitFieldDefinition("org_type")
	if !ok || again.DataSourceConfig["dict_code"] != "org_type" {
		t.Fatalf("unexpected again=%v", again.DataSourceConfig["dict_code"])
	}
}

func TestOrgUnitFieldMetadata_DictIntegration(t *testing.T) {
	if err := dictpkg.RegisterResolver(orgunitDictResolverStub{
		listFn: func(_ context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
			if tenantID != "t1" || asOf != "2026-01-01" || dictCode != "org_type" || keyword != "dep" || limit != 10 {
				t.Fatalf("unexpected args")
			}
			return []dictpkg.Option{{Code: "10", Label: "部门"}}, nil
		},
		resolveFn: func(_ context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
			if tenantID != "t1" || asOf != "2026-01-01" || dictCode != "org_type" || code != "10" {
				t.Fatalf("unexpected args")
			}
			return "部门", true, nil
		},
	}); err != nil {
		t.Fatalf("register err=%v", err)
	}

	options, err := listOrgUnitDictOptions(context.Background(), "t1", "2026-01-01", "org_type", "dep", 10)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(options) != 1 || options[0].Value != "10" || options[0].Label != "部门" {
		t.Fatalf("options=%+v", options)
	}
	label, ok, err := resolveOrgUnitDictLabel(context.Background(), "t1", "2026-01-01", "org_type", "10")
	if err != nil || !ok || label != "部门" {
		t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
	}
}

func TestOrgUnitFieldMetadata_DictIntegrationError(t *testing.T) {
	wantErr := errors.New("boom")
	if err := dictpkg.RegisterResolver(orgunitDictResolverStub{
		listFn: func(context.Context, string, string, string, string, int) ([]dictpkg.Option, error) {
			return nil, wantErr
		},
		resolveFn: func(context.Context, string, string, string, string) (string, bool, error) {
			return "", false, wantErr
		},
	}); err != nil {
		t.Fatalf("register err=%v", err)
	}

	if _, err := listOrgUnitDictOptions(context.Background(), "t1", "2026-01-01", "org_type", "", 10); !errors.Is(err, wantErr) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := resolveOrgUnitDictLabel(context.Background(), "t1", "2026-01-01", "org_type", "10"); !errors.Is(err, wantErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitFieldMetadata_ExtHelpers(t *testing.T) {
	if !isAllowedOrgUnitExtFieldKey("org_type") {
		t.Fatal("expected builtin key allowed")
	}
	if !isAllowedOrgUnitExtFieldKey("x_cost_center") {
		t.Fatal("expected custom x_ key allowed")
	}
	if isAllowedOrgUnitExtFieldKey("unknown_field") {
		t.Fatal("expected unknown key rejected")
	}

	if _, ok := resolveOrgUnitEnableDefinition("x_cost_center"); !ok {
		t.Fatal("expected custom definition to resolve")
	}
	if _, ok := resolveOrgUnitEnableDefinition("short_name"); !ok {
		t.Fatal("expected builtin definition to resolve")
	}
	if _, ok := resolveOrgUnitEnableDefinition("unknown_field"); ok {
		t.Fatal("expected unknown definition to fail")
	}
}
