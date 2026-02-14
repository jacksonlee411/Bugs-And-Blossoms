package server

import (
	"context"
	"errors"
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

		def, ok := lookupOrgUnitFieldDefinition(fieldKey)
		if !ok {
			return nil, errors.New("orgunit field definition not found")
		}

		valueType := strings.TrimSpace(cfg.ValueType)
		if valueType == "" {
			valueType = def.ValueType
		}
		dataSourceType := strings.TrimSpace(cfg.DataSourceType)
		if dataSourceType == "" {
			dataSourceType = def.DataSourceType
		}

		var value any
		if snapshot.VersionValues != nil && strings.TrimSpace(cfg.PhysicalCol) != "" {
			value = snapshot.VersionValues[cfg.PhysicalCol]
		}

		displayValue, source := resolveOrgUnitExtDisplayValue(def, cfg, valueType, dataSourceType, value, snapshot)

		items = append(items, orgUnitExtFieldAPIItem{
			FieldKey:           fieldKey,
			LabelI18nKey:       def.LabelI18nKey,
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

func resolveOrgUnitExtDisplayValue(def orgUnitFieldDefinition, cfg orgUnitTenantFieldConfig, valueType string, dataSourceType string, value any, snapshot orgUnitVersionExtSnapshot) (*string, string) {
	dataSourceType = strings.ToUpper(strings.TrimSpace(dataSourceType))
	switch dataSourceType {
	case "PLAIN":
		if value == nil {
			return nil, "plain"
		}
		text := formatOrgUnitPlainDisplayValue(valueType, value)
		return &text, "plain"
	case "DICT":
		if label := strings.TrimSpace(snapshot.VersionLabels[def.FieldKey]); label != "" {
			return &label, "versions_snapshot"
		}
		if label := strings.TrimSpace(snapshot.EventLabels[def.FieldKey]); label != "" {
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
		if label, ok := lookupOrgUnitDictLabel(dictCode, code); ok {
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
	case "text", "uuid", "date":
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
