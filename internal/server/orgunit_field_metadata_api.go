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
	ValueType        string          `json:"value_type"`
	DataSourceType   string          `json:"data_source_type"`
	DataSourceConfig json.RawMessage `json:"data_source_config"`
	PhysicalCol      string          `json:"physical_col"`
	EnabledOn        string          `json:"enabled_on"`
	DisabledOn       *string         `json:"disabled_on"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type orgUnitFieldConfigsEnableRequest struct {
	FieldKey         string          `json:"field_key"`
	EnabledOn        string          `json:"enabled_on"`
	RequestCode      string          `json:"request_code"`
	DataSourceConfig json.RawMessage `json:"data_source_config"`
}

type orgUnitFieldConfigsDisableRequest struct {
	FieldKey    string `json:"field_key"`
	DisabledOn  string `json:"disabled_on"`
	RequestCode string `json:"request_code"`
}

type orgUnitFieldConfigStore interface {
	ListTenantFieldConfigs(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error)
	EnableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, valueType string, dataSourceType string, dataSourceConfig json.RawMessage, enabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
	DisableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, disabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
}

type orgUnitDictRegistryStore interface {
	ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error)
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
			items = append(items, orgUnitFieldConfigAPIItem{
				FieldKey:         row.FieldKey,
				ValueType:        row.ValueType,
				DataSourceType:   row.DataSourceType,
				DataSourceConfig: row.DataSourceConfig,
				PhysicalCol:      row.PhysicalCol,
				EnabledOn:        row.EnabledOn,
				DisabledOn:       row.DisabledOn,
				UpdatedAt:        row.UpdatedAt,
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

		def, ok := resolveOrgUnitEnableDefinition(req.FieldKey)
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldDefinitionNotFound, "field definition not found")
			return
		}

		dataSourceConfig, ok, err := normalizeOrgUnitEnableDataSourceConfig(r.Context(), tenant.ID, req.EnabledOn, dictStore, def, req.DataSourceConfig)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_field_config_enable_failed")
			return
		}
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
		_ = json.NewEncoder(w).Encode(orgUnitFieldConfigAPIItem{
			FieldKey:         cfg.FieldKey,
			ValueType:        cfg.ValueType,
			DataSourceType:   cfg.DataSourceType,
			DataSourceConfig: cfg.DataSourceConfig,
			PhysicalCol:      cfg.PhysicalCol,
			EnabledOn:        cfg.EnabledOn,
			DisabledOn:       cfg.DisabledOn,
			UpdatedAt:        cfg.UpdatedAt,
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
	_ = json.NewEncoder(w).Encode(orgUnitFieldConfigAPIItem{
		FieldKey:         cfg.FieldKey,
		ValueType:        cfg.ValueType,
		DataSourceType:   cfg.DataSourceType,
		DataSourceConfig: cfg.DataSourceConfig,
		PhysicalCol:      cfg.PhysicalCol,
		EnabledOn:        cfg.EnabledOn,
		DisabledOn:       cfg.DisabledOn,
		UpdatedAt:        cfg.UpdatedAt,
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
		def, ok := lookupOrgUnitFieldDefinition(fieldKey)
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
			return
		}
		if strings.ToUpper(strings.TrimSpace(def.DataSourceType)) != "DICT" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
			return
		}
		dictCode, ok := dictCodeFromDataSourceConfig(cfg.DataSourceConfig)
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrFieldOptionsNotSupported, "options not supported")
			return
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
			want, err := json.Marshal(opt)
			if err != nil {
				continue
			}
			if bytes.Equal(want, canonical) {
				return json.RawMessage(canonical), true, nil
			}
		}
		return nil, false, nil
	default:
		return nil, false, nil
	}
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
