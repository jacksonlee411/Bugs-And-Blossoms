package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	cubeboxmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/persistence"
	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

type cubeboxLegacyFacade struct {
	assistant *assistantConversationService
}

func (l cubeboxLegacyFacade) ListConversations(ctx context.Context, tenantID string, actorID string, pageSize int, cursor string) ([]cubeboxdomain.ConversationListItem, string, error) {
	if l.assistant == nil {
		return nil, "", errAssistantGateUnavailable
	}
	items, next, err := l.assistant.listConversations(ctx, tenantID, actorID, pageSize, cursor)
	if err != nil {
		return nil, "", err
	}
	out := make([]cubeboxdomain.ConversationListItem, 0, len(items))
	for _, item := range items {
		mapped := cubeboxdomain.ConversationListItem{
			ConversationID: strings.TrimSpace(item.ConversationID),
			State:          strings.TrimSpace(item.State),
			UpdatedAt:      item.UpdatedAt.UTC(),
		}
		if item.LastTurn != nil {
			mapped.LastTurn = &cubeboxdomain.ConversationLastTurn{
				TurnID:    strings.TrimSpace(item.LastTurn.TurnID),
				UserInput: strings.TrimSpace(item.LastTurn.UserInput),
				State:     strings.TrimSpace(item.LastTurn.State),
				RiskTier:  strings.TrimSpace(item.LastTurn.RiskTier),
			}
		}
		out = append(out, mapped)
	}
	return out, next, nil
}

func (l cubeboxLegacyFacade) GetConversation(ctx context.Context, tenantID string, actorID string, conversationID string) (*cubeboxdomain.Conversation, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	conversation, err := l.assistant.getConversation(tenantID, actorID, conversationID)
	if err != nil {
		switch {
		case errors.Is(err, errAssistantConversationNotFound):
			return nil, cubeboxConversationNotFound()
		case errors.Is(err, errAssistantTenantMismatch):
			return nil, cubeboxTenantMismatch()
		case errors.Is(err, errAssistantConversationForbidden):
			return nil, cubeboxConversationForbidden()
		default:
			return nil, err
		}
	}
	return mapAssistantConversation(conversation), nil
}

func (l cubeboxLegacyFacade) CreateConversation(ctx context.Context, tenantID string, principal cubeboxmodule.Principal) (*cubeboxdomain.Conversation, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	conversation, err := l.assistant.createConversationWithContext(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug})
	if err != nil {
		return nil, err
	}
	return mapAssistantConversation(conversation), nil
}

func (l cubeboxLegacyFacade) CreateTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, userInput string) (*cubeboxdomain.Conversation, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	conversation, err := l.assistant.createTurn(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, conversationID, userInput)
	if err != nil {
		return nil, err
	}
	return mapAssistantConversation(conversation), nil
}

func (l cubeboxLegacyFacade) ConfirmTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, candidateID string) (*cubeboxdomain.Conversation, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	conversation, err := l.assistant.confirmTurn(tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, conversationID, turnID, candidateID)
	if err != nil {
		return nil, err
	}
	return mapAssistantConversation(conversation), nil
}

func (l cubeboxLegacyFacade) CommitTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string) (*cubeboxdomain.TaskReceipt, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	receipt, err := l.assistant.submitCommitTask(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, conversationID, turnID)
	if err != nil {
		return nil, err
	}
	return mapAssistantTaskReceipt(receipt), nil
}

func (l cubeboxLegacyFacade) SubmitTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, req cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	receipt, err := l.assistant.submitTask(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, assistantTaskSubmitRequest{
		ConversationID: req.ConversationID,
		TurnID:         req.TurnID,
		TaskType:       req.TaskType,
		RequestID:      req.RequestID,
		TraceID:        req.TraceID,
		ContractSnapshot: assistantTaskContractSnapshot{
			IntentSchemaVersion:      req.ContractSnapshot.IntentSchemaVersion,
			CompilerContractVersion:  req.ContractSnapshot.CompilerContractVersion,
			CapabilityMapVersion:     req.ContractSnapshot.CapabilityMapVersion,
			SkillManifestDigest:      req.ContractSnapshot.SkillManifestDigest,
			ContextHash:              req.ContractSnapshot.ContextHash,
			IntentHash:               req.ContractSnapshot.IntentHash,
			PlanHash:                 req.ContractSnapshot.PlanHash,
			KnowledgeSnapshotDigest:  req.ContractSnapshot.KnowledgeSnapshotDigest,
			RouteCatalogVersion:      req.ContractSnapshot.RouteCatalogVersion,
			ResolverContractVersion:  req.ContractSnapshot.ResolverContractVersion,
			ContextTemplateVersion:   req.ContractSnapshot.ContextTemplateVersion,
			ReplyGuidanceVersion:     req.ContractSnapshot.ReplyGuidanceVersion,
			PolicyContextDigest:      req.ContractSnapshot.PolicyContextDigest,
			EffectivePolicyVersion:   req.ContractSnapshot.EffectivePolicyVersion,
			ResolvedSetID:            req.ContractSnapshot.ResolvedSetID,
			SetIDSource:              req.ContractSnapshot.SetIDSource,
			PrecheckProjectionDigest: req.ContractSnapshot.PrecheckProjectionDigest,
			MutationPolicyVersion:    req.ContractSnapshot.MutationPolicyVersion,
		},
	})
	if err != nil {
		return nil, err
	}
	return mapAssistantTaskReceipt(receipt), nil
}

func (l cubeboxLegacyFacade) GetTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, taskID string) (*cubeboxdomain.TaskDetail, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	task, err := l.assistant.getTask(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, taskID)
	if err != nil {
		if errors.Is(err, errAssistantTaskNotFound) {
			return nil, cubeboxTaskNotFound()
		}
		return nil, err
	}
	return mapAssistantTaskDetail(task), nil
}

func (l cubeboxLegacyFacade) CancelTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, taskID string) (*cubeboxdomain.TaskCancelResponse, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	resp, err := l.assistant.cancelTask(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, taskID)
	if err != nil {
		return nil, err
	}
	return mapAssistantTaskCancelResponse(resp), nil
}

func (l cubeboxLegacyFacade) ExecuteTaskWorkflow(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversation *cubeboxdomain.Conversation, turnID string) (cubeboxservices.TaskWorkflowExecutionResult, error) {
	if l.assistant == nil {
		return cubeboxservices.TaskWorkflowExecutionResult{}, errAssistantGateUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := l.assistant.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return cubeboxservices.TaskWorkflowExecutionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	assistantConversation := assistantConversationFromCubeBox(conversation, tenantID)
	if assistantConversation == nil {
		return cubeboxservices.TaskWorkflowExecutionResult{ApplyErrorCode: cubeboxservices.ErrConversationNotFound.Error()}, nil
	}
	turn := assistantLookupTurn(assistantConversation, turnID)
	if turn == nil {
		return cubeboxservices.TaskWorkflowExecutionResult{ApplyErrorCode: cubeboxservices.ErrTurnNotFound.Error()}, nil
	}
	_, applyErr, execErr := l.assistant.executeCommitCoreTx(ctx, tx, tenantID, Principal{
		ID:       principal.ID,
		TenantID: strings.TrimSpace(tenantID),
		RoleSlug: principal.RoleSlug,
	}, assistantConversation, turn)
	if execErr != nil {
		return cubeboxservices.TaskWorkflowExecutionResult{}, execErr
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return cubeboxservices.TaskWorkflowExecutionResult{}, err
	}
	executedConversation := mapAssistantConversation(assistantConversation)
	if applyErr != nil {
		return cubeboxservices.TaskWorkflowExecutionResult{
			ApplyErrorCode: strings.TrimSpace(assistantTaskErrorCode(applyErr)),
			Conversation:   executedConversation,
		}, nil
	}
	return cubeboxservices.TaskWorkflowExecutionResult{Conversation: executedConversation}, nil
}

func assistantConversationFromCubeBox(conversation *cubeboxdomain.Conversation, tenantID string) *assistantConversation {
	if conversation == nil {
		return nil
	}
	out := &assistantConversation{
		ConversationID: strings.TrimSpace(conversation.ConversationID),
		TenantID:       firstNonEmpty(strings.TrimSpace(conversation.TenantID), strings.TrimSpace(tenantID)),
		ActorID:        strings.TrimSpace(conversation.ActorID),
		ActorRole:      strings.TrimSpace(conversation.ActorRole),
		State:          strings.TrimSpace(conversation.State),
		CurrentPhase:   strings.TrimSpace(conversation.CurrentPhase),
		CreatedAt:      conversation.CreatedAt.UTC(),
		UpdatedAt:      conversation.UpdatedAt.UTC(),
	}
	out.Turns = make([]*assistantTurn, 0, len(conversation.Turns))
	for _, turn := range conversation.Turns {
		mapped := assistantTurnFromCubeBox(turn)
		if mapped == nil {
			continue
		}
		out.Turns = append(out.Turns, mapped)
	}
	out.Transitions = make([]assistantStateTransition, 0, len(conversation.Transitions))
	for _, transition := range conversation.Transitions {
		out.Transitions = append(out.Transitions, assistantStateTransition{
			ID:         transition.ID,
			TurnID:     strings.TrimSpace(transition.TurnID),
			TurnAction: strings.TrimSpace(transition.TurnAction),
			RequestID:  strings.TrimSpace(transition.RequestID),
			TraceID:    strings.TrimSpace(transition.TraceID),
			FromState:  strings.TrimSpace(transition.FromState),
			ToState:    strings.TrimSpace(transition.ToState),
			FromPhase:  strings.TrimSpace(transition.FromPhase),
			ToPhase:    strings.TrimSpace(transition.ToPhase),
			ReasonCode: strings.TrimSpace(transition.ReasonCode),
			ActorID:    strings.TrimSpace(transition.ActorID),
			ChangedAt:  transition.ChangedAt.UTC(),
		})
	}
	assistantRefreshConversationDerivedFields(out)
	return out
}

func assistantTurnFromCubeBox(turn cubeboxdomain.ConversationTurn) *assistantTurn {
	out := &assistantTurn{
		TurnID:              strings.TrimSpace(turn.TurnID),
		UserInput:           strings.TrimSpace(turn.UserInput),
		State:               strings.TrimSpace(turn.State),
		Phase:               strings.TrimSpace(turn.Phase),
		RiskTier:            strings.TrimSpace(turn.RiskTier),
		RequestID:           strings.TrimSpace(turn.RequestID),
		TraceID:             strings.TrimSpace(turn.TraceID),
		PolicyVersion:       strings.TrimSpace(turn.PolicyVersion),
		CompositionVersion:  strings.TrimSpace(turn.CompositionVersion),
		MappingVersion:      strings.TrimSpace(turn.MappingVersion),
		ResolvedCandidateID: strings.TrimSpace(turn.ResolvedCandidateID),
		SelectedCandidateID: strings.TrimSpace(turn.SelectedCandidateID),
		AmbiguityCount:      turn.AmbiguityCount,
		Confidence:          turn.Confidence,
		ResolutionSource:    strings.TrimSpace(turn.ResolutionSource),
		PendingDraftSummary: strings.TrimSpace(turn.PendingDraftSummary),
		MissingFields:       append([]string(nil), turn.MissingFields...),
		ErrorCode:           strings.TrimSpace(turn.ErrorCode),
		CreatedAt:           turn.CreatedAt.UTC(),
		UpdatedAt:           turn.UpdatedAt.UTC(),
	}
	if err := remarshalJSON(turn.Intent, &out.Intent); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.RouteDecision, &out.RouteDecision); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.Clarification, &out.Clarification); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.Plan, &out.Plan); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.DryRun, &out.DryRun); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.Candidates, &out.Candidates); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.CommitResult, &out.CommitResult); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.CommitReply, &out.CommitReply); err != nil {
		return nil
	}
	if err := remarshalJSON(turn.ReplyNLG, &out.ReplyNLG); err != nil {
		return nil
	}
	assistantRefreshTurnDerivedFields(out)
	return out
}

func remarshalJSON(input any, target any) error {
	if input == nil {
		return nil
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return err
	}
	return json.Unmarshal(raw, target)
}

func (l cubeboxLegacyFacade) RenderReply(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, req map[string]any) (map[string]any, error) {
	if l.assistant == nil {
		return nil, errAssistantGateUnavailable
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var renderReq assistantRenderReplyRequest
	if len(payload) > 0 && string(payload) != "null" {
		if err := json.Unmarshal(payload, &renderReq); err != nil {
			return nil, err
		}
	}
	reply, err := l.assistant.renderTurnReply(ctx, tenantID, Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, conversationID, turnID, renderReq)
	if err != nil {
		return nil, err
	}
	if reply == nil {
		return nil, nil
	}
	out := map[string]any{}
	encoded, err := json.Marshal(reply)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type cubeboxRuntimeProbe struct {
	assistant *assistantConversationService
}

func (p cubeboxRuntimeProbe) BackendStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	if p.assistant == nil {
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "unavailable", Reason: "assistant_service_missing"}
	}
	return cubeboxdomain.RuntimeComponentStatus{Healthy: "healthy"}
}

func (p cubeboxRuntimeProbe) KnowledgeRuntimeStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	if p.assistant == nil {
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "unavailable", Reason: "knowledge_runtime_missing"}
	}
	if p.assistant.knowledgeErr != nil {
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "degraded", Reason: "knowledge_runtime_unavailable"}
	}
	return cubeboxdomain.RuntimeComponentStatus{Healthy: "healthy"}
}

func (p cubeboxRuntimeProbe) ModelGatewayStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	switch {
	case p.assistant == nil:
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "unavailable", Reason: "model_gateway_missing"}
	case p.assistant.modelGateway == nil:
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "unavailable", Reason: "model_gateway_missing"}
	case p.assistant.gatewayErr != nil:
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "unavailable", Reason: "model_gateway_unavailable"}
	default:
		return cubeboxdomain.RuntimeComponentStatus{Healthy: "healthy"}
	}
}

func (p cubeboxRuntimeProbe) Models(context.Context) ([]cubeboxdomain.ModelEntry, error) {
	if p.assistant == nil || p.assistant.modelGateway == nil {
		return nil, nil
	}
	rows := p.assistant.modelGateway.listModels()
	out := make([]cubeboxdomain.ModelEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, cubeboxdomain.ModelEntry{
			Provider: row.Name,
			Model:    row.Model,
		})
	}
	return out, nil
}

func newCubeBoxFacade(pool cubeboxmodule.PGBeginner, assistant *assistantConversationService, files *cubeboxservices.FileService) *cubeboxmodule.Facade {
	var pgStore *persistence.PGStore
	if pool != nil {
		pgStore = cubeboxmodule.NewPGStore(pool)
	}
	return cubeboxmodule.NewFacade(pgStore, cubeboxRuntimeProbe{assistant: assistant}, files, cubeboxLegacyFacade{assistant: assistant})
}

func cubeboxConversationNotFound() error  { return cubeboxservices.ErrConversationNotFound }
func cubeboxConversationForbidden() error { return cubeboxservices.ErrConversationForbidden }
func cubeboxTenantMismatch() error        { return cubeboxservices.ErrTenantMismatch }
func cubeboxTaskNotFound() error          { return cubeboxservices.ErrTaskNotFound }

func mapAssistantConversation(conversation *assistantConversation) *cubeboxdomain.Conversation {
	if conversation == nil {
		return nil
	}
	out := &cubeboxdomain.Conversation{
		ConversationID: strings.TrimSpace(conversation.ConversationID),
		TenantID:       strings.TrimSpace(conversation.TenantID),
		ActorID:        strings.TrimSpace(conversation.ActorID),
		ActorRole:      strings.TrimSpace(conversation.ActorRole),
		State:          strings.TrimSpace(conversation.State),
		CurrentPhase:   strings.TrimSpace(conversation.CurrentPhase),
		CreatedAt:      conversation.CreatedAt.UTC(),
		UpdatedAt:      conversation.UpdatedAt.UTC(),
	}
	out.Turns = make([]cubeboxdomain.ConversationTurn, 0, len(conversation.Turns))
	for _, turn := range conversation.Turns {
		if turn == nil {
			continue
		}
		out.Turns = append(out.Turns, cubeboxdomain.ConversationTurn{
			TurnID:              strings.TrimSpace(turn.TurnID),
			UserInput:           strings.TrimSpace(turn.UserInput),
			State:               strings.TrimSpace(turn.State),
			Phase:               strings.TrimSpace(turn.Phase),
			RiskTier:            strings.TrimSpace(turn.RiskTier),
			RequestID:           strings.TrimSpace(turn.RequestID),
			TraceID:             strings.TrimSpace(turn.TraceID),
			PolicyVersion:       strings.TrimSpace(turn.PolicyVersion),
			CompositionVersion:  strings.TrimSpace(turn.CompositionVersion),
			MappingVersion:      strings.TrimSpace(turn.MappingVersion),
			Intent:              assistantJSONMap(turn.Intent),
			RouteDecision:       assistantJSONMap(turn.RouteDecision),
			Clarification:       assistantJSONMap(turn.Clarification),
			Candidates:          assistantJSONMapSlice(turn.Candidates),
			Plan:                assistantJSONMap(turn.Plan),
			DryRun:              assistantJSONMap(turn.DryRun),
			ResolvedCandidateID: strings.TrimSpace(turn.ResolvedCandidateID),
			SelectedCandidateID: strings.TrimSpace(turn.SelectedCandidateID),
			AmbiguityCount:      turn.AmbiguityCount,
			Confidence:          turn.Confidence,
			ResolutionSource:    strings.TrimSpace(turn.ResolutionSource),
			PendingDraftSummary: strings.TrimSpace(turn.PendingDraftSummary),
			MissingFields:       append([]string(nil), turn.MissingFields...),
			CommitResult:        assistantJSONMap(turn.CommitResult),
			CommitReply:         assistantJSONMap(turn.CommitReply),
			ReplyNLG:            assistantJSONMap(turn.ReplyNLG),
			ErrorCode:           strings.TrimSpace(turn.ErrorCode),
			CreatedAt:           turn.CreatedAt.UTC(),
			UpdatedAt:           turn.UpdatedAt.UTC(),
		})
	}
	out.Transitions = make([]cubeboxdomain.StateTransition, 0, len(conversation.Transitions))
	for _, transition := range conversation.Transitions {
		out.Transitions = append(out.Transitions, cubeboxdomain.StateTransition{
			ID:         transition.ID,
			TurnID:     strings.TrimSpace(transition.TurnID),
			TurnAction: strings.TrimSpace(transition.TurnAction),
			RequestID:  strings.TrimSpace(transition.RequestID),
			TraceID:    strings.TrimSpace(transition.TraceID),
			FromState:  strings.TrimSpace(transition.FromState),
			ToState:    strings.TrimSpace(transition.ToState),
			FromPhase:  strings.TrimSpace(transition.FromPhase),
			ToPhase:    strings.TrimSpace(transition.ToPhase),
			ReasonCode: strings.TrimSpace(transition.ReasonCode),
			ActorID:    strings.TrimSpace(transition.ActorID),
			ChangedAt:  transition.ChangedAt.UTC(),
		})
	}
	return out
}

func mapAssistantTaskReceipt(receipt *assistantTaskAsyncReceipt) *cubeboxdomain.TaskReceipt {
	if receipt == nil {
		return nil
	}
	return &cubeboxdomain.TaskReceipt{
		TaskID:      strings.TrimSpace(receipt.TaskID),
		TaskType:    strings.TrimSpace(receipt.TaskType),
		Status:      strings.TrimSpace(receipt.Status),
		WorkflowID:  strings.TrimSpace(receipt.WorkflowID),
		SubmittedAt: receipt.SubmittedAt.UTC(),
		PollURI:     cubeboxTaskPollURI(receipt.TaskID),
	}
}

func mapAssistantTaskDetail(detail *assistantTaskDetailResponse) *cubeboxdomain.TaskDetail {
	if detail == nil {
		return nil
	}
	return &cubeboxdomain.TaskDetail{
		TaskID:            strings.TrimSpace(detail.TaskID),
		TaskType:          strings.TrimSpace(detail.TaskType),
		Status:            strings.TrimSpace(detail.Status),
		DispatchStatus:    strings.TrimSpace(detail.DispatchStatus),
		Attempt:           detail.Attempt,
		MaxAttempts:       detail.MaxAttempts,
		LastErrorCode:     strings.TrimSpace(detail.LastErrorCode),
		WorkflowID:        strings.TrimSpace(detail.WorkflowID),
		RequestID:         strings.TrimSpace(detail.RequestID),
		TraceID:           strings.TrimSpace(detail.TraceID),
		ConversationID:    strings.TrimSpace(detail.ConversationID),
		TurnID:            strings.TrimSpace(detail.TurnID),
		SubmittedAt:       detail.SubmittedAt.UTC(),
		CancelRequestedAt: detail.CancelRequestedAt,
		CompletedAt:       detail.CompletedAt,
		UpdatedAt:         detail.UpdatedAt.UTC(),
		ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:      detail.ContractSnapshot.IntentSchemaVersion,
			CompilerContractVersion:  detail.ContractSnapshot.CompilerContractVersion,
			CapabilityMapVersion:     detail.ContractSnapshot.CapabilityMapVersion,
			SkillManifestDigest:      detail.ContractSnapshot.SkillManifestDigest,
			ContextHash:              detail.ContractSnapshot.ContextHash,
			IntentHash:               detail.ContractSnapshot.IntentHash,
			PlanHash:                 detail.ContractSnapshot.PlanHash,
			KnowledgeSnapshotDigest:  detail.ContractSnapshot.KnowledgeSnapshotDigest,
			RouteCatalogVersion:      detail.ContractSnapshot.RouteCatalogVersion,
			ResolverContractVersion:  detail.ContractSnapshot.ResolverContractVersion,
			ContextTemplateVersion:   detail.ContractSnapshot.ContextTemplateVersion,
			ReplyGuidanceVersion:     detail.ContractSnapshot.ReplyGuidanceVersion,
			PolicyContextDigest:      detail.ContractSnapshot.PolicyContextDigest,
			EffectivePolicyVersion:   detail.ContractSnapshot.EffectivePolicyVersion,
			ResolvedSetID:            detail.ContractSnapshot.ResolvedSetID,
			SetIDSource:              detail.ContractSnapshot.SetIDSource,
			PrecheckProjectionDigest: detail.ContractSnapshot.PrecheckProjectionDigest,
			MutationPolicyVersion:    detail.ContractSnapshot.MutationPolicyVersion,
		},
	}
}

func mapAssistantTaskCancelResponse(resp *assistantTaskCancelResponse) *cubeboxdomain.TaskCancelResponse {
	if resp == nil {
		return nil
	}
	return &cubeboxdomain.TaskCancelResponse{
		TaskDetail:     *mapAssistantTaskDetail(&resp.assistantTaskDetailResponse),
		CancelAccepted: resp.CancelAccepted,
	}
}

func cubeboxTaskPollURI(taskID string) string {
	return "/internal/cubebox/tasks/" + strings.TrimSpace(taskID)
}

func assistantJSONMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func assistantJSONMapSlice(value any) []map[string]any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}
