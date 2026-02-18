package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type orgUnitFieldDefinitionsAPIResponse struct {
	Fields []orgUnitFieldDefinitionAPIItem `json:"fields"`
}

type orgUnitFieldDefinitionAPIItem struct {
	FieldKey                string            `json:"field_key"`
	ValueType               string            `json:"value_type"`
	DataSourceType          string            `json:"data_source_type"`
	DataSourceConfig        json.RawMessage   `json:"data_source_config"`
	DataSourceConfigOptions []json.RawMessage `json:"data_source_config_options,omitempty"`
	LabelI18nKey            string            `json:"label_i18n_key"`
	AllowFilter             bool              `json:"allow_filter"`
	AllowSort               bool              `json:"allow_sort"`
}

func handleOrgUnitFieldDefinitionsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if _, ok := currentTenant(r.Context()); !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	defs := listOrgUnitFieldDefinitions()
	items := make([]orgUnitFieldDefinitionAPIItem, 0, len(defs))
	for _, def := range defs {
		item := orgUnitFieldDefinitionAPIItem{
			FieldKey:         def.FieldKey,
			ValueType:        def.ValueType,
			DataSourceType:   def.DataSourceType,
			DataSourceConfig: orgUnitFieldDataSourceConfigJSON(def),
			LabelI18nKey:     def.LabelI18nKey,
			AllowFilter:      def.AllowFilter,
			AllowSort:        def.AllowSort,
		}
		item.DataSourceConfigOptions = orgUnitFieldDataSourceConfigOptionsJSON(def)
		items = append(items, item)
	}
	// Ensure stable output even if listOrgUnitFieldDefinitions changes.
	sort.SliceStable(items, func(i, j int) bool { return items[i].FieldKey < items[j].FieldKey })

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(orgUnitFieldDefinitionsAPIResponse{Fields: items})
}

type orgUnitFieldConfigsAPIResponse struct {
	AsOf         string                      `json:"as_of"`
	FieldConfigs []orgUnitFieldConfigAPIItem `json:"field_configs"`
}

type orgUnitFieldConfigAPIItem struct {
	FieldKey         string          `json:"field_key"`
	LabelI18nKey     *string         `json:"label_i18n_key"`
	Label            *string         `json:"label,omitempty"`
	ValueType        string          `json:"value_type"`
	DataSourceType   string          `json:"data_source_type"`
	DataSourceConfig json.RawMessage `json:"data_source_config"`
	PhysicalCol      string          `json:"physical_col"`
	EnabledOn        string          `json:"enabled_on"`
	DisabledOn       *string         `json:"disabled_on"`
	UpdatedAt        time.Time       `json:"updated_at"`
	AllowFilter      bool            `json:"allow_filter"`
	AllowSort        bool            `json:"allow_sort"`
}

type orgUnitFieldConfigsEnableRequest struct {
	FieldKey         string          `json:"field_key"`
	EnabledOn        string          `json:"enabled_on"`
	RequestCode      string          `json:"request_code"`
	ValueType        string          `json:"value_type"`
	Label            string          `json:"label"`
	DataSourceConfig json.RawMessage `json:"data_source_config"`
}

type orgUnitFieldConfigsDisableRequest struct {
	FieldKey    string `json:"field_key"`
	DisabledOn  string `json:"disabled_on"`
	RequestCode string `json:"request_code"`
}

type orgUnitFieldConfigStore interface {
	ListTenantFieldConfigs(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error)
	EnableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, valueType string, dataSourceType string, dataSourceConfig json.RawMessage, displayLabel *string, enabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
	DisableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, disabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
}

type orgUnitDictRegistryStore interface {
	ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error)
}

type orgUnitFieldConfigsEnableCandidatesAPIResponse struct {
	EnabledOn       string                             `json:"enabled_on"`
	DictFields      []orgUnitFieldEnableCandidateField `json:"dict_fields"`
	PlainCustomHint orgUnitPlainCustomHint             `json:"plain_custom_hint"`
}

type orgUnitFieldEnableCandidateField struct {
	FieldKey       string `json:"field_key"`
	DictCode       string `json:"dict_code"`
	Name           string `json:"name"`
	ValueType      string `json:"value_type"`
	DataSourceType string `json:"data_source_type"`
}

type orgUnitPlainCustomHint struct {
	Pattern          string   `json:"pattern"`
	ValueTypes       []string `json:"value_types"`
	DefaultValueType string   `json:"default_value_type"`
}

func handleOrgUnitFieldConfigsEnableCandidatesAPI(w http.ResponseWriter, r *http.Request, dictStore orgUnitDictRegistryStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	enabledOn := strings.TrimSpace(r.URL.Query().Get("enabled_on"))
	if enabledOn == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "enabled_on required")
		return
	}
	if _, err := time.Parse("2006-01-02", enabledOn); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "enabled_on invalid")
		return
	}

	if dictStore == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "dict_store_missing", "dict store missing")
		return
	}

	dicts, err := listOrgUnitDicts(r.Context(), dictStore, tenant.ID, enabledOn)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_field_enable_candidates_failed")
		return
	}

	items := make([]orgUnitFieldEnableCandidateField, 0, len(dicts))
	for _, d := range dicts {
		code := strings.TrimSpace(d.DictCode)
		if code == "" {
			continue
		}
		// Contract (DEV-PLAN-106A): field_key length must satisfy tenant_field_configs.field_key check.
		if len(code) > 61 {
			continue
		}
		fieldKey := "d_" + code
		if !isCustomOrgUnitDictFieldKey(fieldKey) {
			continue
		}
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = code
		}

		items = append(items, orgUnitFieldEnableCandidateField{
			FieldKey:       fieldKey,
			DictCode:       code,
			Name:           name,
			ValueType:      "text",
			DataSourceType: "DICT",
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].FieldKey < items[j].FieldKey })

	resp := orgUnitFieldConfigsEnableCandidatesAPIResponse{
		EnabledOn:  enabledOn,
		DictFields: items,
		PlainCustomHint: orgUnitPlainCustomHint{
			Pattern:          "^x_[a-z0-9_]{1,60}$",
			ValueTypes:       orgUnitCustomPlainValueTypes(),
			DefaultValueType: "text",
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleOrgUnitFieldConfigsAPI handles list/enable.
func handleOrgUnitFieldConfigsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, dictStore orgUnitDictRegistryStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	cfgStore, ok := store.(orgUnitFieldConfigStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		asOf, err := orgUnitAPIAsOf(r)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
			return
		}

		status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
		switch status {
		case "", "all", "enabled", "disabled":
			// ok
		default:
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "status invalid")
			return
		}

		rows, err := cfgStore.ListTenantFieldConfigs(r.Context(), tenant.ID)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_field_configs_list_failed")
			return
		}

		items := make([]orgUnitFieldConfigAPIItem, 0, len(rows))
		for _, row := range rows {
			enabled := orgUnitFieldConfigEnabledAsOf(row, asOf)
			if status == "enabled" && !enabled {
				continue
			}
			if status == "disabled" && enabled {
				continue
			}

			labelI18nKey, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(row)

			items = append(items, orgUnitFieldConfigAPIItem{
				FieldKey:         row.FieldKey,
				LabelI18nKey:     labelI18nKey,
				Label:            label,
				ValueType:        row.ValueType,
				DataSourceType:   row.DataSourceType,
				DataSourceConfig: row.DataSourceConfig,
				PhysicalCol:      row.PhysicalCol,
				EnabledOn:        row.EnabledOn,
				DisabledOn:       row.DisabledOn,
				UpdatedAt:        row.UpdatedAt,
				AllowFilter:      allowFilter,
				AllowSort:        allowSort,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(orgUnitFieldConfigsAPIResponse{AsOf: asOf, FieldConfigs: items})
		return
	case http.MethodPost:
		var req orgUnitFieldConfigsEnableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		req.FieldKey = strings.TrimSpace(req.FieldKey)
		req.EnabledOn = strings.TrimSpace(req.EnabledOn)
		req.RequestCode = strings.TrimSpace(req.RequestCode)
		req.ValueType = strings.TrimSpace(req.ValueType)
		req.Label = strings.TrimSpace(req.Label)
		if req.FieldKey == "" || req.EnabledOn == "" || req.RequestCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "field_key/enabled_on/request_code required")
			return
		}
		if _, err := time.Parse("2006-01-02", req.EnabledOn); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "enabled_on invalid")
			return
		}

		if strings.HasPrefix(strings.ToLower(req.FieldKey), "x_") && !isCustomOrgUnitPlainFieldKey(req.FieldKey) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "custom field_key invalid")
			return
		}

		if isCustomOrgUnitPlainFieldKey(req.FieldKey) {
			valueType, ok := normalizeOrgUnitCustomPlainValueType(req.ValueType)
			if !ok {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "custom value_type required")
				return
			}

			dataSourceConfig, ok, _ := normalizeOrgUnitEnableDataSourceConfig(
				r.Context(),
				tenant.ID,
				req.EnabledOn,
				dictStore,
				orgUnitFieldDefinition{DataSourceType: "PLAIN"},
				req.DataSourceConfig,
			)
			if !ok {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrFieldConfigInvalidDataSourceConfig, "data_source_config invalid")
				return
			}

			var displayLabel *string
			if req.Label != "" {
				displayLabel = &req.Label
			}

			cfg, wasRetry, err := cfgStore.EnableTenantFieldConfig(
				r.Context(),
				tenant.ID,
				req.FieldKey,
				valueType,
				"PLAIN",
				dataSourceConfig,
				displayLabel,
				req.EnabledOn,
				req.RequestCode,
				orgUnitInitiatorUUID(r.Context(), tenant.ID),
			)
			if err != nil {
				writeOrgUnitServiceError(w, r, err, "orgunit_field_config_enable_failed")
				return
			}

			status := http.StatusCreated
			if wasRetry {
				status = http.StatusOK
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)
			labelI18nKey, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(cfg)
			_ = json.NewEncoder(w).Encode(orgUnitFieldConfigAPIItem{
				FieldKey:         cfg.FieldKey,
				LabelI18nKey:     labelI18nKey,
				Label:            label,
				ValueType:        cfg.ValueType,
				DataSourceType:   cfg.DataSourceType,
				DataSourceConfig: cfg.DataSourceConfig,
				PhysicalCol:      cfg.PhysicalCol,
				EnabledOn:        cfg.EnabledOn,
				DisabledOn:       cfg.DisabledOn,
				UpdatedAt:        cfg.UpdatedAt,
				AllowFilter:      allowFilter,
				AllowSort:        allowSort,
			})
			return
		}

		// Contract (DEV-PLAN-106A): dict fields are enabled by dict_code-derived field_key (d_<dict_code>).
		if isCustomOrgUnitDictFieldKey(req.FieldKey) {
			dictCode, _ := dictCodeFromOrgUnitDictFieldKey(req.FieldKey)
			dataSourceConfig, dictName, ok, err := normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(r.Context(), tenant.ID, req.EnabledOn, dictStore, dictCode, req.DataSourceConfig)
			if err != nil {
				writeInternalAPIError(w, r, err, "orgunit_field_config_enable_failed")
				return
			}
			if !ok {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrFieldConfigInvalidDataSourceConfig, "data_source_config invalid")
				return
			}

			displayLabel := req.Label
			if displayLabel == "" {
				displayLabel = dictName
			}
			var displayLabelPtr *string
			if strings.TrimSpace(displayLabel) != "" {
				displayLabelPtr = &displayLabel
			}

			cfg, wasRetry, err := cfgStore.EnableTenantFieldConfig(
				r.Context(),
				tenant.ID,
				req.FieldKey,
				"text",
				"DICT",
				dataSourceConfig,
				displayLabelPtr,
				req.EnabledOn,
				req.RequestCode,
				orgUnitInitiatorUUID(r.Context(), tenant.ID),
			)
			if err != nil {
				writeOrgUnitServiceError(w, r, err, "orgunit_field_config_enable_failed")
				return
			}

			status := http.StatusCreated
			if wasRetry {
				status = http.StatusOK
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)
			labelI18nKey, label, _, _ := orgUnitFieldConfigPresentation(cfg)
			_ = json.NewEncoder(w).Encode(orgUnitFieldConfigAPIItem{
				FieldKey:         cfg.FieldKey,
				LabelI18nKey:     labelI18nKey,
				Label:            label,
				ValueType:        cfg.ValueType,
				DataSourceType:   cfg.DataSourceType,
				DataSourceConfig: cfg.DataSourceConfig,
				PhysicalCol:      cfg.PhysicalCol,
				EnabledOn:        cfg.EnabledOn,
				DisabledOn:       cfg.DisabledOn,
				UpdatedAt:        cfg.UpdatedAt,
				AllowFilter:      true,
				AllowSort:        true,
			})
			return
		}

		def, ok := resolveOrgUnitEnableDefinition(req.FieldKey)
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldDefinitionNotFound, "field definition not found")
			return
		}
		// Contract (DEV-PLAN-106A): built-in DICT field_keys are no longer enable targets.
		if strings.EqualFold(strings.TrimSpace(def.DataSourceType), "DICT") {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrFieldConfigInvalidDataSourceConfig, "data_source_config invalid")
			return
		}

		// For built-in non-DICT fields, normalization is pure (no external dependencies),
		// so it never returns an internal error here.
		dataSourceConfig, ok, _ := normalizeOrgUnitEnableDataSourceConfig(r.Context(), tenant.ID, req.EnabledOn, dictStore, def, req.DataSourceConfig)
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrFieldConfigInvalidDataSourceConfig, "data_source_config invalid")
			return
		}

		cfg, wasRetry, err := cfgStore.EnableTenantFieldConfig(
			r.Context(),
			tenant.ID,
			req.FieldKey,
			def.ValueType,
			def.DataSourceType,
			dataSourceConfig,
			nil,
			req.EnabledOn,
			req.RequestCode,
			orgUnitInitiatorUUID(r.Context(), tenant.ID),
		)
		if err != nil {
			writeOrgUnitServiceError(w, r, err, "orgunit_field_config_enable_failed")
			return
		}

		status := http.StatusCreated
		if wasRetry {
			status = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		labelI18nKey, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(cfg)
		_ = json.NewEncoder(w).Encode(orgUnitFieldConfigAPIItem{
			FieldKey:         cfg.FieldKey,
			LabelI18nKey:     labelI18nKey,
			Label:            label,
			ValueType:        cfg.ValueType,
			DataSourceType:   cfg.DataSourceType,
			DataSourceConfig: cfg.DataSourceConfig,
			PhysicalCol:      cfg.PhysicalCol,
			EnabledOn:        cfg.EnabledOn,
			DisabledOn:       cfg.DisabledOn,
			UpdatedAt:        cfg.UpdatedAt,
			AllowFilter:      allowFilter,
			AllowSort:        allowSort,
		})
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleOrgUnitFieldConfigsDisableAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	cfgStore, ok := store.(orgUnitFieldConfigStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}

	var req orgUnitFieldConfigsDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.FieldKey = strings.TrimSpace(req.FieldKey)
	req.DisabledOn = strings.TrimSpace(req.DisabledOn)
	req.RequestCode = strings.TrimSpace(req.RequestCode)
	if req.FieldKey == "" || req.DisabledOn == "" || req.RequestCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "field_key/disabled_on/request_code required")
		return
	}
	if _, err := time.Parse("2006-01-02", req.DisabledOn); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "disabled_on invalid")
		return
	}

	cfg, _, err := cfgStore.DisableTenantFieldConfig(
		r.Context(),
		tenant.ID,
		req.FieldKey,
		req.DisabledOn,
		req.RequestCode,
		orgUnitInitiatorUUID(r.Context(), tenant.ID),
	)
	if err != nil {
		writeOrgUnitServiceError(w, r, err, "orgunit_field_config_disable_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	labelI18nKey, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(cfg)
	_ = json.NewEncoder(w).Encode(orgUnitFieldConfigAPIItem{
		FieldKey:         cfg.FieldKey,
		LabelI18nKey:     labelI18nKey,
		Label:            label,
		ValueType:        cfg.ValueType,
		DataSourceType:   cfg.DataSourceType,
		DataSourceConfig: cfg.DataSourceConfig,
		PhysicalCol:      cfg.PhysicalCol,
		EnabledOn:        cfg.EnabledOn,
		DisabledOn:       cfg.DisabledOn,
		UpdatedAt:        cfg.UpdatedAt,
		AllowFilter:      allowFilter,
		AllowSort:        allowSort,
	})
}

type orgUnitFieldOptionsAPIResponse struct {
	FieldKey string               `json:"field_key"`
	AsOf     string               `json:"as_of"`
	Options  []orgUnitFieldOption `json:"options"`
}

type orgUnitEnabledFieldConfigReader interface {
	GetEnabledTenantFieldConfigAsOf(ctx context.Context, tenantID string, fieldKey string, asOf string) (orgUnitTenantFieldConfig, bool, error)
}

func handleOrgUnitFieldOptionsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	reader, ok := store.(orgUnitEnabledFieldConfigReader)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}

	asOf, err := orgUnitAPIAsOf(r)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	fieldKey := strings.TrimSpace(r.URL.Query().Get("field_key"))
	if fieldKey == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "field_key required")
		return
	}

	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 50 {
		limit = 50
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	cfg, ok, err := reader.GetEnabledTenantFieldConfigAsOf(r.Context(), tenant.ID, fieldKey, asOf)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_field_options_failed")
		return
	}
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsFieldNotEnabled, "field not enabled")
		return
	}

	dataSourceType := strings.ToUpper(strings.TrimSpace(cfg.DataSourceType))
	switch dataSourceType {
	case "PLAIN":
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
		return
	case "DICT":
		dictCode, ok := dictCodeFromDataSourceConfig(cfg.DataSourceConfig)
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
			return
		}
		// Dict field key namespace (d_<dict_code>) must be self-consistent.
		if isCustomOrgUnitDictFieldKey(fieldKey) {
			suffix, _ := dictCodeFromOrgUnitDictFieldKey(fieldKey)
			if !strings.EqualFold(strings.TrimSpace(suffix), strings.TrimSpace(dictCode)) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
				return
			}
		} else {
			// Compatibility: built-in DICT fields must be defined as DICT (DEV-PLAN-106).
			def, ok := lookupOrgUnitFieldDefinition(fieldKey)
			if !ok || strings.ToUpper(strings.TrimSpace(def.DataSourceType)) != "DICT" {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
				return
			}
		}
		options, err := listOrgUnitDictOptions(r.Context(), tenant.ID, asOf, dictCode, keyword, limit)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_field_options_failed")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(orgUnitFieldOptionsAPIResponse{FieldKey: fieldKey, AsOf: asOf, Options: options})
		return
	case "ENTITY":
		fallthrough
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
		return
	}
}

func orgUnitFieldDataSourceConfigOptionsJSON(def orgUnitFieldDefinition) []json.RawMessage {
	opts := orgUnitFieldDataSourceConfigOptions(def)
	if len(opts) == 0 {
		return nil
	}
	raws := make([]json.RawMessage, 0, len(opts))
	for _, opt := range opts {
		raw, err := json.Marshal(opt)
		if err != nil {
			continue
		}
		raws = append(raws, json.RawMessage(raw))
	}
	sort.SliceStable(raws, func(i, j int) bool { return string(raws[i]) < string(raws[j]) })
	return raws
}

func resolveOrgUnitEnableDefinition(fieldKey string) (orgUnitFieldDefinition, bool) {
	fieldKey = strings.TrimSpace(fieldKey)
	if fieldKey == "" {
		return orgUnitFieldDefinition{}, false
	}
	if def, ok := lookupOrgUnitFieldDefinition(fieldKey); ok {
		return def, true
	}
	return buildCustomOrgUnitPlainFieldDefinition(fieldKey)
}

func orgUnitCustomPlainValueTypes() []string {
	return []string{"text", "int", "uuid", "bool", "date", "numeric"}
}

func normalizeOrgUnitCustomPlainValueType(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	for _, valueType := range orgUnitCustomPlainValueTypes() {
		if raw == valueType {
			return valueType, true
		}
	}
	return "", false
}

func normalizeOrgUnitEnableDataSourceConfig(
	ctx context.Context,
	tenantID string,
	enabledOn string,
	dictStore orgUnitDictRegistryStore,
	def orgUnitFieldDefinition,
	raw json.RawMessage,
) (json.RawMessage, bool, error) {
	dataSourceType := strings.ToUpper(strings.TrimSpace(def.DataSourceType))
	switch dataSourceType {
	case "PLAIN":
		if len(bytes.TrimSpace(raw)) == 0 {
			return json.RawMessage(`{}`), true, nil
		}
		var tmp map[string]any
		if err := json.Unmarshal(raw, &tmp); err != nil || tmp == nil {
			return nil, false, nil
		}
		if len(tmp) != 0 {
			return nil, false, nil
		}
		return json.RawMessage(`{}`), true, nil
	case "DICT":
		if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, false, nil
		}
		var tmp map[string]any
		if err := json.Unmarshal(raw, &tmp); err != nil || tmp == nil {
			return nil, false, nil
		}
		if len(tmp) != 1 {
			return nil, false, nil
		}
		codeRaw, ok := tmp["dict_code"]
		if !ok {
			return nil, false, nil
		}
		code, ok := codeRaw.(string)
		if !ok {
			return nil, false, nil
		}
		code = strings.TrimSpace(code)
		if code == "" {
			return nil, false, nil
		}

		if dictStore == nil {
			return nil, false, errors.New("dict store missing")
		}
		dicts, err := listOrgUnitDicts(ctx, dictStore, tenantID, enabledOn)
		if err != nil {
			return nil, false, err
		}
		found := false
		for _, item := range dicts {
			if strings.EqualFold(strings.TrimSpace(item.DictCode), code) {
				found = true
				break
			}
		}
		if !found {
			return nil, false, nil
		}
		canonical, _ := json.Marshal(map[string]any{"dict_code": code})
		return json.RawMessage(canonical), true, nil
	case "ENTITY":
		if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, false, nil
		}
		var tmp map[string]any
		if err := json.Unmarshal(raw, &tmp); err != nil || tmp == nil {
			return nil, false, nil
		}
		canonical, _ := json.Marshal(tmp)
		options := orgUnitFieldDataSourceConfigOptions(def)
		for _, opt := range options {
			want, _ := json.Marshal(opt)
			if bytes.Equal(want, canonical) {
				return json.RawMessage(canonical), true, nil
			}
		}
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

func normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(
	ctx context.Context,
	tenantID string,
	enabledOn string,
	dictStore orgUnitDictRegistryStore,
	dictCode string,
	raw json.RawMessage,
) (json.RawMessage, string, bool, error) {
	dictCode = strings.TrimSpace(dictCode)
	if dictCode == "" {
		return nil, "", false, nil
	}

	// For d_<dict_code>, data_source_config is derived; if provided, it must match.
	if len(bytes.TrimSpace(raw)) != 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		got, ok := dictCodeFromDataSourceConfig(raw)
		if !ok || !strings.EqualFold(strings.TrimSpace(got), dictCode) {
			return nil, "", false, nil
		}
	}

	if dictStore == nil {
		return nil, "", false, errors.New("dict store missing")
	}
	dicts, err := listOrgUnitDicts(ctx, dictStore, tenantID, enabledOn)
	if err != nil {
		return nil, "", false, err
	}
	found := false
	dictName := ""
	for _, item := range dicts {
		if strings.EqualFold(strings.TrimSpace(item.DictCode), dictCode) {
			found = true
			dictName = strings.TrimSpace(item.Name)
			break
		}
	}
	if !found {
		return nil, "", false, nil
	}
	if dictName == "" {
		dictName = dictCode
	}

	canonical, _ := json.Marshal(map[string]any{"dict_code": dictCode})
	return json.RawMessage(canonical), dictName, true, nil
}

func dictCodeFromDataSourceConfig(raw json.RawMessage) (string, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", false
	}
	var tmp map[string]any
	if err := json.Unmarshal(raw, &tmp); err != nil || tmp == nil {
		return "", false
	}
	code, _ := tmp["dict_code"].(string)
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false
	}
	return code, true
}

func orgUnitFieldConfigPresentation(cfg orgUnitTenantFieldConfig) (*string, *string, bool, bool) {
	fieldKey := strings.TrimSpace(cfg.FieldKey)
	if def, ok := lookupOrgUnitFieldDefinition(fieldKey); ok {
		labelKey := strings.TrimSpace(def.LabelI18nKey)
		// Built-in fields always carry i18n key (SSOT: modules/orgunit/domain/fieldmeta).
		return &labelKey, nil, def.AllowFilter, def.AllowSort
	}
	if isCustomOrgUnitDictFieldKey(fieldKey) {
		if cfg.DisplayLabel != nil && strings.TrimSpace(*cfg.DisplayLabel) != "" {
			label := strings.TrimSpace(*cfg.DisplayLabel)
			return nil, &label, true, true
		}
		dictCode, _ := dictCodeFromOrgUnitDictFieldKey(fieldKey)
		label := dictCode
		return nil, &label, true, true
	}
	if cfg.DisplayLabel != nil && strings.TrimSpace(*cfg.DisplayLabel) != "" {
		label := strings.TrimSpace(*cfg.DisplayLabel)
		return nil, &label, false, false
	}
	label := fieldKey
	return nil, &label, false, false
}
