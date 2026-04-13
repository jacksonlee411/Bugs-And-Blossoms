package fieldpolicy

import (
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestResolveMatchesLegacyOracle(t *testing.T) {
	baseRecords := []Record{
		{
			PolicyID:            "local",
			CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:            "d_org_type",
			OrgApplicability:    OrgApplicabilityBusinessUnit,
			ResolvedSetID:       "B1000",
			BusinessUnitNodeKey: "10000001",
			Required:            true,
			Visible:             true,
			Maintainable:        true,
			AllowedValueCodes:   []string{"11", "12"},
			DefaultValue:        "11",
			Priority:            200,
			EffectiveDate:       "2026-01-01",
			CreatedAt:           "2026-01-02T00:00:00Z",
		},
		{
			PolicyID:          "fallback-setid",
			CapabilityKey:     OrgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:          "d_org_type",
			OrgApplicability:  OrgApplicabilityTenant,
			ResolvedSetID:     "B1000",
			Required:          true,
			Visible:           true,
			Maintainable:      true,
			AllowedValueCodes: []string{"12", "13"},
			DefaultValue:      "12",
			Priority:          100,
			EffectiveDate:     "2026-01-01",
			CreatedAt:         "2026-01-01T00:00:00Z",
		},
		{
			PolicyID:          "fallback-wildcard",
			CapabilityKey:     OrgUnitWriteFieldPolicyCapabilityKey,
			FieldKey:          "d_org_type",
			OrgApplicability:  OrgApplicabilityTenant,
			Required:          true,
			Visible:           true,
			Maintainable:      true,
			AllowedValueCodes: []string{"13", "14"},
			DefaultValue:      "13",
			Priority:          50,
			EffectiveDate:     "2026-01-01",
			CreatedAt:         "2025-12-31T00:00:00Z",
		},
	}

	ctx := PolicyContext{
		CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:            "d_org_type",
		ResolvedSetID:       "B1000",
		BusinessUnitNodeKey: "10000001",
	}

	cases := []struct {
		name               string
		ctx                PolicyContext
		baselineCapability string
		records            []Record
	}{
		{
			name:               "prefers first non-empty bucket",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: []Record{
				{
					PolicyID:            "baseline-bu",
					CapabilityKey:       OrgUnitWriteFieldPolicyCapabilityKey,
					FieldKey:            "d_org_type",
					OrgApplicability:    OrgApplicabilityBusinessUnit,
					ResolvedSetID:       "B1000",
					BusinessUnitNodeKey: "10000001",
					Required:            true,
					Visible:             true,
					Maintainable:        true,
					DefaultValue:        "baseline-bu",
					Priority:            300,
					EffectiveDate:       "2026-01-01",
					CreatedAt:           "2026-01-01T00:00:00Z",
				},
				{
					PolicyID:         "intent-setid",
					CapabilityKey:    OrgUnitCreateFieldPolicyCapabilityKey,
					FieldKey:         "d_org_type",
					OrgApplicability: OrgApplicabilityTenant,
					ResolvedSetID:    "B1000",
					Required:         true,
					Visible:          true,
					Maintainable:     true,
					DefaultValue:     "intent-setid",
					Priority:         200,
					EffectiveDate:    "2026-01-01",
					CreatedAt:        "2026-01-01T00:00:00Z",
				},
				{
					PolicyID:            "intent-bu",
					CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
					FieldKey:            "d_org_type",
					OrgApplicability:    OrgApplicabilityBusinessUnit,
					ResolvedSetID:       "B1000",
					BusinessUnitNodeKey: "10000001",
					Required:            true,
					Visible:             true,
					Maintainable:        true,
					DefaultValue:        "intent-bu",
					Priority:            100,
					EffectiveDate:       "2026-01-01",
					CreatedAt:           "2026-01-01T00:00:00Z",
				},
			},
		},
		{
			name:               "blend custom first allow",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords)
				records[0].PriorityMode = PriorityModeBlendCustomFirst
				records[0].LocalOverrideMode = LocalOverrideModeAllow
				return records
			}(),
		},
		{
			name:               "blend custom first no override",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords)
				records[0].PriorityMode = PriorityModeBlendCustomFirst
				records[0].LocalOverrideMode = LocalOverrideModeNoOverride
				return records
			}(),
		},
		{
			name:               "blend deflt first no local",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords)
				records[0].PriorityMode = PriorityModeBlendDefltFirst
				records[0].LocalOverrideMode = LocalOverrideModeNoLocal
				return records
			}(),
		},
		{
			name:               "deflt unsubscribed allow",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords)
				records[0].PriorityMode = PriorityModeDefltUnsubscribed
				records[0].LocalOverrideMode = LocalOverrideModeAllow
				return records
			}(),
		},
		{
			name:               "policy missing",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records:            nil,
		},
		{
			name:               "policy mode invalid",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords[:1])
				records[0].PriorityMode = "bad_mode"
				return records
			}(),
		},
		{
			name:               "policy local override invalid",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords[:1])
				records[0].LocalOverrideMode = "bad_mode"
				return records
			}(),
		},
		{
			name:               "policy mode combination invalid",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: func() []Record {
				records := cloneRecordsForTest(baseRecords)
				records[0].PriorityMode = PriorityModeDefltUnsubscribed
				records[0].LocalOverrideMode = LocalOverrideModeNoLocal
				return records
			}(),
		},
		{
			name:               "default rule missing",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: []Record{
				{
					PolicyID:            "required-default-missing",
					CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
					FieldKey:            "d_org_type",
					OrgApplicability:    OrgApplicabilityBusinessUnit,
					ResolvedSetID:       "B1000",
					BusinessUnitNodeKey: "10000001",
					Required:            true,
					Visible:             true,
					Maintainable:        false,
					Priority:            100,
					EffectiveDate:       "2026-01-01",
					CreatedAt:           "2026-01-01T00:00:00Z",
				},
			},
		},
		{
			name:               "required hidden conflict",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: []Record{
				{
					PolicyID:            "hidden-required",
					CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
					FieldKey:            "d_org_type",
					OrgApplicability:    OrgApplicabilityBusinessUnit,
					ResolvedSetID:       "B1000",
					BusinessUnitNodeKey: "10000001",
					Required:            true,
					Visible:             false,
					Maintainable:        true,
					DefaultValue:        "11",
					Priority:            100,
					EffectiveDate:       "2026-01-01",
					CreatedAt:           "2026-01-01T00:00:00Z",
				},
			},
		},
		{
			name:               "default not allowed conflict",
			ctx:                ctx,
			baselineCapability: OrgUnitWriteFieldPolicyCapabilityKey,
			records: []Record{
				{
					PolicyID:            "default-not-allowed",
					CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
					FieldKey:            "d_org_type",
					OrgApplicability:    OrgApplicabilityBusinessUnit,
					ResolvedSetID:       "B1000",
					BusinessUnitNodeKey: "10000001",
					Required:            true,
					Visible:             true,
					Maintainable:        true,
					DefaultValue:        "99",
					AllowedValueCodes:   []string{"11", "12"},
					Priority:            100,
					EffectiveDate:       "2026-01-01",
					CreatedAt:           "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantDecision, wantErr := legacyResolveOracle(tc.ctx, tc.baselineCapability, cloneRecordsForTest(tc.records))
			gotDecision, gotErr := Resolve(tc.ctx, tc.baselineCapability, cloneRecordsForTest(tc.records))

			if !sameErrorMessage(gotErr, wantErr) {
				t.Fatalf("err=%v want=%v", gotErr, wantErr)
			}
			if gotErr != nil {
				return
			}
			if !reflect.DeepEqual(gotDecision, wantDecision) {
				t.Fatalf("decision=%#v want=%#v", gotDecision, wantDecision)
			}
		})
	}
}

func legacyResolveOracle(ctx PolicyContext, baselineCapabilityKey string, records []Record) (Decision, error) {
	ctx = normalizePolicyContext(ctx)
	records = normalizeRecords(records)
	buckets := legacyBuildBucketSpecs(ctx, baselineCapabilityKey)
	trace := make([]string, 0, len(buckets)+4)

	for idx, bucket := range buckets {
		matches := legacyMatchBucketRecords(records, ctx.FieldKey, bucket)
		if len(matches) == 0 {
			trace = append(trace, "bucket:"+bucket.name+":miss")
			continue
		}
		legacySortRecords(matches)

		matchedPolicyIDs := legacyRecordPolicyIDs(matches)
		trace = append(trace, "bucket:"+bucket.name+":hit:"+strconv.Itoa(len(matches)))

		primary := matches[0]
		if err := legacyValidateModes(primary.PriorityMode, primary.LocalOverrideMode); err != nil {
			return Decision{}, err
		}

		fallbackBuckets := legacyCollectFallbackBucketWinners(records, ctx.FieldKey, buckets[idx+1:])
		fallbackCodes := make([]string, 0, 8)
		winnerPolicyIDs := []string{primary.PolicyID}
		for _, fallback := range fallbackBuckets {
			trace = append(trace, "fallback:"+fallback.bucketName+":"+fallback.record.PolicyID)
			fallbackCodes = append(fallbackCodes, fallback.record.AllowedValueCodes...)
			winnerPolicyIDs = append(winnerPolicyIDs, fallback.record.PolicyID)
		}
		winnerPolicyIDs = legacyNormalizePolicyIDs(winnerPolicyIDs)
		allowedValueCodes, err := legacyMergeAllowedValueCodes(
			primary.AllowedValueCodes,
			fallbackCodes,
			bucket.setIDExact,
			primary.PriorityMode,
			primary.LocalOverrideMode,
		)
		if err != nil {
			return Decision{}, err
		}
		trace = append(trace, "mode:"+primary.PriorityMode+"/"+primary.LocalOverrideMode+":allowed="+strings.Join(allowedValueCodes, ","))
		resolvedDefaultValue := legacyResolveDefaultValue(primary, fallbackBuckets, allowedValueCodes, bucket.setIDExact)

		if primary.Required && !primary.Visible {
			return Decision{}, errors.New(ErrorPolicyConflict)
		}
		if !primary.Maintainable && strings.TrimSpace(primary.DefaultRuleRef) == "" && strings.TrimSpace(resolvedDefaultValue) == "" {
			return Decision{}, errors.New(ErrorDefaultRuleMissing)
		}
		if primary.Required && len(primary.AllowedValueCodes) > 0 && len(allowedValueCodes) == 0 {
			return Decision{}, errors.New(ErrorPolicyConflict)
		}
		if strings.TrimSpace(resolvedDefaultValue) != "" && len(allowedValueCodes) > 0 && !legacyContainsValue(allowedValueCodes, resolvedDefaultValue) {
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

func legacyBuildBucketSpecs(ctx PolicyContext, baselineCapabilityKey string) []bucketSpec {
	intent := normalizeCapabilityKey(ctx.CapabilityKey)
	baseline := normalizeCapabilityKey(baselineCapabilityKey)
	if baseline == "" || baseline == intent {
		return []bucketSpec{
			{name: BucketIntentSetIDBusinessUnitExact, sourceType: SourceTypeIntentOverride, capabilityKey: intent, resolvedSetID: ctx.ResolvedSetID, businessUnitNodeKey: ctx.BusinessUnitNodeKey, setIDExact: true, businessUnitExact: true},
			{name: BucketIntentSetIDWildcard, sourceType: SourceTypeIntentOverride, capabilityKey: intent, resolvedSetID: ctx.ResolvedSetID, setIDExact: true},
			{name: BucketIntentWildcard, sourceType: SourceTypeIntentOverride, capabilityKey: intent},
		}
	}
	return []bucketSpec{
		{name: BucketIntentSetIDBusinessUnitExact, sourceType: SourceTypeIntentOverride, capabilityKey: intent, resolvedSetID: ctx.ResolvedSetID, businessUnitNodeKey: ctx.BusinessUnitNodeKey, setIDExact: true, businessUnitExact: true},
		{name: BucketIntentSetIDWildcard, sourceType: SourceTypeIntentOverride, capabilityKey: intent, resolvedSetID: ctx.ResolvedSetID, setIDExact: true},
		{name: BucketIntentWildcard, sourceType: SourceTypeIntentOverride, capabilityKey: intent},
		{name: BucketBaselineSetIDBusinessUnit, sourceType: SourceTypeBaseline, capabilityKey: baseline, resolvedSetID: ctx.ResolvedSetID, businessUnitNodeKey: ctx.BusinessUnitNodeKey, setIDExact: true, businessUnitExact: true},
		{name: BucketBaselineSetIDWildcard, sourceType: SourceTypeBaseline, capabilityKey: baseline, resolvedSetID: ctx.ResolvedSetID, setIDExact: true},
		{name: BucketBaselineWildcard, sourceType: SourceTypeBaseline, capabilityKey: baseline},
	}
}

func legacyMatchBucketRecords(records []Record, fieldKey string, bucket bucketSpec) []Record {
	out := make([]Record, 0, len(records))
	for _, record := range records {
		if record.CapabilityKey != bucket.capabilityKey || record.FieldKey != fieldKey {
			continue
		}
		if bucket.businessUnitExact {
			if record.OrgApplicability == OrgApplicabilityBusinessUnit &&
				record.ResolvedSetID == bucket.resolvedSetID &&
				record.BusinessUnitNodeKey == bucket.businessUnitNodeKey {
				out = append(out, record)
			}
			continue
		}
		if bucket.setIDExact {
			if record.OrgApplicability == OrgApplicabilityTenant &&
				record.BusinessUnitNodeKey == "" &&
				record.ResolvedSetID == bucket.resolvedSetID {
				out = append(out, record)
			}
			continue
		}
		if record.OrgApplicability == OrgApplicabilityTenant && record.BusinessUnitNodeKey == "" && record.ResolvedSetID == "" {
			out = append(out, record)
		}
	}
	return out
}

func legacyCollectFallbackBucketWinners(records []Record, fieldKey string, buckets []bucketSpec) []fallbackBucketWinner {
	out := make([]fallbackBucketWinner, 0, len(buckets))
	for _, bucket := range buckets {
		matches := legacyMatchBucketRecords(records, fieldKey, bucket)
		if len(matches) == 0 {
			continue
		}
		legacySortRecords(matches)
		out = append(out, fallbackBucketWinner{bucketName: bucket.name, record: matches[0]})
	}
	return out
}

func legacySortRecords(records []Record) {
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

func legacyMergeAllowedValueCodes(localCodes []string, fallbackCodes []string, localExact bool, priorityMode string, localOverrideMode string) ([]string, error) {
	localCodes = normalizeAllowedValueCodes(localCodes)
	fallbackCodes = normalizeAllowedValueCodes(fallbackCodes)
	priorityMode = normalizePriorityMode(priorityMode)
	localOverrideMode = normalizeLocalOverrideMode(localOverrideMode)

	if err := legacyValidateModes(priorityMode, localOverrideMode); err != nil {
		return nil, err
	}
	if !localExact || len(fallbackCodes) == 0 {
		return localCodes, nil
	}

	switch priorityMode {
	case PriorityModeBlendCustomFirst:
		switch localOverrideMode {
		case LocalOverrideModeAllow:
			return legacyMergePreferPrimary(localCodes, fallbackCodes), nil
		case LocalOverrideModeNoOverride:
			return legacyMergePreferPrimary(fallbackCodes, localCodes), nil
		case LocalOverrideModeNoLocal:
			return fallbackCodes, nil
		}
	case PriorityModeBlendDefltFirst:
		switch localOverrideMode {
		case LocalOverrideModeAllow, LocalOverrideModeNoOverride:
			return legacyMergePreferPrimary(fallbackCodes, localCodes), nil
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

func legacyMergePreferPrimary(primary []string, secondary []string) []string {
	merged := make([]string, 0, len(primary)+len(secondary))
	merged = append(merged, primary...)
	merged = append(merged, secondary...)
	return normalizeAllowedValueCodes(merged)
}

func legacyResolveDefaultValue(primary Record, fallbacks []fallbackBucketWinner, allowedValueCodes []string, localExact bool) string {
	defaultValue := strings.TrimSpace(primary.DefaultValue)
	if defaultValue != "" && (len(allowedValueCodes) == 0 || legacyContainsValue(allowedValueCodes, defaultValue)) {
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
		if len(allowedValueCodes) == 0 || legacyContainsValue(allowedValueCodes, candidate) {
			return candidate
		}
	}
	return defaultValue
}

func legacyValidateModes(priorityMode string, localOverrideMode string) error {
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

func legacyRecordPolicyIDs(records []Record) []string {
	out := make([]string, 0, len(records))
	for _, record := range records {
		out = append(out, record.PolicyID)
	}
	return legacyNormalizePolicyIDs(out)
}

func legacyNormalizePolicyIDs(ids []string) []string {
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

func legacyContainsValue(values []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range values {
		if strings.TrimSpace(candidate) == value {
			return true
		}
	}
	return false
}

func cloneRecordsForTest(records []Record) []Record {
	if len(records) == 0 {
		return nil
	}
	out := make([]Record, 0, len(records))
	for _, record := range records {
		copyRecord := record
		copyRecord.AllowedValueCodes = append([]string(nil), record.AllowedValueCodes...)
		out = append(out, copyRecord)
	}
	return out
}

func sameErrorMessage(got error, want error) bool {
	switch {
	case got == nil && want == nil:
		return true
	case got == nil || want == nil:
		return false
	default:
		return got.Error() == want.Error()
	}
}
