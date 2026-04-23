package cubebox

import (
	"context"
	"errors"
)

type PreTurnPreparation struct {
	Sequence           int
	CanonicalContext   CanonicalContext
	ProviderPromptView []PromptItem
}

func PrepareTurnStream(
	ctx context.Context,
	store StreamAppendStore,
	request GatewayStreamRequest,
	canonicalContext CanonicalContext,
) (PreTurnPreparation, error) {
	sequence := request.NextSequence
	if sequence <= 0 {
		sequence = 1
	}
	providerPromptView := BuildPromptViewWithCompaction(nil, canonicalContext, request.Prompt).PromptView
	if sequence > 1 {
		compactPayload, err := store.CompactConversation(ctx, request.TenantID, request.PrincipalID, request.ConversationID, canonicalContext, "pre_turn_auto")
		if err != nil && !errors.Is(err, ErrConversationNotFound) {
			return PreTurnPreparation{}, err
		}
		if compactPayload.NextSequence > sequence {
			sequence = compactPayload.NextSequence
		}
		providerPromptView = promptViewForProvider(compactPayload.PromptView, canonicalContext, request.Prompt)
	}
	return PreTurnPreparation{
		Sequence:           sequence,
		CanonicalContext:   canonicalContext,
		ProviderPromptView: providerPromptView,
	}, nil
}
