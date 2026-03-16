package server

import (
	"encoding/json"
	"strings"
)

type assistantSemanticContextAssemblerInput struct {
	UserInput   string
	PendingTurn *assistantTurn
}

type assistantSemanticResolutionPromptEnvelope struct {
	Context          assistantSemanticPromptEnvelope    `json:"context"`
	SemanticState    assistantConversationSemanticState `json:"semantic_state"`
	RetrievalResults []assistantSemanticRetrievalResult `json:"retrieval_results,omitempty"`
	BoundaryNote     string                             `json:"boundary_note"`
}

func assistantAssembleSemanticContext(input assistantSemanticContextAssemblerInput) assistantSemanticPromptEnvelope {
	envelope := assistantSemanticPromptEnvelope{
		CurrentUserInput: strings.TrimSpace(input.UserInput),
		AllowedActions:   assistantSemanticPromptActions(),
	}
	if pending := assistantSemanticPromptPendingTurn(input.PendingTurn); pending != nil {
		envelope.PendingTurn = pending
	}
	return envelope
}

func assistantBuildSemanticResolutionPrompt(contextEnvelope assistantSemanticPromptEnvelope, state assistantConversationSemanticState) string {
	envelope := assistantSemanticResolutionPromptEnvelope{
		Context:          contextEnvelope,
		SemanticState:    state,
		RetrievalResults: assistantNormalizeSemanticRetrievalResults(state.RetrievalResults),
		BoundaryNote:     "confirm/commit 仍由本地 dry-run、风险控制、鉴权、确认窗口与 One Door 执行边界决定。",
	}
	payload, _ := json.Marshal(envelope)
	return string(payload)
}
