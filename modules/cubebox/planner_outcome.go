package cubebox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrPlannerOutcomeInvalid = errors.New("CUBEBOX_PLANNER_OUTCOME_INVALID")

type PlannerOutcomeType string

const (
	PlannerOutcomeReadPlan PlannerOutcomeType = "READ_PLAN"
	PlannerOutcomeClarify  PlannerOutcomeType = "CLARIFY"
	PlannerOutcomeDone     PlannerOutcomeType = "DONE"
	PlannerOutcomeNoQuery  PlannerOutcomeType = "NO_QUERY"
)

type PlannerOutcome struct {
	Type                PlannerOutcomeType `json:"outcome"`
	Plan                ReadPlan           `json:"plan,omitempty"`
	MissingParams       []string           `json:"missing_params,omitempty"`
	ClarifyingQuestion  string             `json:"clarifying_question,omitempty"`
	CompatibilitySource string             `json:"-"`
}

type plannerOutcomeEnvelope struct {
	Outcome            string          `json:"outcome"`
	Plan               json.RawMessage `json:"plan,omitempty"`
	MissingParams      []string        `json:"missing_params,omitempty"`
	ClarifyingQuestion string          `json:"clarifying_question,omitempty"`
}

func DecodePlannerOutcome(raw []byte) (PlannerOutcome, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return PlannerOutcome{}, wrapPlannerOutcomeError("empty payload")
	}
	if text == "DONE" {
		return PlannerOutcome{}, wrapPlannerOutcomeError("bare DONE is not allowed")
	}
	if text == "NO_QUERY" {
		return PlannerOutcome{Type: PlannerOutcomeNoQuery, CompatibilitySource: "bare_no_query"}, nil
	}

	var probe map[string]json.RawMessage
	if err := decodeStrictJSONObject([]byte(text), &probe); err != nil {
		return PlannerOutcome{}, wrapPlannerOutcomeError(err.Error())
	}
	if _, hasOutcome := probe["outcome"]; !hasOutcome {
		plan, err := DecodeReadPlan([]byte(text))
		if err != nil {
			return PlannerOutcome{}, err
		}
		outcomeType := PlannerOutcomeReadPlan
		if len(plan.MissingParams) > 0 {
			outcomeType = PlannerOutcomeClarify
		}
		return PlannerOutcome{Type: outcomeType, Plan: plan, MissingParams: append([]string(nil), plan.MissingParams...), ClarifyingQuestion: plan.ClarifyingQuestion, CompatibilitySource: "bare_read_plan"}, nil
	}

	var envelope plannerOutcomeEnvelope
	if err := decodeStrictJSONObject([]byte(text), &envelope); err != nil {
		return PlannerOutcome{}, wrapPlannerOutcomeError(err.Error())
	}
	outcomeType := PlannerOutcomeType(strings.TrimSpace(envelope.Outcome))
	switch outcomeType {
	case PlannerOutcomeReadPlan:
		if len(envelope.Plan) == 0 || string(envelope.Plan) == "null" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("READ_PLAN requires plan")
		}
		if len(envelope.MissingParams) > 0 || strings.TrimSpace(envelope.ClarifyingQuestion) != "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("READ_PLAN cannot carry clarification fields")
		}
		plan, err := DecodeReadPlan(envelope.Plan)
		if err != nil {
			return PlannerOutcome{}, err
		}
		if len(plan.MissingParams) > 0 {
			return PlannerOutcome{}, wrapPlannerOutcomeError("READ_PLAN cannot carry missing_params")
		}
		return PlannerOutcome{Type: PlannerOutcomeReadPlan, Plan: plan}, nil
	case PlannerOutcomeClarify:
		if len(envelope.Plan) > 0 {
			return PlannerOutcome{}, wrapPlannerOutcomeError("CLARIFY cannot carry plan")
		}
		missing := normalizePlannerMissingParams(envelope.MissingParams)
		question := strings.TrimSpace(envelope.ClarifyingQuestion)
		if len(missing) == 0 || question == "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("CLARIFY requires missing_params and clarifying_question")
		}
		return PlannerOutcome{
			Type:               PlannerOutcomeClarify,
			Plan:               ReadPlan{Intent: "clarify", Confidence: 0, MissingParams: missing, ClarifyingQuestion: question},
			MissingParams:      missing,
			ClarifyingQuestion: question,
		}, nil
	case PlannerOutcomeDone:
		if len(envelope.Plan) > 0 || len(envelope.MissingParams) > 0 || strings.TrimSpace(envelope.ClarifyingQuestion) != "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("DONE cannot carry plan or clarification fields")
		}
		return PlannerOutcome{Type: PlannerOutcomeDone}, nil
	case PlannerOutcomeNoQuery:
		if len(envelope.Plan) > 0 || len(envelope.MissingParams) > 0 || strings.TrimSpace(envelope.ClarifyingQuestion) != "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("NO_QUERY cannot carry plan or clarification fields")
		}
		return PlannerOutcome{Type: PlannerOutcomeNoQuery}, nil
	default:
		return PlannerOutcome{}, wrapPlannerOutcomeError(fmt.Sprintf("unknown outcome: %s", strings.TrimSpace(envelope.Outcome)))
	}
}

func normalizePlannerMissingParams(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func decodeStrictJSONObject(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := ensurePlannerOutcomeSingleJSONObject(dec); err != nil {
		return err
	}
	return nil
}

func ensurePlannerOutcomeSingleJSONObject(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("extra trailing json value")
}

func wrapPlannerOutcomeError(detail string) error {
	return fmt.Errorf("%w: %s", ErrPlannerOutcomeInvalid, strings.TrimSpace(detail))
}
