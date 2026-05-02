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
  knowledge_pack_invalid: { en: 'Query knowledge pack is invalid. Please ask an administrator to check the module knowledge configuration.', zh: '查询知识包不合法，请联系管理员检查模块知识配置。' },
  api_catalog_drift_or_executor_missing: { en: 'Query execution catalog is inconsistent with the system registry. Please retry later or contact an administrator.', zh: '查询执行目录与系统注册表不一致，请稍后重试或联系管理员。' },
  cubebox_query_planner_outcome_invalid: { en: 'A runnable query plan could not be formed. Please rephrase or add query details.', zh: '未能形成可执行查询计划，请换一种说法或补充查询条件后重试。' },
  cubebox_query_done_without_result: { en: 'The query plan produced no usable result. Please add query details and retry.', zh: '查询计划未产生可用结果，请补充查询条件后重试。' },
  cubebox_query_no_query_after_execution: { en: 'The query plan moved outside the supported scope after execution. Please rephrase and retry.', zh: '查询计划在执行后偏离支持范围，请换一种说法后重试。' },
  cubebox_query_loop_budget_exceeded: { en: 'This query needs more steps than the current single-turn budget allows. Please narrow the scope and retry.', zh: '这次查询需要的步骤超出当前单轮预算，请缩小查询范围后重试。' },
  cubebox_query_loop_repeated_plan: { en: 'The query plan repeated without making progress. Please narrow the scope or rephrase.', zh: '查询计划重复且无法继续推进，请缩小范围或换一种说法后重试。' },
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
  authz_catalog_error: { en: 'Authorization catalog is unavailable. Please retry later.', zh: '授权目录暂不可用，请稍后重试。' },
  authz_capability_key_not_supported: { en: 'This authorization identifier is not supported in the tenant API catalog.', zh: '该授权项标识不支持用于租户 API 目录查询。' },
  authz_org_scope_required: { en: 'This role requires an organization scope.', zh: '该角色需要选择组织范围。' },
  authz_runtime_unavailable: { en: 'Authorization runtime is unavailable. Please retry later.', zh: '授权运行时暂不可用，请稍后重试。' },
  authz_scope_forbidden: { en: 'The organization is outside your authorization scope.', zh: '当前组织不在你的授权范围内。' },
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
  diagnostic_parameter_not_supported: { en: 'This endpoint does not support diagnostic parameters. Use the authz diagnostics view.', zh: '当前接口不支持诊断参数，请使用授权项诊断入口。' },
  conversation_confirmation_required: { en: 'Confirmation is required before commit.', zh: '当前计划仍需补充必填信息或确认候选对象。' },
  conversation_confirmation_expired: { en: 'Confirmation window expired. Please regenerate and confirm again.', zh: '确认窗口已过期，请重新生成并确认后再继续。' },
  conversation_id_required: { en: 'Conversation ID is required.', zh: '缺少会话 ID，请重试。' },
  conversation_not_found: { en: 'Conversation is not found.', zh: '会话不存在，请重新创建。' },
  conversation_state_invalid: { en: 'Conversation state is invalid for this action.', zh: '当前会话状态不允许执行该操作。' },
  conversation_turn_not_found: { en: 'Turn is not found in this conversation.', zh: '当前会话中未找到该回合。' },
  cubebox_conversation_create_failed: { en: 'Failed to create conversation. Please retry.', zh: '创建会话失败，请稍后重试。' },
  cubebox_conversation_list_failed: { en: 'Failed to load conversation list. Please retry.', zh: '加载会话列表失败，请稍后重试。' },
  cubebox_conversation_not_found: { en: 'Conversation is not found.', zh: '会话不存在，请重新选择或新建。' },
  cubebox_conversation_read_failed: { en: 'Failed to load conversation. Please retry.', zh: '读取会话失败，请稍后重试。' },
  cubebox_conversation_update_failed: { en: 'Failed to update conversation. Please retry.', zh: '更新会话失败，请稍后重试。' },
  cubebox_turn_stream_failed: { en: 'CubeBox response failed. Please retry later.', zh: 'CubeBox 回复失败，请稍后重试。' },
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
  invalid_authz_capability_key: { en: 'Authorization identifier format is invalid.', zh: '授权项标识格式无效。' },
  invalid_json: { en: 'Invalid json.', zh: '请求失败（invalid json）。' },
  invalid_request: { en: 'Request parameters are invalid. Please check and retry.', zh: '请求参数无效，请检查后重试。' },
  invalid_role_definition: { en: 'Role definition is invalid. Please check capabilities and name.', zh: '角色定义不合法，请检查授权项和名称。' },
  invalid_role_payload: { en: 'Role definition request is invalid. Please check and retry.', zh: '角色定义请求无效，请检查后重试。' },
  invalid_user_assignment: { en: 'User authorization settings are invalid. Please check roles and organization scope.', zh: '用户授权设置无效，请检查角色和组织范围。' },
  method_not_allowed: { en: 'HTTP method is not allowed for this endpoint.', zh: '请求失败（method not allowed）。' },
  not_found: { en: 'Requested resource is not found.', zh: '请求失败（not found）。' },
  org_code_invalid: { en: 'Org code is invalid.', zh: '组织 org_code 无效，请检查后重试。' },
  org_code_not_found: { en: 'Org code not found.', zh: '组织 org_code 不存在，请检查后重试。' },
  org_unit_not_found: { en: 'Org unit not found.', zh: '请求失败（org unit not found）。' },
  orgunit_resolve_org_code_failed: { en: 'Orgunit resolve org code failed.', zh: '请求失败（orgunit resolve org code failed）。' },
  orgunit_service_missing: { en: 'Orgunit service is missing.', zh: '请求失败（orgunit service missing）。' },
  orgunit_store_missing: { en: 'Orgunit store is missing.', zh: '请求失败（orgunit store missing）。' },
  policy_version_required: { en: 'Policy version is required. Please refresh and retry.', zh: '缺少策略版本，请刷新页面后重试。' },
  policy_version_conflict: { en: 'Policy version is stale. Please refresh and retry.', zh: '策略版本已过期，请刷新页面后重试。' },
  principal_error: { en: 'Principal error.', zh: '请求失败（principal error）。' },
  principal_lookup_error: { en: 'Principal lookup error.', zh: '请求失败（principal lookup error）。' },
  principal_assignment_error: { en: 'User authorization initialization failed. Please retry later.', zh: '用户授权初始化失败，请稍后重试。' },
  principal_missing: { en: 'Your session has expired. Please sign in again.', zh: '登录已失效，请重新登录。' },
  role_not_found: { en: 'Role not found. Please refresh and retry.', zh: '角色不存在，请刷新后重试。' },
  role_slug_conflict: { en: 'Role identifier already exists. Please use another identifier.', zh: '角色标识已存在，请使用其他标识。' },
  session_error: { en: 'Session error.', zh: '请求失败（session error）。' },
  session_lookup_error: { en: 'Session lookup error.', zh: '请求失败（session lookup error）。' },
  stale_revision: { en: 'Data version has changed. Please refresh and retry.', zh: '数据版本已变化，请刷新后重试。' },
  system_role_readonly: { en: 'System-managed roles cannot be modified.', zh: '系统内置角色不可修改。' },
  stream_not_supported: { en: 'Streaming is not supported in the current environment.', zh: '当前环境不支持流式响应，请稍后重试。' },
  tenant_missing: { en: 'Tenant context is missing. Please refresh and retry.', zh: '租户上下文缺失，请刷新后重试。' },
  tenant_not_found: { en: 'Tenant is not found. Please check the current host.', zh: '未找到租户，请检查当前访问域名。' },
  tenant_resolve_error: { en: 'Tenant resolution failed. Please retry later.', zh: '租户解析失败，请稍后重试。' },
  turn_id_required: { en: 'Turn ID is required.', zh: '缺少回合 ID，请重试。' },
  unauthorized: { en: 'Your session has expired. Please sign in again.', zh: '登录已失效，请重新登录。' },
  unknown_authz_capability_key: { en: 'Authorization identifier is not registered.', zh: '授权项标识未登记。' },
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
