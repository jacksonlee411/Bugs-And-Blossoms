import type { TMessage, TSubmission } from 'librechat-data-provider';

export const assistantFormalEntryPrefix = '/app/assistant/librechat';
const assistantFormalConversationStorageKey = 'bugs_and_blossoms.assistant_formal.conversation_id';

export interface AssistantFormalCandidate {
  candidate_id: string;
  candidate_code: string;
  name: string;
  path: string;
  as_of: string;
  is_active: boolean;
  match_score: number;
}

export interface AssistantFormalCommitResult {
  org_code: string;
  parent_org_code: string;
  effective_date: string;
  event_type: string;
  event_uuid: string;
}

export interface AssistantFormalCommitReply {
  outcome?: 'success' | 'failure';
  stage?: string;
  kind?: string;
  text?: string;
  error_code?: string;
  next_action?: string;
}

export interface AssistantFormalReply {
  text: string;
  kind: string;
  stage: string;
  reply_model_name?: string;
  reply_prompt_version?: string;
  reply_source?: string;
  used_fallback?: boolean;
  conversation_id?: string;
  turn_id?: string;
}

export interface AssistantFormalTurn {
  turn_id: string;
  state: string;
  phase?: string;
  pending_draft_summary?: string;
  missing_fields?: string[];
  candidates?: AssistantFormalCandidate[];
  selected_candidate_id?: string;
  commit_result?: AssistantFormalCommitResult;
  commit_reply?: AssistantFormalCommitReply;
  error_code?: string;
  reply_nlg?: AssistantFormalReply;
}

export interface AssistantFormalConversation {
  conversation_id: string;
  turns: AssistantFormalTurn[];
}

export interface AssistantFormalPayload {
  kind: 'assistant_formal';
  backendConversationId: string;
  turnId: string;
  state: string;
  phase?: string;
  pendingDraftSummary?: string;
  missingFields: string[];
  candidates: AssistantFormalCandidate[];
  selectedCandidateId?: string;
  commitResult?: AssistantFormalCommitResult;
  commitReply?: AssistantFormalCommitReply;
  errorCode?: string;
  reply?: AssistantFormalReply;
}

export type AssistantFormalMessage = TMessage & {
  assistantFormalPayload?: AssistantFormalPayload;
  assistantFormalPending?: boolean;
};

export function isFormalAssistantPath(pathname?: string) {
  const resolvedPathname =
    pathname ?? (typeof window !== 'undefined' ? window.location.pathname : '');
  return resolvedPathname.startsWith(assistantFormalEntryPrefix);
}

export function detectAssistantFormalLocale() {
  const language =
    (typeof document !== 'undefined' && document.documentElement.lang) ||
    (typeof navigator !== 'undefined' && navigator.language) ||
    'zh';
  return language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

export function latestAssistantFormalTurn(
  conversation: AssistantFormalConversation,
): AssistantFormalTurn | undefined {
  if (!Array.isArray(conversation.turns) || conversation.turns.length === 0) {
    return undefined;
  }
  return conversation.turns[conversation.turns.length - 1];
}

export function shouldResetAssistantFormalConversation(submission: TSubmission | null) {
  return !submission || !Array.isArray(submission.messages) || submission.messages.length === 0;
}

export function getStoredAssistantFormalConversationId() {
  if (typeof window === 'undefined') {
    return '';
  }
  return window.localStorage.getItem(assistantFormalConversationStorageKey) ?? '';
}

export function setStoredAssistantFormalConversationId(conversationId: string) {
  if (typeof window === 'undefined') {
    return;
  }
  const value = conversationId.trim();
  if (value.length === 0) {
    window.localStorage.removeItem(assistantFormalConversationStorageKey);
    return;
  }
  window.localStorage.setItem(assistantFormalConversationStorageKey, value);
}

export function clearStoredAssistantFormalConversationId() {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.removeItem(assistantFormalConversationStorageKey);
}

export function buildAssistantFormalPayload(
  conversation: AssistantFormalConversation,
  turn: AssistantFormalTurn,
  reply?: AssistantFormalReply,
): AssistantFormalPayload {
  return {
    kind: 'assistant_formal',
    backendConversationId: conversation.conversation_id,
    turnId: turn.turn_id,
    state: turn.state,
    phase: turn.phase,
    pendingDraftSummary: turn.pending_draft_summary,
    missingFields: turn.missing_fields ?? [],
    candidates: turn.candidates ?? [],
    selectedCandidateId: turn.selected_candidate_id,
    commitResult: turn.commit_result,
    commitReply: turn.commit_reply,
    errorCode: turn.error_code,
    reply: reply ?? turn.reply_nlg,
  };
}

export function buildAssistantFormalFailurePayload(
  basePayload: Partial<AssistantFormalPayload>,
  error: { code?: string; message?: string } | null | undefined,
): AssistantFormalPayload {
  const errorCode = error?.code?.trim() || basePayload.errorCode || 'assistant_request_failed';
  const fallbackText =
    error?.message?.trim() ||
    (detectAssistantFormalLocale() === 'en'
      ? 'The request could not be completed. Please adjust the input and try again.'
      : '本次请求未能完成，请根据提示调整后重试。');
  return {
    kind: 'assistant_formal',
    backendConversationId: basePayload.backendConversationId ?? '',
    turnId: basePayload.turnId ?? '',
    state: basePayload.state ?? 'failed',
    phase: basePayload.phase,
    pendingDraftSummary: basePayload.pendingDraftSummary,
    missingFields: basePayload.missingFields ?? [],
    candidates: basePayload.candidates ?? [],
    selectedCandidateId: basePayload.selectedCandidateId,
    commitResult: basePayload.commitResult,
    commitReply: basePayload.commitReply,
    errorCode,
    reply: {
      text: fallbackText,
      kind: 'error',
      stage: 'commit_failed',
      conversation_id: basePayload.backendConversationId ?? '',
      turn_id: basePayload.turnId ?? '',
    },
  };
}

export function resolveAssistantFormalText(payload?: AssistantFormalPayload) {
  if (!payload) {
    return '';
  }
  const replyText = payload.reply?.text?.trim();
  if (replyText) {
    return replyText;
  }
  const commitReplyText = payload.commitReply?.text?.trim();
  if (commitReplyText) {
    return commitReplyText;
  }
  const summary = payload.pendingDraftSummary?.trim();
  if (summary) {
    return summary;
  }
  if (payload.missingFields.length > 0) {
    const prefix = detectAssistantFormalLocale() === 'en' ? 'Missing fields: ' : '仍需补充：';
    return `${prefix}${payload.missingFields.join('、')}`;
  }
  if (payload.candidates.length > 1 && !payload.selectedCandidateId) {
    return detectAssistantFormalLocale() === 'en'
      ? 'Multiple candidates were detected. Please select one to continue.'
      : '检测到多个候选父组织，请选择一个后继续。';
  }
  if (payload.selectedCandidateId) {
    return detectAssistantFormalLocale() === 'en'
      ? 'Candidate confirmed. You can submit now.'
      : '候选已确认，可以继续提交。';
  }
  return detectAssistantFormalLocale() === 'en' ? 'Processing...' : '处理中...';
}

export function patchAssistantFormalMessage(
  messages: TMessage[],
  messageId: string,
  patch: Partial<AssistantFormalMessage>,
): TMessage[] {
  return messages.map((message) => {
    if (message.messageId !== messageId) {
      return message;
    }
    return {
      ...message,
      ...patch,
    } as TMessage;
  });
}

export function isAssistantFormalMessage(message?: TMessage | null): message is AssistantFormalMessage {
  if (!message || typeof message !== 'object') {
    return false;
  }
  return (message as AssistantFormalMessage).assistantFormalPayload?.kind === 'assistant_formal';
}
