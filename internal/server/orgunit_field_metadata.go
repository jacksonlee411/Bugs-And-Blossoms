package server

import (
	"encoding/json"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/fieldmeta"
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

func listOrgUnitDictOptions(dictCode string, keyword string, limit int) []orgUnitFieldOption {
	return fieldmeta.ListDictOptions(dictCode, keyword, limit)
}

func lookupOrgUnitDictLabel(dictCode string, value string) (string, bool) {
	return fieldmeta.LookupDictLabel(dictCode, value)
}

func lookupOrgUnitDictOption(dictCode string, value string) (orgUnitFieldOption, bool) {
	return fieldmeta.LookupDictOption(dictCode, value)
}
