import axios, { type AxiosError } from 'axios'

export type ApiErrorCode =
  | 'BAD_REQUEST'
  | 'FORBIDDEN'
  | 'NETWORK_ERROR'
  | 'NOT_FOUND'
  | 'SERVER_ERROR'
  | 'TIMEOUT'
  | 'UNAUTHORIZED'
  | 'UNKNOWN_ERROR'

export class ApiClientError extends Error {
  constructor(
    message: string,
    public readonly code: ApiErrorCode,
    public readonly status?: number,
    public readonly requestId?: string,
    public readonly details?: unknown
  ) {
    super(message)
    this.name = 'ApiClientError'
  }
}

function statusToCode(status: number): ApiErrorCode {
  if (status === 400) return 'BAD_REQUEST'
  if (status === 401) return 'UNAUTHORIZED'
  if (status === 403) return 'FORBIDDEN'
  if (status === 404) return 'NOT_FOUND'
  if (status >= 500) return 'SERVER_ERROR'
  return 'UNKNOWN_ERROR'
}

export function normalizeApiError(error: unknown): ApiClientError {
  if (error instanceof ApiClientError) {
    return error
  }

  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<{ code?: string; message?: string; request_id?: string }>
    if (axiosError.code === 'ECONNABORTED') {
      return new ApiClientError('Request timeout', 'TIMEOUT')
    }

    if (!axiosError.response) {
      return new ApiClientError('Network error', 'NETWORK_ERROR')
    }

    const status = axiosError.response.status
    const data = axiosError.response.data
    const message = data?.message || axiosError.message || 'API request failed'

    return new ApiClientError(
      message,
      statusToCode(status),
      status,
      data?.request_id,
      data
    )
  }

  if (error instanceof Error) {
    return new ApiClientError(error.message, 'UNKNOWN_ERROR')
  }

  return new ApiClientError('Unknown error', 'UNKNOWN_ERROR')
}
