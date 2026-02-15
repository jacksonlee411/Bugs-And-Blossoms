import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'
import { normalizeApiError } from './errors'

export interface HttpClientOptions {
  baseURL: string
  timeoutMs?: number
  maxRetries?: number
  getAccessToken?: () => string | undefined
  getTenantId?: () => string | undefined
  getRequestId?: () => string
}

export interface RequestConfig extends AxiosRequestConfig {
  retry?: number
}

export interface HttpClient {
  get: <T>(url: string, config?: RequestConfig) => Promise<T>
  post: <T>(url: string, data?: unknown, config?: RequestConfig) => Promise<T>
  put: <T>(url: string, data?: unknown, config?: RequestConfig) => Promise<T>
  delete: <T>(url: string, config?: RequestConfig) => Promise<T>
}

function defaultRequestId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  return `rid-${Date.now()}`
}

export function buildRequestHeaders(options: {
  token?: string
  tenantId?: string
  requestId?: string
}): Record<string, string> {
  const headers: Record<string, string> = {}

  if (options.token) {
    headers.Authorization = `Bearer ${options.token}`
  }

  if (options.tenantId) {
    headers['X-Tenant-ID'] = options.tenantId
  }

  if (options.requestId) {
    headers['X-Request-ID'] = options.requestId
  }

  return headers
}

function shouldRetry(error: unknown): boolean {
  if (!axios.isAxiosError(error)) {
    return false
  }

  if (!error.response) {
    return true
  }

  return error.response.status >= 500
}

export function createHttpClient(options: HttpClientOptions): HttpClient {
  const instance: AxiosInstance = axios.create({
    baseURL: options.baseURL,
    timeout: options.timeoutMs ?? 10000,
    withCredentials: true
  })

  instance.interceptors.request.use((config) => {
    const headers = buildRequestHeaders({
      token: options.getAccessToken?.(),
      tenantId: options.getTenantId?.(),
      requestId: options.getRequestId?.() ?? defaultRequestId()
    })

    Object.entries(headers).forEach(([key, value]) => {
      config.headers.set(key, value)
    })

    return config
  })

  async function request<T>(config: RequestConfig): Promise<T> {
    const maxRetries = config.retry ?? options.maxRetries ?? 1

    for (let attempt = 0; ; attempt += 1) {
      try {
        const response = await instance.request<T>(config)
        return response.data
      } catch (error) {
        if (attempt < maxRetries && shouldRetry(error)) {
          continue
        }

        const normalized = normalizeApiError(error)
        if (normalized.status === 401 && typeof window !== 'undefined') {
          if (!window.location.pathname.startsWith('/app/login')) {
            window.location.assign('/app/login')
          }
        }
        throw normalized
      }
    }
  }

  return {
    get: (url, config = {}) => request({ ...config, method: 'GET', url }),
    post: (url, data, config = {}) => request({ ...config, data, method: 'POST', url }),
    put: (url, data, config = {}) => request({ ...config, data, method: 'PUT', url }),
    delete: (url, config = {}) => request({ ...config, method: 'DELETE', url })
  }
}

const env = import.meta.env

export const httpClient = createHttpClient({
  baseURL: env.VITE_API_BASE_URL ?? 'http://localhost:8080',
  getTenantId: () => env.VITE_TENANT_ID,
  maxRetries: 1,
  timeoutMs: Number(env.VITE_API_TIMEOUT_MS || 10000)
})
