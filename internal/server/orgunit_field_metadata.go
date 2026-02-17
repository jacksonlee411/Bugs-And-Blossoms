package server

import (
	"context"
	"encoding/json"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/fieldmeta"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

// NOTE: This file intentionally keeps server-local helpers as thin wrappers.
// SSOT for field definitions / DICT registry lives in modules/orgunit/domain/fieldmeta.

type orgUnitFieldDefinition = fieldmeta.FieldDefinition
type orgUnitFieldOption = fieldmeta.FieldOption

func listOrgUnitFieldDefinitions() []orgUnitFieldDefinition {
	return fieldmeta.ListFieldDefinitions()
}

func lookupOrgUnitFieldDefinition(fieldKey string) (orgUnitFieldDefinition, bool) {
	return fieldmeta.LookupFieldDefinition(fieldKey)
}

func orgUnitFieldDataSourceConfigJSON(def orgUnitFieldDefinition) json.RawMessage {
	return fieldmeta.DataSourceConfigJSON(def)
}

func orgUnitFieldDataSourceConfigOptions(def orgUnitFieldDefinition) []map[string]any {
	return fieldmeta.DataSourceConfigOptions(def)
}

func listOrgUnitDictOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]orgUnitFieldOption, error) {
	options, err := dictpkg.ListOptions(ctx, tenantID, asOf, dictCode, keyword, limit)
	if err != nil {
		return nil, err
	}
	out := make([]orgUnitFieldOption, 0, len(options))
	for _, option := range options {
		out = append(out, orgUnitFieldOption{Value: option.Code, Label: option.Label})
	}
	return out, nil
}

func resolveOrgUnitDictLabel(ctx context.Context, tenantID string, asOf string, dictCode string, value string) (string, bool, error) {
	return dictpkg.ResolveValueLabel(ctx, tenantID, asOf, dictCode, value)
}
