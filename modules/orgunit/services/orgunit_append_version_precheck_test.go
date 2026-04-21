package services

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

type orgUnitAppendVersionPrecheckReaderStub struct {
	resolveOrgNodeKeyFn func(context.Context, string, string) (string, error)
	treeInitFn          func(context.Context, string) (bool, error)
	listConfigsFn       func(context.Context, string, string) ([]types.TenantFieldConfig, error)
	resolveFactsFn      func(context.Context, string, string, string) (OrgUnitAppendVersionFactsV1, error)
}

func (s orgUnitAppendVersionPrecheckReaderStub) ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error) {
	if s.resolveOrgNodeKeyFn != nil {
		return s.resolveOrgNodeKeyFn(ctx, tenantID, orgCode)
	}
	return "", nil
}

func (s orgUnitAppendVersionPrecheckReaderStub) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	if s.treeInitFn != nil {
		return s.treeInitFn(ctx, tenantID)
	}
	return false, nil
}

func (s orgUnitAppendVersionPrecheckReaderStub) ListEnabledTenantFieldConfigsAsOf(
	ctx context.Context,
	tenantID string,
	asOf string,
) ([]types.TenantFieldConfig, error) {
	if s.listConfigsFn != nil {
		return s.listConfigsFn(ctx, tenantID, asOf)
	}
	return nil, nil
}

func (s orgUnitAppendVersionPrecheckReaderStub) ResolveAppendFacts(
	ctx context.Context,
	tenantID string,
	orgNodeKey string,
	effectiveDate string,
) (OrgUnitAppendVersionFactsV1, error) {
	if s.resolveFactsFn != nil {
		return s.resolveFactsFn(ctx, tenantID, orgNodeKey, effectiveDate)
	}
	return OrgUnitAppendVersionFactsV1{}, nil
}

func TestOrgUnitAppendVersionPrecheckHelpers(t *testing.T) {
	t.Run("policy context error methods", func(t *testing.T) {
		var nilErr *OrgUnitAppendVersionPolicyContextErrorV1
		if nilErr.Error() != "" {
			t.Fatalf("nil error string=%q", nilErr.Error())
		}
		if nilErr.Unwrap() != nil {
			t.Fatalf("nil unwrap=%v", nilErr.Unwrap())
		}

		cause := errors.New("boom")
		errValue := &OrgUnitAppendVersionPolicyContextErrorV1{Code: " org_context_invalid ", Cause: cause}
		if errValue.Error() != "org_context_invalid" {
			t.Fatalf("error string=%q", errValue.Error())
		}
		if !errors.Is(errValue.Unwrap(), cause) {
			t.Fatalf("unwrap=%v", errValue.Unwrap())
		}
	})

		t.Run("normalize input and small helpers", func(t *testing.T) {
			input := normalizeOrgUnitAppendVersionPrecheckInput(OrgUnitAppendVersionPrecheckInputV1{
				Intent:           " add_version ",
				TenantID:         " tenant-1 ",
				EffectiveDate:    " 2026-01-01 ",
				OrgCode:          " flower-c ",
				NewName:          " 运营一部 ",
				NewParentOrgCode: " flower-a ",
			})
		if input.Intent != "add_version" || input.TenantID != "tenant-1" {
			t.Fatalf("normalized input=%+v", input)
		}
		if !input.NewParentRequested || input.NewParentOrgCode != "flower-a" {
			t.Fatalf("new parent normalization=%+v", input)
		}

		readerCalled := false
		if _, _, err := resolveOrgUnitAppendVersionEnabledFieldConfigs(context.Background(), orgUnitAppendVersionPrecheckReaderStub{
			listConfigsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				readerCalled = true
				return nil, nil
			},
		}, OrgUnitAppendVersionPrecheckInputV1{
			EnabledFieldConfigs: []types.TenantFieldConfig{{FieldKey: "custom_text_demo"}},
		}); err != nil {
			t.Fatalf("enabled config pass-through err=%v", err)
		}
		if readerCalled {
			t.Fatal("expected input.EnabledFieldConfigs to bypass reader")
		}

		if got := normalizeOrgUnitAppendVersionCandidateRequirements(false, nil); len(got) != 0 {
			t.Fatalf("unexpected empty requirements=%v", got)
		}
		if got := normalizeOrgUnitAppendVersionCandidateRequirements(true, nil); !slices.Equal(got, []string{orgUnitAppendVersionCandidateRequirementNewParent}) {
			t.Fatalf("default requirements=%v", got)
		}
		gotReqs := normalizeOrgUnitAppendVersionCandidateRequirements(true, []string{" other ", "", orgUnitAppendVersionCandidateRequirementNewParent, "other"})
		if !slices.Equal(gotReqs, []string{orgUnitAppendVersionCandidateRequirementNewParent, "other"}) {
			t.Fatalf("requirements=%v", gotReqs)
		}

		missing := normalizeOrgUnitAppendVersionMissingFields(
			OrgUnitAppendVersionPrecheckInputV1{},
			orgUnitAppendVersionPrecheckEvaluation{
				NameFound:      true,
				NameDecision:   orgUnitFieldDecision{Required: true},
				ParentFound:    true,
				ParentDecision: orgUnitFieldDecision{Required: true},
			},
		)
		wantMissing := []string{"org_code", "effective_date", "change_fields", "new_name", "new_parent_ref_text"}
		if !slices.Equal(missing, wantMissing) {
			t.Fatalf("missing=%v want=%v", missing, wantMissing)
		}

		payloadMissingReasons := normalizeOrgUnitAppendVersionFieldRejectionReasons(
			OrgUnitAppendVersionPrecheckInputV1{NewName: "运营一部", NewParentRequested: true},
			orgUnitAppendVersionPrecheckEvaluation{MutationDecision: OrgUnitWriteCapabilitiesDecision{FieldPayloadKeys: map[string]string{}}},
		)
		if !slices.Equal(payloadMissingReasons, []string{errPatchFieldNotAllowed, errPatchFieldNotAllowed}) {
			t.Fatalf("payload reasons=%v", payloadMissingReasons)
		}

		fieldReasons := normalizeOrgUnitAppendVersionFieldRejectionReasons(
			OrgUnitAppendVersionPrecheckInputV1{
				NewName:            "BAD-NAME",
				NewParentRequested: true,
				NewParentOrgCode:   "BAD-PARENT",
			},
			orgUnitAppendVersionPrecheckEvaluation{
				MutationDecision: OrgUnitWriteCapabilitiesDecision{FieldPayloadKeys: map[string]string{
					"name":            "name",
					"parent_org_code": "parent_org_node_key",
				}},
				NameFound: true,
				NameDecision: orgUnitFieldDecision{
					Maintainable:      false,
					AllowedValueCodes: []string{"GOOD-NAME"},
				},
				ParentFound: true,
				ParentDecision: orgUnitFieldDecision{
					Maintainable:      false,
					AllowedValueCodes: []string{"GOOD-PARENT"},
				},
			},
		)
		wantFieldReasons := []string{
			errPatchFieldNotAllowed,
			errPatchFieldNotAllowed,
			errFieldOptionNotAllowed,
			errFieldOptionNotAllowed,
		}
		if !slices.Equal(fieldReasons, wantFieldReasons) {
			t.Fatalf("field reasons=%v want=%v", fieldReasons, wantFieldReasons)
		}

			ordered := normalizeOrgUnitAppendVersionRejectionReasons([]string{
				"PATCH_FIELD_NOT_ALLOWED",
				"FORBIDDEN",
				"PATCH_FIELD_NOT_ALLOWED",
				"ORG_TREE_NOT_INITIALIZED",
				"policy_missing",
				"",
			})
			wantOrdered := []string{
				"FORBIDDEN",
				"ORG_TREE_NOT_INITIALIZED",
				"PATCH_FIELD_NOT_ALLOWED",
			"policy_missing",
		}
		if !slices.Equal(ordered, wantOrdered) {
			t.Fatalf("ordered=%v want=%v", ordered, wantOrdered)
		}

		if got := resolveOrgUnitAppendVersionReadiness(OrgUnitAppendVersionPrecheckProjectionV1{RejectionReasons: []string{"FORBIDDEN"}}); got != orgUnitAppendVersionReadinessRejected {
			t.Fatalf("rejected readiness=%q", got)
		}
		if got := resolveOrgUnitAppendVersionReadiness(OrgUnitAppendVersionPrecheckProjectionV1{CandidateConfirmationRequirements: []string{"new_parent_org_code"}}); got != orgUnitAppendVersionReadinessCandidateConfirmation {
			t.Fatalf("candidate readiness=%q", got)
		}
		if got := resolveOrgUnitAppendVersionReadiness(OrgUnitAppendVersionPrecheckProjectionV1{MissingFields: []string{"new_name"}}); got != orgUnitAppendVersionReadinessMissingFields {
			t.Fatalf("missing readiness=%q", got)
		}
		if got := resolveOrgUnitAppendVersionReadiness(OrgUnitAppendVersionPrecheckProjectionV1{}); got != orgUnitAppendVersionReadinessReady {
			t.Fatalf("ready readiness=%q", got)
		}

		notFoundDecision := buildOrgUnitAppendVersionPDPFieldDecision("name", orgUnitFieldDecision{}, false, "name")
		if !notFoundDecision.Visible || !notFoundDecision.Maintainable || notFoundDecision.Required {
			t.Fatalf("not found decision=%+v", notFoundDecision)
		}
		foundDecision := buildOrgUnitAppendVersionPDPFieldDecision(
			"parent_org_code",
			orgUnitFieldDecision{
				Visible:           true,
				Required:          true,
				Maintainable:      true,
				DefaultValue:      " FLOWER-A ",
				DefaultRuleRef:    " pick_parent ",
				AllowedValueCodes: []string{" FLOWER-A ", "FLOWER-A", "FLOWER-B"},
			},
			true,
			" parent_org_node_key ",
		)
		if foundDecision.ResolvedDefaultValue != "FLOWER-A" || foundDecision.DefaultRuleRef != "pick_parent" || !slices.Equal(foundDecision.AllowedValueCodes, []string{"FLOWER-A", "FLOWER-B"}) {
			t.Fatalf("found decision=%+v", foundDecision)
		}

		summary := buildOrgUnitAppendVersionPendingDraftSummary(OrgUnitAppendVersionPrecheckInputV1{
			OrgCode:          "FLOWER-C",
			NewName:          "运营一部",
			NewParentOrgCode: "FLOWER-A",
			EffectiveDate:    "2026-01-01",
		})
		if summary != "目标组织：FLOWER-C；新名称：运营一部；新上级组织：FLOWER-A；生效日期：2026-01-01" {
			t.Fatalf("summary=%q", summary)
		}

		if got := buildOrgUnitAppendVersionPolicyExplain(OrgUnitAppendVersionPrecheckProjectionV1{Readiness: orgUnitAppendVersionReadinessReady}); got != "计划已生成，等待确认后可提交" {
			t.Fatalf("ready explain=%q", got)
		}
		if got := buildOrgUnitAppendVersionPolicyExplain(OrgUnitAppendVersionPrecheckProjectionV1{Readiness: orgUnitAppendVersionReadinessMissingFields}); got != "仍有必填字段未补全" {
			t.Fatalf("missing explain=%q", got)
		}
		if got := buildOrgUnitAppendVersionPolicyExplain(OrgUnitAppendVersionPrecheckProjectionV1{Readiness: orgUnitAppendVersionReadinessCandidateConfirmation}); got != "仍需确认新的上级组织" {
			t.Fatalf("candidate explain=%q", got)
		}
		if got := buildOrgUnitAppendVersionPolicyExplain(OrgUnitAppendVersionPrecheckProjectionV1{Readiness: orgUnitAppendVersionReadinessRejected}); got != "当前草案已被策略拒绝" {
			t.Fatalf("rejected explain=%q", got)
		}
		if got := buildOrgUnitAppendVersionPolicyExplain(OrgUnitAppendVersionPrecheckProjectionV1{Readiness: orgUnitAppendVersionReadinessRejected, RejectionReasons: []string{"FORBIDDEN", "policy_missing"}}); got != "FORBIDDEN,policy_missing" {
			t.Fatalf("rejected reasons explain=%q", got)
		}
		if got := buildOrgUnitAppendVersionPolicyExplain(OrgUnitAppendVersionPrecheckProjectionV1{Readiness: "other"}); got != "" {
			t.Fatalf("default explain=%q", got)
		}
		if got := CloneOrgUnitAppendVersionFieldDecisions(nil); got != nil {
			t.Fatalf("nil clone=%v", got)
		}
		appendFieldDecisions := []OrgUnitAppendVersionFieldDecisionV1{{
			FieldKey:             " name ",
			Visible:              true,
			Required:             true,
			Maintainable:         true,
			FieldPayloadKey:      " new_name ",
			ResolvedDefaultValue: " 运营一部 ",
			DefaultRuleRef:       " default_name ",
			AllowedValueCodes:    []string{"A", "B"},
		}}
		clonedFieldDecisions := CloneOrgUnitAppendVersionFieldDecisions(appendFieldDecisions)
		appendFieldDecisions[0].AllowedValueCodes[0] = "X"
		if clonedFieldDecisions[0].AllowedValueCodes[0] != "A" {
			t.Fatalf("cloned field decisions=%+v", clonedFieldDecisions)
		}
			appendProjection := OrgUnitAppendVersionPrecheckProjectionV1{
				Readiness:                         " ready ",
				MissingFields:                     []string{"effective_date"},
				FieldDecisions:                    appendFieldDecisions,
				CandidateConfirmationRequirements: []string{"new_parent_org_code"},
				PendingDraftSummary:               " 目标组织：FLOWER-C ",
				PolicyExplain:                     " 已就绪 ",
				RejectionReasons:                  []string{"FORBIDDEN"},
				ProjectionDigest:                  " digest ",
		}
		clonedProjection := CloneOrgUnitAppendVersionProjectionV1(appendProjection)
		appendProjection.MissingFields[0] = "changed"
		appendProjection.FieldDecisions[0].AllowedValueCodes[1] = "Y"
		appendProjection.CandidateConfirmationRequirements[0] = "changed"
		appendProjection.RejectionReasons[0] = "changed"
			if clonedProjection.Readiness != "ready" ||
				clonedProjection.MissingFields[0] != "effective_date" ||
				clonedProjection.FieldDecisions[0].AllowedValueCodes[1] != "B" ||
				clonedProjection.CandidateConfirmationRequirements[0] != "new_parent_org_code" ||
				clonedProjection.RejectionReasons[0] != "FORBIDDEN" ||
				clonedProjection.PendingDraftSummary != "目标组织：FLOWER-C" ||
				clonedProjection.PolicyExplain != "已就绪" ||
				clonedProjection.ProjectionDigest != "digest" {
				t.Fatalf("cloned projection=%+v", clonedProjection)
		}
		if _, found, errMsg := resolveOrgUnitAppendVersionFieldDecision(context.Background(), nil, OrgUnitAppendVersionPrecheckInputV1{}, "", "name"); found || errMsg != "" {
			t.Fatalf("nil reader err=%q", errMsg)
		}
	})
}

func TestResolveOrgUnitAppendVersionPolicyContextBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("empty org code only builds digest", func(t *testing.T) {
		policyCtx, contextErr := resolveOrgUnitAppendVersionPolicyContextV1(ctx, nil, OrgUnitAppendVersionPrecheckInputV1{
			TenantID:      "tenant-1",
			Intent:        "add_version",
			EffectiveDate: "2026-01-01",
		})
		if contextErr != nil || policyCtx.PolicyContextDigest == "" {
			t.Fatalf("ctx=%+v err=%v", policyCtx, contextErr)
		}
	})

	t.Run("reader missing", func(t *testing.T) {
		_, contextErr := resolveOrgUnitAppendVersionPolicyContextV1(ctx, nil, OrgUnitAppendVersionPrecheckInputV1{TenantID: "tenant-1", OrgCode: "FLOWER-C"})
		if contextErr == nil || contextErr.Code != orgUnitAppendVersionContextCodeOrgInvalid {
			t.Fatalf("err=%+v", contextErr)
		}
	})

	t.Run("invalid org code", func(t *testing.T) {
		_, contextErr := resolveOrgUnitAppendVersionPolicyContextV1(ctx, orgUnitAppendVersionPrecheckReaderStub{}, OrgUnitAppendVersionPrecheckInputV1{TenantID: "tenant-1", OrgCode: "bad\ncode"})
		if contextErr == nil || contextErr.Code != orgUnitAppendVersionContextCodeOrgInvalid || contextErr.Cause == nil {
			t.Fatalf("err=%+v", contextErr)
		}
	})

	t.Run("resolve org node key error", func(t *testing.T) {
		_, contextErr := resolveOrgUnitAppendVersionPolicyContextV1(ctx, orgUnitAppendVersionPrecheckReaderStub{
			resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) { return "", errors.New("missing") },
		}, OrgUnitAppendVersionPrecheckInputV1{TenantID: "tenant-1", OrgCode: "FLOWER-C"})
		if contextErr == nil || contextErr.Code != orgUnitAppendVersionContextCodeOrgInvalid {
			t.Fatalf("err=%+v", contextErr)
		}
	})

	t.Run("success", func(t *testing.T) {
		policyCtx, contextErr := resolveOrgUnitAppendVersionPolicyContextV1(ctx, orgUnitAppendVersionPrecheckReaderStub{
			resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) { return "10000001", nil },
		}, OrgUnitAppendVersionPrecheckInputV1{
			TenantID:      "tenant-1",
			Intent:        "add_version",
			EffectiveDate: "2026-01-01",
			OrgCode:       "flower-c",
		})
		if contextErr != nil {
			t.Fatalf("unexpected err=%v", contextErr)
		}
		if policyCtx.OrgCode != "FLOWER-C" || policyCtx.OrgNodeKey != "10000001" || policyCtx.PolicyContextDigest == "" {
			t.Fatalf("ctx=%+v", policyCtx)
		}
	})
}

func TestBuildOrgUnitAppendVersionPrecheckProjectionBranches(t *testing.T) {
	ctx := context.Background()
	baseInput := OrgUnitAppendVersionPrecheckInputV1{
		Intent:        string(OrgUnitWriteIntentAddVersion),
		TenantID:      "tenant-1",
		EffectiveDate: "2026-01-01",
		OrgCode:       "FLOWER-C",
		CanAdmin:      true,
		NewName:       "运营一部",
	}

	readyReader := orgUnitAppendVersionPrecheckReaderStub{
		resolveOrgNodeKeyFn: func(context.Context, string, string) (string, error) { return "10000003", nil },
		treeInitFn:          func(context.Context, string) (bool, error) { return true, nil },
		listConfigsFn:       func(context.Context, string, string) ([]types.TenantFieldConfig, error) { return nil, nil },
		resolveFactsFn: func(context.Context, string, string, string) (OrgUnitAppendVersionFactsV1, error) {
			return OrgUnitAppendVersionFactsV1{TargetExistsAsOf: true}, nil
		},
	}

	t.Run("enabled field config error bubbles", func(t *testing.T) {
		if _, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, orgUnitAppendVersionPrecheckReaderStub{
			listConfigsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		}, baseInput); err == nil {
			t.Fatal("expected list config error")
		}
	})

	t.Run("tree initialization error bubbles", func(t *testing.T) {
		reader := readyReader
		reader.treeInitFn = func(context.Context, string) (bool, error) { return false, errors.New("tree boom") }
		if _, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, reader, baseInput); err == nil {
			t.Fatal("expected tree error")
		}
	})

	t.Run("append facts error bubbles", func(t *testing.T) {
		reader := readyReader
		reader.resolveFactsFn = func(context.Context, string, string, string) (OrgUnitAppendVersionFactsV1, error) {
			return OrgUnitAppendVersionFactsV1{}, errors.New("facts boom")
		}
		if _, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, reader, baseInput); err == nil {
			t.Fatal("expected append facts error")
		}
	})

	t.Run("invalid intent bubbles from mutation capabilities", func(t *testing.T) {
		input := baseInput
		input.Intent = "unknown"
		if _, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, readyReader, input); err == nil {
			t.Fatal("expected invalid intent error")
		}
	})

	t.Run("org code empty keeps digest path", func(t *testing.T) {
		input := OrgUnitAppendVersionPrecheckInputV1{Intent: string(OrgUnitWriteIntentAddVersion), TenantID: "tenant-1"}
		result, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, readyReader, input)
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if result.PolicyContext.PolicyContextDigest == "" {
			t.Fatalf("policy context digest missing: %+v", result.PolicyContext)
		}
		if !slices.Equal(result.Projection.MissingFields, []string{"org_code", "effective_date", "change_fields"}) {
			t.Fatalf("missing=%v", result.Projection.MissingFields)
		}
	})

	t.Run("context and field rejections are accumulated", func(t *testing.T) {
		result, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, nil, baseInput)
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if result.ContextError == nil || result.ContextError.Code != orgUnitAppendVersionContextCodeOrgInvalid {
			t.Fatalf("context err=%+v", result.ContextError)
		}
		if !slices.Contains(result.Projection.RejectionReasons, orgUnitAppendVersionContextCodeOrgInvalid) {
			t.Fatalf("rejections=%v", result.Projection.RejectionReasons)
		}
	})

	t.Run("deny reasons propagate", func(t *testing.T) {
		reader := readyReader
		reader.treeInitFn = func(context.Context, string) (bool, error) { return false, nil }
		reader.resolveFactsFn = func(context.Context, string, string, string) (OrgUnitAppendVersionFactsV1, error) {
			return OrgUnitAppendVersionFactsV1{TargetExistsAsOf: false}, nil
		}
		input := baseInput
		input.CanAdmin = false
		result, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, reader, input)
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if got := result.Projection.Readiness; got != orgUnitAppendVersionReadinessRejected {
			t.Fatalf("readiness=%q", got)
		}
		if !slices.Contains(result.Projection.RejectionReasons, "FORBIDDEN") ||
			!slices.Contains(result.Projection.RejectionReasons, "ORG_TREE_NOT_INITIALIZED") ||
			!slices.Contains(result.Projection.RejectionReasons, "ORG_NOT_FOUND_AS_OF") {
			t.Fatalf("rejections=%v", result.Projection.RejectionReasons)
		}
	})

	t.Run("success builds ready projection", func(t *testing.T) {
		result, err := BuildOrgUnitAppendVersionPrecheckProjectionV1(ctx, readyReader, baseInput)
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if result.ContextError != nil {
			t.Fatalf("unexpected context err=%v", result.ContextError)
		}
		if result.PolicyContext.PolicyContextDigest == "" || result.Projection.ProjectionDigest == "" {
			t.Fatalf("digests missing result=%+v", result)
		}
		if result.Projection.Readiness != orgUnitAppendVersionReadinessReady || result.Projection.PolicyExplain != "计划已生成，等待确认后可提交" {
			t.Fatalf("projection=%+v", result.Projection)
		}
	})
}
