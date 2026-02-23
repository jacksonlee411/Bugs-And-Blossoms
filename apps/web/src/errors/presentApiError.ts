type LocalizedErrorMessage = {
  en: string
  zh: string
}

const localizedMessages: Record<string, LocalizedErrorMessage> = {
  ACTOR_SCOPE_FORBIDDEN: { en: 'Actor scope forbidden.', zh: '请求失败（actor scope forbidden）。' },
  CAPABILITY_CONTEXT_MISMATCH: { en: 'Capability context mismatch.', zh: '上下文与服务端判定不一致，请检查后重试。' },
  DEFAULT_RULE_REQUIRED: { en: 'This field is system-managed but no default rule is configured.', zh: '该字段为系统维护，但未配置默认规则。' },
  FIELD_POLICY_EXPR_INVALID: { en: 'Default rule expression is invalid.', zh: '默认规则表达式不合法。' },
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
  OWNER_CONTEXT_FORBIDDEN: { en: 'Owner context forbidden.', zh: '请求失败（owner context forbidden）。' },
  OWNER_CONTEXT_REQUIRED: { en: 'Owner context is required.', zh: '请求失败（owner context required）。' },
  OWNER_SETID_FORBIDDEN: { en: 'Owner setid forbidden.', zh: '请求失败（owner setid forbidden）。' },
  PACKAGE_CODE_INVALID: { en: 'Package code is invalid.', zh: '请求失败（package code invalid）。' },
  PACKAGE_CODE_RESERVED: { en: 'Package code reserved.', zh: '请求失败（package code reserved）。' },
  PACKAGE_OWNER_INVALID: { en: 'Package owner is invalid.', zh: '请求失败（package owner invalid）。' },
  PERSON_CREATE_FAILED: { en: 'Person create failed.', zh: '请求失败（person create failed）。' },
  PERSON_INTERNAL: { en: 'Person internal.', zh: '请求失败（person internal）。' },
  PERSON_NOT_FOUND: { en: 'Person not found.', zh: '请求失败（person not found）。' },
  PERSON_PERNR_INVALID: { en: 'Person pernr is invalid.', zh: '请求失败（person pernr invalid）。' },
  audit_error: { en: 'Audit error.', zh: '请求失败（audit error）。' },
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
  forbidden: { en: 'You do not have permission to perform this action.', zh: '你没有执行该操作的权限。' },
  identity_error: { en: 'Identity error.', zh: '请求失败（identity error）。' },
  identity_provider_error: { en: 'Identity provider error.', zh: '请求失败（identity provider error）。' },
  idp_error: { en: 'Idp error.', zh: '请求失败（idp error）。' },
  internal_error: { en: 'Internal server error.', zh: '请求失败（internal error）。' },
  invalid_as_of: { en: 'Invalid as of.', zh: '请求失败（invalid as of）。' },
  invalid_business_unit_id: { en: 'Invalid business unit ID.', zh: '请求失败（invalid business unit id）。' },
  invalid_credentials: { en: 'Invalid credentials.', zh: '请求失败（invalid credentials）。' },
  invalid_effective_date: { en: 'Invalid effective date.', zh: '请求失败（invalid effective date）。' },
  invalid_explain_level: { en: 'Invalid explain level.', zh: '请求失败（invalid explain level）。' },
  invalid_form: { en: 'Invalid form.', zh: '请求失败（invalid form）。' },
  invalid_hostname: { en: 'Invalid hostname.', zh: '请求失败（invalid hostname）。' },
  invalid_identity_role: { en: 'Invalid identity role.', zh: '请求失败（invalid identity role）。' },
  invalid_input: { en: 'Invalid input.', zh: '请求失败（invalid input）。' },
  invalid_json: { en: 'Invalid json.', zh: '请求失败（invalid json）。' },
  invalid_org_unit_id: { en: 'Invalid org unit ID.', zh: '请求失败（invalid org unit id）。' },
  invalid_request: { en: 'Request parameters are invalid. Please check and retry.', zh: '请求参数无效，请检查后重试。' },
  invalid_status: { en: 'Invalid status.', zh: '请求失败（invalid status）。' },
  invalid_target_effective_date: { en: 'Invalid target effective date.', zh: '请求失败（invalid target effective date）。' },
  jobcatalog_store_missing: { en: 'Jobcatalog store is missing.', zh: '请求失败（jobcatalog store missing）。' },
  method_not_allowed: { en: 'HTTP method is not allowed for this endpoint.', zh: '请求失败（method not allowed）。' },
  missing_assignment_uuid: { en: 'Missing assignment UUID.', zh: '请求失败（missing assignment uuid）。' },
  missing_person_uuid: { en: 'Missing person UUID.', zh: '请求失败（missing person uuid）。' },
  missing_target_effective_date: { en: 'Missing target effective date.', zh: '请求失败（missing target effective date）。' },
  not_found: { en: 'Requested resource is not found.', zh: '请求失败（not found）。' },
  org_code_invalid: { en: 'Org code is invalid.', zh: '请求失败（org code invalid）。' },
  org_code_not_found: { en: 'Org code not found.', zh: '请求失败（org code not found）。' },
  org_unit_not_found: { en: 'Org unit not found.', zh: '请求失败（org unit not found）。' },
  orgunit_id_invalid: { en: 'Orgunit ID is invalid.', zh: '请求失败（orgunit id invalid）。' },
  orgunit_resolve_org_code_failed: { en: 'Orgunit resolve org code failed.', zh: '请求失败（orgunit resolve org code failed）。' },
  orgunit_resolver_missing: { en: 'Orgunit resolver is missing.', zh: '请求失败（orgunit resolver missing）。' },
  orgunit_service_missing: { en: 'Orgunit service is missing.', zh: '请求失败（orgunit service missing）。' },
  orgunit_store_missing: { en: 'Orgunit store is missing.', zh: '请求失败（orgunit store missing）。' },
  principal_error: { en: 'Principal error.', zh: '请求失败（principal error）。' },
  principal_lookup_error: { en: 'Principal lookup error.', zh: '请求失败（principal lookup error）。' },
  session_error: { en: 'Session error.', zh: '请求失败（session error）。' },
  session_lookup_error: { en: 'Session lookup error.', zh: '请求失败（session lookup error）。' },
  setid_missing: { en: 'Setid is missing.', zh: '请求失败（setid missing）。' },
  setid_resolver_missing: { en: 'Setid resolver is missing.', zh: '请求失败（setid resolver missing）。' },
  setid_strategy_registry_list_failed: { en: 'Setid strategy registry list failed.', zh: '请求失败（setid strategy registry list failed）。' },
  setid_strategy_registry_upsert_failed: { en: 'Setid strategy registry upsert failed.', zh: '请求失败（setid strategy registry upsert failed）。' },
  tenant_missing: { en: 'Tenant context is missing. Please refresh and retry.', zh: '租户上下文缺失，请刷新后重试。' },
  tenant_not_found: { en: 'Tenant is not found. Please check the current host.', zh: '未找到租户，请检查当前访问域名。' },
  tenant_resolve_error: { en: 'Tenant resolution failed. Please retry later.', zh: '租户解析失败，请稍后重试。' },
  unauthorized: { en: 'Your session has expired. Please sign in again.', zh: '登录已失效，请重新登录。' },
  web_mui_index_missing: { en: 'Web mui index is missing.', zh: '请求失败（web mui index missing）。' },
  write_disabled: { en: 'Write disabled.', zh: '请求失败（write disabled）。' },
}

function shouldUseZh(): boolean {
  if (typeof navigator === 'undefined') {
    return true
  }
  const candidates = [navigator.language, ...navigator.languages].filter((item) => typeof item === 'string')
  return candidates.some((item) => item.toLowerCase().startsWith('zh'))
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

export function resolveApiErrorMessage(code: string | undefined, fallback: string): string {
  const normalizedCode = (code ?? '').trim()
  const explicit = normalizedCode.length > 0 ? localizedMessages[normalizedCode] : undefined
  if (explicit) {
    return shouldUseZh() ? explicit.zh : explicit.en
  }

  if (!isGenericMessage(normalizedCode, fallback)) {
    return fallback
  }

  const synthesized = sentenceFromCode(normalizedCode)
  if (synthesized.length > 0) {
    return shouldUseZh() ? `请求失败（${normalizedCode}）` : synthesized
  }
  return shouldUseZh() ? '请求失败，请稍后重试。' : 'Request failed. Please retry later.'
}
