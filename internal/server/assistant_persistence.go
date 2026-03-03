package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

const (
	assistantIdempotencyTTLDays     = 30
	assistantDefaultRetryAfterSecs  = "1"
	assistantConversationCursorSalt = "assistant-conversation-cursor-v1"
)

type assistantIdempotencyKey struct {
	TenantID       string
	ConversationID string
	TurnID         string
	TurnAction     string
	RequestID      string
}

type assistantIdempotencyClaimState string

const (
	assistantIdempotencyClaimInserted   assistantIdempotencyClaimState = "inserted"
	assistantIdempotencyClaimDone       assistantIdempotencyClaimState = "done"
	assistantIdempotencyClaimInProgress assistantIdempotencyClaimState = "in_progress"
	assistantIdempotencyClaimConflict   assistantIdempotencyClaimState = "conflict"
)

type assistantIdempotencyClaim struct {
	State      assistantIdempotencyClaimState
	ErrorCode  string
	HTTPStatus int
	Body       []byte
}

type assistantConversationCursor struct {
	UpdatedAt      time.Time
	ConversationID string
}

func (s *assistantConversationService) createConversationPG(ctx context.Context, tenantID string, principal Principal) (*assistantConversation, error) {
	conversation := s.createConversation(tenantID, principal)
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.persistConversationCreate(ctx, conversation); err != nil {
		return nil, err
	}
	s.cacheConversation(conversation)
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) getConversationPG(ctx context.Context, tenantID string, actorID string, conversationID string) (*assistantConversation, error) {
	if cached, ok := s.getCachedConversation(conversationID); ok {
		if cached == nil {
			return nil, errAssistantConversationCorrupted
		}
		if cached.TenantID != tenantID {
			return nil, errAssistantTenantMismatch
		}
		if cached.ActorID != actorID {
			return nil, errAssistantConversationForbidden
		}
		return cloneConversation(cached), nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	conversation, err := s.loadConversationByTenant(ctx, tenantID, conversationID, false)
	if err != nil {
		return nil, err
	}
	if conversation.ActorID != actorID {
		return nil, errAssistantConversationForbidden
	}
	s.cacheConversation(conversation)
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) listConversationsPG(ctx context.Context, tenantID string, actorID string, pageSize int, cursor string) ([]assistantConversationListItem, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	decoded, err := assistantDecodeConversationCursor(cursor, tenantID, actorID)
	if err != nil {
		return nil, "", err
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	query := `
SELECT
  c.conversation_id,
  c.state,
  c.updated_at,
  t.turn_id,
  t.user_input,
  t.state,
  t.risk_tier
FROM iam.assistant_conversations c
LEFT JOIN LATERAL (
  SELECT turn_id, user_input, state, risk_tier
  FROM iam.assistant_turns at
  WHERE at.tenant_uuid = $1::uuid AND at.conversation_id = c.conversation_id
  ORDER BY at.created_at DESC, at.turn_id DESC
  LIMIT 1
) t ON TRUE
WHERE c.tenant_uuid = $1::uuid
  AND c.actor_id = $2`
	args := make([]any, 0, 5)
	args = append(args, tenantID, actorID)
	if decoded != nil {
		query += `
  AND (c.updated_at, c.conversation_id) < ($3, $4)`
		args = append(args, decoded.UpdatedAt, decoded.ConversationID)
	}
	query += `
ORDER BY c.updated_at DESC, c.conversation_id DESC
LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, pageSize+1)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := make([]assistantConversationListItem, 0, pageSize+1)
	for rows.Next() {
		var (
			item          assistantConversationListItem
			lastTurnID    *string
			lastUserInput *string
			lastTurnState *string
			lastRiskTier  *string
		)
		if err := rows.Scan(
			&item.ConversationID,
			&item.State,
			&item.UpdatedAt,
			&lastTurnID,
			&lastUserInput,
			&lastTurnState,
			&lastRiskTier,
		); err != nil {
			return nil, "", err
		}
		if lastTurnID != nil {
			item.LastTurn = &assistantConversationListLastTurn{
				TurnID:    strings.TrimSpace(*lastTurnID),
				UserInput: strings.TrimSpace(valueOrEmpty(lastUserInput)),
				State:     strings.TrimSpace(valueOrEmpty(lastTurnState)),
				RiskTier:  strings.TrimSpace(valueOrEmpty(lastRiskTier)),
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(items) > pageSize {
		last := items[pageSize-1]
		nextCursor = assistantEncodeConversationCursor(assistantConversationCursor{
			UpdatedAt:      last.UpdatedAt,
			ConversationID: last.ConversationID,
		}, tenantID, actorID)
		items = items[:pageSize]
	}
	return items, nextCursor, nil
}

func (s *assistantConversationService) createTurnPG(ctx context.Context, tenantID string, principal Principal, conversationID string, userInput string) (*assistantConversation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	resolvedIntent, err := s.resolveIntent(ctx, tenantID, conversationID, userInput)
	if err != nil {
		return nil, err
	}
	intent := resolvedIntent.Intent
	candidates := make([]assistantCandidate, 0)
	resolvedCandidateID := ""
	resolutionSource := ""
	ambiguityCount := 0
	confidence := 0.65
	if intent.Action == assistantIntentCreateOrgUnit && intent.ParentRefText != "" {
		resolved, resolveErr := s.resolveCandidates(ctx, tenantID, intent.ParentRefText, intent.EffectiveDate)
		if resolveErr != nil {
			return nil, resolveErr
		}
		candidates = resolved
		ambiguityCount = len(candidates)
		switch len(candidates) {
		case 0:
			confidence = 0.3
		case 1:
			resolvedCandidateID = candidates[0].CandidateID
			resolutionSource = assistantResolutionAuto
			confidence = 0.95
		default:
			confidence = 0.55
		}
	}

	plan := assistantBuildPlan(intent)
	plan.ModelProvider = resolvedIntent.ProviderName
	plan.ModelName = resolvedIntent.ModelName
	plan.ModelRevision = resolvedIntent.ModelRevision
	skillExecutionPlan, configDeltaPlan := assistantCompileIntentToPlans(intent, resolvedCandidateID)
	plan.SkillExecutionPlan = skillExecutionPlan
	plan.ConfigDeltaPlan = configDeltaPlan
	if _, ok := capabilityDefinitionForKey(plan.CapabilityKey); !ok {
		return nil, errAssistantPlanBoundaryViolation
	}
	dryRun := assistantBuildDryRun(intent, candidates, resolvedCandidateID)
	if err := assistantAnnotateIntentPlanFn(tenantID, conversationID, userInput, &intent, &plan, &dryRun); err != nil {
		return nil, err
	}
	policyVersion, compositionVersion, mappingVersion := assistantTurnVersionSnapshot(plan.CapabilityKey)
	now := time.Now().UTC()
	turn := &assistantTurn{
		TurnID:              "turn_" + strings.ReplaceAll(newUUIDString(), "-", ""),
		UserInput:           userInput,
		State:               assistantStateValidated,
		RiskTier:            assistantRiskTierForIntent(intent),
		RequestID:           "assistant_" + strings.ReplaceAll(newUUIDString(), "-", ""),
		TraceID:             strings.ReplaceAll(newUUIDString(), "-", ""),
		PolicyVersion:       policyVersion,
		CompositionVersion:  compositionVersion,
		MappingVersion:      mappingVersion,
		Intent:              intent,
		Plan:                plan,
		Candidates:          candidates,
		ResolvedCandidateID: resolvedCandidateID,
		AmbiguityCount:      ambiguityCount,
		Confidence:          confidence,
		ResolutionSource:    resolutionSource,
		DryRun:              dryRun,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conversation, err := s.loadConversationTx(ctx, tx, tenantID, conversationID, true)
	if err != nil {
		return nil, err
	}
	if conversation.ActorID != principal.ID {
		return nil, errAssistantConversationForbidden
	}

	conversation.Turns = append(conversation.Turns, turn)
	conversation.State = turn.State
	conversation.UpdatedAt = now
	conversation.Transitions = append(conversation.Transitions, assistantStateTransition{
		TurnID:     turn.TurnID,
		RequestID:  turn.RequestID,
		TraceID:    turn.TraceID,
		FromState:  "init",
		ToState:    assistantStateValidated,
		ReasonCode: "turn_created",
		ActorID:    principal.ID,
		ChangedAt:  now,
	})

	if err := s.upsertTurnTx(ctx, tx, tenantID, conversation.ConversationID, turn); err != nil {
		return nil, err
	}
	if err := s.updateConversationStateTx(ctx, tx, tenantID, conversation.ConversationID, conversation.State, conversation.UpdatedAt); err != nil {
		return nil, err
	}
	if err := s.insertTransitionTx(ctx, tx, tenantID, conversation.ConversationID, &conversation.Transitions[len(conversation.Transitions)-1]); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.cacheConversation(conversation)
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) confirmTurnPG(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string, candidateID string) (*assistantConversation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conversation, err := s.loadConversationTx(ctx, tx, tenantID, conversationID, true)
	if err != nil {
		return nil, err
	}
	if conversation.ActorID != principal.ID {
		return nil, errAssistantConversationForbidden
	}

	turn := assistantLookupTurn(conversation, turnID)
	if turn == nil {
		return nil, errAssistantTurnNotFound
	}

	requestHash := assistantHashText("confirm\n" + candidateID)
	claimKey := assistantIdempotencyKey{
		TenantID:       tenantID,
		ConversationID: conversationID,
		TurnID:         turnID,
		TurnAction:     "confirm",
		RequestID:      turn.RequestID,
	}
	claim, err := s.claimIdempotencyTx(ctx, tx, claimKey, requestHash)
	if err != nil {
		return nil, err
	}
	switch claim.State {
	case assistantIdempotencyClaimConflict:
		return nil, errAssistantIdempotencyKeyConflict
	case assistantIdempotencyClaimInProgress:
		return nil, errAssistantRequestInProgress
	case assistantIdempotencyClaimDone:
		return s.restoreIdempotentResult(claim)
	}

	result, applyErr := s.applyConfirmTurn(conversation, turn, principal, candidateID)
	if applyErr != nil {
		if finalizeErr := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, applyErr); finalizeErr != nil {
			return nil, finalizeErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return nil, applyErr
	}

	if err := s.upsertTurnTx(ctx, tx, tenantID, conversation.ConversationID, turn); err != nil {
		return nil, err
	}
	if err := s.updateConversationStateTx(ctx, tx, tenantID, conversation.ConversationID, conversation.State, conversation.UpdatedAt); err != nil {
		return nil, err
	}
	if result.Transition != nil {
		if err := s.insertTransitionTx(ctx, tx, tenantID, conversation.ConversationID, result.Transition); err != nil {
			return nil, err
		}
	}
	if err := s.finalizeIdempotencySuccessTx(ctx, tx, claimKey, conversation); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.cacheConversation(conversation)
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) commitTurnPG(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*assistantConversation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conversation, err := s.loadConversationTx(ctx, tx, tenantID, conversationID, true)
	if err != nil {
		return nil, err
	}
	if principal.ID != conversation.ActorID {
		return nil, errAssistantAuthSnapshotExpired
	}
	if strings.TrimSpace(principal.RoleSlug) != strings.TrimSpace(conversation.ActorRole) {
		return nil, errAssistantRoleDriftDetected
	}
	turn := assistantLookupTurn(conversation, turnID)
	if turn == nil {
		return nil, errAssistantTurnNotFound
	}

	requestHash := assistantHashText("commit\n")
	claimKey := assistantIdempotencyKey{
		TenantID:       tenantID,
		ConversationID: conversationID,
		TurnID:         turnID,
		TurnAction:     "commit",
		RequestID:      turn.RequestID,
	}
	claim, err := s.claimIdempotencyTx(ctx, tx, claimKey, requestHash)
	if err != nil {
		return nil, err
	}
	switch claim.State {
	case assistantIdempotencyClaimConflict:
		return nil, errAssistantIdempotencyKeyConflict
	case assistantIdempotencyClaimInProgress:
		return nil, errAssistantRequestInProgress
	case assistantIdempotencyClaimDone:
		return s.restoreIdempotentResult(claim)
	}

	result, applyErr := s.applyCommitTurn(ctx, conversation, turn, principal, tenantID)
	if applyErr != nil {
		if result.PersistTurn {
			if err := s.upsertTurnTx(ctx, tx, tenantID, conversation.ConversationID, turn); err != nil {
				return nil, err
			}
			if err := s.updateConversationStateTx(ctx, tx, tenantID, conversation.ConversationID, conversation.State, conversation.UpdatedAt); err != nil {
				return nil, err
			}
		}
		if result.Transition != nil {
			if err := s.insertTransitionTx(ctx, tx, tenantID, conversation.ConversationID, result.Transition); err != nil {
				return nil, err
			}
		}
		if finalizeErr := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, applyErr); finalizeErr != nil {
			return nil, finalizeErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return nil, applyErr
	}

	if err := s.upsertTurnTx(ctx, tx, tenantID, conversation.ConversationID, turn); err != nil {
		return nil, err
	}
	if err := s.updateConversationStateTx(ctx, tx, tenantID, conversation.ConversationID, conversation.State, conversation.UpdatedAt); err != nil {
		return nil, err
	}
	if result.Transition != nil {
		if err := s.insertTransitionTx(ctx, tx, tenantID, conversation.ConversationID, result.Transition); err != nil {
			return nil, err
		}
	}
	if err := s.finalizeIdempotencySuccessTx(ctx, tx, claimKey, conversation); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.cacheConversation(conversation)
	return cloneConversation(conversation), nil
}

type assistantTurnMutationResult struct {
	Transition  *assistantStateTransition
	PersistTurn bool
}

func (s *assistantConversationService) applyConfirmTurn(conversation *assistantConversation, turn *assistantTurn, principal Principal, candidateID string) (assistantTurnMutationResult, error) {
	if turn.State == assistantStateCommitted {
		return assistantTurnMutationResult{}, nil
	}
	if turn.State == assistantStateCanceled || turn.State == assistantStateExpired {
		return assistantTurnMutationResult{}, errAssistantConversationStateInvalid
	}
	if turn.State == assistantStateConfirmed {
		if turn.AmbiguityCount > 1 {
			if candidateID == "" || candidateID == turn.ResolvedCandidateID {
				return assistantTurnMutationResult{}, nil
			}
			if !assistantCandidateExists(turn.Candidates, candidateID) {
				return assistantTurnMutationResult{}, errAssistantCandidateNotFound
			}
			return assistantTurnMutationResult{}, errAssistantConversationStateInvalid
		}
		return assistantTurnMutationResult{}, nil
	}
	if turn.State != assistantStateValidated {
		return assistantTurnMutationResult{}, errAssistantConfirmationRequired
	}
	if turn.AmbiguityCount > 1 {
		if candidateID == "" {
			return assistantTurnMutationResult{}, errAssistantConfirmationRequired
		}
		if !assistantCandidateExists(turn.Candidates, candidateID) {
			return assistantTurnMutationResult{}, errAssistantCandidateNotFound
		}
		turn.ResolvedCandidateID = candidateID
		turn.ResolutionSource = assistantResolutionUserConfirmed
	}
	if turn.Intent.Action == assistantIntentCreateOrgUnit && turn.ResolvedCandidateID == "" {
		return assistantTurnMutationResult{}, errAssistantConfirmationRequired
	}
	fromState := turn.State
	turn.PolicyVersion, turn.CompositionVersion, turn.MappingVersion = assistantTurnVersionSnapshot(turn.Plan.CapabilityKey)
	turn.State = assistantStateConfirmed
	turn.UpdatedAt = time.Now().UTC()
	conversation.UpdatedAt = turn.UpdatedAt
	conversation.State = turn.State
	transition := &assistantStateTransition{
		TurnID:     turn.TurnID,
		TurnAction: "confirm",
		RequestID:  turn.RequestID,
		TraceID:    turn.TraceID,
		FromState:  fromState,
		ToState:    turn.State,
		ReasonCode: "confirmed",
		ActorID:    principal.ID,
		ChangedAt:  turn.UpdatedAt,
	}
	conversation.Transitions = append(conversation.Transitions, *transition)
	return assistantTurnMutationResult{Transition: transition, PersistTurn: true}, nil
}

func (s *assistantConversationService) applyCommitTurn(ctx context.Context, conversation *assistantConversation, turn *assistantTurn, principal Principal, tenantID string) (assistantTurnMutationResult, error) {
	if turn.State == assistantStateCommitted {
		return assistantTurnMutationResult{}, nil
	}
	if turn.State == assistantStateCanceled || turn.State == assistantStateExpired {
		return assistantTurnMutationResult{}, errAssistantConversationStateInvalid
	}
	if turn.State != assistantStateConfirmed {
		return assistantTurnMutationResult{}, errAssistantConfirmationRequired
	}
	if assistantTurnContractVersionMismatched(turn) {
		fromState := turn.State
		turn.State = assistantStateValidated
		turn.UpdatedAt = time.Now().UTC()
		conversation.UpdatedAt = turn.UpdatedAt
		conversation.State = turn.State
		transition := &assistantStateTransition{
			TurnID:     turn.TurnID,
			TurnAction: "commit",
			RequestID:  turn.RequestID,
			TraceID:    turn.TraceID,
			FromState:  fromState,
			ToState:    turn.State,
			ReasonCode: "contract_version_mismatch",
			ActorID:    principal.ID,
			ChangedAt:  turn.UpdatedAt,
		}
		conversation.Transitions = append(conversation.Transitions, *transition)
		return assistantTurnMutationResult{Transition: transition, PersistTurn: true}, errAssistantPlanContractVersionMismatch
	}
	if assistantTurnVersionDrifted(turn) {
		fromState := turn.State
		turn.State = assistantStateValidated
		turn.UpdatedAt = time.Now().UTC()
		conversation.UpdatedAt = turn.UpdatedAt
		conversation.State = turn.State
		transition := &assistantStateTransition{
			TurnID:     turn.TurnID,
			TurnAction: "commit",
			RequestID:  turn.RequestID,
			TraceID:    turn.TraceID,
			FromState:  fromState,
			ToState:    turn.State,
			ReasonCode: "version_drift",
			ActorID:    principal.ID,
			ChangedAt:  turn.UpdatedAt,
		}
		conversation.Transitions = append(conversation.Transitions, *transition)
		return assistantTurnMutationResult{Transition: transition, PersistTurn: true}, errAssistantConfirmationRequired
	}
	if turn.Intent.Action != assistantIntentCreateOrgUnit {
		return assistantTurnMutationResult{}, errAssistantUnsupportedIntent
	}
	if turn.ResolvedCandidateID == "" {
		return assistantTurnMutationResult{}, errAssistantCandidateNotFound
	}
	if s.writeSvc == nil {
		return assistantTurnMutationResult{}, errAssistantServiceMissing
	}
	resolved, ok := assistantFindCandidate(turn.Candidates, turn.ResolvedCandidateID)
	if !ok {
		return assistantTurnMutationResult{}, errAssistantCandidateNotFound
	}
	name := turn.Intent.EntityName
	if strings.TrimSpace(name) == "" {
		name = "新建组织"
	}
	parentOrgCode := resolved.CandidateCode
	orgCode := assistantGeneratedOrgCode(turn.TurnID)
	result, err := s.writeSvc.Write(ctx, tenantID, orgunitservices.WriteOrgUnitRequest{
		Intent:        string(orgunitservices.OrgUnitWriteIntentCreateOrg),
		OrgCode:       orgCode,
		EffectiveDate: turn.Intent.EffectiveDate,
		PolicyVersion: turn.PolicyVersion,
		RequestID:     turn.RequestID,
		Patch: orgunitservices.OrgUnitWritePatch{
			Name:          ptrString(name),
			ParentOrgCode: ptrString(parentOrgCode),
		},
		InitiatorUUID: principal.ID,
	})
	if err != nil {
		return assistantTurnMutationResult{}, err
	}
	turn.CommitResult = &assistantCommitResult{
		OrgCode:       result.OrgCode,
		ParentOrgCode: parentOrgCode,
		EffectiveDate: result.EffectiveDate,
		EventType:     result.EventType,
		EventUUID:     result.EventUUID,
	}
	fromState := turn.State
	turn.State = assistantStateCommitted
	turn.UpdatedAt = time.Now().UTC()
	conversation.UpdatedAt = turn.UpdatedAt
	conversation.State = turn.State
	transition := &assistantStateTransition{
		TurnID:     turn.TurnID,
		TurnAction: "commit",
		RequestID:  turn.RequestID,
		TraceID:    turn.TraceID,
		FromState:  fromState,
		ToState:    turn.State,
		ReasonCode: "committed",
		ActorID:    principal.ID,
		ChangedAt:  turn.UpdatedAt,
	}
	conversation.Transitions = append(conversation.Transitions, *transition)
	return assistantTurnMutationResult{Transition: transition, PersistTurn: true}, nil
}

func (s *assistantConversationService) persistConversationCreate(ctx context.Context, conversation *assistantConversation) error {
	tx, err := s.beginAssistantTx(ctx, conversation.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
INSERT INTO iam.assistant_conversations (
  tenant_uuid, conversation_id, actor_id, actor_role, state, created_at, updated_at
) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
`, conversation.TenantID, conversation.ConversationID, conversation.ActorID, conversation.ActorRole, conversation.State, conversation.CreatedAt, conversation.UpdatedAt)
	if err != nil {
		return err
	}
	if len(conversation.Transitions) > 0 {
		if err := s.insertTransitionTx(ctx, tx, conversation.TenantID, conversation.ConversationID, &conversation.Transitions[0]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *assistantConversationService) beginAssistantTx(ctx context.Context, tenantID string) (pgx.Tx, error) {
	if s == nil || s.pool == nil {
		return nil, errAssistantServiceMissing
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (s *assistantConversationService) loadConversationByTenant(ctx context.Context, tenantID string, conversationID string, forUpdate bool) (*assistantConversation, error) {
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	conversation, err := s.loadConversationTx(ctx, tx, tenantID, conversationID, forUpdate)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *assistantConversationService) loadConversationTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, forUpdate bool) (*assistantConversation, error) {
	query := `
SELECT conversation_id, tenant_uuid::text, actor_id, actor_role, state, created_at, updated_at
FROM iam.assistant_conversations
WHERE tenant_uuid = $1::uuid AND conversation_id = $2`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	conversation := assistantConversation{}
	if err := tx.QueryRow(ctx, query, tenantID, conversationID).Scan(
		&conversation.ConversationID,
		&conversation.TenantID,
		&conversation.ActorID,
		&conversation.ActorRole,
		&conversation.State,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errAssistantConversationNotFound
		}
		return nil, err
	}

	turnRows, err := tx.Query(ctx, `
SELECT
  turn_id,
  user_input,
  state,
  risk_tier,
  request_id,
  trace_id,
  policy_version,
  composition_version,
  mapping_version,
  intent_json,
  plan_json,
  candidates_json,
  resolved_candidate_id,
  ambiguity_count,
  confidence,
  resolution_source,
  dry_run_json,
  commit_result_json,
  created_at,
  updated_at
FROM iam.assistant_turns
WHERE tenant_uuid = $1::uuid AND conversation_id = $2
ORDER BY created_at, turn_id
`, tenantID, conversationID)
	if err != nil {
		return nil, err
	}
	defer turnRows.Close()
	conversation.Turns = make([]*assistantTurn, 0, 4)
	for turnRows.Next() {
		var (
			turn                assistantTurn
			intentJSON          []byte
			planJSON            []byte
			candidatesJSON      []byte
			dryRunJSON          []byte
			commitResultJSON    []byte
			resolvedCandidateID *string
			resolutionSource    *string
		)
		if err := turnRows.Scan(
			&turn.TurnID,
			&turn.UserInput,
			&turn.State,
			&turn.RiskTier,
			&turn.RequestID,
			&turn.TraceID,
			&turn.PolicyVersion,
			&turn.CompositionVersion,
			&turn.MappingVersion,
			&intentJSON,
			&planJSON,
			&candidatesJSON,
			&resolvedCandidateID,
			&turn.AmbiguityCount,
			&turn.Confidence,
			&resolutionSource,
			&dryRunJSON,
			&commitResultJSON,
			&turn.CreatedAt,
			&turn.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if resolvedCandidateID != nil {
			turn.ResolvedCandidateID = *resolvedCandidateID
		}
		if resolutionSource != nil {
			turn.ResolutionSource = *resolutionSource
		}
		if err := json.Unmarshal(intentJSON, &turn.Intent); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(planJSON, &turn.Plan); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(candidatesJSON, &turn.Candidates); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(dryRunJSON, &turn.DryRun); err != nil {
			return nil, err
		}
		if len(commitResultJSON) > 0 && string(commitResultJSON) != "null" {
			var commitResult assistantCommitResult
			if err := json.Unmarshal(commitResultJSON, &commitResult); err != nil {
				return nil, err
			}
			turn.CommitResult = &commitResult
		}
		copyTurn := turn
		conversation.Turns = append(conversation.Turns, &copyTurn)
	}
	if err := turnRows.Err(); err != nil {
		return nil, err
	}

	transitionRows, err := tx.Query(ctx, `
SELECT id, turn_id, turn_action, request_id, trace_id, from_state, to_state, reason_code, actor_id, changed_at
FROM iam.assistant_state_transitions
WHERE tenant_uuid = $1::uuid AND conversation_id = $2
ORDER BY changed_at, id
`, tenantID, conversationID)
	if err != nil {
		return nil, err
	}
	defer transitionRows.Close()
	conversation.Transitions = make([]assistantStateTransition, 0, 8)
	for transitionRows.Next() {
		var (
			transition assistantStateTransition
			turnID     *string
			action     *string
			reasonCode *string
		)
		if err := transitionRows.Scan(
			&transition.ID,
			&turnID,
			&action,
			&transition.RequestID,
			&transition.TraceID,
			&transition.FromState,
			&transition.ToState,
			&reasonCode,
			&transition.ActorID,
			&transition.ChangedAt,
		); err != nil {
			return nil, err
		}
		if turnID != nil {
			transition.TurnID = *turnID
		}
		if action != nil {
			transition.TurnAction = *action
		}
		if reasonCode != nil {
			transition.ReasonCode = *reasonCode
		}
		conversation.Transitions = append(conversation.Transitions, transition)
	}
	if err := transitionRows.Err(); err != nil {
		return nil, err
	}

	return &conversation, nil
}

func (s *assistantConversationService) upsertTurnTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, turn *assistantTurn) error {
	intentJSON, _ := json.Marshal(turn.Intent)
	planJSON, err := json.Marshal(turn.Plan)
	if err != nil {
		return err
	}
	candidatesJSON, _ := json.Marshal(turn.Candidates)
	dryRunJSON, err := json.Marshal(turn.DryRun)
	if err != nil {
		return err
	}
	var commitResultJSON []byte
	if turn.CommitResult != nil {
		commitResultJSON, _ = json.Marshal(turn.CommitResult)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO iam.assistant_turns (
  tenant_uuid,
  conversation_id,
  turn_id,
  user_input,
  state,
  risk_tier,
  request_id,
  trace_id,
  policy_version,
  composition_version,
  mapping_version,
  intent_json,
  plan_json,
  candidates_json,
  resolved_candidate_id,
  ambiguity_count,
  confidence,
  resolution_source,
  dry_run_json,
  commit_result_json,
  created_at,
  updated_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11,
  $12::jsonb,
  $13::jsonb,
  $14::jsonb,
  NULLIF($15, ''),
  $16,
  $17,
  NULLIF($18, ''),
  $19::jsonb,
  $20::jsonb,
  $21,
  $22
)
ON CONFLICT (tenant_uuid, conversation_id, turn_id)
DO UPDATE SET
  user_input = EXCLUDED.user_input,
  state = EXCLUDED.state,
  risk_tier = EXCLUDED.risk_tier,
  request_id = EXCLUDED.request_id,
  trace_id = EXCLUDED.trace_id,
  policy_version = EXCLUDED.policy_version,
  composition_version = EXCLUDED.composition_version,
  mapping_version = EXCLUDED.mapping_version,
  intent_json = EXCLUDED.intent_json,
  plan_json = EXCLUDED.plan_json,
  candidates_json = EXCLUDED.candidates_json,
  resolved_candidate_id = EXCLUDED.resolved_candidate_id,
  ambiguity_count = EXCLUDED.ambiguity_count,
  confidence = EXCLUDED.confidence,
  resolution_source = EXCLUDED.resolution_source,
  dry_run_json = EXCLUDED.dry_run_json,
  commit_result_json = EXCLUDED.commit_result_json,
  updated_at = EXCLUDED.updated_at
`,
		tenantID,
		conversationID,
		turn.TurnID,
		turn.UserInput,
		turn.State,
		turn.RiskTier,
		turn.RequestID,
		turn.TraceID,
		turn.PolicyVersion,
		turn.CompositionVersion,
		turn.MappingVersion,
		string(intentJSON),
		string(planJSON),
		string(candidatesJSON),
		turn.ResolvedCandidateID,
		turn.AmbiguityCount,
		turn.Confidence,
		turn.ResolutionSource,
		string(dryRunJSON),
		nilIfEmptyJSON(commitResultJSON),
		turn.CreatedAt,
		turn.UpdatedAt,
	)
	return err
}

func nilIfEmptyJSON(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	return string(data)
}

func (s *assistantConversationService) updateConversationStateTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, state string, updatedAt time.Time) error {
	_, err := tx.Exec(ctx, `
UPDATE iam.assistant_conversations
SET state = $3,
    updated_at = $4
WHERE tenant_uuid = $1::uuid AND conversation_id = $2
`, tenantID, conversationID, state, updatedAt)
	return err
}

func (s *assistantConversationService) insertTransitionTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, transition *assistantStateTransition) error {
	if transition == nil {
		return nil
	}
	var turnID any
	if strings.TrimSpace(transition.TurnID) != "" {
		turnID = transition.TurnID
	}
	var turnAction any
	if strings.TrimSpace(transition.TurnAction) != "" {
		turnAction = transition.TurnAction
	}
	var reasonCode any
	if strings.TrimSpace(transition.ReasonCode) != "" {
		reasonCode = transition.ReasonCode
	}
	if transition.ChangedAt.IsZero() {
		transition.ChangedAt = time.Now().UTC()
	}
	if strings.TrimSpace(transition.RequestID) == "" {
		transition.RequestID = "assistant_" + strings.ReplaceAll(newUUIDString(), "-", "")
	}
	if strings.TrimSpace(transition.TraceID) == "" {
		transition.TraceID = strings.ReplaceAll(newUUIDString(), "-", "")
	}
	if strings.TrimSpace(transition.ActorID) == "" {
		transition.ActorID = conversationID
	}
	var id int64
	err := tx.QueryRow(ctx, `
INSERT INTO iam.assistant_state_transitions (
  tenant_uuid,
  conversation_id,
  turn_id,
  turn_action,
  request_id,
  trace_id,
  from_state,
  to_state,
  reason_code,
  actor_id,
  changed_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11
)
RETURNING id
`, tenantID, conversationID, turnID, turnAction, transition.RequestID, transition.TraceID, transition.FromState, transition.ToState, reasonCode, transition.ActorID, transition.ChangedAt).Scan(&id)
	if err != nil {
		return err
	}
	transition.ID = id
	return nil
}

func (s *assistantConversationService) claimIdempotencyTx(ctx context.Context, tx pgx.Tx, key assistantIdempotencyKey, requestHash string) (assistantIdempotencyClaim, error) {
	expiresAt := time.Now().UTC().Add(assistantIdempotencyTTLDays * 24 * time.Hour)
	var marker int
	err := tx.QueryRow(ctx, `
INSERT INTO iam.assistant_idempotency (
  tenant_uuid,
  conversation_id,
  turn_id,
  turn_action,
  request_id,
  request_hash,
  status,
  created_at,
  expires_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  'pending',
  now(),
  $7
)
ON CONFLICT DO NOTHING
RETURNING 1
`, key.TenantID, key.ConversationID, key.TurnID, key.TurnAction, key.RequestID, requestHash, expiresAt).Scan(&marker)
	if err == nil {
		return assistantIdempotencyClaim{State: assistantIdempotencyClaimInserted}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return assistantIdempotencyClaim{}, err
	}

	var (
		storedHash  string
		status      string
		httpStatus  *int
		errorCode   *string
		responseRaw []byte
	)
	if err := tx.QueryRow(ctx, `
SELECT request_hash, status, http_status, error_code, response_body
FROM iam.assistant_idempotency
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
  AND turn_action = $4
  AND request_id = $5
FOR UPDATE
`, key.TenantID, key.ConversationID, key.TurnID, key.TurnAction, key.RequestID).Scan(&storedHash, &status, &httpStatus, &errorCode, &responseRaw); err != nil {
		return assistantIdempotencyClaim{}, err
	}
	if strings.TrimSpace(storedHash) != strings.TrimSpace(requestHash) {
		return assistantIdempotencyClaim{State: assistantIdempotencyClaimConflict}, nil
	}
	if status == "done" {
		claim := assistantIdempotencyClaim{State: assistantIdempotencyClaimDone, Body: responseRaw}
		if httpStatus != nil {
			claim.HTTPStatus = *httpStatus
		}
		if errorCode != nil {
			claim.ErrorCode = *errorCode
		}
		return claim, nil
	}
	return assistantIdempotencyClaim{State: assistantIdempotencyClaimInProgress}, nil
}

func (s *assistantConversationService) finalizeIdempotencySuccessTx(ctx context.Context, tx pgx.Tx, key assistantIdempotencyKey, conversation *assistantConversation) error {
	body, err := json.Marshal(conversation)
	if err != nil {
		return err
	}
	responseHash := assistantHashBytes(body)
	_, err = tx.Exec(ctx, `
UPDATE iam.assistant_idempotency
SET status = 'done',
    http_status = 200,
    error_code = NULL,
    response_body = $6::jsonb,
    response_hash = $7,
    finalized_at = now()
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
  AND turn_action = $4
  AND request_id = $5
`, key.TenantID, key.ConversationID, key.TurnID, key.TurnAction, key.RequestID, string(body), responseHash)
	return err
}

func (s *assistantConversationService) finalizeIdempotencyErrorTx(ctx context.Context, tx pgx.Tx, key assistantIdempotencyKey, failure error) error {
	status, code, ok := assistantIdempotencyErrorPayload(failure)
	if !ok {
		_, err := tx.Exec(ctx, `
DELETE FROM iam.assistant_idempotency
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
  AND turn_action = $4
  AND request_id = $5
  AND status = 'pending'
`, key.TenantID, key.ConversationID, key.TurnID, key.TurnAction, key.RequestID)
		return err
	}
	_, err := tx.Exec(ctx, `
UPDATE iam.assistant_idempotency
SET status = 'done',
    http_status = $6,
    error_code = $7,
    response_body = NULL,
    response_hash = $8,
    finalized_at = now()
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
  AND turn_action = $4
  AND request_id = $5
`, key.TenantID, key.ConversationID, key.TurnID, key.TurnAction, key.RequestID, status, code, assistantHashText(code))
	return err
}

func assistantIdempotencyErrorPayload(err error) (status int, code string, ok bool) {
	switch {
	case errors.Is(err, errAssistantConfirmationRequired):
		return http.StatusConflict, errAssistantConfirmationRequired.Error(), true
	case errors.Is(err, errAssistantConversationStateInvalid):
		return http.StatusConflict, errAssistantConversationStateInvalid.Error(), true
	case errors.Is(err, errAssistantPlanContractVersionMismatch):
		return http.StatusConflict, errAssistantPlanContractVersionMismatch.Error(), true
	case errors.Is(err, errAssistantCandidateNotFound):
		return http.StatusUnprocessableEntity, errAssistantCandidateNotFound.Error(), true
	case errors.Is(err, errAssistantAuthSnapshotExpired):
		return http.StatusForbidden, errAssistantAuthSnapshotExpired.Error(), true
	case errors.Is(err, errAssistantRoleDriftDetected):
		return http.StatusForbidden, errAssistantRoleDriftDetected.Error(), true
	case errors.Is(err, errAssistantUnsupportedIntent):
		return http.StatusUnprocessableEntity, errAssistantUnsupportedIntent.Error(), true
	case errors.Is(err, errAssistantServiceMissing):
		return http.StatusInternalServerError, errAssistantServiceMissing.Error(), true
	}
	if status, code, _, known := assistantResolveCommitError(err); known {
		return status, code, true
	}
	return 0, "", false
}

func (s *assistantConversationService) restoreIdempotentResult(claim assistantIdempotencyClaim) (*assistantConversation, error) {
	if claim.ErrorCode != "" {
		return nil, assistantErrorFromIdempotencyCode(claim.ErrorCode)
	}
	if len(claim.Body) == 0 || string(claim.Body) == "null" {
		return nil, errAssistantRequestInProgress
	}
	var conversation assistantConversation
	if err := json.Unmarshal(claim.Body, &conversation); err != nil {
		return nil, err
	}
	return &conversation, nil
}

func assistantErrorFromIdempotencyCode(code string) error {
	switch strings.TrimSpace(code) {
	case errAssistantConfirmationRequired.Error():
		return errAssistantConfirmationRequired
	case errAssistantConversationStateInvalid.Error():
		return errAssistantConversationStateInvalid
	case errAssistantPlanContractVersionMismatch.Error():
		return errAssistantPlanContractVersionMismatch
	case errAssistantCandidateNotFound.Error():
		return errAssistantCandidateNotFound
	case errAssistantAuthSnapshotExpired.Error():
		return errAssistantAuthSnapshotExpired
	case errAssistantRoleDriftDetected.Error():
		return errAssistantRoleDriftDetected
	case errAssistantUnsupportedIntent.Error():
		return errAssistantUnsupportedIntent
	case errAssistantServiceMissing.Error():
		return errAssistantServiceMissing
	case "assistant_idempotency_key_conflict", errAssistantIdempotencyKeyConflict.Error():
		return errAssistantIdempotencyKeyConflict
	case "assistant_request_in_progress", errAssistantRequestInProgress.Error():
		return errAssistantRequestInProgress
	default:
		return errors.New(code)
	}
}

func (s *assistantConversationService) cacheConversation(conversation *assistantConversation) {
	if conversation == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[conversation.ConversationID] = cloneConversation(conversation)
	if conversation.ActorID != "" {
		ids := s.byActorID[conversation.ActorID]
		for _, id := range ids {
			if id == conversation.ConversationID {
				return
			}
		}
		s.byActorID[conversation.ActorID] = append(s.byActorID[conversation.ActorID], conversation.ConversationID)
	}
}

func (s *assistantConversationService) getCachedConversation(conversationID string) (*assistantConversation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.byID[conversationID]
	if !ok {
		return nil, false
	}
	return cloneConversation(conversation), true
}

func assistantEncodeConversationCursor(cursor assistantConversationCursor, tenantID string, actorID string) string {
	if cursor.UpdatedAt.IsZero() || strings.TrimSpace(cursor.ConversationID) == "" {
		return ""
	}
	base := strings.Join([]string{
		strings.TrimSpace(tenantID),
		strings.TrimSpace(actorID),
		cursor.UpdatedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(cursor.ConversationID),
	}, "|")
	signature := assistantHashText(base + "|" + assistantConversationCursorSalt)
	raw := base + "|" + signature
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func assistantDecodeConversationCursor(encoded string, tenantID string, actorID string) (*assistantConversationCursor, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, errAssistantConversationCursorInvalid
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 5 {
		return nil, errAssistantConversationCursorInvalid
	}
	if parts[0] != strings.TrimSpace(tenantID) || parts[1] != strings.TrimSpace(actorID) {
		return nil, errAssistantConversationCursorInvalid
	}
	signatureBase := strings.Join(parts[:4], "|")
	expected := assistantHashText(signatureBase + "|" + assistantConversationCursorSalt)
	if parts[4] != expected {
		return nil, errAssistantConversationCursorInvalid
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, parts[2])
	if err != nil {
		return nil, errAssistantConversationCursorInvalid
	}
	conversationID := strings.TrimSpace(parts[3])
	if conversationID == "" {
		return nil, errAssistantConversationCursorInvalid
	}
	return &assistantConversationCursor{
		UpdatedAt:      updatedAt.UTC(),
		ConversationID: conversationID,
	}, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func assistantLookupTurn(conversation *assistantConversation, turnID string) *assistantTurn {
	if conversation == nil {
		return nil
	}
	for _, turn := range conversation.Turns {
		if turn != nil && turn.TurnID == turnID {
			return turn
		}
	}
	return nil
}

func assistantHashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func assistantHashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func newUUIDString() string {
	return uuid.NewString()
}
