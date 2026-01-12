package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
	PositionID    string `json:"position_id"`
	Status        string `json:"status"`
	BaseSalary    string `json:"base_salary"`
	AllocatedFte  string `json:"allocated_fte"`
}

type assignmentEventsCorrectAPIRequest struct {
	AssignmentID        string          `json:"assignment_id"`
	TargetEffectiveDate string          `json:"target_effective_date"`
	ReplacementPayload  json.RawMessage `json:"replacement_payload"`
}

type assignmentEventsRescindAPIRequest struct {
	AssignmentID        string          `json:"assignment_id"`
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":       asOf,
			"tenant":      tenantID,
			"person_uuid": personUUID,
			"assignments": assigns,
		})
		return

	case http.MethodPost:
		var req assignmentsAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

		a, err := c.Facade.UpsertPrimaryAssignmentForPerson(r.Context(), tenantID, req.EffectiveDate, req.PersonUUID, req.PositionID, req.Status, req.BaseSalary, req.AllocatedFte)
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

	var req assignmentEventsCorrectAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.AssignmentID = strings.TrimSpace(req.AssignmentID)
	req.TargetEffectiveDate = strings.TrimSpace(req.TargetEffectiveDate)
	if req.AssignmentID == "" {
		writeError(w, r, http.StatusBadRequest, "missing_assignment_id", "assignment_id is required")
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

	eventID, err := c.Facade.CorrectAssignmentEvent(r.Context(), tenantID, req.AssignmentID, req.TargetEffectiveDate, req.ReplacementPayload)
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
		"correction_event_id":   eventID,
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

	var req assignmentEventsRescindAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.AssignmentID = strings.TrimSpace(req.AssignmentID)
	req.TargetEffectiveDate = strings.TrimSpace(req.TargetEffectiveDate)
	if req.AssignmentID == "" {
		writeError(w, r, http.StatusBadRequest, "missing_assignment_id", "assignment_id is required")
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

	eventID, err := c.Facade.RescindAssignmentEvent(r.Context(), tenantID, req.AssignmentID, req.TargetEffectiveDate, req.Payload)
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
		"rescind_event_id":      eventID,
		"target_effective_date": req.TargetEffectiveDate,
	})
}

type errorEnvelope struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Meta      errorEnvelopeMeta `json:"meta"`
}

type errorEnvelopeMeta struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Code:      code,
		Message:   message,
		RequestID: "",
		Meta: errorEnvelopeMeta{
			Path:   r.URL.Path,
			Method: r.Method,
		},
	})
}

func pgErrorMessage(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		msg := strings.TrimSpace(pgErr.Message)
		if msg != "" {
			return msg
		}
	}
	return "UNKNOWN"
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
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

func stablePgMessage(err error) string {
	msg := pgErrorMessage(err)
	if isStableDBCode(msg) {
		return msg
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
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
