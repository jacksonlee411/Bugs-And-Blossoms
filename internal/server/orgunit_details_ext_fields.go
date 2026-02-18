package server

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type orgUnitDetailsExtFieldStore interface {
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	GetOrgUnitVersionExtSnapshot(ctx context.Context, tenantID string, orgID int, asOf string) (orgUnitVersionExtSnapshot, error)
}

func buildOrgUnitDetailsExtFields(ctx context.Context, store orgUnitDetailsExtFieldStore, tenantID string, orgID int, asOf string) ([]orgUnitExtFieldAPIItem, error) {
	cfgs, err := store.ListEnabledTenantFieldConfigsAsOf(ctx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	if len(cfgs) == 0 {
		return []orgUnitExtFieldAPIItem{}, nil
	}

	snapshot, err := store.GetOrgUnitVersionExtSnapshot(ctx, tenantID, orgID, asOf)
	if err != nil {
		return nil, err
	}

	items := make([]orgUnitExtFieldAPIItem, 0, len(cfgs))
	for _, cfg := range cfgs {
		fieldKey := strings.TrimSpace(cfg.FieldKey)
		if fieldKey == "" {
			continue
		}

		valueType := strings.TrimSpace(cfg.ValueType)
		dataSourceType := strings.TrimSpace(cfg.DataSourceType)
		if valueType == "" || dataSourceType == "" {
			if def, ok := lookupOrgUnitFieldDefinition(fieldKey); ok {
				if valueType == "" {
					valueType = def.ValueType
				}
				if dataSourceType == "" {
					dataSourceType = def.DataSourceType
				}
			}
		}
		if valueType == "" {
			valueType = "text"
		}
		if dataSourceType == "" {
			dataSourceType = "PLAIN"
		}

		labelI18nKey, label := resolveOrgUnitExtFieldLabel(cfg)

		var value any
		if snapshot.VersionValues != nil && strings.TrimSpace(cfg.PhysicalCol) != "" {
			value = snapshot.VersionValues[cfg.PhysicalCol]
		}

		displayValue, source := resolveOrgUnitExtDisplayValue(ctx, tenantID, asOf, fieldKey, cfg, valueType, dataSourceType, value, snapshot)

		items = append(items, orgUnitExtFieldAPIItem{
			FieldKey:           fieldKey,
			LabelI18nKey:       labelI18nKey,
			Label:              label,
			ValueType:          valueType,
			DataSourceType:     dataSourceType,
			Value:              value,
			DisplayValue:       displayValue,
			DisplayValueSource: source,
		})
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].FieldKey < items[j].FieldKey })
	return items, nil
}

func resolveOrgUnitExtFieldLabel(cfg orgUnitTenantFieldConfig) (*string, *string) {
	fieldKey := strings.TrimSpace(cfg.FieldKey)
	def, ok := lookupOrgUnitFieldDefinition(fieldKey)
	if ok {
		labelKey := strings.TrimSpace(def.LabelI18nKey)
		if labelKey != "" {
			return &labelKey, nil
		}
	}
	if isCustomOrgUnitDictFieldKey(fieldKey) && cfg.DisplayLabel != nil {
		label := strings.TrimSpace(*cfg.DisplayLabel)
		if label != "" {
			return nil, &label
		}
	}
	label := fieldKey
	return nil, &label
}

func resolveOrgUnitExtDisplayValue(ctx context.Context, tenantID string, asOf string, fieldKey string, cfg orgUnitTenantFieldConfig, valueType string, dataSourceType string, value any, snapshot orgUnitVersionExtSnapshot) (*string, string) {
	dataSourceType = strings.ToUpper(strings.TrimSpace(dataSourceType))
	switch dataSourceType {
	case "PLAIN":
		if value == nil {
			return nil, "plain"
		}
		text := formatOrgUnitPlainDisplayValue(valueType, value)
		return &text, "plain"
	case "DICT":
		if label := strings.TrimSpace(snapshot.VersionLabels[fieldKey]); label != "" {
			return &label, "versions_snapshot"
		}
		if label := strings.TrimSpace(snapshot.EventLabels[fieldKey]); label != "" {
			return &label, "events_snapshot"
		}

		dictCode, ok := dictCodeFromDataSourceConfig(cfg.DataSourceConfig)
		if !ok || value == nil {
			return nil, "unresolved"
		}

		code := ""
		if s, ok := value.(string); ok {
			code = s
		} else {
			code = fmt.Sprint(value)
		}
		label, ok, err := resolveOrgUnitDictLabel(ctx, tenantID, asOf, dictCode, code)
		if err == nil && ok {
			return &label, "dict_fallback"
		}
		return nil, "unresolved"
	case "ENTITY":
		fallthrough
	default:
		return nil, "unresolved"
	}
}

func formatOrgUnitPlainDisplayValue(valueType string, value any) string {
	valueType = strings.ToLower(strings.TrimSpace(valueType))
	switch valueType {
	case "text", "uuid", "date", "numeric":
		if s, ok := value.(string); ok {
			return s
		}
		return fmt.Sprint(value)
	case "int":
		switch v := value.(type) {
		case int:
			return strconv.Itoa(v)
		case int32:
			return strconv.FormatInt(int64(v), 10)
		case int64:
			return strconv.FormatInt(v, 10)
		case float64:
			if v == float64(int64(v)) {
				return strconv.FormatInt(int64(v), 10)
			}
			return strconv.FormatFloat(v, 'f', -1, 64)
		default:
			return fmt.Sprint(value)
		}
	case "bool":
		if b, ok := value.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
		return fmt.Sprint(value)
	default:
		return fmt.Sprint(value)
	}
}
