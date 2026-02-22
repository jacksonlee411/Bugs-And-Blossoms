package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type jobCatalogViewAPI struct {
	HasSelection bool   `json:"has_selection"`
	ReadOnly     bool   `json:"read_only"`
	PackageCode  string `json:"package_code,omitempty"`
	SetID        string `json:"setid,omitempty"`
	OwnerSetID   string `json:"owner_setid,omitempty"`
}

type jobFamilyGroupAPIItem struct {
	UUID         string `json:"job_family_group_uuid"`
	Code         string `json:"job_family_group_code"`
	Name         string `json:"name"`
	IsActive     bool   `json:"is_active"`
	EffectiveDay string `json:"effective_day"`
}

type jobFamilyAPIItem struct {
	UUID         string `json:"job_family_uuid"`
	Code         string `json:"job_family_code"`
	GroupCode    string `json:"job_family_group_code"`
	Name         string `json:"name"`
	IsActive     bool   `json:"is_active"`
	EffectiveDay string `json:"effective_day"`
}

type jobLevelAPIItem struct {
	UUID         string `json:"job_level_uuid"`
	Code         string `json:"job_level_code"`
	Name         string `json:"name"`
	IsActive     bool   `json:"is_active"`
	EffectiveDay string `json:"effective_day"`
}

type jobProfileAPIItem struct {
	UUID              string `json:"job_profile_uuid"`
	Code              string `json:"job_profile_code"`
	Name              string `json:"name"`
	IsActive          bool   `json:"is_active"`
	EffectiveDay      string `json:"effective_day"`
	FamilyCodesCSV    string `json:"family_codes_csv"`
	PrimaryFamilyCode string `json:"primary_family_code"`
}

func handleJobCatalogAPI(w http.ResponseWriter, r *http.Request, setidStore jobCatalogSetIDStore, store JobCatalogStore) {
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
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "as_of required")
		return
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	packageCode := normalizePackageCode(r.URL.Query().Get("package_code"))
	setID := normalizeSetID(r.URL.Query().Get("setid"))
	if packageCode != "" && setID != "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "package_code and setid are mutually exclusive")
		return
	}

	view := jobCatalogViewAPI{HasSelection: false}
	if packageCode != "" || setID != "" {
		v, errMsg := resolveJobCatalogView(r.Context(), store, setidStore, tenant.ID, asOf, packageCode, setID)
		if errMsg != "" {
			status := jobCatalogStatusForError(errMsg)
			code := strings.TrimSpace(errMsg)
			if code == "" {
				code = "jobcatalog_view_invalid"
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, errMsg)
			return
		}

		view.HasSelection = v.HasSelection
		view.ReadOnly = v.ReadOnly
		view.PackageCode = v.PackageCode
		view.SetID = v.SetID
		view.OwnerSetID = v.OwnerSetID

		listSetID := v.listSetID()

		groups, err := store.ListJobFamilyGroups(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			if isStableDBCode(code) {
				status = http.StatusUnprocessableEntity
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "list groups failed")
			return
		}
		families, err := store.ListJobFamilies(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			if isStableDBCode(code) {
				status = http.StatusUnprocessableEntity
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "list families failed")
			return
		}
		levels, err := store.ListJobLevels(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			if isStableDBCode(code) {
				status = http.StatusUnprocessableEntity
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "list levels failed")
			return
		}
		profiles, err := store.ListJobProfiles(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			if isStableDBCode(code) {
				status = http.StatusUnprocessableEntity
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "list profiles failed")
			return
		}

		outGroups := make([]jobFamilyGroupAPIItem, 0, len(groups))
		for _, g := range groups {
			outGroups = append(outGroups, jobFamilyGroupAPIItem{
				UUID:         g.JobFamilyGroupUUID,
				Code:         g.JobFamilyGroupCode,
				Name:         g.Name,
				IsActive:     g.IsActive,
				EffectiveDay: g.EffectiveDay,
			})
		}
		outFamilies := make([]jobFamilyAPIItem, 0, len(families))
		for _, f := range families {
			outFamilies = append(outFamilies, jobFamilyAPIItem{
				UUID:         f.JobFamilyUUID,
				Code:         f.JobFamilyCode,
				GroupCode:    f.JobFamilyGroupCode,
				Name:         f.Name,
				IsActive:     f.IsActive,
				EffectiveDay: f.EffectiveDay,
			})
		}
		outLevels := make([]jobLevelAPIItem, 0, len(levels))
		for _, l := range levels {
			outLevels = append(outLevels, jobLevelAPIItem{
				UUID:         l.JobLevelUUID,
				Code:         l.JobLevelCode,
				Name:         l.Name,
				IsActive:     l.IsActive,
				EffectiveDay: l.EffectiveDay,
			})
		}
		outProfiles := make([]jobProfileAPIItem, 0, len(profiles))
		for _, p := range profiles {
			outProfiles = append(outProfiles, jobProfileAPIItem{
				UUID:              p.JobProfileUUID,
				Code:              p.JobProfileCode,
				Name:              p.Name,
				IsActive:          p.IsActive,
				EffectiveDay:      p.EffectiveDay,
				FamilyCodesCSV:    p.FamilyCodesCSV,
				PrimaryFamilyCode: p.PrimaryFamilyCode,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":             asOf,
			"tenant_id":         tenant.ID,
			"view":              view,
			"job_family_groups": outGroups,
			"job_families":      outFamilies,
			"job_levels":        outLevels,
			"job_profiles":      outProfiles,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"as_of":             asOf,
		"tenant_id":         tenant.ID,
		"view":              view,
		"job_family_groups": []jobFamilyGroupAPIItem{},
		"job_families":      []jobFamilyAPIItem{},
		"job_levels":        []jobLevelAPIItem{},
		"job_profiles":      []jobProfileAPIItem{},
	})
}

type jobCatalogWriteRequest struct {
	PackageCode    string `json:"package_code"`
	EffectiveDate  string `json:"effective_date"`
	RequestAction  string `json:"action"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	GroupCode      string `json:"group_code"`
	FamilyCodesCSV string `json:"family_codes_csv"`
	PrimaryFamily  string `json:"primary_family_code"`
}

func handleJobCatalogWriteAPI(w http.ResponseWriter, r *http.Request, setidStore jobCatalogSetIDStore, store JobCatalogStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req jobCatalogWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.PackageCode = normalizePackageCode(req.PackageCode)
	req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
	if req.EffectiveDate == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "effective_date required")
		return
	}
	if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
		return
	}

	action := strings.TrimSpace(strings.ToLower(req.RequestAction))
	if action == "" {
		action = "create_job_family_group"
	}
	if req.PackageCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "package_code required")
		return
	}

	view, errMsg := resolveJobCatalogView(r.Context(), store, setidStore, tenant.ID, req.EffectiveDate, req.PackageCode, "")
	if errMsg != "" {
		status := jobCatalogStatusForError(errMsg)
		code := strings.TrimSpace(errMsg)
		if code == "" {
			code = "jobcatalog_view_invalid"
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, errMsg)
		return
	}
	ownerSetID := view.OwnerSetID

	switch action {
	case "create_job_family_group":
		req.Code = strings.TrimSpace(req.Code)
		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		if req.Code == "" || req.Name == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "code/name required")
			return
		}
		if err := store.CreateJobFamilyGroup(r.Context(), tenant.ID, ownerSetID, req.EffectiveDate, req.Code, req.Name, req.Description); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "create group failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_code":          req.PackageCode,
			"owner_setid":           ownerSetID,
			"effective_date":        req.EffectiveDate,
			"job_family_group_code": strings.ToUpper(req.Code),
		})
		return

	case "create_job_family":
		req.Code = strings.TrimSpace(req.Code)
		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		req.GroupCode = strings.TrimSpace(req.GroupCode)
		if req.Code == "" || req.Name == "" || req.GroupCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "code/name/group_code required")
			return
		}
		if err := store.CreateJobFamily(r.Context(), tenant.ID, ownerSetID, req.EffectiveDate, req.Code, req.Name, req.Description, req.GroupCode); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "create family failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_code":    req.PackageCode,
			"owner_setid":     ownerSetID,
			"effective_date":  req.EffectiveDate,
			"job_family_code": strings.ToUpper(req.Code),
		})
		return

	case "update_job_family_group":
		familyCode := strings.TrimSpace(req.Code)
		groupCode := strings.TrimSpace(req.GroupCode)
		if familyCode == "" || groupCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "code/group_code required")
			return
		}
		if err := store.UpdateJobFamilyGroup(r.Context(), tenant.ID, ownerSetID, req.EffectiveDate, familyCode, groupCode); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "update family group failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_code":          req.PackageCode,
			"owner_setid":           ownerSetID,
			"effective_date":        req.EffectiveDate,
			"job_family_code":       strings.ToUpper(familyCode),
			"job_family_group_code": strings.ToUpper(groupCode),
		})
		return

	case "create_job_level":
		req.Code = strings.TrimSpace(req.Code)
		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		if req.Code == "" || req.Name == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "code/name required")
			return
		}
		if err := store.CreateJobLevel(r.Context(), tenant.ID, ownerSetID, req.EffectiveDate, req.Code, req.Name, req.Description); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "create level failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_code":   req.PackageCode,
			"owner_setid":    ownerSetID,
			"effective_date": req.EffectiveDate,
			"job_level_code": strings.ToUpper(req.Code),
		})
		return

	case "create_job_profile":
		req.Code = strings.TrimSpace(req.Code)
		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		req.FamilyCodesCSV = strings.TrimSpace(req.FamilyCodesCSV)
		req.PrimaryFamily = strings.TrimSpace(req.PrimaryFamily)
		familyCodes := splitCSV(req.FamilyCodesCSV)
		if req.Code == "" || req.Name == "" || len(familyCodes) == 0 || req.PrimaryFamily == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "code/name/family_codes_csv/primary_family_code required")
			return
		}
		if err := store.CreateJobProfile(r.Context(), tenant.ID, ownerSetID, req.EffectiveDate, req.Code, req.Name, req.Description, familyCodes, req.PrimaryFamily); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, stablePgMessage(err), "create profile failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package_code":     req.PackageCode,
			"owner_setid":      ownerSetID,
			"effective_date":   req.EffectiveDate,
			"job_profile_code": strings.ToUpper(req.Code),
		})
		return

	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "unknown action")
		return
	}
}
