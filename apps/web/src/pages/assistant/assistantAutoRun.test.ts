import { describe, expect, it } from 'vitest'
import {
  composeCreateOrgUnitPrompt,
  extractIntentDraftFromText,
  formatCandidatePrompt,
  hasCompleteCreateIntent,
  isExecutionConfirmationText,
  looksLikeCreateOrgUnitRequest,
  mergeIntentDraft,
  resolveCandidateFromInput
} from './assistantAutoRun'

describe('assistantAutoRun', () => {
  it('extracts parent/name/date from mixed CN input', () => {
    const draft = extractIntentDraftFromText('在 AI治理办公室 之下，新建一个名为 人力资源部2 的部门，生效日期 2026年1月1日')
    expect(draft).toEqual({
      parent_ref_text: 'AI治理办公室',
      entity_name: '人力资源部2',
      effective_date: '2026-01-01'
    })
  })

  it('supports short form 在xx下 + 新建xxx', () => {
    const draft = extractIntentDraftFromText('在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01')
    expect(draft).toEqual({
      parent_ref_text: 'AI治理办公室',
      entity_name: '人力资源部2',
      effective_date: '2026-01-01'
    })
  })

  it('merges supplement draft and composes canonical prompt', () => {
    const merged = mergeIntentDraft(
      {
        parent_ref_text: 'AI治理办公室',
        entity_name: '人力资源部2'
      },
      {
        effective_date: '2026-01-01'
      }
    )
    expect(hasCompleteCreateIntent(merged)).toBe(true)
    expect(composeCreateOrgUnitPrompt(merged)).toBe(
      '在AI治理办公室之下，新建一个名为人力资源部2的部门，成立日期是2026-01-01。'
    )
  })

  it('recognizes create request and execution confirmation intent', () => {
    expect(looksLikeCreateOrgUnitRequest('在鲜花组织之下新建一个名为运营部的部门')).toBe(true)
    expect(looksLikeCreateOrgUnitRequest('确认执行')).toBe(false)
    expect(isExecutionConfirmationText('请确认执行')).toBe(true)
    expect(isExecutionConfirmationText('ok')).toBe(true)
    expect(isExecutionConfirmationText('继续聊聊')).toBe(false)
    expect(isExecutionConfirmationText('我们继续执行排查这个问题')).toBe(false)
  })

  it('resolves candidate by code/name/path and index', () => {
    const candidates = [
      { candidate_id: 'FLOWER-A', candidate_code: 'FLOWER-A', name: '鲜花组织', path: '/鲜花组织/华东' },
      { candidate_id: 'FLOWER-B', candidate_code: 'FLOWER-B', name: '鲜花组织', path: '/鲜花组织/华南' }
    ]

    expect(resolveCandidateFromInput('请选择 FLOWER-B', candidates)).toBe('FLOWER-B')
    expect(resolveCandidateFromInput('选第1个', candidates)).toBe('FLOWER-A')
    expect(resolveCandidateFromInput('/鲜花组织/华南', candidates)).toBe('FLOWER-B')
  })

  it('formats candidate prompt text', () => {
    const message = formatCandidatePrompt([
      { candidate_id: 'FLOWER-A', candidate_code: 'FLOWER-A', name: '鲜花组织', path: '/鲜花组织/华东' }
    ])
    expect(message).toContain('1. 鲜花组织 / FLOWER-A (/鲜花组织/华东)')
  })
})
