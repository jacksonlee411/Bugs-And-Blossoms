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

  it('returns explicit mapped message for disable-not-allowed', () => {
    const message = resolveApiErrorMessage('policy_disable_not_allowed', 'policy_disable_not_allowed')
    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toBe('policy_disable_not_allowed')
  })

  it('returns explicit mapped message for assistant state invalid', () => {
    const message = resolveApiErrorMessage('conversation_state_invalid', 'conversation_state_invalid')
    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toBe('conversation_state_invalid')
  })

  it('returns explicit mapped message for assistant confirmation expired', () => {
    const message = resolveApiErrorMessage('conversation_confirmation_expired', 'conversation_confirmation_expired')
    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toBe('conversation_confirmation_expired')
  })

  it('returns explicit mapped message for assistant successor session errors', () => {
    const sessionMessage = resolveApiErrorMessage('assistant_session_invalid', 'assistant_session_invalid')
    const principalMessage = resolveApiErrorMessage('assistant_principal_invalid', 'assistant_principal_invalid')
    const bootstrapMessage = resolveApiErrorMessage('assistant_ui_bootstrap_unavailable', 'assistant_ui_bootstrap_unavailable')
    const retiredCompatMessage = resolveApiErrorMessage('assistant_vendored_api_retired', 'assistant_vendored_api_retired')
    expect(sessionMessage).not.toBe('assistant_session_invalid')
    expect(principalMessage).not.toBe('assistant_principal_invalid')
    expect(bootstrapMessage).not.toBe('assistant_ui_bootstrap_unavailable')
    expect(retiredCompatMessage).not.toBe('assistant_vendored_api_retired')
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
