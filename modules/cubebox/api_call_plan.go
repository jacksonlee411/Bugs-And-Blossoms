package cubebox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrAPICallPlanSchemaConstrainedDecodeFailed = errors.New("CUBEBOX_API_CALL_PLAN_SCHEMA_CONSTRAINED_DECODE_FAILED")
var ErrAPICallPlanBoundaryViolation = errors.New("CUBEBOX_API_CALL_PLAN_BOUNDARY_VIOLATION")

type APICallPlan struct {
	Calls []APICallStep `json:"calls"`
}

type APICallStep struct {
	ID          string         `json:"id"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Params      map[string]any `json:"params"`
	ResultFocus []string       `json:"result_focus,omitempty"`
	DependsOn   []string       `json:"depends_on"`
}

func DecodeAPICallPlan(raw []byte) (APICallPlan, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return APICallPlan{}, wrapAPICallPlanDecodeError("empty payload")
	}

	var plan APICallPlan
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return APICallPlan{}, wrapAPICallPlanDecodeError(err.Error())
	}
	if err := ensureSingleAPICallPlanJSONObject(dec); err != nil {
		return APICallPlan{}, wrapAPICallPlanDecodeError(err.Error())
	}
	if err := ValidateAPICallPlan(plan); err != nil {
		return APICallPlan{}, err
	}
	return normalizeAPICallPlan(plan), nil
}

func ValidateAPICallPlan(plan APICallPlan) error {
	if len(plan.Calls) == 0 {
		return wrapAPICallPlanBoundaryError("calls required")
	}

	seenIDs := make(map[string]struct{}, len(plan.Calls))
	for i, call := range plan.Calls {
		if err := validateAPICallStep(call, i, plan.Calls, seenIDs); err != nil {
			return err
		}
		seenIDs[strings.TrimSpace(call.ID)] = struct{}{}
	}
	return nil
}

func normalizeAPICallPlan(plan APICallPlan) APICallPlan {
	normalized := plan
	if normalized.Calls == nil {
		normalized.Calls = []APICallStep{}
	}
	for i := range normalized.Calls {
		normalized.Calls[i].ID = strings.TrimSpace(normalized.Calls[i].ID)
		normalized.Calls[i].Method = strings.ToUpper(strings.TrimSpace(normalized.Calls[i].Method))
		normalized.Calls[i].Path = normalizeAPICallPath(normalized.Calls[i].Path)
		if normalized.Calls[i].Params == nil {
			normalized.Calls[i].Params = map[string]any{}
		}
		if normalized.Calls[i].ResultFocus == nil {
			normalized.Calls[i].ResultFocus = []string{}
		}
		if normalized.Calls[i].DependsOn == nil {
			normalized.Calls[i].DependsOn = []string{}
		}
	}
	return normalized
}

func normalizeAPICallPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func validateAPICallStep(call APICallStep, index int, calls []APICallStep, seenIDs map[string]struct{}) error {
	id := strings.TrimSpace(call.ID)
	if id == "" {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].id required", index))
	}
	if _, exists := seenIDs[id]; exists {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].id duplicated", index))
	}
	if strings.ToUpper(strings.TrimSpace(call.Method)) == "" {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].method required", index))
	}
	if normalizeAPICallPath(call.Path) == "" {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].path required", index))
	}
	if call.Params == nil {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].params required", index))
	}
	if call.DependsOn == nil {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].depends_on required", index))
	}
	if len(call.DependsOn) > 1 {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].depends_on must be linear", index))
	}
	if index == 0 {
		if len(call.DependsOn) != 0 {
			return wrapAPICallPlanBoundaryError("calls[0].depends_on must be empty")
		}
		return nil
	}
	if len(call.DependsOn) == 0 {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].depends_on required", index))
	}

	expected := strings.TrimSpace(calls[index-1].ID)
	actual := strings.TrimSpace(call.DependsOn[0])
	if actual == "" {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].depends_on[0] required", index))
	}
	if actual != expected {
		return wrapAPICallPlanBoundaryError(fmt.Sprintf("calls[%d].depends_on must reference immediate previous call", index))
	}
	return nil
}

func wrapAPICallPlanDecodeError(detail string) error {
	return fmt.Errorf("%w: %s", ErrAPICallPlanSchemaConstrainedDecodeFailed, strings.TrimSpace(detail))
}

func wrapAPICallPlanBoundaryError(detail string) error {
	return fmt.Errorf("%w: %s", ErrAPICallPlanBoundaryViolation, strings.TrimSpace(detail))
}

func ensureSingleAPICallPlanJSONObject(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("extra trailing json value")
}
