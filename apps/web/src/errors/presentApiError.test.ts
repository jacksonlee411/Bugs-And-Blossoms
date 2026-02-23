import { describe, expect, it } from 'vitest'
import { resolveApiErrorMessage } from './presentApiError'

describe('resolveApiErrorMessage', () => {
  it('returns explicit mapped message for known code', () => {
    const message = resolveApiErrorMessage('ORG_ROOT_ALREADY_EXISTS', 'orgunit_write_failed')
    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toBe('orgunit_write_failed')
  })

  it('returns explicit mapped message for tenant resolve error', () => {
    const message = resolveApiErrorMessage('tenant_resolve_error', 'tenant_resolve_error')
    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toBe('tenant_resolve_error')
  })

  it('keeps backend message when it is explicit', () => {
    const fallback = 'default rule evaluation failed. please check the rule.'
    expect(resolveApiErrorMessage('DEFAULT_RULE_EVAL_FAILED', fallback)).toBeTruthy()
  })

  it('synthesizes message for unknown generic code', () => {
    const message = resolveApiErrorMessage('dict_release_failed', 'dict_release_failed')
    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toBe('dict_release_failed')
  })
})
