package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type assistantActionHandlerSpec struct {
	DryRunKey        string
	CommitAdapterKey string
}

type assistantActionSecuritySpec struct {
	AuthObject     string
	AuthAction     string
	RiskTier       string
	RequiredChecks []string
}

type assistantActionSpec struct {
	ID            string
	Version       string
	CapabilityKey string
	PlanTitle     string
	PlanSummary   string
	Security      assistantActionSecuritySpec
	Handler       assistantActionHandlerSpec
}

type assistantActionRegistry interface {
	Lookup(actionID string) (assistantActionSpec, bool)
}

type assistantActionRegistryMap struct {
	specs map[string]assistantActionSpec
}

func (r assistantActionRegistryMap) Lookup(actionID string) (assistantActionSpec, bool) {
	spec, ok := r.specs[strings.TrimSpace(actionID)]
	return spec, ok
}

var assistantDefaultActionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
	assistantIntentPlanOnly: {
		ID:            assistantIntentPlanOnly,
		Version:       "v1",
		CapabilityKey: "org.assistant_conversation.manage",
		PlanTitle:     "只读规划",
		PlanSummary:   "生成只读计划，不执行提交",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "low",
			RequiredChecks: []string{"strict_decode", "boundary_lint"},
		},
	},
	assistantIntentCreateOrgUnit: {
		ID:            assistantIntentCreateOrgUnit,
		Version:       "v1",
		CapabilityKey: "org.orgunit_create.field_policy",
		PlanTitle:     "创建组织计划",
		PlanSummary:   "在指定父组织下创建部门，提交前需要确认候选主键",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "candidate_confirmation", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{
			DryRunKey:        "orgunit_create_dry_run_v1",
			CommitAdapterKey: "orgunit_create_v1",
		},
	},
	assistantIntentAddOrgUnitVersion: {
		ID:            assistantIntentAddOrgUnitVersion,
		Version:       "v1",
		CapabilityKey: "org.orgunit_add_version.field_policy",
		PlanTitle:     "新增版本计划",
		PlanSummary:   "为指定组织新增未来版本，提交前进行字段校验",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "candidate_confirmation", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_add_version_dry_run_v1", CommitAdapterKey: "orgunit_add_version_v1"},
	},
	assistantIntentInsertOrgUnitVersion: {
		ID:            assistantIntentInsertOrgUnitVersion,
		Version:       "v1",
		CapabilityKey: "org.orgunit_insert_version.field_policy",
		PlanTitle:     "插入版本计划",
		PlanSummary:   "为指定组织插入历史版本，提交前进行字段校验",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "candidate_confirmation", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_insert_version_dry_run_v1", CommitAdapterKey: "orgunit_insert_version_v1"},
	},
	assistantIntentCorrectOrgUnit: {
		ID:            assistantIntentCorrectOrgUnit,
		Version:       "v1",
		CapabilityKey: "org.orgunit_correct.field_policy",
		PlanTitle:     "更正组织计划",
		PlanSummary:   "更正指定组织版本的字段内容，提交前进行字段校验",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "candidate_confirmation", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_correct_dry_run_v1", CommitAdapterKey: "orgunit_correct_v1"},
	},
	assistantIntentDisableOrgUnit: {
		ID:            assistantIntentDisableOrgUnit,
		Version:       "v1",
		CapabilityKey: "org.orgunit_write.field_policy",
		PlanTitle:     "停用组织计划",
		PlanSummary:   "停用指定组织，提交前进行字段校验",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_disable_dry_run_v1", CommitAdapterKey: "orgunit_disable_v1"},
	},
	assistantIntentEnableOrgUnit: {
		ID:            assistantIntentEnableOrgUnit,
		Version:       "v1",
		CapabilityKey: "org.orgunit_write.field_policy",
		PlanTitle:     "启用组织计划",
		PlanSummary:   "启用指定组织，提交前进行字段校验",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_enable_dry_run_v1", CommitAdapterKey: "orgunit_enable_v1"},
	},
	assistantIntentMoveOrgUnit: {
		ID:            assistantIntentMoveOrgUnit,
		Version:       "v1",
		CapabilityKey: "org.orgunit_write.field_policy",
		PlanTitle:     "移动组织计划",
		PlanSummary:   "将组织移动到新的上级组织，提交前需要确认候选主键",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "candidate_confirmation", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_move_dry_run_v1", CommitAdapterKey: "orgunit_move_v1"},
	},
	assistantIntentRenameOrgUnit: {
		ID:            assistantIntentRenameOrgUnit,
		Version:       "v1",
		CapabilityKey: "org.orgunit_write.field_policy",
		PlanTitle:     "重命名组织计划",
		PlanSummary:   "重命名指定组织，提交前进行字段校验",
		Security: assistantActionSecuritySpec{
			AuthObject:     authz.ObjectOrgSetIDCapability,
			AuthAction:     authz.ActionAdmin,
			RiskTier:       "high",
			RequiredChecks: []string{"strict_decode", "boundary_lint", "dry_run"},
		},
		Handler: assistantActionHandlerSpec{DryRunKey: "orgunit_rename_dry_run_v1", CommitAdapterKey: "orgunit_rename_v1"},
	},
}}

func assistantLookupDefaultActionSpec(actionID string) (assistantActionSpec, bool) {
	return assistantDefaultActionRegistry.Lookup(actionID)
}

func (s *assistantConversationService) lookupActionSpec(actionID string) (assistantActionSpec, bool) {
	if s != nil && s.actionRegistry != nil {
		return s.actionRegistry.Lookup(actionID)
	}
	return assistantLookupDefaultActionSpec(actionID)
}

type assistantCommitRequest struct {
	TenantID          string
	Principal         Principal
	Turn              *assistantTurn
	ResolvedCandidate assistantCandidate
}

type assistantCommitAdapter interface {
	Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error)
}

type assistantCommitAdapterRegistry interface {
	Lookup(key string) (assistantCommitAdapter, bool)
}

type assistantCommitAdapterRegistryMap struct {
	adapters map[string]assistantCommitAdapter
}

func (r assistantCommitAdapterRegistryMap) Lookup(key string) (assistantCommitAdapter, bool) {
	adapter, ok := r.adapters[strings.TrimSpace(key)]
	return adapter, ok
}

func newAssistantDefaultCommitAdapterRegistry(writeSvc orgunitservices.OrgUnitWriteService) assistantCommitAdapterRegistry {
	return assistantCommitAdapterRegistryMap{adapters: map[string]assistantCommitAdapter{
		"orgunit_create_v1":         assistantCreateOrgUnitCommitAdapter{writeSvc: writeSvc},
		"orgunit_add_version_v1":    assistantAddOrgUnitVersionCommitAdapter{writeSvc: writeSvc},
		"orgunit_insert_version_v1": assistantInsertOrgUnitVersionCommitAdapter{writeSvc: writeSvc},
		"orgunit_correct_v1":        assistantCorrectOrgUnitCommitAdapter{writeSvc: writeSvc},
		"orgunit_disable_v1":        assistantDisableOrgUnitCommitAdapter{writeSvc: writeSvc},
		"orgunit_enable_v1":         assistantEnableOrgUnitCommitAdapter{writeSvc: writeSvc},
		"orgunit_move_v1":           assistantMoveOrgUnitCommitAdapter{writeSvc: writeSvc},
		"orgunit_rename_v1":         assistantRenameOrgUnitCommitAdapter{writeSvc: writeSvc},
	}}
}

func (s *assistantConversationService) lookupCommitAdapter(key string) (assistantCommitAdapter, bool) {
	if s != nil && s.commitAdapterRegistry != nil {
		return s.commitAdapterRegistry.Lookup(key)
	}
	if s == nil {
		return nil, false
	}
	return newAssistantDefaultCommitAdapterRegistry(s.writeSvc).Lookup(key)
}

type assistantCreateOrgUnitCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

func (a assistantCreateOrgUnitCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if a.writeSvc == nil {
		return nil, errAssistantServiceMissing
	}
	if req.Turn == nil {
		return nil, errAssistantConversationCorrupted
	}
	name := strings.TrimSpace(req.Turn.Intent.EntityName)
	if name == "" {
		name = "新建组织"
	}
	parentOrgCode := strings.TrimSpace(req.ResolvedCandidate.CandidateCode)
	result, err := a.writeSvc.Write(ctx, req.TenantID, orgunitservices.WriteOrgUnitRequest{
		Intent:        string(orgunitservices.OrgUnitWriteIntentCreateOrg),
		EffectiveDate: req.Turn.Intent.EffectiveDate,
		PolicyVersion: req.Turn.PolicyVersion,
		RequestID:     req.Turn.RequestID,
		Patch: orgunitservices.OrgUnitWritePatch{
			Name:          ptrString(name),
			ParentOrgCode: ptrString(parentOrgCode),
		},
		InitiatorUUID: req.Principal.ID,
	})
	if err != nil {
		return nil, err
	}
	return &assistantCommitResult{
		OrgCode:       result.OrgCode,
		ParentOrgCode: parentOrgCode,
		EffectiveDate: result.EffectiveDate,
		EventType:     result.EventType,
		EventUUID:     result.EventUUID,
	}, nil
}

type assistantAddOrgUnitVersionCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

type assistantInsertOrgUnitVersionCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

type assistantCorrectOrgUnitCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

type assistantDisableOrgUnitCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

type assistantEnableOrgUnitCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

type assistantMoveOrgUnitCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

type assistantRenameOrgUnitCommitAdapter struct {
	writeSvc orgunitservices.OrgUnitWriteService
}

func assistantCommitAdapterReady(writeSvc orgunitservices.OrgUnitWriteService, req assistantCommitRequest) error {
	if writeSvc == nil {
		return errAssistantServiceMissing
	}
	if req.Turn == nil {
		return errAssistantConversationCorrupted
	}
	return nil
}

func assistantResolvedCandidateOrgCode(candidate assistantCandidate) string {
	if code := strings.TrimSpace(candidate.CandidateCode); code != "" {
		return code
	}
	return strings.TrimSpace(candidate.CandidateID)
}

func assistantWritePatchFromIntent(intent assistantIntentSpec, resolvedParentOrgCode string) orgunitservices.OrgUnitWritePatch {
	patch := orgunitservices.OrgUnitWritePatch{}
	if newName := strings.TrimSpace(intent.NewName); newName != "" {
		patch.Name = ptrString(newName)
	}
	if parentOrgCode := strings.TrimSpace(resolvedParentOrgCode); parentOrgCode != "" {
		patch.ParentOrgCode = ptrString(parentOrgCode)
	}
	return patch
}

func (a assistantAddOrgUnitVersionCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	parentOrgCode := assistantResolvedCandidateOrgCode(req.ResolvedCandidate)
	result, err := a.writeSvc.Write(ctx, req.TenantID, orgunitservices.WriteOrgUnitRequest{
		Intent:        string(orgunitservices.OrgUnitWriteIntentAddVersion),
		OrgCode:       strings.TrimSpace(req.Turn.Intent.OrgCode),
		EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate),
		PolicyVersion: req.Turn.PolicyVersion,
		RequestID:     req.Turn.RequestID,
		Patch:         assistantWritePatchFromIntent(req.Turn.Intent, parentOrgCode),
		InitiatorUUID: req.Principal.ID,
	})
	if err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: result.OrgCode, ParentOrgCode: parentOrgCode, EffectiveDate: result.EffectiveDate, EventType: result.EventType, EventUUID: result.EventUUID}, nil
}

func (a assistantInsertOrgUnitVersionCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	parentOrgCode := assistantResolvedCandidateOrgCode(req.ResolvedCandidate)
	result, err := a.writeSvc.Write(ctx, req.TenantID, orgunitservices.WriteOrgUnitRequest{
		Intent:        string(orgunitservices.OrgUnitWriteIntentInsertVersion),
		OrgCode:       strings.TrimSpace(req.Turn.Intent.OrgCode),
		EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate),
		PolicyVersion: req.Turn.PolicyVersion,
		RequestID:     req.Turn.RequestID,
		Patch:         assistantWritePatchFromIntent(req.Turn.Intent, parentOrgCode),
		InitiatorUUID: req.Principal.ID,
	})
	if err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: result.OrgCode, ParentOrgCode: parentOrgCode, EffectiveDate: result.EffectiveDate, EventType: result.EventType, EventUUID: result.EventUUID}, nil
}

func (a assistantCorrectOrgUnitCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	parentOrgCode := assistantResolvedCandidateOrgCode(req.ResolvedCandidate)
	patch := orgunitservices.OrgUnitCorrectionPatch{}
	if newName := strings.TrimSpace(req.Turn.Intent.NewName); newName != "" {
		patch.Name = ptrString(newName)
	}
	if parentOrgCode != "" {
		patch.ParentOrgCode = ptrString(parentOrgCode)
	}
	result, err := a.writeSvc.Correct(ctx, req.TenantID, orgunitservices.CorrectOrgUnitRequest{
		OrgCode:             strings.TrimSpace(req.Turn.Intent.OrgCode),
		TargetEffectiveDate: strings.TrimSpace(req.Turn.Intent.TargetEffectiveDate),
		Patch:               patch,
		RequestID:           req.Turn.RequestID,
		InitiatorUUID:       req.Principal.ID,
	})
	if err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: result.OrgCode, ParentOrgCode: parentOrgCode, EffectiveDate: result.EffectiveDate, EventType: "UPDATE"}, nil
}

func (a assistantDisableOrgUnitCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	if err := a.writeSvc.Disable(ctx, req.TenantID, orgunitservices.DisableOrgUnitRequest{EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), InitiatorUUID: req.Principal.ID}); err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), EventType: "DISABLE"}, nil
}

func (a assistantEnableOrgUnitCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	if err := a.writeSvc.Enable(ctx, req.TenantID, orgunitservices.EnableOrgUnitRequest{EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), InitiatorUUID: req.Principal.ID}); err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), EventType: "ENABLE"}, nil
}

func (a assistantMoveOrgUnitCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	parentOrgCode := assistantResolvedCandidateOrgCode(req.ResolvedCandidate)
	if err := a.writeSvc.Move(ctx, req.TenantID, orgunitservices.MoveOrgUnitRequest{EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), NewParentOrgCode: parentOrgCode, InitiatorUUID: req.Principal.ID}); err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), ParentOrgCode: parentOrgCode, EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), EventType: "MOVE"}, nil
}

func (a assistantRenameOrgUnitCommitAdapter) Commit(ctx context.Context, req assistantCommitRequest) (*assistantCommitResult, error) {
	if err := assistantCommitAdapterReady(a.writeSvc, req); err != nil {
		return nil, err
	}
	if err := a.writeSvc.Rename(ctx, req.TenantID, orgunitservices.RenameOrgUnitRequest{EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), NewName: strings.TrimSpace(req.Turn.Intent.NewName), InitiatorUUID: req.Principal.ID}); err != nil {
		return nil, err
	}
	return &assistantCommitResult{OrgCode: strings.TrimSpace(req.Turn.Intent.OrgCode), EffectiveDate: strings.TrimSpace(req.Turn.Intent.EffectiveDate), EventType: "RENAME"}, nil
}

type assistantVersionTuple struct {
	ParentCandidateID string `json:"parent_candidate_id"`
	ParentOrgCode     string `json:"parent_org_code,omitempty"`
	ParentEventUUID   string `json:"parent_event_uuid,omitempty"`
	ParentUpdatedAt   string `json:"parent_updated_at,omitempty"`
	EffectiveDate     string `json:"effective_date,omitempty"`
}

func assistantVersionTupleTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func (s *assistantConversationService) refreshTurnVersionTuple(ctx context.Context, tenantID string, turn *assistantTurn) error {
	if turn == nil {
		return nil
	}
	if strings.TrimSpace(turn.ResolvedCandidateID) == "" {
		turn.Plan.VersionTuple = nil
		hash := assistantPlanHashFn(turn.Intent, turn.Plan, turn.DryRun)
		if strings.TrimSpace(hash) == "" {
			return errAssistantPlanDeterminismViolation
		}
		turn.DryRun.PlanHash = hash
		return nil
	}
	spec, ok := s.lookupActionSpec(turn.Intent.Action)
	if !ok {
		return errAssistantUnsupportedIntent
	}
	turn.Plan.ActionID = spec.ID
	turn.Plan.ActionVersion = spec.Version
	turn.Plan.CommitAdapterKey = spec.Handler.CommitAdapterKey
	raw, err := s.captureSelectedCandidateVersionTuple(ctx, tenantID, turn.Intent, turn.Candidates, turn.ResolvedCandidateID)
	if err != nil {
		switch {
		case errors.Is(err, errAssistantCandidateNotFound), errors.Is(err, errAssistantServiceMissing), errors.Is(err, errOrgUnitNotFound):
			turn.Plan.VersionTuple = nil
		default:
			return err
		}
	} else {
		turn.Plan.VersionTuple = raw
	}
	hash := assistantPlanHashFn(turn.Intent, turn.Plan, turn.DryRun)
	if strings.TrimSpace(hash) == "" {
		return errAssistantPlanDeterminismViolation
	}
	turn.DryRun.PlanHash = hash
	return nil
}

func (s *assistantConversationService) captureSelectedCandidateVersionTuple(ctx context.Context, tenantID string, intent assistantIntentSpec, candidates []assistantCandidate, selectedCandidateID string) (json.RawMessage, error) {
	if strings.TrimSpace(selectedCandidateID) == "" || strings.TrimSpace(intent.Action) != assistantIntentCreateOrgUnit {
		return nil, nil
	}
	candidate, details, err := s.lookupCandidateDetails(ctx, tenantID, intent.EffectiveDate, candidates, selectedCandidateID)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(assistantVersionTuple{
		ParentCandidateID: strings.TrimSpace(candidate.CandidateID),
		ParentOrgCode:     strings.TrimSpace(candidate.CandidateCode),
		ParentEventUUID:   strings.TrimSpace(details.EventUUID),
		ParentUpdatedAt:   assistantVersionTupleTimestamp(details.UpdatedAt),
		EffectiveDate:     strings.TrimSpace(intent.EffectiveDate),
	})
	return raw, nil
}

func (s *assistantConversationService) validateTurnVersionTuple(ctx context.Context, tenantID string, turn *assistantTurn) error {
	if turn == nil || strings.TrimSpace(turn.Intent.Action) != assistantIntentCreateOrgUnit {
		return nil
	}
	payload := strings.TrimSpace(string(turn.Plan.VersionTuple))
	if payload == "" || payload == "null" {
		return errAssistantVersionTupleStale
	}
	var tuple assistantVersionTuple
	if err := json.Unmarshal(turn.Plan.VersionTuple, &tuple); err != nil {
		return errAssistantVersionTupleStale
	}
	candidateID := strings.TrimSpace(turn.ResolvedCandidateID)
	if candidateID == "" {
		candidateID = strings.TrimSpace(tuple.ParentCandidateID)
	}
	candidate, details, err := s.lookupCandidateDetails(ctx, tenantID, turn.Intent.EffectiveDate, turn.Candidates, candidateID)
	if err != nil {
		return errAssistantVersionTupleStale
	}
	if code := strings.TrimSpace(tuple.ParentOrgCode); code != "" && code != strings.TrimSpace(candidate.CandidateCode) {
		return errAssistantVersionTupleStale
	}
	if updatedAt := strings.TrimSpace(tuple.ParentUpdatedAt); updatedAt != "" && updatedAt != assistantVersionTupleTimestamp(details.UpdatedAt) {
		return errAssistantVersionTupleStale
	}
	if eventUUID := strings.TrimSpace(tuple.ParentEventUUID); eventUUID != "" && strings.TrimSpace(details.EventUUID) != "" && eventUUID != strings.TrimSpace(details.EventUUID) {
		return errAssistantVersionTupleStale
	}
	return nil
}

func (s *assistantConversationService) lookupCandidateDetails(ctx context.Context, tenantID string, asOf string, candidates []assistantCandidate, selectedCandidateID string) (assistantCandidate, OrgUnitNodeDetails, error) {
	candidate, ok := assistantFindCandidate(candidates, selectedCandidateID)
	if !ok {
		return assistantCandidate{}, OrgUnitNodeDetails{}, errAssistantCandidateNotFound
	}
	if s == nil || s.orgStore == nil {
		return assistantCandidate{}, OrgUnitNodeDetails{}, errAssistantServiceMissing
	}
	orgNodeKey := strings.TrimSpace(candidate.OrgNodeKey)
	if orgNodeKey == "" && candidate.OrgID > 0 {
		encodedOrgNodeKey, err := encodeOrgNodeKeyFromID(candidate.OrgID)
		if err != nil {
			return assistantCandidate{}, OrgUnitNodeDetails{}, err
		}
		orgNodeKey = encodedOrgNodeKey
	}
	if orgNodeKey == "" {
		resolvedOrgNodeKey, err := s.resolveAssistantCandidateOrgNodeKey(ctx, tenantID, candidate)
		if err != nil {
			return assistantCandidate{}, OrgUnitNodeDetails{}, err
		}
		orgNodeKey = resolvedOrgNodeKey
		candidate.OrgNodeKey = orgNodeKey
	}
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return assistantCandidate{}, OrgUnitNodeDetails{}, err
	}
	details, err := s.orgStore.GetNodeDetails(ctx, tenantID, orgID, asOf)
	if err != nil {
		return assistantCandidate{}, OrgUnitNodeDetails{}, err
	}
	if candidate.OrgID == 0 {
		candidate.OrgID = details.OrgID
	}
	return candidate, details, nil
}

func (s *assistantConversationService) resolveAssistantCandidateOrgNodeKey(ctx context.Context, tenantID string, candidate assistantCandidate) (string, error) {
	if s == nil || s.orgStore == nil {
		return "", errAssistantServiceMissing
	}
	if orgNodeKey := strings.TrimSpace(candidate.OrgNodeKey); orgNodeKey != "" {
		return orgNodeKey, nil
	}
	if candidate.OrgID > 0 {
		orgNodeKey, err := encodeOrgNodeKeyFromID(candidate.OrgID)
		if err == nil {
			return orgNodeKey, nil
		}
	}
	if code := strings.TrimSpace(candidate.CandidateCode); code != "" {
		orgNodeKey, err := s.orgStore.ResolveOrgNodeKeyByCode(ctx, tenantID, code)
		if err == nil {
			return orgNodeKey, nil
		}
	}
	query := strings.TrimSpace(candidate.CandidateCode)
	if query == "" {
		query = strings.TrimSpace(candidate.CandidateID)
	}
	if query == "" {
		query = strings.TrimSpace(candidate.Name)
	}
	if query == "" {
		return "", errAssistantCandidateNotFound
	}
	rows, err := s.orgStore.SearchNodeCandidates(ctx, tenantID, query, candidate.AsOf, 10)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		orgNodeKey := strings.TrimSpace(row.OrgNodeKey)
		if orgNodeKey == "" && row.OrgID > 0 {
			encoded, encodeErr := encodeOrgNodeKeyFromID(row.OrgID)
			if encodeErr == nil {
				orgNodeKey = encoded
			}
		}
		if orgNodeKey == "" {
			continue
		}
		if code := strings.TrimSpace(candidate.CandidateCode); code != "" && strings.TrimSpace(row.OrgCode) == code {
			return orgNodeKey, nil
		}
		if name := strings.TrimSpace(candidate.Name); name != "" && strings.TrimSpace(row.Name) == name {
			return orgNodeKey, nil
		}
	}
	if len(rows) == 1 {
		orgNodeKey := strings.TrimSpace(rows[0].OrgNodeKey)
		if orgNodeKey == "" && rows[0].OrgID > 0 {
			encoded, encodeErr := encodeOrgNodeKeyFromID(rows[0].OrgID)
			if encodeErr == nil {
				orgNodeKey = encoded
			}
		}
		if orgNodeKey != "" {
			return orgNodeKey, nil
		}
	}
	return "", errAssistantCandidateNotFound
}

func (s *assistantConversationService) resolveAssistantCandidateOrgID(ctx context.Context, tenantID string, candidate assistantCandidate) (int, error) {
	orgNodeKey, err := s.resolveAssistantCandidateOrgNodeKey(ctx, tenantID, candidate)
	if err != nil {
		return 0, err
	}
	return decodeOrgNodeKeyToID(orgNodeKey)
}
