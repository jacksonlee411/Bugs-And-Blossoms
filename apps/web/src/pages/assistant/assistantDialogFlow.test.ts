import { describe, expect, it } from 'vitest'
import {
  analyzeTurnForDialog,
  createDialogFlowState,
  formatCandidateConfirmMessage,
  formatCommitSuccessMessage,
  formatMissingFieldMessageText,
  withDialogPhase
} from './assistantDialogFlow'

function makeTurn(overrides: Record<string, unknown> = {}) {
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
    intent: {
      action: 'create_orgunit',
      parent_ref_text: 'AI治理办公室',
      entity_name: '人力资源部2',
      effective_date: '2026-01-01'
    },
    ambiguity_count: 1,
    confidence: 0.9,
    candidates: [],
    resolved_candidate_id: 'AI-GOV-A',
    plan: {
      title: '创建组织计划',
      capability_key: 'org.orgunit_create.field_policy',
      summary: '在指定父组织下创建部门'
    },
    dry_run: {
      explain: '计划已生成，可提交',
      diff: [],
      validation_errors: []
    },
    ...overrides
  }
}

describe('assistantDialogFlow', () => {
  it('creates default state and updates phase', () => {
    const state = createDialogFlowState()
    expect(state.phase).toBe('idle')
    const next = withDialogPhase(state, 'await_commit_confirm', { turn_id: 'turn_2' })
    expect(next.phase).toBe('await_commit_confirm')
    expect(next.turn_id).toBe('turn_2')
  })

  it('detects missing-field phase from validation errors', () => {
    const analysis = analyzeTurnForDialog(
      makeTurn({
        dry_run: {
          explain: '缺少生效日期',
          diff: [],
          validation_errors: ['missing_effective_date']
        }
      }) as never
    )
    expect(analysis.phase).toBe('await_missing_fields')
    expect(formatMissingFieldMessageText(analysis.missing_field_messages)).toContain('请补充生效日期')
  })

  it('treats parent candidate not found as missing-field phase', () => {
    const analysis = analyzeTurnForDialog(
      makeTurn({
        ambiguity_count: 0,
        resolved_candidate_id: '',
        candidates: [],
        dry_run: {
          explain: '未找到匹配的上级组织，请补充更准确的名称或编码后继续。',
          diff: [],
          validation_errors: ['parent_candidate_not_found']
        }
      }) as never
    )
    expect(analysis.phase).toBe('await_missing_fields')
    expect(formatMissingFieldMessageText(analysis.missing_field_messages)).toContain('未找到匹配的上级组织')
  })

  it('detects candidate pick phase when ambiguity exists', () => {
    const analysis = analyzeTurnForDialog(
      makeTurn({
        ambiguity_count: 2,
        resolved_candidate_id: '',
        candidates: [
          { candidate_id: 'A', candidate_code: 'A', name: '共享服务中心', path: '/共享/一部' },
          { candidate_id: 'B', candidate_code: 'B', name: '共享服务中心', path: '/共享/二部' }
        ]
      }) as never
    )
    expect(analysis.phase).toBe('await_candidate_pick')
    expect(analysis.candidates).toHaveLength(2)
  })

  it('detects commit confirm phase and builds draft summary', () => {
    const analysis = analyzeTurnForDialog(makeTurn() as never)
    expect(analysis.phase).toBe('await_commit_confirm')
    expect(analysis.draft_summary).toContain('请回复“确认执行”后提交')
  })

  it('formats candidate confirm and commit success message', () => {
    const candidateMessage = formatCandidateConfirmMessage(
      [{ candidate_id: 'SSC-2', candidate_code: 'SSC-2', name: '共享服务中心', path: '/集团/共享服务中心/二部' }],
      'SSC-2'
    )
    expect(candidateMessage).toContain('SSC-2')
    expect(candidateMessage).toContain('确认该候选并提交')

    const success = formatCommitSuccessMessage(
      makeTurn({
        state: 'committed',
        commit_result: {
          org_code: 'HR2',
          parent_org_code: 'SSC-2',
          effective_date: '2026-01-01',
          event_type: 'CREATE',
          event_uuid: 'evt-1'
        }
      }) as never
    )
    expect(success).toContain('org_code=HR2')
  })
})

