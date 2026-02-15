package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	staffingcontrollers "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/presentation/controllers"
	staffingservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitSetIDResolver interface {
	ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error)
}

type staffingPositionsAPIRequest struct {
	EffectiveDate         string `json:"effective_date"`
	PositionUUID          string `json:"position_uuid"`
	OrgCode               string `json:"org_code"`
	ReportsToPositionUUID string `json:"reports_to_position_uuid"`
	JobProfileUUID        string `json:"job_profile_uuid"`
	CapacityFTE           string `json:"capacity_fte"`
	Name                  string `json:"name"`
	LifecycleStatus       string `json:"lifecycle_status"`
}

type staffingPositionAPIResponse struct {
	PositionUUID          string `json:"position_uuid"`
	OrgCode               string `json:"org_code"`
	ReportsToPositionUUID string `json:"reports_to_position_uuid"`
	JobCatalogSetID       string `json:"jobcatalog_setid"`
	JobCatalogSetIDAsOf   string `json:"jobcatalog_setid_as_of"`
	JobProfileUUID        string `json:"job_profile_uuid"`
	JobProfileCode        string `json:"job_profile_code"`
	Name                  string `json:"name"`
	LifecycleStatus       string `json:"lifecycle_status"`
	CapacityFTE           string `json:"capacity_fte"`
	EffectiveDate         string `json:"effective_date"`
}

type staffingJobProfileOptionAPIItem struct {
	JobProfileUUID string `json:"job_profile_uuid"`
	JobProfileCode string `json:"job_profile_code"`
	Name           string `json:"name"`
}

type staffingPositionsOptionsAPIResponse struct {
	AsOf            string                            `json:"as_of"`
	OrgCode         string                            `json:"org_code"`
	JobCatalogSetID string                            `json:"jobcatalog_setid"`
	JobProfiles     []staffingJobProfileOptionAPIItem `json:"job_profiles"`
}

func handlePositionsOptionsAPI(w http.ResponseWriter, r *http.Request, orgStore OrgUnitStore, jobStore JobCatalogStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	rawOrgCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	if rawOrgCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
		return
	}
	orgCode, err := orgunitpkg.NormalizeOrgCode(rawOrgCode)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		return
	}

	orgID, err := orgStore.ResolveOrgID(r.Context(), tenant.ID, orgCode)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}

	resolver, ok := any(orgStore).(orgUnitSetIDResolver)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_resolver_missing", "setid resolver missing")
		return
	}
	setID, err := resolver.ResolveSetID(r.Context(), tenant.ID, strconv.Itoa(orgID), asOf)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "resolve setid failed")
		return
	}
	if strings.TrimSpace(setID) == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "setid_missing", "setid missing")
		return
	}
	if jobStore == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "jobcatalog_store_missing", "jobcatalog store missing")
		return
	}

	jobProfiles, err := jobStore.ListJobProfiles(r.Context(), tenant.ID, setID, asOf)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "list job profiles failed")
		return
	}

	resp := staffingPositionsOptionsAPIResponse{
		AsOf:            asOf,
		OrgCode:         orgCode,
		JobCatalogSetID: setID,
		JobProfiles:     make([]staffingJobProfileOptionAPIItem, 0, len(jobProfiles)),
	}
	for _, p := range jobProfiles {
		resp.JobProfiles = append(resp.JobProfiles, staffingJobProfileOptionAPIItem{
			JobProfileUUID: p.JobProfileUUID,
			JobProfileCode: p.JobProfileCode,
			Name:           p.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func handlePositionsAPI(w http.ResponseWriter, r *http.Request, orgResolver OrgUnitCodeResolver, store PositionStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}
	if deprecatedField := findDeprecatedField(r.URL.Query(), "org_unit_id", "position_id", "reports_to_position_id", "job_profile_id"); deprecatedField != "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "use org_code/position_uuid")
		return
	}

	switch r.Method {
	case http.MethodGet:
		positions, err := store.ListPositionsCurrent(r.Context(), tenant.ID, asOf)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			if isStableDBCode(code) {
				status = http.StatusUnprocessableEntity
			}
			if isBadRequestError(err) || isPgInvalidInput(err) {
				status = http.StatusBadRequest
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "list failed")
			return
		}
		orgCodes := map[string]string{}
		orgIDByStr := map[string]int{}
		orgIDs := make([]int, 0)
		for _, p := range positions {
			if p.OrgUnitID == "" {
				continue
			}
			if _, ok := orgIDByStr[p.OrgUnitID]; ok {
				continue
			}
			orgID, err := strconv.Atoi(p.OrgUnitID)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_id_invalid", "invalid orgunit id")
				return
			}
			orgIDByStr[p.OrgUnitID] = orgID
			orgIDs = append(orgIDs, orgID)
		}
		if len(orgIDs) > 0 {
			if orgResolver == nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolver_missing", "orgunit resolver missing")
				return
			}
			codes, err := orgResolver.ResolveOrgCodes(r.Context(), tenant.ID, orgIDs)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolve_org_code_failed", "resolve org_code failed")
				return
			}
			for orgIDStr, orgID := range orgIDByStr {
				code, ok := codes[orgID]
				if !ok {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolve_org_code_failed", "resolve org_code failed")
					return
				}
				orgCodes[orgIDStr] = code
			}
		}
		responsePositions := make([]staffingPositionAPIResponse, 0, len(positions))
		for _, p := range positions {
			responsePositions = append(responsePositions, staffingPositionAPIResponse{
				PositionUUID:          p.PositionUUID,
				OrgCode:               orgCodes[p.OrgUnitID],
				ReportsToPositionUUID: p.ReportsToPositionUUID,
				JobCatalogSetID:       p.JobCatalogSetID,
				JobCatalogSetIDAsOf:   p.JobCatalogSetIDAsOf,
				JobProfileUUID:        p.JobProfileUUID,
				JobProfileCode:        p.JobProfileCode,
				Name:                  p.Name,
				LifecycleStatus:       p.LifecycleStatus,
				CapacityFTE:           p.CapacityFTE,
				EffectiveDate:         p.EffectiveAt,
			})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":     asOf,
			"tenant":    tenant.ID,
			"positions": responsePositions,
		})
		return
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		for _, key := range []string{"org_unit_id", "position_id", "reports_to_position_id", "job_profile_id"} {
			if _, ok := raw[key]; ok {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "use org_code/position_uuid")
				return
			}
		}
		var req staffingPositionsAPIRequest
		if err := json.Unmarshal(body, &req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if req.EffectiveDate == "" {
			req.EffectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}

		orgUnitID := ""
		if strings.TrimSpace(req.OrgCode) != "" {
			normalized, err := orgunitpkg.NormalizeOrgCode(req.OrgCode)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
				return
			}
			if orgResolver == nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolver_missing", "orgunit resolver missing")
				return
			}
			resolvedID, err := orgResolver.ResolveOrgID(r.Context(), tenant.ID, normalized)
			if err != nil {
				switch {
				case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
				case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
				default:
					writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
				}
				return
			}
			orgUnitID = strconv.Itoa(resolvedID)
			req.OrgCode = normalized
		} else if strings.TrimSpace(req.PositionUUID) == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
			return
		}

		var p Position
		if strings.TrimSpace(req.PositionUUID) == "" {
			p, err = store.CreatePositionCurrent(r.Context(), tenant.ID, req.EffectiveDate, orgUnitID, req.JobProfileUUID, req.CapacityFTE, req.Name)
		} else {
			p, err = store.UpdatePositionCurrent(r.Context(), tenant.ID, req.PositionUUID, req.EffectiveDate, orgUnitID, req.ReportsToPositionUUID, req.JobProfileUUID, req.CapacityFTE, req.Name, req.LifecycleStatus)
		}
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			switch pgErrorMessage(err) {
			case "STAFFING_IDEMPOTENCY_REUSED":
				status = http.StatusConflict
			default:
				if isStableDBCode(code) {
					status = http.StatusUnprocessableEntity
				}
				if isBadRequestError(err) || isPgInvalidInput(err) {
					status = http.StatusBadRequest
				}
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "upsert failed")
			return
		}
		respOrgCode := ""
		if p.OrgUnitID != "" {
			if orgResolver == nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolver_missing", "orgunit resolver missing")
				return
			}
			orgID, err := strconv.Atoi(p.OrgUnitID)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_id_invalid", "invalid orgunit id")
				return
			}
			code, err := orgResolver.ResolveOrgCode(r.Context(), tenant.ID, orgID)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolve_org_code_failed", "resolve org_code failed")
				return
			}
			respOrgCode = code
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(staffingPositionAPIResponse{
			PositionUUID:          p.PositionUUID,
			OrgCode:               respOrgCode,
			ReportsToPositionUUID: p.ReportsToPositionUUID,
			JobCatalogSetID:       p.JobCatalogSetID,
			JobCatalogSetIDAsOf:   p.JobCatalogSetIDAsOf,
			JobProfileUUID:        p.JobProfileUUID,
			JobProfileCode:        p.JobProfileCode,
			Name:                  p.Name,
			LifecycleStatus:       p.LifecycleStatus,
			CapacityFTE:           p.CapacityFTE,
			EffectiveDate:         p.EffectiveAt,
		})
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleAssignmentsAPI(w http.ResponseWriter, r *http.Request, store AssignmentStore) {
	c := staffingcontrollers.AssignmentsController{
		TenantID: func(ctx context.Context) (string, bool) {
			tenant, ok := currentTenant(ctx)
			return tenant.ID, ok
		},
		NowUTC: time.Now,
		Facade: staffingservices.NewAssignmentsFacade(store),
	}
	c.HandleAssignmentsAPI(w, r)
}

func handleAssignmentEventsCorrectAPI(w http.ResponseWriter, r *http.Request, store AssignmentStore) {
	c := staffingcontrollers.AssignmentsController{
		TenantID: func(ctx context.Context) (string, bool) {
			tenant, ok := currentTenant(ctx)
			return tenant.ID, ok
		},
		NowUTC: time.Now,
		Facade: staffingservices.NewAssignmentsFacade(store),
	}
	c.HandleAssignmentEventsCorrectAPI(w, r)
}

func handleAssignmentEventsRescindAPI(w http.ResponseWriter, r *http.Request, store AssignmentStore) {
	c := staffingcontrollers.AssignmentsController{
		TenantID: func(ctx context.Context) (string, bool) {
			tenant, ok := currentTenant(ctx)
			return tenant.ID, ok
		},
		NowUTC: time.Now,
		Facade: staffingservices.NewAssignmentsFacade(store),
	}
	c.HandleAssignmentEventsRescindAPI(w, r)
}

func findDeprecatedField(values url.Values, keys ...string) string {
	for _, key := range keys {
		if values.Has(key) {
			return key
		}
	}
	return ""
}
