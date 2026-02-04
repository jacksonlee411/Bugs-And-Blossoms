package server

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
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

func handlePositions(w http.ResponseWriter, r *http.Request, orgStore OrgUnitStore, store PositionStore, jobStore JobCatalogStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}
	if legacy := findLegacyField(r.URL.Query(), "position_id", "assignment_id"); legacy != "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "use position_uuid/assignment_uuid")
		return
	}
	if legacy := findLegacyField(r.URL.Query(), "org_unit_id", "position_id", "reports_to_position_id", "job_profile_id"); legacy != "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "use org_code/position_uuid")
		return
	}

	orgCodeInput := r.URL.Query().Get("org_code")
	orgCode := ""
	orgUnitID := ""
	orgMsg := ""
	if orgCodeInput != "" {
		normalized, err := orgunitpkg.NormalizeOrgCode(orgCodeInput)
		if err != nil {
			orgMsg = mergeMsg(orgMsg, "org_code invalid")
		} else {
			orgCode = normalized
			resolvedID, err := orgStore.ResolveOrgID(r.Context(), tenant.ID, normalized)
			if err != nil {
				switch {
				case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
					orgMsg = mergeMsg(orgMsg, "org_code invalid")
				case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
					orgMsg = mergeMsg(orgMsg, "org_code not found")
				default:
					orgMsg = mergeMsg(orgMsg, stablePgMessage(err))
				}
			} else {
				orgUnitID = strconv.Itoa(resolvedID)
			}
		}
	}

	nodes, err := orgStore.ListNodesCurrent(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderPositions(nil, nil, tenant, asOf, orgCode, "", nil, mergeMsg(orgMsg, stablePgMessage(err))))
		return
	}
	positions, err := store.ListPositionsCurrent(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderPositions(nil, nodes, tenant, asOf, orgCode, "", nil, mergeMsg(orgMsg, stablePgMessage(err))))
		return
	}

	var jobProfiles []JobProfile
	jobCatalogMsg := orgMsg
	setID := ""
	if jobStore != nil && orgUnitID != "" {
		if resolver, ok := orgStore.(orgUnitSetIDResolver); ok {
			resolved, err := resolver.ResolveSetID(r.Context(), tenant.ID, orgUnitID, asOf)
			if err != nil {
				jobCatalogMsg = mergeMsg(jobCatalogMsg, stablePgMessage(err))
			} else {
				setID = resolved
				profiles, err := jobStore.ListJobProfiles(r.Context(), tenant.ID, resolved, asOf)
				if err != nil {
					jobCatalogMsg = mergeMsg(jobCatalogMsg, stablePgMessage(err))
				} else {
					jobProfiles = profiles
				}
			}
		}
	}

	switch r.Method {
	case http.MethodGet:
		writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, jobCatalogMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, "bad form")))
			return
		}
		if legacy := findLegacyField(r.Form, "org_unit_id", "position_id", "reports_to_position_id", "job_profile_id"); legacy != "" {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "use org_code/position_uuid")
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, "effective_date 无效: "+err.Error())))
			return
		}

		positionUUID := strings.TrimSpace(r.Form.Get("position_uuid"))
		formOrgCode := r.Form.Get("org_code")
		formOrgUnitID := ""
		if formOrgCode != "" {
			normalized, err := orgunitpkg.NormalizeOrgCode(formOrgCode)
			if err != nil {
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, "org_code invalid")))
				return
			}
			resolvedID, err := orgStore.ResolveOrgID(r.Context(), tenant.ID, normalized)
			if err != nil {
				msg := "org_code invalid"
				switch {
				case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
					msg = "org_code invalid"
				case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
					msg = "org_code not found"
				default:
					msg = stablePgMessage(err)
				}
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, msg)))
				return
			}
			formOrgCode = normalized
			orgCode = normalized
			formOrgUnitID = strconv.Itoa(resolvedID)
		}
		reportsToPositionUUID := strings.TrimSpace(r.Form.Get("reports_to_position_uuid"))
		jobProfileUUID := strings.TrimSpace(r.Form.Get("job_profile_uuid"))
		capacityFTE := strings.TrimSpace(r.Form.Get("capacity_fte"))
		name := strings.TrimSpace(r.Form.Get("name"))
		lifecycleStatus := strings.TrimSpace(r.Form.Get("lifecycle_status"))

		if positionUUID == "" {
			if formOrgUnitID == "" {
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, "org_code is required")))
				return
			}
			if _, err := store.CreatePositionCurrent(r.Context(), tenant.ID, effectiveDate, formOrgUnitID, jobProfileUUID, capacityFTE, name); err != nil {
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, stablePgMessage(err))))
				return
			}
		} else {
			if _, err := store.UpdatePositionCurrent(r.Context(), tenant.ID, positionUUID, effectiveDate, formOrgUnitID, reportsToPositionUUID, jobProfileUUID, capacityFTE, name, lifecycleStatus); err != nil {
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, orgCode, setID, jobProfiles, mergeMsg(jobCatalogMsg, stablePgMessage(err))))
				return
			}
		}

		redirectURL := "/org/positions?as_of=" + url.QueryEscape(effectiveDate)
		if formOrgCode != "" {
			redirectURL += "&org_code=" + url.QueryEscape(formOrgCode)
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
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
	if legacy := findLegacyField(r.URL.Query(), "org_unit_id", "position_id", "reports_to_position_id", "job_profile_id"); legacy != "" {
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

func handleAssignments(w http.ResponseWriter, r *http.Request, positionStore PositionStore, assignmentStore AssignmentStore, personStore PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	positions, err := positionStore.ListPositionsCurrent(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderAssignments(nil, nil, tenant, asOf, "", "", "", stablePgMessage(err)))
		return
	}
	positions = filterActivePositions(positions)

	pernr := strings.TrimSpace(r.URL.Query().Get("pernr"))
	personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
	displayName := ""
	if pernr != "" && personUUID == "" {
		p, err := personStore.FindPersonByPernr(r.Context(), tenant.ID, pernr)
		if err != nil {
			writePage(w, r, renderAssignments(nil, positions, tenant, asOf, "", pernr, "", stablePgMessage(err)))
			return
		}
		personUUID = p.UUID
		pernr = p.Pernr
		displayName = p.DisplayName
	}

	list := func() ([]Assignment, string) {
		if personUUID == "" {
			return nil, ""
		}
		assigns, err := assignmentStore.ListAssignmentsForPerson(r.Context(), tenant.ID, asOf, personUUID)
		if err != nil {
			return nil, stablePgMessage(err)
		}
		return assigns, ""
	}

	switch r.Method {
	case http.MethodGet:
		assigns, errMsg := list()
		writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, errMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "bad form")))
			return
		}
		if legacy := findLegacyField(r.Form, "position_id", "assignment_id"); legacy != "" {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "use position_uuid/assignment_uuid")
			return
		}

		action := strings.TrimSpace(r.Form.Get("action"))

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "effective_date 无效: "+err.Error())))
			return
		}

		postPernr := strings.TrimSpace(r.Form.Get("pernr"))
		postPersonUUID := strings.TrimSpace(r.Form.Get("person_uuid"))
		positionID := strings.TrimSpace(r.Form.Get("position_uuid"))
		allocatedFte := strings.TrimSpace(r.Form.Get("allocated_fte"))
		status := strings.TrimSpace(r.Form.Get("status"))
		if status != "" && status != "active" && status != "inactive" {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "status 无效")))
			return
		}

		if postPernr != "" {
			p, err := personStore.FindPersonByPernr(r.Context(), tenant.ID, postPernr)
			if err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, stablePgMessage(err))))
				return
			}
			if postPersonUUID != "" && postPersonUUID != p.UUID {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "pernr/person_uuid 不一致")))
				return
			}
			postPersonUUID = p.UUID
			postPernr = p.Pernr
			displayName = p.DisplayName
		} else if postPersonUUID == "" {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "pernr is required")))
			return
		}

		switch action {
		case "":
			if _, err := assignmentStore.UpsertPrimaryAssignmentForPerson(r.Context(), tenant.ID, effectiveDate, postPersonUUID, positionID, status, allocatedFte); err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, stablePgMessage(err))))
				return
			}
		case "correct_event":
			assignmentID := strings.TrimSpace(r.Form.Get("assignment_uuid"))
			targetEffectiveDate := strings.TrimSpace(r.Form.Get("target_effective_date"))
			if assignmentID == "" || targetEffectiveDate == "" {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, "assignment_uuid/target_effective_date is required")))
				return
			}
			if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, "target_effective_date 无效")))
				return
			}
			if positionID == "" {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, "position_uuid is required")))
				return
			}

			replacementPayload := map[string]any{
				"position_uuid": positionID,
			}
			if allocatedFte != "" {
				replacementPayload["allocated_fte"] = allocatedFte
			}
			if status != "" {
				replacementPayload["status"] = status
			}
			raw, _ := json.Marshal(replacementPayload)

			if _, err := assignmentStore.CorrectAssignmentEvent(r.Context(), tenant.ID, assignmentID, targetEffectiveDate, raw); err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, stablePgMessage(err))))
				return
			}
		case "rescind_event":
			assignmentID := strings.TrimSpace(r.Form.Get("assignment_uuid"))
			targetEffectiveDate := strings.TrimSpace(r.Form.Get("target_effective_date"))
			note := strings.TrimSpace(r.Form.Get("note"))
			if assignmentID == "" || targetEffectiveDate == "" {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, "assignment_uuid/target_effective_date is required")))
				return
			}
			if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, "target_effective_date 无效")))
				return
			}

			payload := map[string]any{}
			if note != "" {
				payload["note"] = note
			}
			raw, _ := json.Marshal(payload)

			if _, err := assignmentStore.RescindAssignmentEvent(r.Context(), tenant.ID, assignmentID, targetEffectiveDate, raw); err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, stablePgMessage(err))))
				return
			}
		default:
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, "unknown action")))
			return
		}

		if postPernr != "" {
			http.Redirect(w, r, "/org/assignments?as_of="+url.QueryEscape(effectiveDate)+"&pernr="+url.QueryEscape(postPernr), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/org/assignments?as_of="+url.QueryEscape(effectiveDate)+"&person_uuid="+url.QueryEscape(postPersonUUID), http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func findLegacyField(values url.Values, keys ...string) string {
	for _, key := range keys {
		if values.Has(key) {
			return key
		}
	}
	return ""
}

func mergeMsg(a string, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "；" + b
}

func renderPositions(
	positions []Position,
	nodes []OrgUnitNode,
	tenant Tenant,
	asOf string,
	orgCode string,
	setID string,
	jobProfiles []JobProfile,
	errMsg string,
) string {
	b := strings.Builder{}
	action := "/org/positions?as_of=" + url.QueryEscape(asOf)
	sortedNodes := nodes
	if len(nodes) > 1 {
		sortedNodes = append([]OrgUnitNode(nil), nodes...)
		sort.Slice(sortedNodes, func(i, j int) bool { return sortedNodes[i].Name < sortedNodes[j].Name })
	}
	orgCodeByID := make(map[string]string, len(nodes))
	nodeByCode := make(map[string]OrgUnitNode, len(nodes))
	for _, n := range nodes {
		orgCodeByID[n.ID] = n.OrgCode
		if n.OrgCode != "" {
			nodeByCode[n.OrgCode] = n
		}
	}
	orgCodeLabel := "(not set)"
	if orgCode != "" {
		orgCodeLabel = orgCode
		if n, ok := nodeByCode[orgCode]; ok {
			label := n.Name + " (" + n.OrgCode + ")"
			if n.IsBusinessUnit {
				label = label + " [BU]"
			}
			orgCodeLabel = label
		}
	}
	b.WriteString("<h1>Staffing / Positions</h1>")
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.Name) + `</code> (<code>` + html.EscapeString(tenant.ID) + `</code>)</p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code> | <a href="/org/assignments?as_of=` + url.QueryEscape(asOf) + `">Assignments</a></p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Org Unit Context</h2>`)
	b.WriteString(`<form method="GET" action="/org/positions" hx-get="/org/positions" hx-target="#content" hx-push-url="true" hx-trigger="change">`)
	b.WriteString(`<input type="hidden" name="as_of" value="` + html.EscapeString(asOf) + `" />`)
	b.WriteString(`<label>Org Unit <select name="org_code">`)
	b.WriteString(`<option value="">(not set)</option>`)
	for _, n := range sortedNodes {
		selected := ""
		if n.OrgCode == orgCode {
			selected = " selected"
		}
		label := n.Name + " (" + n.OrgCode + ")"
		if n.IsBusinessUnit {
			label = label + " [BU]"
		}
		b.WriteString(`<option value="` + html.EscapeString(n.OrgCode) + `"` + selected + `>` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Load</button>`)
	b.WriteString(`</form>`)
	if orgCode == "" {
		b.WriteString(`<p style="color:#555">请选择 Org Unit 以加载可用的 Job Profile。</p>`)
	}
	if setID != "" {
		b.WriteString(`<p>SetID: <code>` + html.EscapeString(setID) + `</code></p>`)
	}

	b.WriteString(`<h2>Create</h2>`)
	b.WriteString(`<form method="POST" action="` + html.EscapeString(action) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Org Unit <input type="text" value="` + html.EscapeString(orgCodeLabel) + `" disabled /></label><br/>`)
	b.WriteString(`<input type="hidden" name="org_code" value="` + html.EscapeString(orgCode) + `" />`)
	b.WriteString(`<label>Job Profile <select name="job_profile_uuid">`)
	b.WriteString(`<option value="">(not set)</option>`)
	for _, jp := range jobProfiles {
		label := jp.JobProfileCode + " (" + jp.JobProfileUUID + ")"
		b.WriteString(`<option value="` + html.EscapeString(jp.JobProfileUUID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Capacity FTE <input type="number" name="capacity_fte" step="0.01" min="0.01" value="1.0" /></label><br/>`)
	b.WriteString(`<label>Name <input type="text" name="name" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Update / Disable</h2>`)
	b.WriteString(`<form method="POST" action="` + html.EscapeString(action) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Position <select name="position_uuid">`)
	if len(positions) == 0 {
		b.WriteString(`<option value="">(no positions)</option>`)
	} else {
		for _, p := range positions {
			label := p.PositionUUID
			if p.Name != "" {
				label = p.Name + " (" + p.PositionUUID + ")"
			}
			b.WriteString(`<option value="` + html.EscapeString(p.PositionUUID) + `">` + html.EscapeString(label) + `</option>`)
		}
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Org Unit <select name="org_code">`)
	b.WriteString(`<option value="">(no change)</option>`)
	for _, n := range sortedNodes {
		label := n.Name + " (" + n.OrgCode + ")"
		if n.IsBusinessUnit {
			label = label + " [BU]"
		}
		b.WriteString(`<option value="` + html.EscapeString(n.OrgCode) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Reports To <select name="reports_to_position_uuid">`)
	b.WriteString(`<option value="">(no change)</option>`)
	for _, p := range filterActivePositions(positions) {
		label := p.PositionUUID
		if p.Name != "" {
			label = p.Name + " (" + p.PositionUUID + ")"
		}
		b.WriteString(`<option value="` + html.EscapeString(p.PositionUUID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Job Profile <select name="job_profile_uuid">` +
		`<option value="">(no change)</option>`)
	for _, jp := range jobProfiles {
		label := jp.JobProfileCode + " (" + jp.JobProfileUUID + ")"
		b.WriteString(`<option value="` + html.EscapeString(jp.JobProfileUUID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Capacity FTE <input type="number" name="capacity_fte" step="0.01" min="0.01" placeholder="(no change)" /></label><br/>`)
	b.WriteString(`<label>Name <input type="text" name="name" placeholder="(no change)" /></label><br/>`)
	b.WriteString(`<label>Lifecycle <select name="lifecycle_status">` +
		`<option value="">(no change)</option>` +
		`<option value="active">active</option>` +
		`<option value="disabled">disabled</option>` +
		`</select></label><br/>`)
	b.WriteString(`<button type="submit">Update</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Current</h2>`)
	if len(positions) == 0 {
		b.WriteString("<p>(empty)</p>")
		return b.String()
	}

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr>` +
		`<th>effective_date</th><th>position_uuid</th><th>reports_to_position_uuid</th><th>jobcatalog_setid</th><th>jobcatalog_setid_as_of</th><th>job_profile</th><th>capacity_fte</th><th>lifecycle_status</th><th>org_code</th><th>name</th>` +
		`</tr></thead><tbody>`)
	for _, p := range positions {
		jobProfileLabel := ""
		if p.JobProfileUUID != "" {
			jobProfileLabel = p.JobProfileUUID
			if p.JobProfileCode != "" {
				jobProfileLabel = p.JobProfileCode + " (" + p.JobProfileUUID + ")"
			}
		}
		b.WriteString(`<tr><td><code>` + html.EscapeString(p.EffectiveAt) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.PositionUUID) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.ReportsToPositionUUID) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.JobCatalogSetID) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.JobCatalogSetIDAsOf) + `</code></td>` +
			`<td><code>` + html.EscapeString(jobProfileLabel) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.CapacityFTE) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.LifecycleStatus) + `</code></td>` +
			`<td><code>` + html.EscapeString(orgCodeByID[p.OrgUnitID]) + `</code></td>` +
			`<td>` + html.EscapeString(p.Name) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func filterActivePositions(in []Position) []Position {
	out := make([]Position, 0, len(in))
	for _, p := range in {
		if p.LifecycleStatus == "active" {
			out = append(out, p)
		}
	}
	return out
}

func renderAssignments(assignments []Assignment, positions []Position, tenant Tenant, asOf string, personUUID string, pernr string, displayName string, errMsg string) string {
	b := strings.Builder{}
	b.WriteString("<h1>Staffing / Assignments</h1>")
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.Name) + `</code> (<code>` + html.EscapeString(tenant.ID) + `</code>)</p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code> | <a href="/org/positions?as_of=` + url.QueryEscape(asOf) + `">Positions</a></p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Select Person</h2>`)
	b.WriteString(`<form method="GET" action="/org/assignments">`)
	b.WriteString(`<input type="hidden" name="as_of" value="` + html.EscapeString(asOf) + `" />`)
	b.WriteString(`<label>Pernr <input type="text" name="pernr" value="` + html.EscapeString(pernr) + `" /></label> `)
	b.WriteString(`<button type="submit">Load</button>`)
	b.WriteString(`</form>`)

	if personUUID != "" {
		label := pernr
		if displayName != "" {
			label = pernr + " / " + displayName
		}
		b.WriteString(`<p>Person: <code>` + html.EscapeString(label) + `</code> (<code>` + html.EscapeString(personUUID) + `</code>)</p>`)
	}

	b.WriteString(`<h2>Upsert Primary</h2>`)
	b.WriteString(`<form method="POST" action="/org/assignments?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Pernr <input type="text" name="pernr" value="` + html.EscapeString(pernr) + `" /></label><br/>`)
	b.WriteString(`<input type="hidden" name="person_uuid" value="` + html.EscapeString(personUUID) + `" />`)
	b.WriteString(`<label>Position <select name="position_uuid">`)
	if len(positions) == 0 {
		b.WriteString(`<option value="">(no positions)</option>`)
	} else {
		for _, p := range positions {
			label := p.PositionUUID
			if p.Name != "" {
				label = p.Name + " (" + p.PositionUUID + ")"
			}
			b.WriteString(`<option value="` + html.EscapeString(p.PositionUUID) + `">` + html.EscapeString(label) + `</option>`)
		}
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Status <select name="status">`)
	b.WriteString(`<option value="">active</option>`)
	b.WriteString(`<option value="inactive">inactive</option>`)
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Allocated FTE <input type="number" name="allocated_fte" step="0.01" min="0.01" max="1.00" value="1.0" /></label><br/>`)
	b.WriteString(`<button type="submit">Submit</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Timeline</h2>`)
	if personUUID == "" {
		b.WriteString("<p>(select a person first)</p>")
		return b.String()
	}
	if len(assignments) == 0 {
		b.WriteString("<p>(empty)</p>")
		return b.String()
	}

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr>` +
		`<th>effective_date</th><th>assignment_uuid</th><th>position_uuid</th><th>status</th><th>actions</th>` +
		`</tr></thead><tbody>`)
	for _, a := range assignments {
		b.WriteString(`<tr><td><code>` + html.EscapeString(a.EffectiveAt) + `</code></td>` +
			`<td><code>` + html.EscapeString(a.AssignmentUUID) + `</code></td>` +
			`<td><code>` + html.EscapeString(a.PositionUUID) + `</code></td>` +
			`<td>` + html.EscapeString(a.Status) + `</td>` +
			`<td>` +
			`<form method="POST" action="/org/assignments?as_of=` + url.QueryEscape(asOf) + `" style="display:inline-block;margin-right:8px">` +
			`<input type="hidden" name="action" value="rescind_event" />` +
			`<input type="hidden" name="effective_date" value="` + html.EscapeString(asOf) + `" />` +
			`<input type="hidden" name="pernr" value="` + html.EscapeString(pernr) + `" />` +
			`<input type="hidden" name="person_uuid" value="` + html.EscapeString(personUUID) + `" />` +
			`<input type="hidden" name="assignment_uuid" value="` + html.EscapeString(a.AssignmentUUID) + `" />` +
			`<input type="hidden" name="target_effective_date" value="` + html.EscapeString(a.EffectiveAt) + `" />` +
			`<button type="submit">Rescind</button>` +
			`</form>` +
			`<form method="POST" action="/org/assignments?as_of=` + url.QueryEscape(asOf) + `" style="display:inline-block">` +
			`<input type="hidden" name="action" value="correct_event" />` +
			`<input type="hidden" name="effective_date" value="` + html.EscapeString(asOf) + `" />` +
			`<input type="hidden" name="pernr" value="` + html.EscapeString(pernr) + `" />` +
			`<input type="hidden" name="person_uuid" value="` + html.EscapeString(personUUID) + `" />` +
			`<input type="hidden" name="assignment_uuid" value="` + html.EscapeString(a.AssignmentUUID) + `" />` +
			`<input type="hidden" name="target_effective_date" value="` + html.EscapeString(a.EffectiveAt) + `" />` +
			`<input type="hidden" name="position_uuid" value="` + html.EscapeString(a.PositionUUID) + `" />` +
			`<label>allocated_fte <input type="number" name="allocated_fte" step="0.01" min="0.01" max="1.00" style="width:80px" /></label> ` +
			`<label>status <select name="status">` +
			`<option value=""></option>` +
			`<option value="active">active</option>` +
			`<option value="inactive">inactive</option>` +
			`</select></label> ` +
			`<button type="submit">Correct</button>` +
			`</form>` +
			`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
