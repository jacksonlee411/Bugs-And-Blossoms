import type { AssistantTurn } from '../../api/assistant'

const terminalStates = new Set(['committed', 'canceled', 'expired'])

export interface AssistantActionState {
  canRegenerate: boolean
  canConfirm: boolean
  canCommit: boolean
  showRiskBlocker: boolean
  showCandidateBlocker: boolean
  showRequiredFieldBlocker: boolean
}

interface AssistantActionInput {
  hasConversation: boolean
  loading: boolean
  selectedCandidateID: string
  turn: AssistantTurn | null
}

function normalized(value: string | undefined): string {
  return (value ?? '').trim()
}

function requiredFieldValidationError(code: string): boolean {
  return code === 'missing_parent_ref_text' ||
    code === 'missing_entity_name' ||
    code === 'missing_effective_date' ||
    code === 'invalid_effective_date_format' ||
    code === 'FIELD_REQUIRED_VALUE_MISSING' ||
    code === 'PATCH_FIELD_NOT_ALLOWED'
}

export function deriveAssistantActionState(input: AssistantActionInput): AssistantActionState {
  const canRegenerate = input.hasConversation && !input.loading
  if (!input.turn) {
    return {
      canRegenerate,
      canConfirm: false,
      canCommit: false,
      showRiskBlocker: false,
      showCandidateBlocker: false,
      showRequiredFieldBlocker: false
    }
  }

  const state = normalized(input.turn.state)
  const riskTier = normalized(input.turn.risk_tier)
  const candidateCount = Array.isArray(input.turn.candidates) ? input.turn.candidates.length : 0
  const hasResolvedCandidate = normalized(input.turn.resolved_candidate_id).length > 0
  const hasSelectedCandidate = normalized(input.selectedCandidateID).length > 0
  const hasCandidateAmbiguity = candidateCount > 1
  const isTerminal = terminalStates.has(state)
  const isValidated = state === 'validated'
  const isConfirmed = state === 'confirmed'
  const validationErrors = Array.isArray(input.turn.dry_run?.validation_errors) ? input.turn.dry_run.validation_errors.map((item) => normalized(item)) : []
  const hasRequiredFieldBlocker = validationErrors.some(requiredFieldValidationError)

  let canConfirm = isValidated && !isTerminal
  if (canConfirm && hasRequiredFieldBlocker) {
    canConfirm = false
  }
  if (canConfirm && hasCandidateAmbiguity && !hasResolvedCandidate && !hasSelectedCandidate) {
    canConfirm = false
  }
  if (input.loading || !input.hasConversation) {
    canConfirm = false
  }

  let canCommit = isConfirmed && !isTerminal
  if (canCommit && hasRequiredFieldBlocker) {
    canCommit = false
  }
  if (canCommit && hasCandidateAmbiguity && !hasResolvedCandidate) {
    canCommit = false
  }
  if (canCommit && riskTier === 'high' && !isConfirmed) {
    canCommit = false
  }
  if (input.loading || !input.hasConversation) {
    canCommit = false
  }

  const showRiskBlocker = riskTier === 'high' && !isConfirmed && !isTerminal
  const showCandidateBlocker = hasCandidateAmbiguity && !isConfirmed && !isTerminal
  const showRequiredFieldBlocker = hasRequiredFieldBlocker && !isTerminal

  return {
    canRegenerate,
    canConfirm,
    canCommit,
    showRiskBlocker,
    showCandidateBlocker,
    showRequiredFieldBlocker
  }
}
