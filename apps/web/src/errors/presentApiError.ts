import type { Locale } from '../i18n/messages'

type LocalizedErrorMessage = {
  en: string
  zh: string
}

const localizedMessages: Record<string, LocalizedErrorMessage> = {
  DEFAULT_RULE_REQUIRED: { en: 'This field is system-managed but no default rule is configured.', zh: '该字段为系统维护，但未配置默认规则。' },
  FIELD_OPTION_NOT_ALLOWED: { en: 'Selected field value is not allowed.', zh: '字段值不在允许范围内，请重新选择。' },
  policy_conflict_ambiguous: { en: 'Field policy is conflicting.', zh: '字段策略存在冲突，请联系管理员修复。' },
  policy_missing: { en: 'Field policy is missing for current context.', zh: '当前上下文缺少字段策略，请刷新后重试。' },
  policy_version_required: { en: 'Policy version is required.', zh: '缺少策略版本，请刷新页面后重试。' },
  policy_version_conflict: { en: 'Policy version is stale. Please refresh and retry.', zh: '策略版本已变化，请刷新页面后重试。' },
  FIELD_POLICY_EXPR_INVALID: { en: 'Default rule expression is invalid.', zh: '默认规则表达式不合法。' },
  FIELD_REQUIRED_VALUE_MISSING: { en: 'Required field value is missing.', zh: '策略决议后必填字段仍为空，请补全后重试。' },
  ORG_ALREADY_EXISTS: { en: 'Org already exists.', zh: '请求失败（org already exists）。' },
  ORG_CODE_INVALID: { en: 'Org code is invalid.', zh: '请求失败（org code invalid）。' },
  ORG_CODE_NOT_FOUND: { en: 'Org code not found.', zh: '请求失败（org code not found）。' },
  ORG_EVENT_NOT_FOUND: { en: 'Target effective-date record is not found.', zh: '未找到目标生效日记录。' },
  ORG_EVENT_RESCINDED: { en: 'Target record has been rescinded.', zh: '目标记录已被撤销。' },
  ORG_EXT_QUERY_FIELD_NOT_ALLOWED: { en: 'Org ext query field not allowed.', zh: '请求失败（org ext query field not allowed）。' },
  ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG: { en: 'Org field config invalid data source config.', zh: '请求失败（org field config invalid data source config）。' },
  ORG_FIELD_DEFINITION_NOT_FOUND: { en: 'Org field definition not found.', zh: '请求失败（org field definition not found）。' },
  ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF: { en: 'Org field options field not enabled as of.', zh: '请求失败（org field options field not enabled as of）。' },
  ORG_FIELD_OPTIONS_NOT_SUPPORTED: { en: 'Org field options not supported.', zh: '请求失败（org field options not supported）。' },
  ORG_INTENT_NOT_SUPPORTED: { en: 'Org intent not supported.', zh: '请求失败（org intent not supported）。' },
  ORG_NOT_FOUND_AS_OF: { en: 'Org not found as of.', zh: '请求失败（org not found as of）。' },
  ORG_ROOT_ALREADY_EXISTS: { en: 'Org root already exists.', zh: '请求失败（org root already exists）。' },
  ORG_TREE_NOT_INITIALIZED: { en: 'Org tree not initialized.', zh: '请求失败（org tree not initialized）。' },
  audit_error: { en: 'Audit error.', zh: '请求失败（audit error）。' },
  ai_actor_auth_snapshot_expired: { en: 'Auth snapshot expired. Please re-auth and retry.', zh: '身份快照已过期，请重新认证后重试。' },
  ai_actor_role_drift_detected: { en: 'Role changed during this conversation. Please re-confirm.', zh: '会话期间角色发生变化，请重新确认后提交。' },
  ai_plan_boundary_violation: { en: 'Plan violates assistant execution boundary.', zh: '计划超出助手执行边界，请调整后重试。' },
  ai_plan_schema_constrained_decode_failed: { en: 'Plan schema decode failed under strict constraints.', zh: '计划结构化解析失败，请补全必填信息后重试。' },
  ai_plan_contract_version_mismatch: { en: 'Plan contract version mismatch. Please regenerate and confirm again.', zh: '计划契约版本不一致，请重新生成并确认后再提交。' },
  ai_version_tuple_stale: { en: 'Confirmation baseline changed. Please confirm again before commit.', zh: '确认基线已变化，请重新确认后再提交。' },
  ai_plan_determinism_violation: { en: 'Plan determinism check failed. Please regenerate and retry.', zh: '计划确定性校验失败，请重新生成后重试。' },
  ai_model_provider_unavailable: { en: 'No available model provider. Please check provider health and retry.', zh: '当前无可用模型服务，请检查模型健康状态后重试。' },
  ai_model_timeout: { en: 'Model request timed out. Please retry later.', zh: '模型请求超时，请稍后重试。' },
  ai_model_rate_limited: { en: 'Model provider is rate limited. Please retry later.', zh: '模型服务限流，请稍后重试。' },
  ai_model_config_invalid: { en: 'Model provider configuration is invalid. Please fix and apply again.', zh: '模型配置不合法，请修正后重新应用。' },
  ai_runtime_config_invalid: { en: 'Assistant runtime model configuration is invalid. Please fix and restart service.', zh: '助手运行时模型配置不合法，请修正配置并重启服务。' },
  ai_runtime_config_missing: { en: 'Assistant runtime model configuration is missing. Please configure and restart service.', zh: '助手运行时模型配置缺失，请完成配置并重启服务。' },
  ai_model_secret_missing: { en: 'Model provider secret is missing. Please configure key reference and retry.', zh: '模型密钥缺失，请检查 key_ref 配置后重试。' },
  ai_reply_render_failed: { en: 'Assistant reply generation failed. Please retry.', zh: '助手回复生成失败，请稍后重试。' },
  ai_reply_model_target_mismatch: { en: 'Assistant reply did not come from the expected model pipeline. Please retry later.', zh: '助手回复未命中预期的大模型链路，请稍后重试。' },
  authz_error: { en: 'Authz error.', zh: '请求失败（authz error）。' },
  bad_json: { en: 'Request body JSON is invalid.', zh: '请求失败（bad json）。' },
  bad_request: { en: 'Request is invalid.', zh: '请求失败（bad request）。' },
  db_error: { en: 'DB error.', zh: '请求失败（db error）。' },
  dict_code_invalid: { en: 'Dict code is invalid.', zh: '请求失败（dict code invalid）。' },
  dict_code_required: { en: 'Dict code is required.', zh: '请求失败（dict code required）。' },
  dict_disabled_on_required: { en: 'Dict disabled on is required.', zh: '请求失败（dict disabled on required）。' },
  dict_enabled_on_required: { en: 'Dict enabled on is required.', zh: '请求失败（dict enabled on required）。' },
  dict_name_required: { en: 'Dict name is required.', zh: '请求失败（dict name required）。' },
  dict_release_store_missing: { en: 'Dict release store is missing.', zh: '请求失败（dict release store missing）。' },
  dict_store_missing: { en: 'Dict store is missing.', zh: '请求失败（dict store missing）。' },
  dict_value_code_required: { en: 'Dict value code is required.', zh: '请求失败（dict value code required）。' },
  dict_value_label_required: { en: 'Dict value label is required.', zh: '请求失败（dict value label required）。' },
  conversation_confirmation_required: { en: 'Confirmation is required before commit.', zh: '当前计划仍需补充必填信息或确认候选对象。' },
  conversation_confirmation_expired: { en: 'Confirmation window expired. Please regenerate and confirm again.', zh: '确认窗口已过期，请重新生成并确认后再继续。' },
  conversation_not_found: { en: 'Conversation is not found.', zh: '会话不存在，请重新创建。' },
  conversation_state_invalid: { en: 'Conversation state is invalid for this action.', zh: '当前会话状态不允许执行该操作。' },
  conversation_turn_not_found: { en: 'Turn is not found in this conversation.', zh: '当前会话中未找到该回合。' },
  idempotency_key_conflict: { en: 'Request payload conflicts with existing idempotency key.', zh: '请求载荷与已有幂等键冲突，请使用新的 request_id 重试。' },
  request_in_progress: { en: 'Request is still in progress. Please retry shortly.', zh: '请求仍在处理中，请稍后重试。' },
  tenant_mismatch: { en: 'Conversation belongs to another tenant.', zh: '该会话不属于当前租户。' },
  forbidden: { en: 'You do not have permission to perform this action.', zh: '你没有执行该操作的权限。' },
  identity_error: { en: 'Identity error.', zh: '请求失败（identity error）。' },
  identity_provider_error: { en: 'Identity provider error.', zh: '请求失败（identity provider error）。' },
  idp_error: { en: 'Idp error.', zh: '请求失败（idp error）。' },
  internal_error: { en: 'Internal server error.', zh: '请求失败（internal error）。' },
  invalid_as_of: { en: 'Invalid as of.', zh: '请求失败（invalid as of）。' },
  invalid_credentials: { en: 'Invalid credentials.', zh: '请求失败（invalid credentials）。' },
  invalid_effective_date: { en: 'Invalid effective date.', zh: '请求失败（invalid effective date）。' },
  invalid_form: { en: 'Invalid form.', zh: '请求失败（invalid form）。' },
  invalid_hostname: { en: 'Invalid hostname.', zh: '请求失败（invalid hostname）。' },
  invalid_identity_role: { en: 'Invalid identity role.', zh: '请求失败（invalid identity role）。' },
  invalid_input: { en: 'Invalid input.', zh: '请求失败（invalid input）。' },
  invalid_json: { en: 'Invalid json.', zh: '请求失败（invalid json）。' },
  invalid_request: { en: 'Request parameters are invalid. Please check and retry.', zh: '请求参数无效，请检查后重试。' },
  method_not_allowed: { en: 'HTTP method is not allowed for this endpoint.', zh: '请求失败（method not allowed）。' },
  not_found: { en: 'Requested resource is not found.', zh: '请求失败（not found）。' },
  org_code_invalid: { en: 'Org code is invalid.', zh: '组织 org_code 无效，请检查后重试。' },
  org_code_not_found: { en: 'Org code not found.', zh: '组织 org_code 不存在，请检查后重试。' },
  org_unit_not_found: { en: 'Org unit not found.', zh: '请求失败（org unit not found）。' },
  orgunit_resolve_org_code_failed: { en: 'Orgunit resolve org code failed.', zh: '请求失败（orgunit resolve org code failed）。' },
  orgunit_service_missing: { en: 'Orgunit service is missing.', zh: '请求失败（orgunit service missing）。' },
  orgunit_store_missing: { en: 'Orgunit store is missing.', zh: '请求失败（orgunit store missing）。' },
  principal_error: { en: 'Principal error.', zh: '请求失败（principal error）。' },
  principal_lookup_error: { en: 'Principal lookup error.', zh: '请求失败（principal lookup error）。' },
  session_error: { en: 'Session error.', zh: '请求失败（session error）。' },
  session_lookup_error: { en: 'Session lookup error.', zh: '请求失败（session lookup error）。' },
  tenant_missing: { en: 'Tenant context is missing. Please refresh and retry.', zh: '租户上下文缺失，请刷新后重试。' },
  tenant_not_found: { en: 'Tenant is not found. Please check the current host.', zh: '未找到租户，请检查当前访问域名。' },
  tenant_resolve_error: { en: 'Tenant resolution failed. Please retry later.', zh: '租户解析失败，请稍后重试。' },
  unauthorized: { en: 'Your session has expired. Please sign in again.', zh: '登录已失效，请重新登录。' },
  web_mui_index_missing: { en: 'Web mui index is missing.', zh: '请求失败（web mui index missing）。' },
  write_disabled: { en: 'Write disabled.', zh: '请求失败（write disabled）。' },
}

function resolveLocale(locale?: Locale): Locale {
  if (locale === 'en' || locale === 'zh') {
    return locale
  }

  if (typeof navigator === 'undefined') {
    return 'zh'
  }
  const candidates = [navigator.language, ...navigator.languages].filter((item) => typeof item === 'string')
  return candidates.some((item) => item.toLowerCase().startsWith('zh')) ? 'zh' : 'en'
}

function isGenericMessage(code: string, fallback: string): boolean {
  const normalized = fallback.trim().toLowerCase()
  if (normalized.length === 0) {
    return true
  }
  if (code.length > 0 && normalized === code.toLowerCase()) {
    return true
  }
  if (/^[a-z0-9_]+_failed$/.test(normalized)) {
    return true
  }
  if (/^[a-z]+(?: [a-z]+){0,2} failed$/.test(normalized)) {
    return true
  }
  return false
}

function sentenceFromCode(code: string): string {
  const normalized = code.trim().replaceAll('-', '_')
  if (normalized.length === 0) {
    return ''
  }
  const parts = normalized.split('_').map((part) => part.trim().toLowerCase()).filter((part) => part.length > 0)
  if (parts.length === 0) {
    return ''
  }
  if (parts[parts.length - 1] === 'failed') {
    const action = parts.slice(0, -1).join(' ')
    if (action.length === 0) {
      return 'Request failed.'
    }
    return `${action.charAt(0).toUpperCase()}${action.slice(1)} failed.`
  }
  const sentence = parts.join(' ')
  return `${sentence.charAt(0).toUpperCase()}${sentence.slice(1)}.`
}

export function resolveApiErrorMessage(code: string | undefined, fallback: string, locale?: Locale): string {
  const normalizedCode = (code ?? '').trim()
  const resolvedLocale = resolveLocale(locale)
  const explicit = normalizedCode.length > 0 ? localizedMessages[normalizedCode] : undefined
  if (explicit) {
    return resolvedLocale === 'zh' ? explicit.zh : explicit.en
  }

  if (!isGenericMessage(normalizedCode, fallback)) {
    return fallback
  }

  const synthesized = sentenceFromCode(normalizedCode)
  if (synthesized.length > 0) {
    return resolvedLocale === 'zh' ? `请求失败（${normalizedCode}）` : synthesized
  }
  return resolvedLocale === 'zh' ? '请求失败，请稍后重试。' : 'Request failed. Please retry later.'
}
