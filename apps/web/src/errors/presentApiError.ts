type LocalizedErrorMessage = {
  en: string
  zh: string
}

const localizedMessages: Record<string, LocalizedErrorMessage> = {
  forbidden: { en: 'You do not have permission to perform this action.', zh: '你没有执行该操作的权限。' },
  unauthorized: { en: 'Your session has expired. Please sign in again.', zh: '登录已失效，请重新登录。' },
  invalid_request: { en: 'Request parameters are invalid. Please check and retry.', zh: '请求参数无效，请检查后重试。' },
  tenant_not_found: { en: 'Tenant is not found. Please check the current host.', zh: '未找到租户，请检查当前访问域名。' },
  tenant_missing: { en: 'Tenant context is missing. Please refresh and retry.', zh: '租户上下文缺失，请刷新后重试。' },
  ORG_ROOT_ALREADY_EXISTS: { en: 'Root org already exists. Please choose a parent org to create.', zh: '根组织已存在，请选择上级组织后再创建。' },
  ORG_TREE_NOT_INITIALIZED: { en: 'Org tree is not initialized. Please create the first root org.', zh: '组织树尚未初始化，请先创建首个根组织。' },
  ORG_ALREADY_EXISTS: { en: 'Org code already exists. Please use another code.', zh: '组织编码已存在，请使用其他编码。' },
  ORG_CODE_INVALID: { en: 'Org code is invalid.', zh: '组织编码格式无效。' },
  ORG_CODE_NOT_FOUND: { en: 'Org code is not found.', zh: '组织编码不存在。' },
  ORG_NOT_FOUND_AS_OF: { en: 'Target org is not found at the selected as-of date.', zh: '在当前查询时点未找到目标组织。' },
  ORG_EVENT_NOT_FOUND: { en: 'Target effective-date record is not found.', zh: '未找到目标生效日记录。' },
  ORG_EVENT_RESCINDED: { en: 'Target record has been rescinded.', zh: '目标记录已被撤销。' },
  PARENT_NOT_FOUND_AS_OF: { en: 'Parent org is not found at the selected as-of date.', zh: '在当前查询时点未找到上级组织。' },
  PATCH_FIELD_NOT_ALLOWED: { en: 'One or more fields are not allowed for this action.', zh: '当前动作不允许修改部分字段。' },
  PATCH_REQUIRED: { en: 'At least one field change is required.', zh: '至少需要提供一个变更字段。' },
  DEFAULT_RULE_REQUIRED: { en: 'This field is system-managed but no default rule is configured.', zh: '该字段为系统维护，但未配置默认规则。' },
  DEFAULT_RULE_EVAL_FAILED: { en: 'Default rule evaluation failed. Please check the rule.', zh: '默认规则执行失败，请检查规则配置。' },
  FIELD_POLICY_EXPR_INVALID: { en: 'Default rule expression is invalid.', zh: '默认规则表达式不合法。' },
  ORG_CODE_EXHAUSTED: { en: 'Org code space is exhausted. Please adjust the rule.', zh: '组织编码空间已耗尽，请调整规则。' },
  ORG_CODE_CONFLICT: { en: 'Org code conflict occurred. Please retry.', zh: '组织编码冲突，请重试。' },
  FIELD_NOT_MAINTAINABLE: { en: 'This field is system-managed and cannot be edited manually.', zh: '该字段由系统维护，不能手动编辑。' }
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
