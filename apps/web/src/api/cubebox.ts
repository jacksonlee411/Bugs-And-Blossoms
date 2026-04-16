import { httpClient } from './httpClient'
import type {
  CubeBoxConversation,
  CubeBoxConversationListResponse,
  CubeBoxRenderReplyRequest,
  CubeBoxReply,
  CubeBoxRuntimeStatusResponse,
  CubeBoxSessionRefreshResponse,
  CubeBoxSessionResponse,
  CubeBoxTask,
  CubeBoxTaskCancelResponse,
  CubeBoxTaskReceipt,
  CubeBoxTaskSubmitRequest,
  CubeBoxUIBootstrapResponse
} from './cubeboxTypes'

export type {
  CubeBoxConversation,
  CubeBoxConversationListItem,
  CubeBoxConversationListResponse,
  CubeBoxRenderReplyRequest,
  CubeBoxReply,
  CubeBoxRuntimeCapabilities,
  CubeBoxRuntimeComponent,
  CubeBoxRuntimeStatusResponse,
  CubeBoxSessionRefreshResponse,
  CubeBoxSessionResponse,
  CubeBoxTask,
  CubeBoxTaskCancelResponse,
  CubeBoxTaskContractSnapshot,
  CubeBoxTaskReceipt,
  CubeBoxTaskSubmitRequest,
  CubeBoxTurn,
  CubeBoxUIBootstrapResponse
} from './cubeboxTypes'

const DEFAULT_CUBEBOX_TURN_TIMEOUT_MS = 60000

function resolveCubeBoxTurnTimeoutMs(raw: string | undefined): number {
  const parsed = Number(raw ?? '')
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed
  }
  return DEFAULT_CUBEBOX_TURN_TIMEOUT_MS
}

const cubeboxTurnTimeoutMs = resolveCubeBoxTurnTimeoutMs(
  import.meta.env.VITE_CUBEBOX_TURN_TIMEOUT_MS ?? import.meta.env.VITE_ASSISTANT_TURN_TIMEOUT_MS
)

export interface CubeBoxFile {
  file_id: string
  filename?: string
  content_type?: string
  scan_status?: string
  created_at?: string
  links?: Array<{
    link_role: string
    conversation_id: string
    turn_id?: string
  }>
  conversation_id?: string
  file_name: string
  media_type: string
  size_bytes: number
  sha256: string
  storage_key: string
  uploaded_by: string
  uploaded_at: string
}

export interface CubeBoxFileListResponse {
  items: CubeBoxFile[]
}

export interface CubeBoxModelsResponse {
  models: Array<{
    provider: string
    model: string
  }>
}

export async function createCubeBoxConversation(): Promise<CubeBoxConversation> {
  return httpClient.post<CubeBoxConversation>('/internal/cubebox/conversations', {})
}

export async function listCubeBoxConversations(params?: {
  page_size?: number
  cursor?: string
}): Promise<CubeBoxConversationListResponse> {
  const query = new URLSearchParams()
  if (typeof params?.page_size === 'number' && Number.isFinite(params.page_size)) {
    query.set('page_size', String(params.page_size))
  }
  if (typeof params?.cursor === 'string' && params.cursor.trim().length > 0) {
    query.set('cursor', params.cursor.trim())
  }
  const suffix = query.toString()
  const path = suffix.length > 0 ? `/internal/cubebox/conversations?${suffix}` : '/internal/cubebox/conversations'
  return httpClient.get<CubeBoxConversationListResponse>(path)
}

export async function getCubeBoxConversation(conversationID: string): Promise<CubeBoxConversation> {
  return httpClient.get<CubeBoxConversation>(`/internal/cubebox/conversations/${encodeURIComponent(conversationID)}`)
}

export async function createCubeBoxTurn(conversationID: string, userInput: string): Promise<CubeBoxConversation> {
  return httpClient.post<CubeBoxConversation>(
    `/internal/cubebox/conversations/${encodeURIComponent(conversationID)}/turns`,
    { user_input: userInput },
    { timeout: cubeboxTurnTimeoutMs, retry: 0 }
  )
}

export async function confirmCubeBoxTurn(
  conversationID: string,
  turnID: string,
  candidateID?: string
): Promise<CubeBoxConversation> {
  return httpClient.post<CubeBoxConversation>(
    `/internal/cubebox/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turnID)}:confirm`,
    candidateID ? { candidate_id: candidateID } : {}
  )
}

export async function commitCubeBoxTurn(conversationID: string, turnID: string): Promise<CubeBoxTaskReceipt> {
  return httpClient.post<CubeBoxTaskReceipt>(
    `/internal/cubebox/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turnID)}:commit`,
    {}
  )
}

export async function renderCubeBoxTurnReply(
  conversationID: string,
  turnID: string,
  payload: CubeBoxRenderReplyRequest
): Promise<CubeBoxReply> {
  return httpClient.post<CubeBoxReply>(
    `/internal/cubebox/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turnID)}:reply`,
    payload
  )
}

export async function submitCubeBoxTask(payload: CubeBoxTaskSubmitRequest): Promise<CubeBoxTaskReceipt> {
  return httpClient.post<CubeBoxTaskReceipt>('/internal/cubebox/tasks', payload)
}

export async function getCubeBoxTask(taskID: string): Promise<CubeBoxTask> {
  return httpClient.get<CubeBoxTask>(`/internal/cubebox/tasks/${encodeURIComponent(taskID)}`)
}

export async function cancelCubeBoxTask(taskID: string): Promise<CubeBoxTaskCancelResponse> {
  return httpClient.post<CubeBoxTaskCancelResponse>(`/internal/cubebox/tasks/${encodeURIComponent(taskID)}:cancel`, {})
}

export async function listCubeBoxFiles(params?: { conversation_id?: string }): Promise<CubeBoxFileListResponse> {
  const query = new URLSearchParams()
  if (typeof params?.conversation_id === 'string' && params.conversation_id.trim().length > 0) {
    query.set('conversation_id', params.conversation_id.trim())
  }
  const suffix = query.toString()
  const path = suffix.length > 0 ? `/internal/cubebox/files?${suffix}` : '/internal/cubebox/files'
  return httpClient.get<CubeBoxFileListResponse>(path)
}

export async function uploadCubeBoxFile(file: File, conversationID?: string): Promise<CubeBoxFile> {
  const form = new FormData()
  form.append('file', file)
  if (conversationID && conversationID.trim().length > 0) {
    form.append('conversation_id', conversationID.trim())
  }
  return httpClient.post<CubeBoxFile>('/internal/cubebox/files', form, {
    timeout: 30000,
    retry: 0
  })
}

export async function deleteCubeBoxFile(fileID: string): Promise<void> {
  return httpClient.delete<void>(`/internal/cubebox/files/${encodeURIComponent(fileID)}`)
}

export async function getCubeBoxModels(): Promise<CubeBoxModelsResponse> {
  return httpClient.get<CubeBoxModelsResponse>('/internal/cubebox/models')
}

export async function getCubeBoxUIBootstrap(): Promise<CubeBoxUIBootstrapResponse> {
  return httpClient.get<CubeBoxUIBootstrapResponse>('/internal/cubebox/ui-bootstrap')
}

export async function getCubeBoxSession(): Promise<CubeBoxSessionResponse> {
  return httpClient.get<CubeBoxSessionResponse>('/internal/cubebox/session')
}

export async function refreshCubeBoxSession(): Promise<CubeBoxSessionRefreshResponse> {
  return httpClient.post<CubeBoxSessionRefreshResponse>('/internal/cubebox/session/refresh', {})
}

export async function logoutCubeBoxSession(): Promise<void> {
  return httpClient.post<void>('/internal/cubebox/session/logout', {})
}

export async function getCubeBoxRuntimeStatus(): Promise<CubeBoxRuntimeStatusResponse> {
  try {
    return await httpClient.get<CubeBoxRuntimeStatusResponse>('/internal/cubebox/runtime-status')
  } catch (error) {
    const details = (error as { details?: unknown })?.details
    if (details && typeof details === 'object') {
      const candidate = details as Partial<CubeBoxRuntimeStatusResponse>
      if (
        typeof candidate.status === 'string' &&
        candidate.frontend &&
        candidate.backend &&
        candidate.knowledge_runtime &&
        candidate.model_gateway &&
        candidate.file_store
      ) {
        return candidate as CubeBoxRuntimeStatusResponse
      }
    }
    throw error
  }
}
