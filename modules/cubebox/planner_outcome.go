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
	PlannerOutcomeAPICalls PlannerOutcomeType = "API_CALLS"
	PlannerOutcomeClarify  PlannerOutcomeType = "CLARIFY"
	PlannerOutcomeDone     PlannerOutcomeType = "DONE"
	PlannerOutcomeNoQuery  PlannerOutcomeType = "NO_QUERY"
)

type PlannerOutcome struct {
	Type               PlannerOutcomeType `json:"outcome"`
	Calls              APICallPlan        `json:"calls,omitempty"`
	MissingParams      []string           `json:"missing_params,omitempty"`
	ClarifyingQuestion string             `json:"clarifying_question,omitempty"`
}

type plannerOutcomeEnvelope struct {
	Outcome            string          `json:"outcome"`
	Calls              json.RawMessage `json:"calls,omitempty"`
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
		return PlannerOutcome{}, wrapPlannerOutcomeError("bare NO_QUERY is not allowed")
	}

	var probe map[string]json.RawMessage
	if err := decodeStrictJSONObject([]byte(text), &probe); err != nil {
		return PlannerOutcome{}, wrapPlannerOutcomeError(err.Error())
	}
	if _, hasOutcome := probe["outcome"]; !hasOutcome {
		return PlannerOutcome{}, wrapPlannerOutcomeError("outcome required")
	}

	var envelope plannerOutcomeEnvelope
	if err := decodeStrictJSONObject([]byte(text), &envelope); err != nil {
		return PlannerOutcome{}, wrapPlannerOutcomeError(err.Error())
	}
	outcomeType := PlannerOutcomeType(strings.TrimSpace(envelope.Outcome))
	switch outcomeType {
	case PlannerOutcomeAPICalls:
		if len(envelope.Calls) == 0 || string(envelope.Calls) == "null" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("API_CALLS requires calls")
		}
		if len(envelope.MissingParams) > 0 || strings.TrimSpace(envelope.ClarifyingQuestion) != "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("API_CALLS cannot carry clarification fields")
		}
		var body []byte
		if bytes.HasPrefix(bytes.TrimSpace(envelope.Calls), []byte("[")) {
			wrapped, err := json.Marshal(map[string]json.RawMessage{"calls": envelope.Calls})
			if err != nil {
				return PlannerOutcome{}, wrapPlannerOutcomeError(err.Error())
			}
			body = wrapped
		} else {
			body = envelope.Calls
		}
		plan, err := DecodeAPICallPlan(body)
		if err != nil {
			return PlannerOutcome{}, err
		}
		return PlannerOutcome{Type: PlannerOutcomeAPICalls, Calls: plan}, nil
	case PlannerOutcomeClarify:
		if len(envelope.Calls) > 0 {
			return PlannerOutcome{}, wrapPlannerOutcomeError("CLARIFY cannot carry calls")
		}
		missing := normalizePlannerMissingParams(envelope.MissingParams)
		question := strings.TrimSpace(envelope.ClarifyingQuestion)
		if len(missing) == 0 || question == "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("CLARIFY requires missing_params and clarifying_question")
		}
		return PlannerOutcome{
			Type:               PlannerOutcomeClarify,
			MissingParams:      missing,
			ClarifyingQuestion: question,
		}, nil
	case PlannerOutcomeDone:
		if len(envelope.Calls) > 0 || len(envelope.MissingParams) > 0 || strings.TrimSpace(envelope.ClarifyingQuestion) != "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("DONE cannot carry calls or clarification fields")
		}
		return PlannerOutcome{Type: PlannerOutcomeDone}, nil
	case PlannerOutcomeNoQuery:
		if len(envelope.Calls) > 0 || len(envelope.MissingParams) > 0 || strings.TrimSpace(envelope.ClarifyingQuestion) != "" {
			return PlannerOutcome{}, wrapPlannerOutcomeError("NO_QUERY cannot carry calls or clarification fields")
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
