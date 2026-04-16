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
    const retiredAPIMessage = resolveApiErrorMessage('assistant_api_gone', 'assistant_api_gone')
    const retiredEntryMessage = resolveApiErrorMessage('assistant_ui_retired', 'assistant_ui_retired')
    const runtimeMessage = resolveApiErrorMessage('assistant_runtime_unavailable', 'assistant_runtime_unavailable')
    const gateMessage = resolveApiErrorMessage('assistant_gate_unavailable', 'assistant_gate_unavailable')
    const retiredCompatMessage = resolveApiErrorMessage('assistant_vendored_api_retired', 'assistant_vendored_api_retired')
    expect(sessionMessage).not.toBe('assistant_session_invalid')
    expect(principalMessage).not.toBe('assistant_principal_invalid')
    expect(bootstrapMessage).not.toBe('assistant_ui_bootstrap_unavailable')
    expect(retiredAPIMessage).not.toBe('assistant_api_gone')
    expect(retiredEntryMessage).not.toBe('assistant_ui_retired')
    expect(runtimeMessage).not.toBe('assistant_runtime_unavailable')
    expect(gateMessage).not.toBe('assistant_gate_unavailable')
    expect(retiredCompatMessage).not.toBe('assistant_vendored_api_retired')
  })

  it('returns explicit mapped messages for cubebox errors', () => {
    const serviceMissing = resolveApiErrorMessage('cubebox_service_missing', 'cubebox_service_missing')
    const cursorInvalid = resolveApiErrorMessage('cubebox_conversation_cursor_invalid', 'cubebox_conversation_cursor_invalid')
    const listFailed = resolveApiErrorMessage('cubebox_conversation_list_failed', 'cubebox_conversation_list_failed')
    const loadFailed = resolveApiErrorMessage('cubebox_conversation_load_failed', 'cubebox_conversation_load_failed')
    const createFailed = resolveApiErrorMessage('cubebox_conversation_create_failed', 'cubebox_conversation_create_failed')
    const deleteBlocked = resolveApiErrorMessage('cubebox_conversation_delete_blocked_by_running_task', 'cubebox_conversation_delete_blocked_by_running_task')
    const turnCreateFailed = resolveApiErrorMessage('cubebox_turn_create_failed', 'cubebox_turn_create_failed')
    const turnActionFailed = resolveApiErrorMessage('cubebox_turn_action_failed', 'cubebox_turn_action_failed')
    const taskNotFound = resolveApiErrorMessage('cubebox_task_not_found', 'cubebox_task_not_found')
    const taskLoadFailed = resolveApiErrorMessage('cubebox_task_load_failed', 'cubebox_task_load_failed')
    const taskCancelFailed = resolveApiErrorMessage('cubebox_task_cancel_failed', 'cubebox_task_cancel_failed')
    const taskDispatchFailed = resolveApiErrorMessage('cubebox_task_dispatch_failed', 'cubebox_task_dispatch_failed')
    const modelsUnavailable = resolveApiErrorMessage('cubebox_models_unavailable', 'cubebox_models_unavailable')

    expect(serviceMissing).not.toBe('cubebox_service_missing')
    expect(cursorInvalid).not.toBe('cubebox_conversation_cursor_invalid')
    expect(listFailed).not.toBe('cubebox_conversation_list_failed')
    expect(loadFailed).not.toBe('cubebox_conversation_load_failed')
    expect(createFailed).not.toBe('cubebox_conversation_create_failed')
    expect(deleteBlocked).not.toBe('cubebox_conversation_delete_blocked_by_running_task')
    expect(turnCreateFailed).not.toBe('cubebox_turn_create_failed')
    expect(turnActionFailed).not.toBe('cubebox_turn_action_failed')
    expect(taskNotFound).not.toBe('cubebox_task_not_found')
    expect(taskLoadFailed).not.toBe('cubebox_task_load_failed')
    expect(taskCancelFailed).not.toBe('cubebox_task_cancel_failed')
    expect(taskDispatchFailed).not.toBe('cubebox_task_dispatch_failed')
    expect(modelsUnavailable).not.toBe('cubebox_models_unavailable')
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
