package server

import (
	"context"
	"strings"
)

func assistantCommitTurnSyncForTest(s *assistantConversationService, ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*assistantConversation, error) {
	if s == nil {
		return nil, errAssistantTaskWorkflowUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok := s.byID[conversationID]
	if !ok {
		return nil, errAssistantConversationNotFound
	}
	if conversation == nil {
		return nil, errAssistantConversationCorrupted
	}
	if conversation.TenantID != tenantID {
		return nil, errAssistantTenantMismatch
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
	result, applyErr := s.applyCommitTurn(ctx, conversation, turn, principal, tenantID)
	assistantRefreshConversationDerivedFields(conversation)
	if applyErr != nil {
		return nil, applyErr
	}
	if result.Transition == nil {
		return cloneConversation(conversation), nil
	}
	return cloneConversation(conversation), nil
}
