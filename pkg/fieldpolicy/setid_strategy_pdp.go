package fieldpolicy

import (
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
	return resolveWithOPA(ctx, baselineCapabilityKey, records)
}

type fallbackBucketWinner struct {
	bucketName string
	record     Record
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
