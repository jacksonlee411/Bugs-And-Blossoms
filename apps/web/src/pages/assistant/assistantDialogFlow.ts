import type { AssistantTurn } from '../../api/assistant'
import type { AssistantCandidateOption } from './assistantAutoRun'

export type DialogFlowPhase =
  | 'idle'
  | 'await_missing_fields'
  | 'await_candidate_pick'
  | 'await_candidate_confirm'
  | 'await_commit_confirm'
  | 'committing'
  | 'committed'
  | 'failed'

export type DialogMessageKind = 'info' | 'warning' | 'success' | 'error'

export type DialogMessageStage =
  | 'draft'
  | 'missing_fields'
  | 'candidate_list'
  | 'candidate_confirm'
  | 'commit_result'
  | 'commit_failed'

export interface DialogFlowState {
  phase: DialogFlowPhase
  conversation_id: string
  turn_id: string
  pending_draft_summary: string
  missing_fields: string[]
  candidates: AssistantCandidateOption[]
  selected_candidate_id: string
}

export interface DialogTurnAnalysis {
  phase: DialogFlowPhase
  missing_field_messages: string[]
  candidates: AssistantCandidateOption[]
  draft_summary: string
}

const missingFieldGuidanceByCode: Record<string, string> = {
  missing_parent_ref_text: '请补充上级组织名称（例如：AI治理办公室）',
  missing_entity_name: '请补充部门名称（例如：人力资源部2）',
  missing_effective_date: '请补充生效日期（YYYY-MM-DD）',
  invalid_effective_date_format: '生效日期格式不正确，请使用 YYYY-MM-DD'
}

function normalized(value: string | undefined): string {
  return (value ?? '').trim()
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter((value) => value.trim().length > 0)))
}

function candidateLabel(candidate: AssistantCandidateOption): string {
  const code = normalized(candidate.candidate_code)
  const name = normalized(candidate.name)
  return [name || candidate.candidate_id, code].filter((value) => value.length > 0).join(' / ')
}

function resolveValidationCodes(turn: AssistantTurn | null): string[] {
  const rawCodes = Array.isArray(turn?.dry_run?.validation_errors) ? turn?.dry_run?.validation_errors : []
  return rawCodes
    .map((item) => normalized(item))
    .filter((item) => item.length > 0)
}

function hasMissingFieldError(codes: string[]): boolean {
  return codes.some((code) =>
    ['missing_parent_ref_text', 'missing_entity_name', 'missing_effective_date', 'invalid_effective_date_format'].includes(code)
  )
}

function hasCandidatePending(turn: AssistantTurn | null): boolean {
  if (!turn) {
    return false
  }
  return turn.ambiguity_count > 1 && normalized(turn.resolved_candidate_id).length === 0
}

export function createDialogFlowState(): DialogFlowState {
  return {
    phase: 'idle',
    conversation_id: '',
    turn_id: '',
    pending_draft_summary: '',
    missing_fields: [],
    candidates: [],
    selected_candidate_id: ''
  }
}

export function resetDialogFlowForConversation(conversationID: string, turnID: string): DialogFlowState {
  return {
    ...createDialogFlowState(),
    conversation_id: normalized(conversationID),
    turn_id: normalized(turnID)
  }
}

export function withDialogPhase(
  base: DialogFlowState,
  phase: DialogFlowPhase,
  patch?: Partial<Omit<DialogFlowState, 'phase'>>
): DialogFlowState {
  return {
    ...base,
    phase,
    ...(patch ?? {})
  }
}

export function formatMissingFieldMessagesFromCodes(codes: string[]): string[] {
  return unique(codes.map((code) => missingFieldGuidanceByCode[normalized(code)] ?? normalized(code)))
}

export function formatMissingFieldMessageText(messages: string[]): string {
  if (messages.length === 0) {
    return '信息不完整，请补充缺失字段后继续。'
  }
  return `信息不完整，请补充后继续：${messages.join('；')}`
}

export function formatDraftSummary(turn: AssistantTurn): string {
  const parent = normalized(turn.intent.parent_ref_text) || '-'
  const entity = normalized(turn.intent.entity_name) || '-'
  const effectiveDate = normalized(turn.intent.effective_date) || '-'
  const resolvedCandidateID = normalized(turn.resolved_candidate_id)
  const candidateText = resolvedCandidateID.length > 0 ? resolvedCandidateID : '待确认'
  return `已生成提交草案：\n- 上级组织：${parent}\n- 新组织：${entity}\n- 生效日期：${effectiveDate}\n- 候选：${candidateText}\n请回复“确认执行”后提交。`
}

export function formatCandidateConfirmMessage(candidates: AssistantCandidateOption[], candidateID: string): string {
  const selectedID = normalized(candidateID)
  const selected = candidates.find((candidate) => candidate.candidate_id === selectedID)
  if (!selected) {
    return '候选已选择。请回复“确认执行”以确认该候选并继续提交。'
  }
  const path = normalized(selected.path)
  const label = candidateLabel(selected)
  if (path.length > 0) {
    return `已选择候选：${label}（${path}）。请回复“确认执行”以确认该候选并提交。`
  }
  return `已选择候选：${label}。请回复“确认执行”以确认该候选并提交。`
}

export function formatCommitSuccessMessage(turn: AssistantTurn): string {
  if (!turn.commit_result) {
    return '提交成功。'
  }
  return `提交成功：org_code=${turn.commit_result.org_code} / parent=${turn.commit_result.parent_org_code} / effective_date=${turn.commit_result.effective_date}`
}

export function analyzeTurnForDialog(turn: AssistantTurn | null): DialogTurnAnalysis {
  if (!turn) {
    return {
      phase: 'idle',
      missing_field_messages: [],
      candidates: [],
      draft_summary: ''
    }
  }
  const codes = resolveValidationCodes(turn)
  if (hasMissingFieldError(codes)) {
    return {
      phase: 'await_missing_fields',
      missing_field_messages: formatMissingFieldMessagesFromCodes(codes),
      candidates: [],
      draft_summary: ''
    }
  }
  if (hasCandidatePending(turn)) {
    return {
      phase: 'await_candidate_pick',
      missing_field_messages: [],
      candidates: turn.candidates ?? [],
      draft_summary: ''
    }
  }
  if (normalized(turn.state) === 'committed' || turn.commit_result) {
    return {
      phase: 'committed',
      missing_field_messages: [],
      candidates: [],
      draft_summary: ''
    }
  }
  if (normalized(turn.state) === 'validated' || normalized(turn.state) === 'confirmed') {
    return {
      phase: 'await_commit_confirm',
      missing_field_messages: [],
      candidates: turn.candidates ?? [],
      draft_summary: formatDraftSummary(turn)
    }
  }
  return {
    phase: 'idle',
    missing_field_messages: [],
    candidates: turn.candidates ?? [],
    draft_summary: ''
  }
}
