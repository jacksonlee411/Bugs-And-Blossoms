import type { AssistantRenderReplyRequest } from '../../api/assistant'
import { extractIntentDraftFromText, looksLikeCreateOrgUnitRequest } from './assistantAutoRun'
import { formatMissingFieldMessageText } from './assistantDialogFlow'

function normalized(value: string | undefined): string {
  return (value ?? '').trim()
}

function deriveMissingFieldMessages(userInput: string): string[] {
  const draft = extractIntentDraftFromText(userInput)
  const messages: string[] = []
  if (normalized(draft.parent_ref_text).length === 0) {
    messages.push('请补充上级组织名称（例如：AI治理办公室）')
  }
  if (normalized(draft.entity_name).length === 0) {
    messages.push('请补充部门名称（例如：人力资源部2）')
  }
  if (normalized(draft.effective_date).length === 0) {
    messages.push('请补充生效日期（YYYY-MM-DD）')
  }
  return messages
}

export function buildAssistantTurnCreationFailureReplyPayload(
  userInput: string,
  errorCode: string,
  errorMessage: string
): AssistantRenderReplyRequest {
  if (looksLikeCreateOrgUnitRequest(userInput)) {
    const missingFieldMessages = deriveMissingFieldMessages(userInput)
    if (missingFieldMessages.length > 0) {
      return {
        stage: 'missing_fields',
        kind: 'warning',
        outcome: 'failure',
        error_code: normalized(errorCode),
        error_message: normalized(errorMessage),
        locale: 'zh',
        fallback_text: formatMissingFieldMessageText(missingFieldMessages),
        allow_missing_turn: true
      }
    }
  }
  return {
    stage: 'commit_failed',
    kind: 'error',
    outcome: 'failure',
    error_code: normalized(errorCode),
    error_message: normalized(errorMessage),
    locale: 'zh',
    allow_missing_turn: true
  }
}
