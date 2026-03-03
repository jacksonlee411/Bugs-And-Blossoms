import { describe, expect, it } from 'vitest'
import type { AssistantTurn } from '../../api/assistant'
import { deriveAssistantActionState } from './assistantUiState'

function makeTurn(overrides: Partial<AssistantTurn>): AssistantTurn {
  return {
    turn_id: 'turn_1',
    user_input: 'input',
    state: 'validated',
    risk_tier: 'high',
    request_id: 'request_1',
    trace_id: 'trace_1',
    policy_version: 'v1',
    composition_version: 'v1',
    mapping_version: 'v1',
    intent: { action: 'create_orgunit' },
    ambiguity_count: 0,
    confidence: 0.8,
    candidates: [],
    plan: {
      title: 'title',
      capability_key: 'org.orgunit_create.field_policy',
      summary: 'summary'
    },
    dry_run: {
      explain: 'explain',
      diff: []
    },
    ...overrides
  }
}

describe('deriveAssistantActionState', () => {
  it('disables actions when conversation is unavailable', () => {
    const result = deriveAssistantActionState({
      hasConversation: false,
      loading: false,
      selectedCandidateID: '',
      turn: null
    })

    expect(result.canRegenerate).toBe(false)
    expect(result.canConfirm).toBe(false)
    expect(result.canCommit).toBe(false)
    expect(result.showRequiredFieldBlocker).toBe(false)
  })

  it('blocks confirm when candidate is ambiguous and not selected', () => {
    const result = deriveAssistantActionState({
      hasConversation: true,
      loading: false,
      selectedCandidateID: '',
      turn: makeTurn({
        candidates: [
          { candidate_id: 'A', candidate_code: 'A', name: 'A', path: 'A', as_of: '2026-01-01', is_active: true, match_score: 0.9 },
          { candidate_id: 'B', candidate_code: 'B', name: 'B', path: 'B', as_of: '2026-01-01', is_active: true, match_score: 0.8 }
        ]
      })
    })

    expect(result.canConfirm).toBe(false)
    expect(result.canCommit).toBe(false)
    expect(result.showRiskBlocker).toBe(true)
    expect(result.showCandidateBlocker).toBe(true)
    expect(result.showRequiredFieldBlocker).toBe(false)
  })

  it('allows confirm after candidate selection in validated state', () => {
    const result = deriveAssistantActionState({
      hasConversation: true,
      loading: false,
      selectedCandidateID: 'A',
      turn: makeTurn({
        candidates: [
          { candidate_id: 'A', candidate_code: 'A', name: 'A', path: 'A', as_of: '2026-01-01', is_active: true, match_score: 0.9 },
          { candidate_id: 'B', candidate_code: 'B', name: 'B', path: 'B', as_of: '2026-01-01', is_active: true, match_score: 0.8 }
        ]
      })
    })

    expect(result.canConfirm).toBe(true)
    expect(result.canCommit).toBe(false)
    expect(result.showRequiredFieldBlocker).toBe(false)
  })

  it('allows commit only when confirmed and candidate is resolved', () => {
    const result = deriveAssistantActionState({
      hasConversation: true,
      loading: false,
      selectedCandidateID: '',
      turn: makeTurn({
        state: 'confirmed',
        resolved_candidate_id: 'A',
        candidates: [
          { candidate_id: 'A', candidate_code: 'A', name: 'A', path: 'A', as_of: '2026-01-01', is_active: true, match_score: 0.9 },
          { candidate_id: 'B', candidate_code: 'B', name: 'B', path: 'B', as_of: '2026-01-01', is_active: true, match_score: 0.8 }
        ]
      })
    })

    expect(result.canConfirm).toBe(false)
    expect(result.canCommit).toBe(true)
    expect(result.showRiskBlocker).toBe(false)
    expect(result.showCandidateBlocker).toBe(false)
    expect(result.showRequiredFieldBlocker).toBe(false)
  })

  it('treats terminal states as non-actionable', () => {
    const result = deriveAssistantActionState({
      hasConversation: true,
      loading: false,
      selectedCandidateID: '',
      turn: makeTurn({ state: 'committed' })
    })

    expect(result.canConfirm).toBe(false)
    expect(result.canCommit).toBe(false)
    expect(result.showRiskBlocker).toBe(false)
    expect(result.showRequiredFieldBlocker).toBe(false)
  })

  it('blocks confirm and commit when required fields are missing', () => {
    const result = deriveAssistantActionState({
      hasConversation: true,
      loading: false,
      selectedCandidateID: '',
      turn: makeTurn({
        dry_run: {
          explain: '信息不完整',
          diff: [],
          validation_errors: ['missing_parent_ref_text', 'missing_effective_date']
        }
      })
    })

    expect(result.canConfirm).toBe(false)
    expect(result.canCommit).toBe(false)
    expect(result.showRequiredFieldBlocker).toBe(true)
  })
})
