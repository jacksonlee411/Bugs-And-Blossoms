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
		"orgunit_create_v1": assistantCreateOrgUnitCommitAdapter{writeSvc: writeSvc},
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
	orgID := candidate.OrgID
	if orgID <= 0 {
		resolvedOrgID, err := s.resolveAssistantCandidateOrgID(ctx, tenantID, candidate)
		if err != nil {
			return assistantCandidate{}, OrgUnitNodeDetails{}, err
		}
		orgID = resolvedOrgID
		candidate.OrgID = orgID
	}
	details, err := s.orgStore.GetNodeDetails(ctx, tenantID, orgID, asOf)
	if err != nil {
		return assistantCandidate{}, OrgUnitNodeDetails{}, err
	}
	return candidate, details, nil
}

func (s *assistantConversationService) resolveAssistantCandidateOrgID(ctx context.Context, tenantID string, candidate assistantCandidate) (int, error) {
	if s == nil || s.orgStore == nil {
		return 0, errAssistantServiceMissing
	}
	if candidate.OrgID > 0 {
		return candidate.OrgID, nil
	}
	if code := strings.TrimSpace(candidate.CandidateCode); code != "" {
		orgID, err := s.orgStore.ResolveOrgID(ctx, tenantID, code)
		if err == nil {
			return orgID, nil
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
		return 0, errAssistantCandidateNotFound
	}
	rows, err := s.orgStore.SearchNodeCandidates(ctx, tenantID, query, candidate.AsOf, 10)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if row.OrgID <= 0 {
			continue
		}
		if code := strings.TrimSpace(candidate.CandidateCode); code != "" && strings.TrimSpace(row.OrgCode) == code {
			return row.OrgID, nil
		}
		if name := strings.TrimSpace(candidate.Name); name != "" && strings.TrimSpace(row.Name) == name {
			return row.OrgID, nil
		}
	}
	if len(rows) == 1 && rows[0].OrgID > 0 {
		return rows[0].OrgID, nil
	}
	return 0, errAssistantCandidateNotFound
}
