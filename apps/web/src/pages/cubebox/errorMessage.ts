import type { Locale } from '../../i18n/messages'
import { ApiClientError } from '../../api/errors'
import { resolveApiErrorMessage } from '../../errors/presentApiError'

export function cubeBoxErrorMessage(error: unknown, fallback: string, locale: Locale): string {
  if (error instanceof ApiClientError) {
    if (error.details && typeof error.details === 'object') {
      const details = error.details as { code?: string; message?: string }
      return resolveApiErrorMessage(details.code, details.message ?? error.message, locale)
    }
    return resolveApiErrorMessage(error.code, fallback, locale)
  }

  const message = (error as { message?: string })?.message
  if (typeof message === 'string' && message.trim().length > 0) {
    return message
  }

  return fallback
}
