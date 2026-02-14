package server

import (
	"encoding/json"
	"sort"
	"strings"
)

type orgUnitFieldDefinition struct {
	FieldKey                string
	ValueType               string
	DataSourceType          string
	DataSourceConfig        map[string]any
	DataSourceConfigOptions []map[string]any
	LabelI18nKey            string
	AllowFilter             bool
	AllowSort               bool
}

type orgUnitFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var orgUnitFieldDefinitions = []orgUnitFieldDefinition{
	{
		FieldKey:         "short_name",
		ValueType:        "text",
		DataSourceType:   "PLAIN",
		DataSourceConfig: map[string]any{},
		LabelI18nKey:     "org.fields.short_name",
	},
	{
		FieldKey:         "description",
		ValueType:        "text",
		DataSourceType:   "PLAIN",
		DataSourceConfig: map[string]any{},
		LabelI18nKey:     "org.fields.description",
	},
	{
		FieldKey:       "org_type",
		ValueType:      "text",
		DataSourceType: "DICT",
		DataSourceConfig: map[string]any{
			"dict_code": "org_type",
		},
		DataSourceConfigOptions: []map[string]any{
			{"dict_code": "org_type"},
		},
		LabelI18nKey: "org.fields.org_type",
		AllowFilter:  true,
		AllowSort:    true,
	},
	{
		FieldKey:         "location_code",
		ValueType:        "text",
		DataSourceType:   "PLAIN",
		DataSourceConfig: map[string]any{},
		LabelI18nKey:     "org.fields.location_code",
	},
	{
		FieldKey:         "cost_center",
		ValueType:        "text",
		DataSourceType:   "PLAIN",
		DataSourceConfig: map[string]any{},
		LabelI18nKey:     "org.fields.cost_center",
	},
}

var orgUnitDictOptionsRegistry = map[string][]orgUnitFieldOption{
	"org_type": {
		{Value: "BUSINESS_UNIT", Label: "Business Unit"},
		{Value: "COMPANY", Label: "Company"},
		{Value: "COST_CENTER", Label: "Cost Center"},
		{Value: "DEPARTMENT", Label: "Department"},
		{Value: "LOCATION", Label: "Location"},
	},
}

var orgUnitFieldDefinitionByKey = func() map[string]orgUnitFieldDefinition {
	out := make(map[string]orgUnitFieldDefinition, len(orgUnitFieldDefinitions))
	for _, def := range orgUnitFieldDefinitions {
		out[def.FieldKey] = def
	}
	return out
}()

func listOrgUnitFieldDefinitions() []orgUnitFieldDefinition {
	out := make([]orgUnitFieldDefinition, 0, len(orgUnitFieldDefinitions))
	for _, def := range orgUnitFieldDefinitions {
		out = append(out, cloneOrgUnitFieldDefinition(def))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].FieldKey < out[j].FieldKey
	})
	return out
}

func lookupOrgUnitFieldDefinition(fieldKey string) (orgUnitFieldDefinition, bool) {
	def, ok := orgUnitFieldDefinitionByKey[fieldKey]
	if !ok {
		return orgUnitFieldDefinition{}, false
	}
	return cloneOrgUnitFieldDefinition(def), true
}

func cloneOrgUnitFieldDefinition(def orgUnitFieldDefinition) orgUnitFieldDefinition {
	copiedConfig := make(map[string]any, len(def.DataSourceConfig))
	for key, value := range def.DataSourceConfig {
		copiedConfig[key] = value
	}
	def.DataSourceConfig = copiedConfig

	if len(def.DataSourceConfigOptions) > 0 {
		copiedOptions := make([]map[string]any, 0, len(def.DataSourceConfigOptions))
		for _, opt := range def.DataSourceConfigOptions {
			copiedOpt := make(map[string]any, len(opt))
			for key, value := range opt {
				copiedOpt[key] = value
			}
			copiedOptions = append(copiedOptions, copiedOpt)
		}
		def.DataSourceConfigOptions = copiedOptions
	} else {
		def.DataSourceConfigOptions = nil
	}
	return def
}

func orgUnitFieldDataSourceConfigJSON(def orgUnitFieldDefinition) json.RawMessage {
	raw, err := json.Marshal(def.DataSourceConfig)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func orgUnitFieldDataSourceConfigOptions(def orgUnitFieldDefinition) []map[string]any {
	dataSourceType := strings.ToUpper(strings.TrimSpace(def.DataSourceType))
	switch dataSourceType {
	case "DICT", "ENTITY":
		if len(def.DataSourceConfigOptions) > 0 {
			out := make([]map[string]any, 0, len(def.DataSourceConfigOptions))
			for _, opt := range def.DataSourceConfigOptions {
				copied := make(map[string]any, len(opt))
				for key, value := range opt {
					copied[key] = value
				}
				out = append(out, copied)
			}
			return out
		}
		return []map[string]any{cloneOrgUnitFieldDefinition(def).DataSourceConfig}
	default:
		return nil
	}
}

func listOrgUnitDictOptions(dictCode string, keyword string, limit int) []orgUnitFieldOption {
	items := append([]orgUnitFieldOption(nil), orgUnitDictOptionsRegistry[dictCode]...)
	if len(items) == 0 {
		return []orgUnitFieldOption{}
	}
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle != "" {
		filtered := make([]orgUnitFieldOption, 0, len(items))
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Label), needle) || strings.Contains(strings.ToLower(item.Value), needle) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Label == items[j].Label {
			return items[i].Value < items[j].Value
		}
		return items[i].Label < items[j].Label
	})
	if limit < 0 {
		limit = 0
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func lookupOrgUnitDictLabel(dictCode string, value string) (string, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", false
	}
	for _, item := range orgUnitDictOptionsRegistry[dictCode] {
		if strings.EqualFold(item.Value, candidate) {
			return item.Label, true
		}
	}
	return "", false
}

func lookupOrgUnitDictOption(dictCode string, value string) (orgUnitFieldOption, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return orgUnitFieldOption{}, false
	}
	for _, item := range orgUnitDictOptionsRegistry[dictCode] {
		if strings.EqualFold(item.Value, candidate) {
			return item, true
		}
	}
	return orgUnitFieldOption{}, false
}
