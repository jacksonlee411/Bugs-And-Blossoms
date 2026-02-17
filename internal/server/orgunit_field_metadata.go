package server

import (
	"context"
	"encoding/json"
	"strings"

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

func isCustomOrgUnitPlainFieldKey(fieldKey string) bool {
	return fieldmeta.IsCustomPlainFieldKey(fieldKey)
}

func isAllowedOrgUnitExtFieldKey(fieldKey string) bool {
	fieldKey = strings.TrimSpace(fieldKey)
	if fieldKey == "" {
		return false
	}
	if _, ok := lookupOrgUnitFieldDefinition(fieldKey); ok {
		return true
	}
	return isCustomOrgUnitPlainFieldKey(fieldKey)
}

func buildCustomOrgUnitPlainFieldDefinition(fieldKey string) (orgUnitFieldDefinition, bool) {
	fieldKey = strings.TrimSpace(fieldKey)
	if !isCustomOrgUnitPlainFieldKey(fieldKey) {
		return orgUnitFieldDefinition{}, false
	}
	return orgUnitFieldDefinition{
		FieldKey:         fieldKey,
		ValueType:        "text",
		DataSourceType:   "PLAIN",
		DataSourceConfig: map[string]any{},
	}, true
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

func listOrgUnitDicts(ctx context.Context, store orgUnitDictRegistryStore, tenantID string, asOf string) ([]DictItem, error) {
	return store.ListDicts(ctx, tenantID, asOf)
}
