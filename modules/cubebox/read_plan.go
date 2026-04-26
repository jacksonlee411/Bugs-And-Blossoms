package cubebox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrReadPlanSchemaConstrainedDecodeFailed = errors.New("CUBEBOX_READ_PLAN_SCHEMA_CONSTRAINED_DECODE_FAILED")
var ErrReadPlanBoundaryViolation = errors.New("CUBEBOX_READ_PLAN_BOUNDARY_VIOLATION")
var ErrKnowledgePackInvalid = errors.New("CUBEBOX_KNOWLEDGE_PACK_INVALID")

type ReadPlan struct {
	Intent             string         `json:"intent"`
	Confidence         float64        `json:"confidence"`
	MissingParams      []string       `json:"missing_params"`
	Steps              []ReadPlanStep `json:"steps"`
	ExplainFocus       []string       `json:"explain_focus"`
	ClarifyingQuestion string         `json:"clarifying_question,omitempty"`
}

type ReadPlanStep struct {
	ID          string         `json:"id"`
	APIKey      string         `json:"api_key"`
	Params      map[string]any `json:"params"`
	ResultFocus []string       `json:"result_focus,omitempty"`
	DependsOn   []string       `json:"depends_on"`
}

func DecodeReadPlan(raw []byte) (ReadPlan, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return ReadPlan{}, wrapReadPlanDecodeError("empty payload")
	}

	var plan ReadPlan
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return ReadPlan{}, wrapReadPlanDecodeError(err.Error())
	}
	if err := ensureSingleJSONObject(dec); err != nil {
		return ReadPlan{}, wrapReadPlanDecodeError(err.Error())
	}
	if err := ValidateReadPlan(plan); err != nil {
		return ReadPlan{}, err
	}
	return normalizeReadPlan(plan), nil
}

func ValidateReadPlan(plan ReadPlan) error {
	if strings.TrimSpace(plan.Intent) == "" {
		return wrapReadPlanBoundaryError("intent required")
	}
	if plan.Confidence < 0 || plan.Confidence > 1 {
		return wrapReadPlanBoundaryError("confidence out of range")
	}

	hasMissingParams := len(plan.MissingParams) > 0
	if hasMissingParams {
		if strings.TrimSpace(plan.ClarifyingQuestion) == "" {
			return wrapReadPlanBoundaryError("clarifying_question required when missing_params present")
		}
		if len(plan.Steps) > 0 {
			return wrapReadPlanBoundaryError("steps must be empty when clarifying_question present")
		}
		return nil
	}

	if len(plan.Steps) == 0 {
		return wrapReadPlanBoundaryError("steps required")
	}

	seenIDs := make(map[string]struct{}, len(plan.Steps))
	for i, step := range plan.Steps {
		if err := validateReadPlanStep(step, i, plan.Steps, seenIDs); err != nil {
			return err
		}
		seenIDs[strings.TrimSpace(step.ID)] = struct{}{}
	}

	return nil
}

func normalizeReadPlan(plan ReadPlan) ReadPlan {
	normalized := plan
	normalized.Intent = strings.TrimSpace(plan.Intent)
	normalized.ClarifyingQuestion = strings.TrimSpace(plan.ClarifyingQuestion)
	if normalized.MissingParams == nil {
		normalized.MissingParams = []string{}
	}
	if normalized.ExplainFocus == nil {
		normalized.ExplainFocus = []string{}
	}
	if normalized.Steps == nil {
		normalized.Steps = []ReadPlanStep{}
	}

	for i := range normalized.Steps {
		normalized.Steps[i].ID = strings.TrimSpace(normalized.Steps[i].ID)
		normalized.Steps[i].APIKey = strings.TrimSpace(normalized.Steps[i].APIKey)
		if normalized.Steps[i].Params == nil {
			normalized.Steps[i].Params = map[string]any{}
		}
		if normalized.Steps[i].ResultFocus == nil {
			normalized.Steps[i].ResultFocus = []string{}
		}
		if normalized.Steps[i].DependsOn == nil {
			normalized.Steps[i].DependsOn = []string{}
		}
	}

	return normalized
}

func validateReadPlanStep(step ReadPlanStep, index int, steps []ReadPlanStep, seenIDs map[string]struct{}) error {
	id := strings.TrimSpace(step.ID)
	if id == "" {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].id required", index))
	}
	if _, exists := seenIDs[id]; exists {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].id duplicated", index))
	}
	if strings.TrimSpace(step.APIKey) == "" {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].api_key required", index))
	}
	if step.Params == nil {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].params required", index))
	}
	if step.DependsOn == nil {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].depends_on required", index))
	}
	if len(step.DependsOn) > 1 {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].depends_on must be linear", index))
	}
	if index == 0 {
		if len(step.DependsOn) != 0 {
			return wrapReadPlanBoundaryError("steps[0].depends_on must be empty")
		}
		return nil
	}
	if len(step.DependsOn) == 0 {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].depends_on required", index))
	}

	expected := strings.TrimSpace(steps[index-1].ID)
	actual := strings.TrimSpace(step.DependsOn[0])
	if actual == "" {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].depends_on[0] required", index))
	}
	if actual != expected {
		return wrapReadPlanBoundaryError(fmt.Sprintf("steps[%d].depends_on must reference immediate previous step", index))
	}
	return nil
}

func wrapReadPlanDecodeError(detail string) error {
	return fmt.Errorf("%w: %s", ErrReadPlanSchemaConstrainedDecodeFailed, strings.TrimSpace(detail))
}

func wrapReadPlanBoundaryError(detail string) error {
	return fmt.Errorf("%w: %s", ErrReadPlanBoundaryViolation, strings.TrimSpace(detail))
}

func ensureSingleJSONObject(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("extra trailing json value")
}
