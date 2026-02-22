import type { DictReleasePreviewResponse } from '../../api/dicts'

export const GLOBAL_TENANT_ID = '00000000-0000-0000-0000-000000000000'

export type DictReleaseStage = 'idle' | 'previewing' | 'conflict' | 'ready' | 'releasing' | 'success' | 'fail'

export type DictReleaseValidationIssue =
  | 'dict_release_error_source_tenant_invalid'
  | 'dict_release_error_as_of_required'
  | 'dict_release_error_release_id_required'
  | 'dict_release_error_request_id_required'
  | 'dict_release_error_max_conflicts_invalid'

export interface DictReleaseFormValues {
  sourceTenantID: string
  asOf: string
  releaseID: string
  requestID: string
  maxConflicts: string
}

export function isValidDateYYYYMMDD(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value)
}

export function isUUID(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)
}

export function toMaxConflicts(raw: string): number {
  const trimmed = raw.trim()
  if (trimmed.length === 0) {
    return 200
  }
  const parsed = Number.parseInt(trimmed, 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 200
  }
  return parsed
}

export function nextStageAfterPreview(preview: DictReleasePreviewResponse): DictReleaseStage {
  const hasConflict =
    preview.missing_dict_count > 0 ||
    preview.dict_name_mismatch_count > 0 ||
    preview.missing_value_count > 0 ||
    preview.value_label_mismatch_count > 0 ||
    preview.conflicts.length > 0
  return hasConflict ? 'conflict' : 'ready'
}

export function validatePreviewForm(values: DictReleaseFormValues): DictReleaseValidationIssue[] {
  const issues: DictReleaseValidationIssue[] = []
  if (!isUUID(values.sourceTenantID.trim())) {
    issues.push('dict_release_error_source_tenant_invalid')
  }
  if (!isValidDateYYYYMMDD(values.asOf.trim())) {
    issues.push('dict_release_error_as_of_required')
  }
  if (values.releaseID.trim().length === 0) {
    issues.push('dict_release_error_release_id_required')
  }
  const trimmedMaxConflicts = values.maxConflicts.trim()
  if (trimmedMaxConflicts.length > 0) {
    const parsed = Number.parseInt(trimmedMaxConflicts, 10)
    if (!Number.isFinite(parsed) || parsed <= 0) {
      issues.push('dict_release_error_max_conflicts_invalid')
    }
  }
  return issues
}

export function validateReleaseForm(values: DictReleaseFormValues): DictReleaseValidationIssue[] {
  const issues = validatePreviewForm(values)
  if (values.requestID.trim().length === 0) {
    issues.push('dict_release_error_request_id_required')
  }
  return issues
}
