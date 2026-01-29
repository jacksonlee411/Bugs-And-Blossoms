package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type scopePackageCreateAPIRequest struct {
	ScopeCode     string `json:"scope_code"`
	PackageCode   string `json:"package_code"`
	Name          string `json:"name"`
	EffectiveDate string `json:"effective_date"`
	RequestID     string `json:"request_id"`
}

type scopePackageDisableAPIRequest struct {
	RequestID string `json:"request_id"`
}

type scopeSubscriptionAPIRequest struct {
	SetID         string `json:"setid"`
	ScopeCode     string `json:"scope_code"`
	PackageID     string `json:"package_id"`
	PackageOwner  string `json:"package_owner"`
	EffectiveDate string `json:"effective_date"`
	RequestID     string `json:"request_id"`
}

var packageCodePattern = regexp.MustCompile(`^[A-Z0-9_]{1,16}$`)

func handleScopePackagesAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		scopeCode := strings.TrimSpace(r.URL.Query().Get("scope_code"))
		if scopeCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "scope_code required")
			return
		}
		rows, err := store.ListScopePackages(r.Context(), tenant.ID, scopeCode)
		if err != nil {
			writeScopeAPIError(w, r, err, "scope_package_list_failed")
			return
		}
		if rows == nil {
			rows = []ScopePackage{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(rows)
		return
	case http.MethodPost:
		var req scopePackageCreateAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		req.ScopeCode = strings.TrimSpace(req.ScopeCode)
		req.PackageCode = strings.ToUpper(strings.TrimSpace(req.PackageCode))
		req.Name = strings.TrimSpace(req.Name)
		req.RequestID = strings.TrimSpace(req.RequestID)
		req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
		if req.ScopeCode == "" || req.Name == "" || req.RequestID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "scope_code/name/request_id required")
			return
		}
		if req.EffectiveDate == "" {
			req.EffectiveDate = time.Now().UTC().Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
		if req.PackageCode == "" {
			req.PackageCode = generatePackageCode()
		}
		if req.PackageCode == "DEFLT" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "PACKAGE_CODE_RESERVED", "PACKAGE_CODE_RESERVED")
			return
		}
		if !packageCodePattern.MatchString(req.PackageCode) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "PACKAGE_CODE_INVALID", "PACKAGE_CODE_INVALID")
			return
		}

		pkg, err := store.CreateScopePackage(r.Context(), tenant.ID, req.ScopeCode, req.PackageCode, req.Name, req.EffectiveDate, req.RequestID, tenant.ID)
		if err != nil {
			writeScopeAPIError(w, r, err, "scope_package_create_failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_id":   pkg.PackageID,
			"scope_code":   pkg.ScopeCode,
			"package_code": pkg.PackageCode,
			"status":       pkg.Status,
		})
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleScopePackageDisableAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	packageID := parseScopePackageID(r.URL.Path)
	if packageID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "package_id required")
		return
	}
	var req scopePackageDisableAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "request_id required")
		return
	}
	pkg, err := store.DisableScopePackage(r.Context(), tenant.ID, packageID, req.RequestID, tenant.ID)
	if err != nil {
		writeScopeAPIError(w, r, err, "scope_package_disable_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"package_id": pkg.PackageID,
		"status":     pkg.Status,
	})
}

func handleScopeSubscriptionsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		setID := strings.TrimSpace(r.URL.Query().Get("setid"))
		scopeCode := strings.TrimSpace(r.URL.Query().Get("scope_code"))
		asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
		if setID == "" || scopeCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "setid/scope_code required")
			return
		}
		if asOf == "" {
			asOf = time.Now().UTC().Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", asOf); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
			return
		}
		sub, err := store.GetScopeSubscription(r.Context(), tenant.ID, setID, scopeCode, asOf)
		if err != nil {
			writeScopeAPIError(w, r, err, "scope_subscription_get_failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"setid":          sub.SetID,
			"scope_code":     sub.ScopeCode,
			"package_id":     sub.PackageID,
			"package_owner":  sub.PackageOwner,
			"effective_date": sub.EffectiveDate,
			"end_date":       sub.EndDate,
		})
		return
	case http.MethodPost:
		var req scopeSubscriptionAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		req.SetID = strings.TrimSpace(req.SetID)
		req.ScopeCode = strings.TrimSpace(req.ScopeCode)
		req.PackageID = strings.TrimSpace(req.PackageID)
		req.PackageOwner = strings.TrimSpace(req.PackageOwner)
		req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
		req.RequestID = strings.TrimSpace(req.RequestID)
		if req.SetID == "" || req.ScopeCode == "" || req.PackageID == "" || req.PackageOwner == "" || req.EffectiveDate == "" || req.RequestID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "setid/scope_code/package_id/package_owner/effective_date/request_id required")
			return
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
		owner := strings.ToLower(req.PackageOwner)
		if owner != "tenant" && owner != "global" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "PACKAGE_OWNER_INVALID", "PACKAGE_OWNER_INVALID")
			return
		}
		sub, err := store.CreateScopeSubscription(r.Context(), tenant.ID, req.SetID, req.ScopeCode, req.PackageID, owner, req.EffectiveDate, req.RequestID, tenant.ID)
		if err != nil {
			writeScopeAPIError(w, r, err, "scope_subscription_create_failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"setid":          sub.SetID,
			"scope_code":     sub.ScopeCode,
			"package_id":     sub.PackageID,
			"package_owner":  sub.PackageOwner,
			"effective_date": sub.EffectiveDate,
		})
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleGlobalScopePackagesAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	switch r.Method {
	case http.MethodGet:
		actorScope := strings.TrimSpace(r.Header.Get("X-Actor-Scope"))
		if actorScope == "" {
			actorScope = strings.TrimSpace(r.Header.Get("x-actor-scope"))
		}
		if strings.ToLower(actorScope) != "saas" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "actor_scope_forbidden", "actor scope forbidden")
			return
		}
		scopeCode := strings.TrimSpace(r.URL.Query().Get("scope_code"))
		if scopeCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "scope_code required")
			return
		}
		rows, err := store.ListGlobalScopePackages(r.Context(), scopeCode)
		if err != nil {
			writeScopeAPIError(w, r, err, "global_scope_package_list_failed")
			return
		}
		if rows == nil {
			rows = []ScopePackage{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(rows)
		return
	case http.MethodPost:
		tenant, ok := currentTenant(r.Context())
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
			return
		}
		var req scopePackageCreateAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		req.ScopeCode = strings.TrimSpace(req.ScopeCode)
		req.PackageCode = strings.ToUpper(strings.TrimSpace(req.PackageCode))
		req.Name = strings.TrimSpace(req.Name)
		req.RequestID = strings.TrimSpace(req.RequestID)
		req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
		if req.ScopeCode == "" || req.Name == "" || req.RequestID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "scope_code/name/request_id required")
			return
		}
		if req.EffectiveDate == "" {
			req.EffectiveDate = time.Now().UTC().Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
		if req.PackageCode == "" {
			req.PackageCode = generatePackageCode()
		}
		if req.PackageCode == "DEFLT" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "PACKAGE_CODE_RESERVED", "PACKAGE_CODE_RESERVED")
			return
		}
		if !packageCodePattern.MatchString(req.PackageCode) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "PACKAGE_CODE_INVALID", "PACKAGE_CODE_INVALID")
			return
		}
		actorScope := strings.TrimSpace(r.Header.Get("X-Actor-Scope"))
		if actorScope == "" {
			actorScope = strings.TrimSpace(r.Header.Get("x-actor-scope"))
		}
		if strings.ToLower(actorScope) != "saas" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "actor_scope_forbidden", "actor scope forbidden")
			return
		}
		pkg, err := store.CreateGlobalScopePackage(r.Context(), req.ScopeCode, req.PackageCode, req.Name, req.EffectiveDate, req.RequestID, tenant.ID, actorScope)
		if err != nil {
			writeScopeAPIError(w, r, err, "global_scope_package_create_failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_id":   pkg.PackageID,
			"scope_code":   pkg.ScopeCode,
			"package_code": pkg.PackageCode,
			"status":       pkg.Status,
		})
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func parseScopePackageID(path string) string {
	const prefix = "/orgunit/api/scope-packages/"
	const suffix = "/disable"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.Trim(trimmed, "/")
}

func generatePackageCode() string {
	n := strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	n = strings.ToUpper(n)
	if len(n) > 8 {
		n = n[len(n)-8:]
	}
	return "PKG_" + n
}

func writeScopeAPIError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	code := stablePgMessage(err)
	status := http.StatusInternalServerError
	switch code {
	case "PACKAGE_NOT_FOUND", "SETID_NOT_FOUND", "SCOPE_SUBSCRIPTION_MISSING":
		status = http.StatusNotFound
	case "PACKAGE_CODE_DUPLICATE", "PACKAGE_INACTIVE_AS_OF", "SUBSCRIPTION_OVERLAP":
		status = http.StatusConflict
	case "PACKAGE_CODE_RESERVED", "PACKAGE_CODE_INVALID", "REQUEST_ID_REQUIRED", "SCOPE_CODE_INVALID", "PACKAGE_OWNER_INVALID", "PACKAGE_SCOPE_MISMATCH", "SETID_RESERVED_WORD":
		status = http.StatusUnprocessableEntity
	default:
		if isStableDBCode(code) {
			status = http.StatusUnprocessableEntity
		}
		if isBadRequestError(err) || isPgInvalidInput(err) {
			status = http.StatusBadRequest
		}
	}
	if code == "" || code == "UNKNOWN" {
		code = defaultCode
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, defaultCode)
}
