package fieldpolicy

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	OrgUnitWriteFieldPolicyCapabilityKey         = "org.orgunit_write.field_policy"
	OrgUnitCreateFieldPolicyCapabilityKey        = "org.orgunit_create.field_policy"
	OrgUnitAddVersionFieldPolicyCapabilityKey    = "org.orgunit_add_version.field_policy"
	OrgUnitInsertVersionFieldPolicyCapabilityKey = "org.orgunit_insert_version.field_policy"
	OrgUnitCorrectFieldPolicyCapabilityKey       = "org.orgunit_correct.field_policy"

	OrgApplicabilityTenant       = "tenant"
	OrgApplicabilityBusinessUnit = "business_unit"

	PriorityModeBlendCustomFirst  = "blend_custom_first"
	PriorityModeBlendDefltFirst   = "blend_deflt_first"
	PriorityModeDefltUnsubscribed = "deflt_unsubscribed"

	LocalOverrideModeAllow      = "allow"
	LocalOverrideModeNoOverride = "no_override"
	LocalOverrideModeNoLocal    = "no_local"

	ErrorPolicyConflict          = "policy_conflict_ambiguous"
	ErrorPolicyMissing           = "policy_missing"
	ErrorDefaultRuleMissing      = "FIELD_DEFAULT_RULE_MISSING"
	ErrorPolicyPriorityMode      = "policy_mode_invalid"
	ErrorPolicyLocalOverrideMode = "policy_mode_invalid"
	ErrorPolicyModeCombination   = "policy_mode_combination_invalid"

	BucketIntentSetIDBusinessUnitExact = "intent_setid_exact_business_unit_exact"
	BucketIntentSetIDWildcard          = "intent_setid_exact_business_unit_wildcard"
	BucketIntentWildcard               = "intent_setid_wildcard_business_unit_wildcard"
	BucketBaselineSetIDBusinessUnit    = "baseline_setid_exact_business_unit_exact"
	BucketBaselineSetIDWildcard        = "baseline_setid_exact_business_unit_wildcard"
	BucketBaselineWildcard             = "baseline_setid_wildcard_business_unit_wildcard"

	SourceTypeBaseline       = "baseline"
	SourceTypeIntentOverride = "intent_override"
)

type PolicyContext struct {
	CapabilityKey       string
	FieldKey            string
	ResolvedSetID       string
	BusinessUnitNodeKey string
}

type Record struct {
	PolicyID            string
	CapabilityKey       string
	FieldKey            string
	OrgApplicability    string
	ResolvedSetID       string
	BusinessUnitNodeKey string
	Required            bool
	Visible             bool
	Maintainable        bool
	DefaultRuleRef      string
	DefaultValue        string
	AllowedValueCodes   []string
	Priority            int
	PriorityMode        string
	LocalOverrideMode   string
	EffectiveDate       string
	CreatedAt           string
}

type Decision struct {
	CapabilityKey      string
	FieldKey           string
	SourceType         string
	Required           bool
	Visible            bool
	Maintainable       bool
	DefaultRuleRef     string
	ResolvedDefaultVal string
	AllowedValueCodes  []string
	PriorityMode       string
	LocalOverrideMode  string
	MatchedBucket      string
	PrimaryPolicyID    string
	WinnerPolicyIDs    []string
	MatchedPolicyIDs   []string
	ResolutionTrace    []string
}

type bucketSpec struct {
	name                string
	sourceType          string
	capabilityKey       string
	resolvedSetID       string
	businessUnitNodeKey string
	setIDExact          bool
	businessUnitExact   bool
}

func OrgUnitBaselineCapabilityKey(capabilityKey string) (string, bool) {
	switch normalizeCapabilityKey(capabilityKey) {
	case OrgUnitCreateFieldPolicyCapabilityKey,
		OrgUnitAddVersionFieldPolicyCapabilityKey,
		OrgUnitInsertVersionFieldPolicyCapabilityKey,
		OrgUnitCorrectFieldPolicyCapabilityKey:
		return OrgUnitWriteFieldPolicyCapabilityKey, true
	case OrgUnitWriteFieldPolicyCapabilityKey:
		return OrgUnitWriteFieldPolicyCapabilityKey, true
	default:
		return "", false
	}
}

func Resolve(ctx PolicyContext, baselineCapabilityKey string, records []Record) (Decision, error) {
	ctx = normalizePolicyContext(ctx)
	records = normalizeRecords(records)
	buckets := buildBucketSpecs(ctx, baselineCapabilityKey)
	trace := make([]string, 0, len(buckets)+4)

	for idx, bucket := range buckets {
		matches := matchBucketRecords(records, ctx.FieldKey, bucket)
		if len(matches) == 0 {
			trace = append(trace, fmt.Sprintf("bucket:%s:miss", bucket.name))
			continue
		}
		sortRecords(matches)

		matchedPolicyIDs := recordPolicyIDs(matches)
		trace = append(trace, fmt.Sprintf("bucket:%s:hit:%d", bucket.name, len(matches)))

		primary := matches[0]
		if err := validateModes(primary.PriorityMode, primary.LocalOverrideMode); err != nil {
			return Decision{}, err
		}

		fallbackBuckets := collectFallbackBucketWinners(records, ctx.FieldKey, buckets[idx+1:])
		fallbackCodes := make([]string, 0, 8)
		winnerPolicyIDs := []string{primary.PolicyID}
		for _, fallback := range fallbackBuckets {
			trace = append(trace, fmt.Sprintf("fallback:%s:%s", fallback.bucketName, fallback.record.PolicyID))
			fallbackCodes = append(fallbackCodes, fallback.record.AllowedValueCodes...)
			winnerPolicyIDs = append(winnerPolicyIDs, fallback.record.PolicyID)
		}
		winnerPolicyIDs = normalizePolicyIDs(winnerPolicyIDs)
		allowedValueCodes, err := mergeAllowedValueCodes(
			primary.AllowedValueCodes,
			fallbackCodes,
			bucket.setIDExact,
			primary.PriorityMode,
			primary.LocalOverrideMode,
		)
		if err != nil {
			return Decision{}, err
		}
		trace = append(trace, fmt.Sprintf(
			"mode:%s/%s:allowed=%s",
			primary.PriorityMode,
			primary.LocalOverrideMode,
			strings.Join(allowedValueCodes, ","),
		))
		resolvedDefaultValue := resolveDefaultValue(primary, fallbackBuckets, allowedValueCodes, bucket.setIDExact)

		if primary.Required && !primary.Visible {
			return Decision{}, errors.New(ErrorPolicyConflict)
		}
		if !primary.Maintainable && strings.TrimSpace(primary.DefaultRuleRef) == "" && strings.TrimSpace(resolvedDefaultValue) == "" {
			return Decision{}, errors.New(ErrorDefaultRuleMissing)
		}
		if primary.Required && len(primary.AllowedValueCodes) > 0 && len(allowedValueCodes) == 0 {
			return Decision{}, errors.New(ErrorPolicyConflict)
		}
		if strings.TrimSpace(resolvedDefaultValue) != "" && len(allowedValueCodes) > 0 && !containsValue(allowedValueCodes, resolvedDefaultValue) {
			return Decision{}, errors.New(ErrorPolicyConflict)
		}

		return Decision{
			CapabilityKey:      primary.CapabilityKey,
			FieldKey:           primary.FieldKey,
			SourceType:         bucket.sourceType,
			Required:           primary.Required,
			Visible:            primary.Visible,
			Maintainable:       primary.Maintainable,
			DefaultRuleRef:     primary.DefaultRuleRef,
			ResolvedDefaultVal: resolvedDefaultValue,
			AllowedValueCodes:  allowedValueCodes,
			PriorityMode:       primary.PriorityMode,
			LocalOverrideMode:  primary.LocalOverrideMode,
			MatchedBucket:      bucket.name,
			PrimaryPolicyID:    primary.PolicyID,
			WinnerPolicyIDs:    winnerPolicyIDs,
			MatchedPolicyIDs:   matchedPolicyIDs,
			ResolutionTrace:    trace,
		}, nil
	}

	return Decision{}, errors.New(ErrorPolicyMissing)
}

type fallbackBucketWinner struct {
	bucketName string
	record     Record
}

func buildBucketSpecs(ctx PolicyContext, baselineCapabilityKey string) []bucketSpec {
	intent := normalizeCapabilityKey(ctx.CapabilityKey)
	baseline := normalizeCapabilityKey(baselineCapabilityKey)
	if baseline == "" || baseline == intent {
		return []bucketSpec{
			{
				name:                BucketIntentSetIDBusinessUnitExact,
				sourceType:          SourceTypeIntentOverride,
				capabilityKey:       intent,
				resolvedSetID:       ctx.ResolvedSetID,
				businessUnitNodeKey: ctx.BusinessUnitNodeKey,
				setIDExact:          true,
				businessUnitExact:   true,
			},
			{
				name:          BucketIntentSetIDWildcard,
				sourceType:    SourceTypeIntentOverride,
				capabilityKey: intent,
				resolvedSetID: ctx.ResolvedSetID,
				setIDExact:    true,
			},
			{
				name:          BucketIntentWildcard,
				sourceType:    SourceTypeIntentOverride,
				capabilityKey: intent,
			},
		}
	}
	return []bucketSpec{
		{
			name:                BucketIntentSetIDBusinessUnitExact,
			sourceType:          SourceTypeIntentOverride,
			capabilityKey:       intent,
			resolvedSetID:       ctx.ResolvedSetID,
			businessUnitNodeKey: ctx.BusinessUnitNodeKey,
			setIDExact:          true,
			businessUnitExact:   true,
		},
		{
			name:          BucketIntentSetIDWildcard,
			sourceType:    SourceTypeIntentOverride,
			capabilityKey: intent,
			resolvedSetID: ctx.ResolvedSetID,
			setIDExact:    true,
		},
		{
			name:          BucketIntentWildcard,
			sourceType:    SourceTypeIntentOverride,
			capabilityKey: intent,
		},
		{
			name:                BucketBaselineSetIDBusinessUnit,
			sourceType:          SourceTypeBaseline,
			capabilityKey:       baseline,
			resolvedSetID:       ctx.ResolvedSetID,
			businessUnitNodeKey: ctx.BusinessUnitNodeKey,
			setIDExact:          true,
			businessUnitExact:   true,
		},
		{
			name:          BucketBaselineSetIDWildcard,
			sourceType:    SourceTypeBaseline,
			capabilityKey: baseline,
			resolvedSetID: ctx.ResolvedSetID,
			setIDExact:    true,
		},
		{
			name:          BucketBaselineWildcard,
			sourceType:    SourceTypeBaseline,
			capabilityKey: baseline,
		},
	}
}

func matchBucketRecords(records []Record, fieldKey string, bucket bucketSpec) []Record {
	out := make([]Record, 0, len(records))
	for _, record := range records {
		if record.CapabilityKey != bucket.capabilityKey {
			continue
		}
		if record.FieldKey != fieldKey {
			continue
		}
		if bucket.businessUnitExact {
			if record.OrgApplicability != OrgApplicabilityBusinessUnit {
				continue
			}
			if record.ResolvedSetID != bucket.resolvedSetID {
				continue
			}
			if record.BusinessUnitNodeKey != bucket.businessUnitNodeKey {
				continue
			}
			out = append(out, record)
			continue
		}
		if bucket.setIDExact {
			if record.OrgApplicability != OrgApplicabilityTenant {
				continue
			}
			if record.BusinessUnitNodeKey != "" {
				continue
			}
			if record.ResolvedSetID != bucket.resolvedSetID {
				continue
			}
			out = append(out, record)
			continue
		}
		if record.OrgApplicability != OrgApplicabilityTenant {
			continue
		}
		if record.BusinessUnitNodeKey != "" || record.ResolvedSetID != "" {
			continue
		}
		out = append(out, record)
	}
	return out
}

func collectFallbackBucketWinners(records []Record, fieldKey string, buckets []bucketSpec) []fallbackBucketWinner {
	out := make([]fallbackBucketWinner, 0, len(buckets))
	for _, bucket := range buckets {
		matches := matchBucketRecords(records, fieldKey, bucket)
		if len(matches) == 0 {
			continue
		}
		sortRecords(matches)
		out = append(out, fallbackBucketWinner{
			bucketName: bucket.name,
			record:     matches[0],
		})
	}
	return out
}

func sortRecords(records []Record) {
	slices.SortStableFunc(records, func(a, b Record) int {
		if a.Priority != b.Priority {
			return b.Priority - a.Priority
		}
		if a.EffectiveDate != b.EffectiveDate {
			return strings.Compare(b.EffectiveDate, a.EffectiveDate)
		}
		if a.CreatedAt != b.CreatedAt {
			return strings.Compare(b.CreatedAt, a.CreatedAt)
		}
		return strings.Compare(a.PolicyID, b.PolicyID)
	})
}

func mergeAllowedValueCodes(localCodes []string, fallbackCodes []string, localExact bool, priorityMode string, localOverrideMode string) ([]string, error) {
	localCodes = normalizeAllowedValueCodes(localCodes)
	fallbackCodes = normalizeAllowedValueCodes(fallbackCodes)
	priorityMode = normalizePriorityMode(priorityMode)
	localOverrideMode = normalizeLocalOverrideMode(localOverrideMode)

	if err := validateModes(priorityMode, localOverrideMode); err != nil {
		return nil, err
	}
	if !localExact || len(fallbackCodes) == 0 {
		return localCodes, nil
	}

	switch priorityMode {
	case PriorityModeBlendCustomFirst:
		switch localOverrideMode {
		case LocalOverrideModeAllow:
			return mergePreferPrimary(localCodes, fallbackCodes), nil
		case LocalOverrideModeNoOverride:
			return mergePreferPrimary(fallbackCodes, localCodes), nil
		case LocalOverrideModeNoLocal:
			return fallbackCodes, nil
		}
	case PriorityModeBlendDefltFirst:
		switch localOverrideMode {
		case LocalOverrideModeAllow, LocalOverrideModeNoOverride:
			return mergePreferPrimary(fallbackCodes, localCodes), nil
		case LocalOverrideModeNoLocal:
			return fallbackCodes, nil
		}
	case PriorityModeDefltUnsubscribed:
		switch localOverrideMode {
		case LocalOverrideModeAllow, LocalOverrideModeNoOverride:
			return localCodes, nil
		case LocalOverrideModeNoLocal:
			return nil, errors.New(ErrorPolicyModeCombination)
		}
	}
	return nil, errors.New(ErrorPolicyConflict)
}

func mergePreferPrimary(primary []string, secondary []string) []string {
	merged := make([]string, 0, len(primary)+len(secondary))
	merged = append(merged, primary...)
	merged = append(merged, secondary...)
	return normalizeAllowedValueCodes(merged)
}

func resolveDefaultValue(primary Record, fallbacks []fallbackBucketWinner, allowedValueCodes []string, localExact bool) string {
	defaultValue := strings.TrimSpace(primary.DefaultValue)
	if defaultValue != "" && (len(allowedValueCodes) == 0 || containsValue(allowedValueCodes, defaultValue)) {
		return defaultValue
	}
	if !localExact {
		return defaultValue
	}
	for _, fallback := range fallbacks {
		candidate := strings.TrimSpace(fallback.record.DefaultValue)
		if candidate == "" {
			continue
		}
		if len(allowedValueCodes) == 0 || containsValue(allowedValueCodes, candidate) {
			return candidate
		}
	}
	return defaultValue
}

func validateModes(priorityMode string, localOverrideMode string) error {
	switch priorityMode {
	case PriorityModeBlendCustomFirst, PriorityModeBlendDefltFirst, PriorityModeDefltUnsubscribed:
	default:
		return errors.New(ErrorPolicyPriorityMode)
	}
	switch localOverrideMode {
	case LocalOverrideModeAllow, LocalOverrideModeNoOverride, LocalOverrideModeNoLocal:
	default:
		return errors.New(ErrorPolicyLocalOverrideMode)
	}
	if priorityMode == PriorityModeDefltUnsubscribed && localOverrideMode == LocalOverrideModeNoLocal {
		return errors.New(ErrorPolicyModeCombination)
	}
	return nil
}

func normalizePolicyContext(ctx PolicyContext) PolicyContext {
	ctx.CapabilityKey = normalizeCapabilityKey(ctx.CapabilityKey)
	ctx.FieldKey = normalizeFieldKey(ctx.FieldKey)
	ctx.ResolvedSetID = normalizeResolvedSetID(ctx.ResolvedSetID)
	ctx.BusinessUnitNodeKey = strings.TrimSpace(ctx.BusinessUnitNodeKey)
	return ctx
}

func normalizeRecords(records []Record) []Record {
	out := make([]Record, 0, len(records))
	for _, record := range records {
		record.CapabilityKey = normalizeCapabilityKey(record.CapabilityKey)
		record.FieldKey = normalizeFieldKey(record.FieldKey)
		record.OrgApplicability = strings.ToLower(strings.TrimSpace(record.OrgApplicability))
		record.ResolvedSetID = normalizeResolvedSetID(record.ResolvedSetID)
		record.BusinessUnitNodeKey = strings.TrimSpace(record.BusinessUnitNodeKey)
		record.DefaultRuleRef = strings.TrimSpace(record.DefaultRuleRef)
		record.DefaultValue = strings.TrimSpace(record.DefaultValue)
		record.AllowedValueCodes = normalizeAllowedValueCodes(record.AllowedValueCodes)
		record.PriorityMode = normalizePriorityMode(record.PriorityMode)
		record.LocalOverrideMode = normalizeLocalOverrideMode(record.LocalOverrideMode)
		record.EffectiveDate = strings.TrimSpace(record.EffectiveDate)
		record.CreatedAt = strings.TrimSpace(record.CreatedAt)
		record.PolicyID = strings.TrimSpace(record.PolicyID)
		if record.PolicyID == "" {
			record.PolicyID = strings.Join([]string{
				record.CapabilityKey,
				record.FieldKey,
				record.OrgApplicability,
				record.ResolvedSetID,
				record.BusinessUnitNodeKey,
				record.EffectiveDate,
				record.CreatedAt,
			}, "|")
		}
		out = append(out, record)
	}
	return out
}

func normalizeAllowedValueCodes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeCapabilityKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeFieldKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeResolvedSetID(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizePriorityMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return PriorityModeBlendCustomFirst
	}
	return value
}

func normalizeLocalOverrideMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return LocalOverrideModeAllow
	}
	return value
}

func recordPolicyIDs(records []Record) []string {
	out := make([]string, 0, len(records))
	for _, record := range records {
		out = append(out, record.PolicyID)
	}
	return normalizePolicyIDs(out)
}

func normalizePolicyIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func containsValue(values []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range values {
		if strings.TrimSpace(candidate) == value {
			return true
		}
	}
	return false
}
