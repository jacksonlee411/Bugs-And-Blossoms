package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/services"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

type TenantIDGetter func(ctx context.Context) (tenantID string, ok bool)

type AssignmentsController struct {
	TenantID TenantIDGetter
	NowUTC   func() time.Time
	Facade   services.AssignmentsFacade
}

type assignmentsAPIRequest struct {
	EffectiveDate string `json:"effective_date"`
	PersonUUID    string `json:"person_uuid"`
	PositionUUID  string `json:"position_uuid"`
	Status        string `json:"status"`
	AllocatedFte  string `json:"allocated_fte"`
}

type assignmentEventsCorrectAPIRequest struct {
	AssignmentUUID      string          `json:"assignment_uuid"`
	TargetEffectiveDate string          `json:"target_effective_date"`
	ReplacementPayload  json.RawMessage `json:"replacement_payload"`
}

type assignmentEventsRescindAPIRequest struct {
	AssignmentUUID      string          `json:"assignment_uuid"`
	TargetEffectiveDate string          `json:"target_effective_date"`
	Payload             json.RawMessage `json:"payload"`
}

func (c AssignmentsController) HandleAssignmentsAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := c.TenantID(r.Context())
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		now := time.Now
		if c.NowUTC != nil {
			now = c.NowUTC
		}
		asOf = now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	switch r.Method {
	case http.MethodGet:
		personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
		if personUUID == "" {
			writeError(w, r, http.StatusBadRequest, "missing_person_uuid", "person_uuid is required")
			return
		}

		assigns, err := c.Facade.ListAssignmentsForPerson(r.Context(), tenantID, asOf, personUUID)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			if isStableDBCode(code) {
				status = http.StatusUnprocessableEntity
			}
			if httperr.IsBadRequest(err) || isPgInvalidInput(err) {
				status = http.StatusBadRequest
			}
			writeError(w, r, status, code, "list failed")
			return
		}
		if assigns == nil {
			assigns = make([]types.Assignment, 0)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":       asOf,
			"tenant":      tenantID,
			"person_uuid": personUUID,
			"assignments": assigns,
		})
		return

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if _, ok := raw["position_id"]; ok {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "use position_uuid")
			return
		}
		var req assignmentsAPIRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if req.EffectiveDate == "" {
			req.EffectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
		req.Status = strings.TrimSpace(req.Status)
		if req.Status != "" && req.Status != "active" && req.Status != "inactive" {
			writeError(w, r, http.StatusBadRequest, "invalid_status", "invalid status")
			return
		}

		a, err := c.Facade.UpsertPrimaryAssignmentForPerson(r.Context(), tenantID, req.EffectiveDate, req.PersonUUID, req.PositionUUID, req.Status, req.AllocatedFte)
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
				if httperr.IsBadRequest(err) || isPgInvalidInput(err) {
					status = http.StatusBadRequest
				}
			}
			writeError(w, r, status, code, "upsert failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(a)
		return

	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func (c AssignmentsController) HandleAssignmentEventsCorrectAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := c.TenantID(r.Context())
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	if _, ok := raw["assignment_id"]; ok {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "use assignment_uuid")
		return
	}
	var req assignmentEventsCorrectAPIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.AssignmentUUID = strings.TrimSpace(req.AssignmentUUID)
	req.TargetEffectiveDate = strings.TrimSpace(req.TargetEffectiveDate)
	if req.AssignmentUUID == "" {
		writeError(w, r, http.StatusBadRequest, "missing_assignment_uuid", "assignment_uuid is required")
		return
	}
	if req.TargetEffectiveDate == "" {
		writeError(w, r, http.StatusBadRequest, "missing_target_effective_date", "target_effective_date is required")
		return
	}
	if _, err := time.Parse("2006-01-02", req.TargetEffectiveDate); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_target_effective_date", "invalid target_effective_date")
		return
	}
	if len(req.ReplacementPayload) > 0 {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(req.ReplacementPayload, &payload); err == nil {
			if _, ok := payload["position_id"]; ok {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "use position_uuid")
				return
			}
		}
	}

	eventID, err := c.Facade.CorrectAssignmentEvent(r.Context(), tenantID, req.AssignmentUUID, req.TargetEffectiveDate, req.ReplacementPayload)
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
			if httperr.IsBadRequest(err) || isPgInvalidInput(err) {
				status = http.StatusBadRequest
			}
		}
		writeError(w, r, status, code, "correct failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"correction_event_uuid": eventID,
		"target_effective_date": req.TargetEffectiveDate,
	})
}

func (c AssignmentsController) HandleAssignmentEventsRescindAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := c.TenantID(r.Context())
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	if _, ok := raw["assignment_id"]; ok {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "use assignment_uuid")
		return
	}
	var req assignmentEventsRescindAPIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.AssignmentUUID = strings.TrimSpace(req.AssignmentUUID)
	req.TargetEffectiveDate = strings.TrimSpace(req.TargetEffectiveDate)
	if req.AssignmentUUID == "" {
		writeError(w, r, http.StatusBadRequest, "missing_assignment_uuid", "assignment_uuid is required")
		return
	}
	if req.TargetEffectiveDate == "" {
		writeError(w, r, http.StatusBadRequest, "missing_target_effective_date", "target_effective_date is required")
		return
	}
	if _, err := time.Parse("2006-01-02", req.TargetEffectiveDate); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_target_effective_date", "invalid target_effective_date")
		return
	}

	eventID, err := c.Facade.RescindAssignmentEvent(r.Context(), tenantID, req.AssignmentUUID, req.TargetEffectiveDate, req.Payload)
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
			if httperr.IsBadRequest(err) || isPgInvalidInput(err) {
				status = http.StatusBadRequest
			}
		}
		writeError(w, r, status, code, "rescind failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rescind_event_uuid":    eventID,
		"target_effective_date": req.TargetEffectiveDate,
	})
}

type errorEnvelope struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	TraceID string            `json:"trace_id"`
	Meta    errorEnvelopeMeta `json:"meta"`
}

type errorEnvelopeMeta struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Code:    code,
		Message: message,
		TraceID: traceIDFromRequest(r),
		Meta: errorEnvelopeMeta{
			Path:   r.URL.Path,
			Method: r.Method,
		},
	})
}

func pgErrorMessage(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		msg := strings.TrimSpace(pgErr.Message)
		if msg != "" {
			return msg
		}
	}
	return "UNKNOWN"
}

func pgErrorCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		return strings.TrimSpace(pgErr.Code)
	}
	return ""
}

func isPgInvalidInput(err error) bool {
	switch pgErrorCode(err) {
	case "22P02", "22003", "22007", "22008":
		return true
	default:
		return false
	}
}

func traceIDFromRequest(r *http.Request) string {
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if len(traceID) != 32 || traceID == "00000000000000000000000000000000" {
		return ""
	}
	for _, ch := range traceID {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return traceID
}

func stablePgMessage(err error) string {
	msg := pgErrorMessage(err)
	if isStableDBCode(msg) {
		return msg
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		switch strings.TrimSpace(pgErr.ConstraintName) {
		case "assignment_versions_position_no_overlap":
			return "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF"
		case "assignment_events_one_per_day_unique":
			return "STAFFING_ASSIGNMENT_ONE_PER_DAY"
		}
	}
	return err.Error()
}

func isStableDBCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || code == "UNKNOWN" {
		return false
	}
	for i := 0; i < len(code); i++ {
		ch := code[i]
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '_' {
			continue
		}
		return false
	}
	if code[0] < 'A' || code[0] > 'Z' {
		return false
	}
	return true
}
