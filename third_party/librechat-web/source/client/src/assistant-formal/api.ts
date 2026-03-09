import type {
  AssistantFormalConversation,
  AssistantFormalReply,
  AssistantFormalTaskDetail,
  AssistantFormalTaskReceipt,
} from './runtime';

export class AssistantFormalAPIError extends Error {
  code: string;
  status: number;

  constructor(code: string, message: string, status: number) {
    super(message);
    this.name = 'AssistantFormalAPIError';
    this.code = code;
    this.status = status;
  }
}

type ErrorEnvelope = {
  code?: string;
  message?: string;
};

async function assistantFormalRequest<T>(
  path: string,
  init: RequestInit = {},
  token?: string | null,
): Promise<T> {
  const headers = new Headers(init.headers ?? {});
  headers.set('Accept', 'application/json');
  if (init.body != null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(path, {
    ...init,
    headers,
    credentials: 'same-origin',
  });

  const isJSON = response.headers.get('content-type')?.includes('application/json');
  const payload = (isJSON ? await response.json() : null) as T | ErrorEnvelope | null;
  if (!response.ok) {
    const code = (payload as ErrorEnvelope | null)?.code?.trim() || `http_${response.status}`;
    const message =
      (payload as ErrorEnvelope | null)?.message?.trim() || response.statusText || 'Request failed';
    throw new AssistantFormalAPIError(code, message, response.status);
  }
  return payload as T;
}

export function createAssistantFormalConversation(token?: string | null) {
  return assistantFormalRequest<AssistantFormalConversation>(
    '/internal/assistant/conversations',
    {
      method: 'POST',
      body: JSON.stringify({}),
    },
    token,
  );
}

export function createAssistantFormalTurn(
  conversationId: string,
  userInput: string,
  token?: string | null,
) {
  return assistantFormalRequest<AssistantFormalConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationId)}/turns`,
    {
      method: 'POST',
      body: JSON.stringify({ user_input: userInput }),
    },
    token,
  );
}

export function confirmAssistantFormalTurn(
  conversationId: string,
  turnId: string,
  candidateId: string,
  token?: string | null,
) {
  return assistantFormalRequest<AssistantFormalConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationId)}/turns/${encodeURIComponent(turnId)}:confirm`,
    {
      method: 'POST',
      body: JSON.stringify({ candidate_id: candidateId }),
    },
    token,
  );
}

export function commitAssistantFormalTurn(
  conversationId: string,
  turnId: string,
  token?: string | null,
) {
  return assistantFormalRequest<AssistantFormalTaskReceipt>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationId)}/turns/${encodeURIComponent(turnId)}:commit`,
    {
      method: 'POST',
      body: JSON.stringify({}),
    },
    token,
  );
}

export function getAssistantFormalConversation(conversationId: string, token?: string | null) {
  return assistantFormalRequest<AssistantFormalConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationId)}`,
    { method: 'GET' },
    token,
  );
}

export function getAssistantFormalTask(taskId: string, token?: string | null) {
  return assistantFormalRequest<AssistantFormalTaskDetail>(
    `/internal/assistant/tasks/${encodeURIComponent(taskId)}`,
    { method: 'GET' },
    token,
  );
}

export function cancelAssistantFormalTask(taskId: string, token?: string | null) {
  return assistantFormalRequest<AssistantFormalTaskDetail>(
    `/internal/assistant/tasks/${encodeURIComponent(taskId)}:cancel`,
    { method: 'POST', body: JSON.stringify({}) },
    token,
  );
}

export function renderAssistantFormalReply(
  conversationId: string,
  turnId: string,
  locale: 'zh' | 'en',
  token?: string | null,
) {
  return assistantFormalRequest<AssistantFormalReply>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationId)}/turns/${encodeURIComponent(turnId)}:reply`,
    {
      method: 'POST',
      body: JSON.stringify({ locale }),
    },
    token,
  );
}
