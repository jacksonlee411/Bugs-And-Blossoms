export interface AssistantIntentDraft {
  parent_ref_text?: string
  entity_name?: string
  effective_date?: string
}

export interface AssistantCandidateOption {
  candidate_id: string
  candidate_code?: string
  name?: string
  path?: string
}

const parentUnderRE = /在\s*(.+?)\s*(?:之)?下/
const deptNameByNameRE = /(?:名为|叫做|叫|名称为)\s*(.+?)\s*(?:的)?(?:部门|组织)/
const deptNameByCreateRE = /(?:新建|创建)\s*(?:一个|一条)?\s*(?:名为)?\s*([^\s，。,；;]+)\s*(?:的)?(?:部门|组织)?/
const dateISORE = /(20\d{2}-\d{2}-\d{2})/
const dateCNRE = /(20\d{2})年(\d{1,2})月(\d{1,2})日/
const executionConfirmRE = /(确认执行|确认提交|立即执行|执行吧|可以执行|同意执行|确认|提交|执行|yes|ok)/i

function trimText(value: string | undefined): string {
  return (value ?? '').trim()
}

function normalizeDate(year: string, month: string, day: string): string {
  const mm = month.padStart(2, '0')
  const dd = day.padStart(2, '0')
  return `${year}-${mm}-${dd}`
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.map((item) => item.trim()).filter((item) => item.length > 0)))
}

export function extractIntentDraftFromText(text: string): AssistantIntentDraft {
  const input = trimText(text)
  const draft: AssistantIntentDraft = {}
  const parentMatch = parentUnderRE.exec(input)
  if (parentMatch && parentMatch[1]) {
    draft.parent_ref_text = trimText(parentMatch[1])
  }

  const deptMatchByName = deptNameByNameRE.exec(input)
  if (deptMatchByName && deptMatchByName[1]) {
    draft.entity_name = trimText(deptMatchByName[1])
  } else {
    const deptMatchByCreate = deptNameByCreateRE.exec(input)
    if (deptMatchByCreate && deptMatchByCreate[1]) {
      draft.entity_name = trimText(deptMatchByCreate[1])
    }
  }

  const isoMatch = dateISORE.exec(input)
  if (isoMatch && isoMatch[1]) {
    draft.effective_date = trimText(isoMatch[1])
    return draft
  }

  const cnMatch = dateCNRE.exec(input)
  if (cnMatch && cnMatch[1] && cnMatch[2] && cnMatch[3]) {
    draft.effective_date = normalizeDate(cnMatch[1], cnMatch[2], cnMatch[3])
  }
  return draft
}

export function mergeIntentDraft(base: AssistantIntentDraft, supplement: AssistantIntentDraft): AssistantIntentDraft {
  const merged: AssistantIntentDraft = {
    parent_ref_text: trimText(supplement.parent_ref_text) || trimText(base.parent_ref_text),
    entity_name: trimText(supplement.entity_name) || trimText(base.entity_name),
    effective_date: trimText(supplement.effective_date) || trimText(base.effective_date)
  }
  return merged
}

export function hasCompleteCreateIntent(draft: AssistantIntentDraft): boolean {
  return (
    trimText(draft.parent_ref_text).length > 0 &&
    trimText(draft.entity_name).length > 0 &&
    trimText(draft.effective_date).length > 0
  )
}

export function composeCreateOrgUnitPrompt(draft: AssistantIntentDraft): string {
  if (!hasCompleteCreateIntent(draft)) {
    return ''
  }
  return `在${trimText(draft.parent_ref_text)}之下，新建一个名为${trimText(draft.entity_name)}的部门，成立日期是${trimText(draft.effective_date)}。`
}

export function looksLikeCreateOrgUnitRequest(text: string): boolean {
  const input = trimText(text)
  if (input.length === 0) {
    return false
  }
  if (parentUnderRE.test(input) || deptNameByNameRE.test(input) || deptNameByCreateRE.test(input)) {
    return true
  }
  return /(新建|创建|部门|组织|orgunit)/i.test(input)
}

export function isExecutionConfirmationText(text: string): boolean {
  return executionConfirmRE.test(trimText(text))
}

function resolveCandidateByIndex(input: string, candidates: AssistantCandidateOption[]): string {
  if (candidates.length === 0) {
    return ''
  }
  const indexMatch = input.match(/(?:候选|选|第)?\s*(\d{1,2})\s*(?:个|号)?/)
  if (!indexMatch || !indexMatch[1]) {
    return ''
  }
  const index = Number(indexMatch[1])
  if (!Number.isInteger(index) || index <= 0 || index > candidates.length) {
    return ''
  }
  return candidates[index - 1]?.candidate_id ?? ''
}

export function resolveCandidateFromInput(inputText: string, candidates: AssistantCandidateOption[]): string {
  const input = trimText(inputText)
  if (input.length === 0 || candidates.length === 0) {
    return ''
  }

  const normalizedInput = input.toLowerCase()
  const exactMatches = uniqueStrings(
    candidates
      .filter((candidate) => {
        const values = [candidate.candidate_id, candidate.candidate_code, candidate.name, candidate.path]
        return values.some((value) => trimText(value).toLowerCase() === normalizedInput)
      })
      .map((candidate) => candidate.candidate_id)
  )
  if (exactMatches.length === 1) {
    return exactMatches[0] ?? ''
  }

  for (const candidate of candidates) {
    const values = [candidate.candidate_id, candidate.candidate_code, candidate.name, candidate.path]
    for (const value of values) {
      const normalizedValue = trimText(value).toLowerCase()
      if (normalizedValue.length > 0 && normalizedInput.includes(normalizedValue)) {
        return candidate.candidate_id
      }
    }
  }

  const indexChoice = resolveCandidateByIndex(input, candidates)
  if (indexChoice.length > 0) {
    return indexChoice
  }

  return ''
}

export function formatCandidatePrompt(candidates: AssistantCandidateOption[]): string {
  if (!Array.isArray(candidates) || candidates.length === 0) {
    return '未检测到可选候选，请补充上级组织名称后重试。'
  }
  const lines = candidates.map((candidate, index) => {
    const code = trimText(candidate.candidate_code)
    const name = trimText(candidate.name)
    const path = trimText(candidate.path)
    const label = [name || candidate.candidate_id, code].filter((item) => item.length > 0).join(' / ')
    if (path.length > 0) {
      return `${index + 1}. ${label} (${path})`
    }
    return `${index + 1}. ${label}`
  })
  return `检测到多个上级组织候选，请在对话中回复候选编号或编码：\n${lines.join('\n')}`
}
