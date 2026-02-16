package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type dictListResponse struct {
	AsOf  string     `json:"as_of"`
	Dicts []DictItem `json:"dicts"`
}

type dictValuesResponse struct {
	DictCode string          `json:"dict_code"`
	AsOf     string          `json:"as_of"`
	Values   []DictValueItem `json:"values"`
}

type dictValueMutationResponse struct {
	DictValueItem
	WasRetry bool `json:"was_retry"`
}

type dictCreateValuePayload struct {
	DictCode    string `json:"dict_code"`
	Code        string `json:"code"`
	Label       string `json:"label"`
	EnabledOn   string `json:"enabled_on"`
	RequestCode string `json:"request_code"`
}

type dictDisableValuePayload struct {
	DictCode    string `json:"dict_code"`
	Code        string `json:"code"`
	DisabledOn  string `json:"disabled_on"`
	RequestCode string `json:"request_code"`
}

type dictCorrectValuePayload struct {
	DictCode      string `json:"dict_code"`
	Code          string `json:"code"`
	Label         string `json:"label"`
	CorrectionDay string `json:"correction_day"`
	RequestCode   string `json:"request_code"`
}

type dictAuditResponse struct {
	DictCode string               `json:"dict_code"`
	Code     string               `json:"code"`
	Limit    int                  `json:"limit"`
	Events   []DictValueAuditItem `json:"events"`
}

func handleDictsAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requiredAsOf(r)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	items, err := store.ListDicts(r.Context(), tenant.ID, asOf)
	if err != nil {
		writeDictAPIError(w, r, err, "dict_list_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dictListResponse{AsOf: asOf, Dicts: items})
}

func handleDictValuesAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	switch r.Method {
	case http.MethodGet:
		handleDictValuesListAPI(w, r, store)
	case http.MethodPost:
		handleDictValuesCreateAPI(w, r, store)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleDictValuesListAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requiredAsOf(r)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}
	dictCode := normalizeDictCode(r.URL.Query().Get("dict_code"))
	if dictCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_code_required", "dict_code required")
		return
	}
	if !supportedDictCode(dictCode) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "dict_not_found", "dict not found")
		return
	}

	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "limit invalid")
			return
		}
		limit = n
	}
	if limit > 50 {
		limit = 50
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "all"
	}
	if status != "active" && status != "inactive" && status != "all" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "status invalid")
		return
	}

	values, err := store.ListDictValues(
		r.Context(),
		tenant.ID,
		dictCode,
		asOf,
		strings.TrimSpace(r.URL.Query().Get("q")),
		limit,
		status,
	)
	if err != nil {
		writeDictAPIError(w, r, err, "dict_values_list_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dictValuesResponse{
		DictCode: dictCode,
		AsOf:     asOf,
		Values:   values,
	})
}

func handleDictValuesCreateAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var req dictCreateValuePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.DictCode = normalizeDictCode(req.DictCode)
	req.Code = strings.TrimSpace(req.Code)
	req.Label = strings.TrimSpace(req.Label)
	req.EnabledOn = strings.TrimSpace(req.EnabledOn)
	req.RequestCode = strings.TrimSpace(req.RequestCode)
	if req.DictCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_code_required", "dict_code required")
		return
	}
	if !supportedDictCode(req.DictCode) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "dict_not_found", "dict not found")
		return
	}
	if req.Code == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_value_code_required", "code required")
		return
	}
	if req.Label == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_value_label_required", "label required")
		return
	}
	if !isDate(req.EnabledOn) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "enabled_on invalid")
		return
	}
	if req.RequestCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "request_code required")
		return
	}

	item, wasRetry, err := store.CreateDictValue(r.Context(), tenant.ID, DictCreateValueRequest{
		DictCode:    req.DictCode,
		Code:        req.Code,
		Label:       req.Label,
		EnabledOn:   req.EnabledOn,
		RequestCode: req.RequestCode,
		Initiator:   orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeDictAPIError(w, r, err, "dict_value_create_failed")
		return
	}

	status := http.StatusCreated
	if wasRetry {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dictValueMutationResponse{DictValueItem: item, WasRetry: wasRetry})
}

func handleDictValuesDisableAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var req dictDisableValuePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.DictCode = normalizeDictCode(req.DictCode)
	req.Code = strings.TrimSpace(req.Code)
	req.DisabledOn = strings.TrimSpace(req.DisabledOn)
	req.RequestCode = strings.TrimSpace(req.RequestCode)

	if req.DictCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_code_required", "dict_code required")
		return
	}
	if !supportedDictCode(req.DictCode) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "dict_not_found", "dict not found")
		return
	}
	if req.Code == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_value_code_required", "code required")
		return
	}
	if !isDate(req.DisabledOn) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "disabled_on invalid")
		return
	}
	if req.RequestCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "request_code required")
		return
	}

	item, wasRetry, err := store.DisableDictValue(r.Context(), tenant.ID, DictDisableValueRequest{
		DictCode:    req.DictCode,
		Code:        req.Code,
		DisabledOn:  req.DisabledOn,
		RequestCode: req.RequestCode,
		Initiator:   orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeDictAPIError(w, r, err, "dict_value_disable_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dictValueMutationResponse{DictValueItem: item, WasRetry: wasRetry})
}

func handleDictValuesCorrectAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var req dictCorrectValuePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.DictCode = normalizeDictCode(req.DictCode)
	req.Code = strings.TrimSpace(req.Code)
	req.Label = strings.TrimSpace(req.Label)
	req.CorrectionDay = strings.TrimSpace(req.CorrectionDay)
	req.RequestCode = strings.TrimSpace(req.RequestCode)

	if req.DictCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_code_required", "dict_code required")
		return
	}
	if !supportedDictCode(req.DictCode) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "dict_not_found", "dict not found")
		return
	}
	if req.Code == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_value_code_required", "code required")
		return
	}
	if req.Label == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_value_label_required", "label required")
		return
	}
	if !isDate(req.CorrectionDay) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "correction_day invalid")
		return
	}
	if req.RequestCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "request_code required")
		return
	}

	item, wasRetry, err := store.CorrectDictValue(r.Context(), tenant.ID, DictCorrectValueRequest{
		DictCode:      req.DictCode,
		Code:          req.Code,
		Label:         req.Label,
		CorrectionDay: req.CorrectionDay,
		RequestCode:   req.RequestCode,
		Initiator:     orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeDictAPIError(w, r, err, "dict_value_correct_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dictValueMutationResponse{DictValueItem: item, WasRetry: wasRetry})
}

func handleDictValuesAuditAPI(w http.ResponseWriter, r *http.Request, store DictStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	dictCode := normalizeDictCode(r.URL.Query().Get("dict_code"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if dictCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_code_required", "dict_code required")
		return
	}
	if !supportedDictCode(dictCode) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "dict_not_found", "dict not found")
		return
	}
	if code == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "dict_value_code_required", "code required")
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "limit invalid")
			return
		}
		limit = n
	}
	if limit > 200 {
		limit = 200
	}

	events, err := store.ListDictValueAudit(r.Context(), tenant.ID, dictCode, code, limit)
	if err != nil {
		writeDictAPIError(w, r, err, "dict_value_audit_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dictAuditResponse{
		DictCode: dictCode,
		Code:     code,
		Limit:    limit,
		Events:   events,
	})
}

func normalizeDictCode(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func requiredAsOf(r *http.Request) (string, bool) {
	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if !isDate(asOf) {
		return "", false
	}
	return asOf, true
}

func isDate(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	_, err := time.Parse("2006-01-02", raw)
	return err == nil
}

func writeDictAPIError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	code := dictErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case "invalid_as_of", "dict_code_required", "dict_value_code_required", "dict_value_label_required", "invalid_request":
		status = http.StatusBadRequest
	case "dict_not_found", "dict_value_not_found_as_of":
		status = http.StatusNotFound
	case "dict_value_conflict":
		status = http.StatusConflict
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, defaultCode)
}

func dictErrorCode(err error) string {
	switch {
	case errors.Is(err, errDictCodeRequired):
		return "dict_code_required"
	case errors.Is(err, errDictNotFound):
		return "dict_not_found"
	case errors.Is(err, errDictValueCodeRequired):
		return "dict_value_code_required"
	case errors.Is(err, errDictValueLabelRequired):
		return "dict_value_label_required"
	case errors.Is(err, errDictValueNotFoundAsOf):
		return "dict_value_not_found_as_of"
	case errors.Is(err, errDictValueConflict):
		return "dict_value_conflict"
	case errors.Is(err, errDictRequestCodeRequired), errors.Is(err, errDictEffectiveDayRequired):
		return "invalid_request"
	}

	code := strings.TrimSpace(strings.ToLower(stablePgMessage(err)))
	switch code {
	case "dict_code_required", "dict_not_found", "dict_value_code_required", "dict_value_label_required",
		"dict_value_not_found_as_of", "dict_value_conflict":
		return code
	case "dict_effective_day_required":
		return "invalid_as_of"
	case "dict_request_code_required":
		return "invalid_request"
	default:
		return strings.TrimSpace(defaultStableCode(code, "internal_error"))
	}
}

func defaultStableCode(code string, fallback string) string {
	if code == "" || code == "unknown" || !isStableDBCode(strings.ToUpper(code)) {
		return fallback
	}
	return code
}
