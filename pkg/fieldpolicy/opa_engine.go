package fieldpolicy

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/rego"
)

const setIDStrategyOPAQuery = "data.fieldpolicy.setid_strategy.result"

//go:embed rego/*.rego
var fieldPolicyRegoFS embed.FS

var (
	setIDStrategyPreparedQueryOnce sync.Once
	setIDStrategyPreparedQuery     *rego.PreparedEvalQuery
	setIDStrategyPreparedQueryErr  error
)

type setIDStrategyOPAInput struct {
	Context               setIDStrategyOPAContext  `json:"ctx"`
	BaselineCapabilityKey string                   `json:"baseline_capability_key"`
	Records               []setIDStrategyOPARecord `json:"records"`
}

type setIDStrategyOPAContext struct {
	CapabilityKey       string `json:"capability_key"`
	FieldKey            string `json:"field_key"`
	ResolvedSetID       string `json:"resolved_setid"`
	BusinessUnitNodeKey string `json:"business_unit_node_key"`
}

type setIDStrategyOPARecord struct {
	PolicyID            string   `json:"policy_id"`
	CapabilityKey       string   `json:"capability_key"`
	FieldKey            string   `json:"field_key"`
	OrgApplicability    string   `json:"org_applicability"`
	ResolvedSetID       string   `json:"resolved_setid"`
	BusinessUnitNodeKey string   `json:"business_unit_node_key"`
	Required            bool     `json:"required"`
	Visible             bool     `json:"visible"`
	Maintainable        bool     `json:"maintainable"`
	DefaultRuleRef      string   `json:"default_rule_ref"`
	DefaultValue        string   `json:"default_value"`
	AllowedValueCodes   []string `json:"allowed_value_codes"`
	Priority            int      `json:"priority"`
	PriorityMode        string   `json:"priority_mode"`
	LocalOverrideMode   string   `json:"local_override_mode"`
	EffectiveDate       string   `json:"effective_date"`
	CreatedAt           string   `json:"created_at"`
}

type setIDStrategyOPAResult struct {
	OK       bool                      `json:"ok"`
	Error    string                    `json:"error,omitempty"`
	Decision *setIDStrategyOPADecision `json:"decision,omitempty"`
}

type setIDStrategyOPADecision struct {
	CapabilityKey      string   `json:"capability_key"`
	FieldKey           string   `json:"field_key"`
	SourceType         string   `json:"source_type"`
	Required           bool     `json:"required"`
	Visible            bool     `json:"visible"`
	Maintainable       bool     `json:"maintainable"`
	DefaultRuleRef     string   `json:"default_rule_ref"`
	ResolvedDefaultVal string   `json:"resolved_default_value"`
	AllowedValueCodes  []string `json:"allowed_value_codes,omitempty"`
	PriorityMode       string   `json:"priority_mode"`
	LocalOverrideMode  string   `json:"local_override_mode"`
	MatchedBucket      string   `json:"matched_bucket"`
	PrimaryPolicyID    string   `json:"primary_policy_id"`
	WinnerPolicyIDs    []string `json:"winner_policy_ids,omitempty"`
	MatchedPolicyIDs   []string `json:"matched_policy_ids,omitempty"`
	ResolutionTrace    []string `json:"resolution_trace,omitempty"`
}

func resolveWithOPA(ctx PolicyContext, baselineCapabilityKey string, records []Record) (Decision, error) {
	input := buildSetIDStrategyOPAInput(ctx, baselineCapabilityKey, records)
	query, err := prepareSetIDStrategyOPAQuery()
	if err != nil {
		return Decision{}, fmt.Errorf("prepare setid strategy opa query: %w", err)
	}

	resultSet, err := query.Eval(context.Background(), rego.EvalInput(input))
	if err != nil {
		return Decision{}, fmt.Errorf("eval setid strategy opa query: %w", err)
	}
	if len(resultSet) != 1 || len(resultSet[0].Expressions) != 1 {
		return Decision{}, fmt.Errorf("unexpected opa result cardinality: results=%d", len(resultSet))
	}

	var result setIDStrategyOPAResult
	if err := remarshalOPAValue(resultSet[0].Expressions[0].Value, &result); err != nil {
		return Decision{}, fmt.Errorf("decode setid strategy opa result: %w", err)
	}
	if !result.OK {
		if strings.TrimSpace(result.Error) == "" {
			return Decision{}, errors.New(ErrorPolicyConflict)
		}
		return Decision{}, errors.New(strings.TrimSpace(result.Error))
	}
	if result.Decision == nil {
		return Decision{}, errors.New(ErrorPolicyConflict)
	}

	return Decision{
		CapabilityKey:      normalizeCapabilityKey(result.Decision.CapabilityKey),
		FieldKey:           normalizeFieldKey(result.Decision.FieldKey),
		SourceType:         strings.TrimSpace(result.Decision.SourceType),
		Required:           result.Decision.Required,
		Visible:            result.Decision.Visible,
		Maintainable:       result.Decision.Maintainable,
		DefaultRuleRef:     strings.TrimSpace(result.Decision.DefaultRuleRef),
		ResolvedDefaultVal: strings.TrimSpace(result.Decision.ResolvedDefaultVal),
		AllowedValueCodes:  normalizeAllowedValueCodes(append([]string(nil), result.Decision.AllowedValueCodes...)),
		PriorityMode:       normalizePriorityMode(result.Decision.PriorityMode),
		LocalOverrideMode:  normalizeLocalOverrideMode(result.Decision.LocalOverrideMode),
		MatchedBucket:      strings.TrimSpace(result.Decision.MatchedBucket),
		PrimaryPolicyID:    strings.TrimSpace(result.Decision.PrimaryPolicyID),
		WinnerPolicyIDs:    normalizePolicyIDs(append([]string(nil), result.Decision.WinnerPolicyIDs...)),
		MatchedPolicyIDs:   normalizePolicyIDs(append([]string(nil), result.Decision.MatchedPolicyIDs...)),
		ResolutionTrace:    normalizeResolutionTrace(append([]string(nil), result.Decision.ResolutionTrace...)),
	}, nil
}

func buildSetIDStrategyOPAInput(ctx PolicyContext, baselineCapabilityKey string, records []Record) setIDStrategyOPAInput {
	ctx = normalizePolicyContext(ctx)
	records = normalizeRecords(records)
	sortRecords(records)

	out := setIDStrategyOPAInput{
		Context: setIDStrategyOPAContext{
			CapabilityKey:       ctx.CapabilityKey,
			FieldKey:            ctx.FieldKey,
			ResolvedSetID:       ctx.ResolvedSetID,
			BusinessUnitNodeKey: ctx.BusinessUnitNodeKey,
		},
		BaselineCapabilityKey: normalizeCapabilityKey(baselineCapabilityKey),
		Records:               make([]setIDStrategyOPARecord, 0, len(records)),
	}
	for _, record := range records {
		out.Records = append(out.Records, setIDStrategyOPARecord{
			PolicyID:            record.PolicyID,
			CapabilityKey:       record.CapabilityKey,
			FieldKey:            record.FieldKey,
			OrgApplicability:    record.OrgApplicability,
			ResolvedSetID:       record.ResolvedSetID,
			BusinessUnitNodeKey: record.BusinessUnitNodeKey,
			Required:            record.Required,
			Visible:             record.Visible,
			Maintainable:        record.Maintainable,
			DefaultRuleRef:      record.DefaultRuleRef,
			DefaultValue:        record.DefaultValue,
			AllowedValueCodes:   append([]string(nil), record.AllowedValueCodes...),
			Priority:            record.Priority,
			PriorityMode:        record.PriorityMode,
			LocalOverrideMode:   record.LocalOverrideMode,
			EffectiveDate:       record.EffectiveDate,
			CreatedAt:           record.CreatedAt,
		})
	}
	return out
}

func prepareSetIDStrategyOPAQuery() (*rego.PreparedEvalQuery, error) {
	setIDStrategyPreparedQueryOnce.Do(func() {
		module, err := fs.ReadFile(fieldPolicyRegoFS, "rego/setid_strategy.rego")
		if err != nil {
			setIDStrategyPreparedQueryErr = err
			return
		}
		prepared, err := rego.New(
			rego.Query(setIDStrategyOPAQuery),
			rego.Module("rego/setid_strategy.rego", string(module)),
		).PrepareForEval(context.Background())
		if err != nil {
			setIDStrategyPreparedQueryErr = err
			return
		}
		setIDStrategyPreparedQuery = &prepared
	})
	return setIDStrategyPreparedQuery, setIDStrategyPreparedQueryErr
}

func remarshalOPAValue(value any, target any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func normalizeResolutionTrace(trace []string) []string {
	if len(trace) == 0 {
		return nil
	}
	out := make([]string, 0, len(trace))
	for _, entry := range trace {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
