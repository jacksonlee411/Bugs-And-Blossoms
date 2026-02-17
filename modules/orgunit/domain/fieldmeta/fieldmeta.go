package fieldmeta

import (
	"encoding/json"
	"sort"
	"strings"
)

type FieldDefinition struct {
	FieldKey                string
	ValueType               string
	DataSourceType          string
	DataSourceConfig        map[string]any
	DataSourceConfigOptions []map[string]any
	LabelI18nKey            string
	AllowFilter             bool
	AllowSort               bool
}

type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var fieldDefinitions = []FieldDefinition{
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

var fieldDefinitionByKey = func() map[string]FieldDefinition {
	out := make(map[string]FieldDefinition, len(fieldDefinitions))
	for _, def := range fieldDefinitions {
		out[def.FieldKey] = def
	}
	return out
}()

func ListFieldDefinitions() []FieldDefinition {
	out := make([]FieldDefinition, 0, len(fieldDefinitions))
	for _, def := range fieldDefinitions {
		out = append(out, cloneFieldDefinition(def))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].FieldKey < out[j].FieldKey
	})
	return out
}

func LookupFieldDefinition(fieldKey string) (FieldDefinition, bool) {
	def, ok := fieldDefinitionByKey[fieldKey]
	if !ok {
		return FieldDefinition{}, false
	}
	return cloneFieldDefinition(def), true
}

func DataSourceConfigJSON(def FieldDefinition) json.RawMessage {
	raw, err := json.Marshal(def.DataSourceConfig)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func DataSourceConfigOptions(def FieldDefinition) []map[string]any {
	dataSourceType := strings.ToUpper(strings.TrimSpace(def.DataSourceType))
	switch dataSourceType {
	case "ENTITY":
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
		return []map[string]any{cloneFieldDefinition(def).DataSourceConfig}
	default:
		return nil
	}
}

func DictCodeFromDataSourceConfig(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var cfg struct {
		DictCode string `json:"dict_code"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", false
	}
	value := strings.TrimSpace(cfg.DictCode)
	if value == "" {
		return "", false
	}
	return value, true
}

func cloneFieldDefinition(def FieldDefinition) FieldDefinition {
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
