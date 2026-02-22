import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'
import { normalizeApiError } from './errors'

export interface HttpClientOptions {
  baseURL: string
  timeoutMs?: number
  maxRetries?: number
  getAccessToken?: () => string | undefined
  getTenantId?: () => string | undefined
  getTraceID?: () => string
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

function randomHex(length: number): string {
  const bytes = new Uint8Array(Math.ceil(length / 2))
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i += 1) {
      bytes[i] = Math.floor(Math.random() * 256)
    }
  }
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('').slice(0, length)
}

function normalizeTraceID(traceID?: string): string {
  const normalized = (traceID ?? '').replace(/-/g, '').toLowerCase()
  if (/^[0-9a-f]{32}$/.test(normalized) && normalized !== '00000000000000000000000000000000') {
    return normalized
  }
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID().replace(/-/g, '')
  }
  return randomHex(32)
}

function buildTraceparent(traceID?: string): string {
  const version = '00'
  const normalizedTraceID = normalizeTraceID(traceID)
  const parentID = randomHex(16)
  const traceFlags = '01'
  return `${version}-${normalizedTraceID}-${parentID}-${traceFlags}`
}

export function buildRequestHeaders(options: {
  token?: string
  tenantId?: string
  traceID?: string
}): Record<string, string> {
  const headers: Record<string, string> = {}

  if (options.token) {
    headers.Authorization = `Bearer ${options.token}`
  }

  if (options.tenantId) {
    headers['X-Tenant-ID'] = options.tenantId
  }

  headers.traceparent = buildTraceparent(options.traceID)

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
      traceID: options.getTraceID?.()
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
