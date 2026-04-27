package cubebox

import (
	"context"
	"errors"
	"strconv"
)

type PreTurnPreparation struct {
	Sequence           int
	TurnIDs            TurnIDs
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
	ids := TurnIDsForSequence(sequence)
	providerPromptView := buildPromptViewForProvider(nil, canonicalContext, request.Prompt)
	if sequence > 1 {
		preparedPromptView, err := store.PrepareConversationPromptView(ctx, request.TenantID, request.PrincipalID, request.ConversationID, canonicalContext, "pre_turn_auto")
		if err != nil && !errors.Is(err, ErrConversationNotFound) {
			return PreTurnPreparation{}, err
		}
		if preparedPromptView.NextSequence > sequence {
			sequence = preparedPromptView.NextSequence
		}
		ids = TurnIDsForSequence(sequence)
		providerPromptView = promptViewForProvider(preparedPromptView.PromptView, canonicalContext, request.Prompt)
	}
	return PreTurnPreparation{
		Sequence:           sequence,
		TurnIDs:            ids,
		CanonicalContext:   canonicalContext,
		ProviderPromptView: providerPromptView,
	}, nil
}

func TurnIDsForSequence(sequence int) TurnIDs {
	if sequence <= 0 {
		sequence = 1
	}
	return TurnIDs{
		TurnID:             formatSequenceID("turn", sequence),
		UserMessageID:      formatSequenceID("msg_user", sequence),
		AssistantMessageID: formatSequenceID("msg_agent", sequence),
	}
}

func formatSequenceID(prefix string, sequence int) string {
	return prefix + "_seq_" + strconv.Itoa(sequence)
}
