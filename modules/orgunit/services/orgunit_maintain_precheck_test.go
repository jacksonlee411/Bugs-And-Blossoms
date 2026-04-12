package services

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

type orgUnitMaintainPrecheckReaderStub struct {
	resolveOrgNodeKeyFn             func(context.Context, string, string) (string, error)
	resolveSetIDFn                  func(context.Context, string, string, string) (string, error)
	isOrgTreeInitializedFn          func(context.Context, string) (bool, error)
	resolveFieldDecisionFn          func(context.Context, string, string, string, string, string) (orgunittypes.SetIDStrategyFieldDecision, bool, error)
	listEnabledTenantFieldConfigsFn func(context.Context, string, string) ([]orgunittypes.TenantFieldConfig, error)
	resolveTargetExistsAsOfFn       func(context.Context, string, string, string) (bool, error)
	resolveMutationTargetEventFn    func(context.Context, string, string, string) (OrgUnitMaintainTargetEventV1, error)
}

func (s orgUnitMaintainPrecheckReaderStub) ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error) {
	if s.resolveOrgNodeKeyFn != nil {
		return s.resolveOrgNodeKeyFn(ctx, tenantID, orgCode)
	}
	return "", nil
}

func (s orgUnitMaintainPrecheckReaderStub) ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error) {
	if s.resolveSetIDFn != nil {
		return s.resolveSetIDFn(ctx, tenantID, orgNodeKey, asOf)
	}
	return "", nil
}

func (s orgUnitMaintainPrecheckReaderStub) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	if s.isOrgTreeInitializedFn != nil {
		return s.isOrgTreeInitializedFn(ctx, tenantID)
	}
	return false, nil
}

func (s orgUnitMaintainPrecheckReaderStub) ResolveSetIDStrategyFieldDecision(
	ctx context.Context,
	tenantID string,
	capabilityKey string,
	fieldKey string,
	businessUnitNodeKey string,
	asOf string,
) (orgunittypes.SetIDStrategyFieldDecision, bool, error) {
	if s.resolveFieldDecisionFn != nil {
		return s.resolveFieldDecisionFn(ctx, tenantID, capabilityKey, fieldKey, businessUnitNodeKey, asOf)
	}
	return orgunittypes.SetIDStrategyFieldDecision{}, false, nil
}

func (s orgUnitMaintainPrecheckReaderStub) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgunittypes.TenantFieldConfig, error) {
	if s.listEnabledTenantFieldConfigsFn != nil {
		return s.listEnabledTenantFieldConfigsFn(ctx, tenantID, asOf)
	}
	return nil, nil
}

func (s orgUnitMaintainPrecheckReaderStub) ResolveTargetExistsAsOf(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (bool, error) {
	if s.resolveTargetExistsAsOfFn != nil {
		return s.resolveTargetExistsAsOfFn(ctx, tenantID, orgNodeKey, asOf)
	}
	return false, nil
}

func (s orgUnitMaintainPrecheckReaderStub) ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (OrgUnitMaintainTargetEventV1, error) {
	if s.resolveMutationTargetEventFn != nil {
		return s.resolveMutationTargetEventFn(ctx, tenantID, orgNodeKey, effectiveDate)
	}
	return OrgUnitMaintainTargetEventV1{}, nil
}

func testMaintainReaderReady() orgUnitMaintainPrecheckReaderStub {
	return orgUnitMaintainPrecheckReaderStub{
		resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) {
			return "10000003", nil
		},
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "S2601", nil
		},
		isOrgTreeInitializedFn: func(context.Context, string) (bool, error) {
			return true, nil
		},
		resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (orgunittypes.SetIDStrategyFieldDecision, bool, error) {
			switch fieldKey {
			case "name", "parent_org_code":
				return orgunittypes.SetIDStrategyFieldDecision{
					FieldKey:     fieldKey,
					Visible:      true,
					Maintainable: true,
				}, true, nil
			default:
				return orgunittypes.SetIDStrategyFieldDecision{}, false, nil
			}
		},
		listEnabledTenantFieldConfigsFn: func(context.Context, string, string) ([]orgunittypes.TenantFieldConfig, error) {
			return []orgunittypes.TenantFieldConfig{
				{FieldKey: " ext_str_01 "},
				{FieldKey: ""},
			}, nil
		},
		resolveTargetExistsAsOfFn: func(context.Context, string, string, string) (bool, error) {
			return true, nil
		},
		resolveMutationTargetEventFn: func(context.Context, string, string, string) (OrgUnitMaintainTargetEventV1, error) {
			return OrgUnitMaintainTargetEventV1{
				HasEffective:       true,
				EffectiveEventType: orgunittypes.OrgUnitEventCreate,
				HasRaw:             true,
				RawEventType:       orgunittypes.OrgUnitEventCreate,
			}, nil
		},
	}
}

func TestBuildOrgUnitMaintainPrecheckProjectionV1_CorrectReady(t *testing.T) {
	result, err := BuildOrgUnitMaintainPrecheckProjectionV1(context.Background(), testMaintainReaderReady(), OrgUnitMaintainPrecheckInputV1{
		Intent:                 OrgUnitMaintainIntentCorrect,
		TenantID:               "tenant_1",
		CapabilityKey:          "org.orgunit_correct.field_policy",
		TargetEffectiveDate:    "2026-01-01",
		OrgCode:                "FLOWER-C",
		EffectivePolicyVersion: "epv1:test",
		CanAdmin:               true,
		NewName:                "运营中心",
	})
	if err != nil {
		t.Fatalf("build err=%v", err)
	}
	if result.Projection.Readiness != orgUnitMaintainReadinessReady {
		t.Fatalf("readiness=%q", result.Projection.Readiness)
	}
	if result.PolicyContext.ResolvedSetID != "S2601" || result.Projection.ResolvedSetID != "S2601" {
		t.Fatalf("setid context=%+v projection=%+v", result.PolicyContext, result.Projection)
	}
	if len(result.Projection.MissingFields) != 0 || len(result.Projection.RejectionReasons) != 0 {
		t.Fatalf("projection=%+v", result.Projection)
	}
	if result.Projection.PolicyExplain != "计划已生成，等待确认后可提交" {
		t.Fatalf("policy explain=%q", result.Projection.PolicyExplain)
	}
	if !strings.Contains(result.Projection.PendingDraftSummary, "目标版本：2026-01-01") {
		t.Fatalf("pending draft=%q", result.Projection.PendingDraftSummary)
	}
	if result.PolicyContext.PolicyContextDigest == "" || result.Projection.ProjectionDigest == "" {
		t.Fatalf("digests missing result=%+v", result)
	}
	if !slices.ContainsFunc(result.Projection.FieldDecisions, func(item OrgUnitMaintainFieldDecisionV1) bool {
		return item.FieldKey == "name" && item.Maintainable && item.FieldPayloadKey == "name"
	}) {
		t.Fatalf("field decisions=%+v", result.Projection.FieldDecisions)
	}
}

func TestBuildOrgUnitMaintainPrecheckProjectionV1_MoveCandidateConfirmationAndRejected(t *testing.T) {
	moveResult, err := BuildOrgUnitMaintainPrecheckProjectionV1(context.Background(), testMaintainReaderReady(), OrgUnitMaintainPrecheckInputV1{
		Intent:                        OrgUnitMaintainIntentMove,
		TenantID:                      "tenant_1",
		CapabilityKey:                 "org.orgunit_write.field_policy",
		EffectiveDate:                 "2026-04-01",
		OrgCode:                       "FLOWER-C",
		EffectivePolicyVersion:        "epv1:test",
		CanAdmin:                      true,
		CandidateConfirmationRequired: true,
		NewParentRequested:            true,
	})
	if err != nil {
		t.Fatalf("move build err=%v", err)
	}
	if moveResult.Projection.Readiness != orgUnitMaintainReadinessCandidateConfirmation {
		t.Fatalf("move readiness=%q", moveResult.Projection.Readiness)
	}
	if !slices.Equal(moveResult.Projection.CandidateConfirmationRequirements, []string{orgUnitMaintainCandidateRequirementNewParent}) {
		t.Fatalf("move requirements=%v", moveResult.Projection.CandidateConfirmationRequirements)
	}
	if moveResult.Projection.PolicyExplain != "仍需确认新的上级组织" {
		t.Fatalf("move explain=%q", moveResult.Projection.PolicyExplain)
	}

	rejectedReader := testMaintainReaderReady()
	rejectedReader.resolveMutationTargetEventFn = func(context.Context, string, string, string) (OrgUnitMaintainTargetEventV1, error) {
		return OrgUnitMaintainTargetEventV1{
			HasEffective: false,
			HasRaw:       true,
			RawEventType: orgunittypes.OrgUnitEventCreate,
		}, nil
	}
	rejectedResult, err := BuildOrgUnitMaintainPrecheckProjectionV1(context.Background(), rejectedReader, OrgUnitMaintainPrecheckInputV1{
		Intent:                 OrgUnitMaintainIntentCorrect,
		TenantID:               "tenant_1",
		CapabilityKey:          "org.orgunit_correct.field_policy",
		TargetEffectiveDate:    "2026-01-01",
		OrgCode:                "FLOWER-C",
		EffectivePolicyVersion: "epv1:test",
		CanAdmin:               true,
		NewName:                "运营中心",
	})
	if err != nil {
		t.Fatalf("rejected build err=%v", err)
	}
	if rejectedResult.Projection.Readiness != orgUnitMaintainReadinessRejected {
		t.Fatalf("rejected readiness=%q", rejectedResult.Projection.Readiness)
	}
	if !slices.Contains(rejectedResult.Projection.RejectionReasons, "ORG_EVENT_RESCINDED") {
		t.Fatalf("rejection reasons=%v", rejectedResult.Projection.RejectionReasons)
	}
	if rejectedResult.Projection.PolicyExplain != "ORG_EVENT_RESCINDED" {
		t.Fatalf("rejected explain=%q", rejectedResult.Projection.PolicyExplain)
	}
}

func TestOrgUnitMaintainPrecheckHelpers(t *testing.T) {
	t.Run("policy context error and clone helpers", func(t *testing.T) {
		var nilErr *OrgUnitMaintainPolicyContextErrorV1
		if nilErr.Error() != "" || nilErr.Unwrap() != nil {
			t.Fatalf("nil err=%v unwrap=%v", nilErr.Error(), nilErr.Unwrap())
		}
		cause := errors.New("boom")
		errValue := &OrgUnitMaintainPolicyContextErrorV1{Code: orgUnitMaintainContextCodeOrgInvalid, Cause: cause}
		if errValue.Error() != orgUnitMaintainContextCodeOrgInvalid || !errors.Is(errValue.Unwrap(), cause) {
			t.Fatalf("err value=%q unwrap=%v", errValue.Error(), errValue.Unwrap())
		}
		if got := CloneOrgUnitMaintainFieldDecisions(nil); got != nil {
			t.Fatalf("nil clone=%v", got)
		}

		original := OrgUnitMaintainPrecheckProjectionV1{
			Readiness:                         " ready ",
			MissingFields:                     []string{"a"},
			FieldDecisions:                    []OrgUnitMaintainFieldDecisionV1{{FieldKey: "name", AllowedValueCodes: []string{"X"}}},
			CandidateConfirmationRequirements: []string{"b"},
			PendingDraftSummary:               " summary ",
			EffectivePolicyVersion:            " epv1 ",
			MutationPolicyVersion:             " mpv1 ",
			ResolvedSetID:                     " S1 ",
			SetIDSource:                       " custom ",
			PolicyExplain:                     " explain ",
			RejectionReasons:                  []string{"FORBIDDEN"},
			ProjectionDigest:                  " digest ",
		}
		cloned := CloneOrgUnitMaintainProjectionV1(original)
		original.MissingFields[0] = "changed"
		original.FieldDecisions[0].AllowedValueCodes[0] = "Y"
		original.CandidateConfirmationRequirements[0] = "changed"
		original.RejectionReasons[0] = "changed"
		if cloned.Readiness != "ready" || cloned.MissingFields[0] != "a" || cloned.FieldDecisions[0].AllowedValueCodes[0] != "X" || cloned.CandidateConfirmationRequirements[0] != "b" || cloned.RejectionReasons[0] != "FORBIDDEN" {
			t.Fatalf("cloned=%+v", cloned)
		}
		if cloned.FieldDecisions[0].FieldPayloadKey != "" {
			t.Fatalf("unexpected field payload key=%q", cloned.FieldDecisions[0].FieldPayloadKey)
		}
	})

	t.Run("candidate, rejection, readiness and explain", func(t *testing.T) {
		if got := normalizeOrgUnitMaintainCandidateRequirements(false, nil); len(got) != 0 {
			t.Fatalf("empty requirements=%v", got)
		}
		if got := normalizeOrgUnitMaintainCandidateRequirements(true, nil); !slices.Equal(got, []string{orgUnitMaintainCandidateRequirementNewParent}) {
			t.Fatalf("default requirements=%v", got)
		}
		if got := normalizeOrgUnitMaintainCandidateRequirements(true, []string{" new_parent_org_code ", "", "new_parent_org_code"}); !slices.Equal(got, []string{"new_parent_org_code"}) {
			t.Fatalf("dedup requirements=%v", got)
		}

		reasons := normalizeOrgUnitMaintainRejectionReasons([]string{"PATCH_FIELD_NOT_ALLOWED", "ORG_EVENT_RESCINDED", orgUnitMaintainContextCodeOrgInvalid, "PATCH_FIELD_NOT_ALLOWED"})
		if !slices.Equal(reasons, []string{orgUnitMaintainContextCodeOrgInvalid, "ORG_EVENT_RESCINDED", "PATCH_FIELD_NOT_ALLOWED"}) {
			t.Fatalf("reasons=%v", reasons)
		}

		if got := resolveOrgUnitMaintainReadiness(OrgUnitMaintainPrecheckProjectionV1{}); got != orgUnitMaintainReadinessReady {
			t.Fatalf("ready=%q", got)
		}
		if got := resolveOrgUnitMaintainReadiness(OrgUnitMaintainPrecheckProjectionV1{MissingFields: []string{"x"}}); got != orgUnitMaintainReadinessMissingFields {
			t.Fatalf("missing=%q", got)
		}
		if got := resolveOrgUnitMaintainReadiness(OrgUnitMaintainPrecheckProjectionV1{CandidateConfirmationRequirements: []string{"x"}}); got != orgUnitMaintainReadinessCandidateConfirmation {
			t.Fatalf("candidate=%q", got)
		}
		if got := resolveOrgUnitMaintainReadiness(OrgUnitMaintainPrecheckProjectionV1{RejectionReasons: []string{"FORBIDDEN"}}); got != orgUnitMaintainReadinessRejected {
			t.Fatalf("rejected=%q", got)
		}

		cases := map[string]string{
			orgUnitMaintainReadinessReady:                 "计划已生成，等待确认后可提交",
			orgUnitMaintainReadinessMissingFields:         "仍有必填字段未补全",
			orgUnitMaintainReadinessCandidateConfirmation: "仍需确认新的上级组织",
		}
		for readiness, want := range cases {
			if got := buildOrgUnitMaintainPolicyExplain(OrgUnitMaintainPrecheckProjectionV1{Readiness: readiness}); got != want {
				t.Fatalf("readiness=%s explain=%q", readiness, got)
			}
		}
		if got := buildOrgUnitMaintainPolicyExplain(OrgUnitMaintainPrecheckProjectionV1{Readiness: orgUnitMaintainReadinessRejected, RejectionReasons: []string{"FORBIDDEN"}}); got != "FORBIDDEN" {
			t.Fatalf("rejected explain=%q", got)
		}
		if got := buildOrgUnitMaintainPolicyExplain(OrgUnitMaintainPrecheckProjectionV1{Readiness: orgUnitMaintainReadinessRejected}); got != "当前草案已被策略拒绝" {
			t.Fatalf("rejected empty explain=%q", got)
		}
		if got := buildOrgUnitMaintainPolicyExplain(OrgUnitMaintainPrecheckProjectionV1{Readiness: "other"}); got != "" {
			t.Fatalf("unexpected explain=%q", got)
		}
	})

	t.Run("field, context and missing-field helpers", func(t *testing.T) {
		missingDecision := buildOrgUnitMaintainPDPFieldDecision("name", orgunittypes.SetIDStrategyFieldDecision{}, false, "name")
		if !missingDecision.Visible || !missingDecision.Maintainable || missingDecision.Required {
			t.Fatalf("missing decision=%+v", missingDecision)
		}
		foundDecision := buildOrgUnitMaintainPDPFieldDecision("name", orgunittypes.SetIDStrategyFieldDecision{
			Visible:           true,
			Required:          true,
			Maintainable:      true,
			DefaultRuleRef:    "rule",
			DefaultValue:      "v",
			AllowedValueCodes: []string{"B", "A", "B"},
		}, true, "name")
		if !foundDecision.Required || !foundDecision.Maintainable || foundDecision.DefaultRuleRef != "rule" || !slices.Equal(foundDecision.AllowedValueCodes, []string{"B", "A"}) {
			t.Fatalf("found decision=%+v", foundDecision)
		}
		if !slices.Equal(orgUnitMaintainFieldDecisionOrder(OrgUnitMaintainIntentCorrect), []string{"org_code", "target_effective_date", "name", "parent_org_code"}) {
			t.Fatalf("correct order=%v", orgUnitMaintainFieldDecisionOrder(OrgUnitMaintainIntentCorrect))
		}
		if !slices.Equal(orgUnitMaintainFieldDecisionOrder(OrgUnitMaintainIntentMove), []string{"org_code", "effective_date", "parent_org_code"}) {
			t.Fatalf("move order=%v", orgUnitMaintainFieldDecisionOrder(OrgUnitMaintainIntentMove))
		}
		if !slices.Equal(orgUnitMaintainFieldDecisionOrder(OrgUnitMaintainIntentRename), []string{"org_code", "effective_date", "name"}) {
			t.Fatalf("rename order=%v", orgUnitMaintainFieldDecisionOrder(OrgUnitMaintainIntentRename))
		}
		if !slices.Equal(orgUnitMaintainFieldDecisionOrder("other"), []string{"org_code"}) {
			t.Fatalf("default order=%v", orgUnitMaintainFieldDecisionOrder("other"))
		}

		correctMissing := normalizeOrgUnitMaintainMissingFields(OrgUnitMaintainPrecheckInputV1{
			Intent: OrgUnitMaintainIntentCorrect,
		}, orgUnitMaintainPrecheckEvaluation{})
		if !slices.Equal(correctMissing, []string{"org_code", "target_effective_date", "change_fields"}) {
			t.Fatalf("correct missing=%v", correctMissing)
		}
		renameMissing := normalizeOrgUnitMaintainMissingFields(OrgUnitMaintainPrecheckInputV1{
			Intent:  OrgUnitMaintainIntentRename,
			OrgCode: "FLOWER-C",
		}, orgUnitMaintainPrecheckEvaluation{})
		if !slices.Equal(renameMissing, []string{"effective_date", "new_name"}) {
			t.Fatalf("rename missing=%v", renameMissing)
		}
		moveMissing := normalizeOrgUnitMaintainMissingFields(OrgUnitMaintainPrecheckInputV1{
			Intent:        OrgUnitMaintainIntentMove,
			OrgCode:       "FLOWER-C",
			EffectiveDate: "2026-04-01",
		}, orgUnitMaintainPrecheckEvaluation{})
		if !slices.Equal(moveMissing, []string{"new_parent_ref_text"}) {
			t.Fatalf("move missing=%v", moveMissing)
		}
		dedupMissing := normalizeOrgUnitMaintainMissingFields(OrgUnitMaintainPrecheckInputV1{
			Intent:  OrgUnitMaintainIntentMove,
			OrgCode: "FLOWER-C",
		}, orgUnitMaintainPrecheckEvaluation{
			ParentFound:    true,
			ParentDecision: orgunittypes.SetIDStrategyFieldDecision{Required: true},
		})
		if !slices.Equal(dedupMissing, []string{"effective_date", "new_parent_ref_text"}) {
			t.Fatalf("dedup missing=%v", dedupMissing)
		}

		rejectionReasons := normalizeOrgUnitMaintainFieldRejectionReasons(OrgUnitMaintainPrecheckInputV1{
			NewName:            "运营中心",
			NewParentOrgCode:   "FLOWER-A",
			NewParentRequested: true,
		}, orgUnitMaintainPrecheckEvaluation{
			MutationDecision: OrgUnitMutationPolicyDecision{FieldPayloadKeys: map[string]string{}},
		})
		if !slices.Equal(rejectionReasons, []string{errPatchFieldNotAllowed, errPatchFieldNotAllowed}) {
			t.Fatalf("field rejection reasons=%v", rejectionReasons)
		}
		fieldReasons := normalizeOrgUnitMaintainFieldRejectionReasons(OrgUnitMaintainPrecheckInputV1{
			NewName:            "BAD-NAME",
			NewParentRequested: true,
			NewParentOrgCode:   "BAD-PARENT",
		}, orgUnitMaintainPrecheckEvaluation{
			MutationDecision: OrgUnitMutationPolicyDecision{FieldPayloadKeys: map[string]string{
				"name":            "name",
				"parent_org_code": "parent_org_node_key",
			}},
			NameFound: true,
			NameDecision: orgunittypes.SetIDStrategyFieldDecision{
				Maintainable:      false,
				AllowedValueCodes: []string{"GOOD-NAME"},
			},
			ParentFound: true,
			ParentDecision: orgunittypes.SetIDStrategyFieldDecision{
				Maintainable:      false,
				AllowedValueCodes: []string{"GOOD-PARENT"},
			},
		})
		wantFieldReasons := []string{
			errPatchFieldNotAllowed,
			errPatchFieldNotAllowed,
			errFieldOptionNotAllowed,
			errFieldOptionNotAllowed,
		}
		if !slices.Equal(fieldReasons, wantFieldReasons) {
			t.Fatalf("field reasons=%v want=%v", fieldReasons, wantFieldReasons)
		}
	})

	t.Run("policy context and target exists branches", func(t *testing.T) {
		ctxValue, ctxErr := resolveOrgUnitMaintainPolicyContextV1(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{
			TenantID:      "tenant_1",
			CapabilityKey: "cap",
		}, "2026-01-01")
		if ctxErr != nil || ctxValue.PolicyContextDigest == "" {
			t.Fatalf("nil reader ctx=%+v err=%v", ctxValue, ctxErr)
		}

		orgErrReader := orgUnitMaintainPrecheckReaderStub{
			resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) {
				return "", errors.New("org invalid")
			},
		}
		if _, ctxErr = resolveOrgUnitMaintainPolicyContextV1(context.Background(), orgErrReader, OrgUnitMaintainPrecheckInputV1{
			TenantID: "tenant_1",
			OrgCode:  "FLOWER-C",
		}, "2026-01-01"); ctxErr == nil || ctxErr.Code != orgUnitMaintainContextCodeOrgInvalid {
			t.Fatalf("org err=%v", ctxErr)
		}

		setIDErrReader := orgUnitMaintainPrecheckReaderStub{
			resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) {
				return "10000003", nil
			},
			resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
				return "", errors.New("setid missing")
			},
		}
		if _, ctxErr = resolveOrgUnitMaintainPolicyContextV1(context.Background(), setIDErrReader, OrgUnitMaintainPrecheckInputV1{
			TenantID: "tenant_1",
			OrgCode:  "FLOWER-C",
		}, "2026-01-01"); ctxErr == nil || ctxErr.Code != orgUnitMaintainContextCodeSetIDBindingMissing {
			t.Fatalf("setid err=%v", ctxErr)
		}
		emptyNodeReader := orgUnitMaintainPrecheckReaderStub{
			resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) {
				return " ", nil
			},
		}
		if ctxValue, ctxErr = resolveOrgUnitMaintainPolicyContextV1(context.Background(), emptyNodeReader, OrgUnitMaintainPrecheckInputV1{
			TenantID: "tenant_1",
			OrgCode:  "FLOWER-C",
		}, "2026-01-01"); ctxErr != nil || ctxValue.OrgNodeKey != "" || ctxValue.PolicyContextDigest == "" {
			t.Fatalf("empty node ctx=%+v err=%v", ctxValue, ctxErr)
		}
		setIDNoneReader := orgUnitMaintainPrecheckReaderStub{
			resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) {
				return "10000003", nil
			},
			resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
				return " ", nil
			},
		}
		if ctxValue, ctxErr = resolveOrgUnitMaintainPolicyContextV1(context.Background(), setIDNoneReader, OrgUnitMaintainPrecheckInputV1{
			TenantID: "tenant_1",
			OrgCode:  "FLOWER-C",
		}, "2026-01-01"); ctxErr != nil || ctxValue.ResolvedSetID != "" || ctxValue.SetIDSource != "none" || ctxValue.PolicyContextDigest == "" {
			t.Fatalf("setid none ctx=%+v err=%v", ctxValue, ctxErr)
		}

		if initialized, err := resolveOrgUnitMaintainTreeInitialized(context.Background(), nil, "tenant_1"); err != nil || initialized {
			t.Fatalf("nil tree initialized=%v err=%v", initialized, err)
		}
		targetReader := orgUnitMaintainPrecheckReaderStub{
			resolveTargetExistsAsOfFn: func(context.Context, string, string, string) (bool, error) {
				return true, nil
			},
		}
		if exists, err := resolveOrgUnitMaintainTargetExistsAsOf(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{}, "10000003"); err != nil || exists {
			t.Fatalf("nil target exists=%v err=%v", exists, err)
		}
		if exists, err := resolveOrgUnitMaintainTargetExistsAsOf(context.Background(), targetReader, OrgUnitMaintainPrecheckInputV1{
			Intent:        OrgUnitMaintainIntentCorrect,
			OrgCode:       "FLOWER-C",
			EffectiveDate: "2026-01-01",
		}, "10000003"); err != nil || !exists {
			t.Fatalf("correct target exists=%v err=%v", exists, err)
		}
		if exists, err := resolveOrgUnitMaintainTargetExistsAsOf(context.Background(), targetReader, OrgUnitMaintainPrecheckInputV1{
			Intent:        OrgUnitMaintainIntentRename,
			OrgCode:       "FLOWER-C",
			EffectiveDate: "2026-01-01",
		}, "10000003"); err != nil || !exists {
			t.Fatalf("rename target exists=%v err=%v", exists, err)
		}
		if exists, err := resolveOrgUnitMaintainTargetExistsAsOf(context.Background(), targetReader, OrgUnitMaintainPrecheckInputV1{
			Intent:  "other",
			OrgCode: "FLOWER-C",
		}, "10000003"); err != nil || exists {
			t.Fatalf("other target exists=%v err=%v", exists, err)
		}
		targetErrReader := orgUnitMaintainPrecheckReaderStub{
			resolveTargetExistsAsOfFn: func(context.Context, string, string, string) (bool, error) {
				return false, errors.New("target exists failed")
			},
		}
		if _, err := resolveOrgUnitMaintainTargetExistsAsOf(context.Background(), targetErrReader, OrgUnitMaintainPrecheckInputV1{
			Intent:        OrgUnitMaintainIntentMove,
			OrgCode:       "FLOWER-C",
			EffectiveDate: "2026-01-01",
		}, "10000003"); err == nil || err.Error() != "target exists failed" {
			t.Fatalf("target exists err=%v", err)
		}
	})

	t.Run("top level build error", func(t *testing.T) {
		_, err := BuildOrgUnitMaintainPrecheckProjectionV1(context.Background(), orgUnitMaintainPrecheckReaderStub{
			listEnabledTenantFieldConfigsFn: func(context.Context, string, string) ([]orgunittypes.TenantFieldConfig, error) {
				return nil, errors.New("field configs boom")
			},
		}, OrgUnitMaintainPrecheckInputV1{Intent: OrgUnitMaintainIntentRename})
		if err == nil || err.Error() != "field configs boom" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("field decision and mutation decision branches", func(t *testing.T) {
		if decision, found, reason := resolveOrgUnitMaintainFieldDecision(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{}, "10000003", "name", "2026-01-01"); found || reason != "" || decision.FieldKey != "" || len(decision.AllowedValueCodes) != 0 {
			t.Fatalf("nil reader decision=%+v found=%v reason=%q", decision, found, reason)
		}
		errReader := orgUnitMaintainPrecheckReaderStub{
			resolveFieldDecisionFn: func(context.Context, string, string, string, string, string) (orgunittypes.SetIDStrategyFieldDecision, bool, error) {
				return orgunittypes.SetIDStrategyFieldDecision{}, false, errors.New(errPatchFieldNotAllowed)
			},
		}
		if _, found, reason := resolveOrgUnitMaintainFieldDecision(context.Background(), errReader, OrgUnitMaintainPrecheckInputV1{
			TenantID:      "tenant_1",
			CapabilityKey: "cap",
		}, "10000003", "name", "2026-01-01"); found || reason != errPatchFieldNotAllowed {
			t.Fatalf("reason=%q found=%v", reason, found)
		}

		renameDecision, targetEvent, err := resolveOrgUnitMaintainMutationDecision(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{
			Intent:   OrgUnitMaintainIntentRename,
			CanAdmin: true,
		}, "10000003", true, true, []string{"ext_str_01"})
		if err != nil || !renameDecision.Enabled || targetEvent != (OrgUnitMaintainTargetEventV1{}) {
			t.Fatalf("rename decision=%+v target=%+v err=%v", renameDecision, targetEvent, err)
		}

		moveDecision, targetEvent, err := resolveOrgUnitMaintainMutationDecision(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{
			Intent:   OrgUnitMaintainIntentMove,
			CanAdmin: true,
			OrgCode:  "ROOT",
		}, "10000001", true, true, nil)
		if err != nil || moveDecision.Enabled || !slices.Contains(moveDecision.DenyReasons, "ORG_ROOT_CANNOT_BE_MOVED") || targetEvent != (OrgUnitMaintainTargetEventV1{}) {
			t.Fatalf("move decision=%+v target=%+v err=%v", moveDecision, targetEvent, err)
		}

		correctDecision, targetEvent, err := resolveOrgUnitMaintainMutationDecision(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{
			Intent:              OrgUnitMaintainIntentCorrect,
			TargetEffectiveDate: "2026-01-01",
			OrgCode:             "FLOWER-C",
		}, "", false, false, nil)
		if err != nil || correctDecision.Enabled || !slices.Contains(correctDecision.DenyReasons, "FORBIDDEN") || targetEvent != (OrgUnitMaintainTargetEventV1{}) {
			t.Fatalf("correct decision=%+v target=%+v err=%v", correctDecision, targetEvent, err)
		}

		errorReader := orgUnitMaintainPrecheckReaderStub{
			resolveMutationTargetEventFn: func(context.Context, string, string, string) (OrgUnitMaintainTargetEventV1, error) {
				return OrgUnitMaintainTargetEventV1{}, errors.New("target event failed")
			},
		}
		if _, _, err := resolveOrgUnitMaintainMutationDecision(context.Background(), errorReader, OrgUnitMaintainPrecheckInputV1{
			Intent:              OrgUnitMaintainIntentCorrect,
			CanAdmin:            true,
			TargetEffectiveDate: "2026-01-01",
			OrgCode:             "FLOWER-C",
		}, "10000003", true, true, nil); err == nil || err.Error() != "target event failed" {
			t.Fatalf("expected target event failure, got=%v", err)
		}

		rawOnlyReader := orgUnitMaintainPrecheckReaderStub{
			resolveMutationTargetEventFn: func(context.Context, string, string, string) (OrgUnitMaintainTargetEventV1, error) {
				return OrgUnitMaintainTargetEventV1{
					HasRaw: true,
				}, nil
			},
		}
		correctDecision, targetEvent, err = resolveOrgUnitMaintainMutationDecision(context.Background(), rawOnlyReader, OrgUnitMaintainPrecheckInputV1{
			Intent:              OrgUnitMaintainIntentCorrect,
			CanAdmin:            true,
			TargetEffectiveDate: "2026-01-01",
			OrgCode:             "FLOWER-C",
		}, "10000003", true, true, nil)
		if err != nil || correctDecision.Enabled || !slices.Contains(correctDecision.DenyReasons, "ORG_EVENT_RESCINDED") || !targetEvent.HasRaw {
			t.Fatalf("raw only correct decision=%+v target=%+v err=%v", correctDecision, targetEvent, err)
		}

		otherDecision, targetEvent, err := resolveOrgUnitMaintainMutationDecision(context.Background(), nil, OrgUnitMaintainPrecheckInputV1{
			Intent: "other",
		}, "10000003", true, true, nil)
		if err != nil || otherDecision.Enabled || len(otherDecision.AllowedFields) != 0 || len(otherDecision.FieldPayloadKeys) != 0 || len(otherDecision.DenyReasons) != 0 || targetEvent != (OrgUnitMaintainTargetEventV1{}) {
			t.Fatalf("other decision=%+v target=%+v err=%v", otherDecision, targetEvent, err)
		}
	})
}
