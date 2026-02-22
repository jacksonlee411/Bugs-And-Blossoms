package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type dictReleasePayload struct {
	SourceTenantID string `json:"source_tenant_id"`
	AsOf           string `json:"as_of"`
	ReleaseID      string `json:"release_id"`
	RequestID      string `json:"request_id"`
	MaxConflicts   int    `json:"max_conflicts"`
}

func handleDictReleaseAPI(w http.ResponseWriter, r *http.Request, releaseStore DictBaselineReleaseStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if releaseStore == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "dict_release_store_missing", "dict release store missing")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var payload dictReleasePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	payload.SourceTenantID = strings.TrimSpace(payload.SourceTenantID)
	payload.AsOf = strings.TrimSpace(payload.AsOf)
	payload.ReleaseID = strings.TrimSpace(payload.ReleaseID)
	payload.RequestID = strings.TrimSpace(payload.RequestID)

	req := DictBaselineReleaseRequest{
		SourceTenantID: payload.SourceTenantID,
		TargetTenantID: tenant.ID,
		AsOf:           payload.AsOf,
		ReleaseID:      payload.ReleaseID,
		RequestID:      payload.RequestID,
		Operator:       orgUnitInitiatorUUID(r.Context(), tenant.ID),
		Initiator:      orgUnitInitiatorUUID(r.Context(), tenant.ID),
		MaxConflicts:   payload.MaxConflicts,
	}

	preview, err := releaseStore.PreviewBaseline(r.Context(), req)
	if err != nil {
		writeDictReleaseAPIError(w, r, err, "dict_release_preview_failed")
		return
	}
	if preview.MissingDictCount > 0 || preview.DictNameMismatchCount > 0 || preview.MissingValueCount > 0 || preview.ValueLabelMismatchCount > 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(preview)
		return
	}

	result, err := releaseStore.PublishBaseline(r.Context(), req)
	if err != nil {
		writeDictReleaseAPIError(w, r, err, "dict_release_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

func handleDictReleasePreviewAPI(w http.ResponseWriter, r *http.Request, releaseStore DictBaselineReleaseStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if releaseStore == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "dict_release_store_missing", "dict release store missing")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var payload dictReleasePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	payload.SourceTenantID = strings.TrimSpace(payload.SourceTenantID)
	payload.AsOf = strings.TrimSpace(payload.AsOf)
	payload.ReleaseID = strings.TrimSpace(payload.ReleaseID)

	preview, err := releaseStore.PreviewBaseline(r.Context(), DictBaselineReleaseRequest{
		SourceTenantID: payload.SourceTenantID,
		TargetTenantID: tenant.ID,
		AsOf:           payload.AsOf,
		ReleaseID:      payload.ReleaseID,
		MaxConflicts:   payload.MaxConflicts,
		Operator:       orgUnitInitiatorUUID(r.Context(), tenant.ID),
		Initiator:      orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeDictReleaseAPIError(w, r, err, "dict_release_preview_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(preview)
}

func writeDictReleaseAPIError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	code := dictErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case "invalid_as_of", "dict_release_id_required", "dict_release_source_invalid", "dict_release_target_required", "invalid_request":
		status = http.StatusBadRequest
	case "dict_baseline_not_ready", "dict_value_conflict", "dict_code_conflict", "dict_release_payload_invalid":
		status = http.StatusConflict
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, defaultCode)
}
