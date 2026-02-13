import { useCallback, useMemo, useState } from 'react'
import { Link as RouterLink, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Link,
  List,
  ListItemButton,
  ListItemText,
  Snackbar,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  correctOrgUnit,
  disableOrgUnit,
  enableOrgUnit,
  getOrgUnitDetails,
  listOrgUnitAudit,
  listOrgUnitVersions,
  moveOrgUnit,
  renameOrgUnit,
  rescindOrgUnit,
  rescindOrgUnitRecord,
  setOrgUnitBusinessUnit
} from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { PageHeader } from '../../components/PageHeader'
import type { MessageKey } from '../../i18n/messages'

type DetailTab = 'profile' | 'records' | 'audit'
type OrgStatus = 'active' | 'inactive'
type OrgActionType =
  | 'rename'
  | 'move'
  | 'set_business_unit'
  | 'enable'
  | 'disable'
  | 'correct'
  | 'rescind_record'
  | 'rescind_org'

const detailTabs: readonly DetailTab[] = ['profile', 'records', 'audit']

interface OrgActionState {
  type: OrgActionType
  targetCode: string | null
}

interface OrgActionForm {
  orgCode: string
  name: string
  parentOrgCode: string
  managerPernr: string
  effectiveDate: string
  correctedEffectiveDate: string
  isBusinessUnit: boolean
  requestId: string
  requestCode: string
  reason: string
}

function formatAsOfDate(date: Date): string {
  return date.toISOString().slice(0, 10)
}

function parseDateOrDefault(raw: string | null, fallback: string): string {
  if (!raw) {
    return fallback
  }
  const value = raw.trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return fallback
  }
  return value
}

function parseOptionalValue(raw: string | null): string | null {
  if (!raw) {
    return null
  }
  const value = raw.trim()
  if (value.length === 0) {
    return null
  }
  return value
}

function parseBool(raw: string | null): boolean {
  if (!raw) {
    return false
  }
  const value = raw.trim().toLowerCase()
  return value === '1' || value === 'true' || value === 'yes' || value === 'on'
}

function parseDetailTab(raw: string | null): DetailTab {
  if (raw && detailTabs.includes(raw as DetailTab)) {
    return raw as DetailTab
  }
  return 'profile'
}

function parseOrgStatus(raw: string): OrgStatus {
  const value = raw.trim().toLowerCase()
  return value === 'disabled' || value === 'inactive' ? 'inactive' : 'active'
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function newRequestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `org-${Date.now()}`
}

function emptyActionForm(effectiveDate: string): OrgActionForm {
  return {
    orgCode: '',
    name: '',
    parentOrgCode: '',
    managerPernr: '',
    effectiveDate,
    correctedEffectiveDate: '',
    isBusinessUnit: false,
    requestId: newRequestID(),
    requestCode: `req-${Date.now()}`,
    reason: ''
  }
}

function actionLabel(type: OrgActionType, t: (key: MessageKey) => string): string {
  switch (type) {
    case 'rename':
      return t('org_action_rename')
    case 'move':
      return t('org_action_move')
    case 'set_business_unit':
      return t('org_action_set_business_unit')
    case 'enable':
      return t('org_action_enable')
    case 'disable':
      return t('org_action_disable')
    case 'correct':
      return t('org_action_correct')
    case 'rescind_record':
      return t('org_action_rescind_record')
    case 'rescind_org':
      return t('org_action_rescind_org')
    default:
      return ''
  }
}

export function OrgUnitDetailsPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { orgCode } = useParams()
  const { hasPermission, t } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => formatAsOfDate(new Date()), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const includeDisabled = parseBool(searchParams.get('include_disabled'))
  const detailTab = parseDetailTab(searchParams.get('tab'))
  const effectiveDateParam = parseOptionalValue(searchParams.get('effective_date'))
  const effectiveDate = parseDateOrDefault(effectiveDateParam, asOf)

  const canWrite = hasPermission('orgunit.admin')
  const orgCodeValue = (orgCode ?? '').trim()

  const [actionState, setActionState] = useState<OrgActionState | null>(null)
  const [actionForm, setActionForm] = useState<OrgActionForm>(() => emptyActionForm(effectiveDate))
  const [actionErrorMessage, setActionErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)
  const [auditLimitByOrg, setAuditLimitByOrg] = useState<Record<string, number>>({})
  const auditLimit = auditLimitByOrg[orgCodeValue] ?? 20

  const updateSearch = useCallback(
    (options: {
      asOf?: string | null
      includeDisabled?: boolean
      effectiveDate?: string | null
      tab?: DetailTab | null
    }) => {
      const nextParams = new URLSearchParams(searchParams)

      if (Object.hasOwn(options, 'asOf')) {
        if (options.asOf && options.asOf.length > 0) {
          nextParams.set('as_of', options.asOf)
        } else {
          nextParams.delete('as_of')
        }
      }

      if (Object.hasOwn(options, 'includeDisabled')) {
        if (options.includeDisabled) {
          nextParams.set('include_disabled', '1')
        } else {
          nextParams.delete('include_disabled')
        }
      }

      if (Object.hasOwn(options, 'effectiveDate')) {
        if (options.effectiveDate && options.effectiveDate.length > 0) {
          nextParams.set('effective_date', options.effectiveDate)
        } else {
          nextParams.delete('effective_date')
        }
      }

      if (Object.hasOwn(options, 'tab')) {
        if (options.tab) {
          nextParams.set('tab', options.tab)
        } else {
          nextParams.delete('tab')
        }
      }

      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  const detailQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'details', orgCodeValue, effectiveDate, includeDisabled],
    queryFn: () => getOrgUnitDetails({ orgCode: orgCodeValue, asOf: effectiveDate, includeDisabled })
  })

  const versionsQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'versions', orgCodeValue],
    queryFn: () => listOrgUnitVersions({ orgCode: orgCodeValue })
  })

  const auditQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'audit', orgCodeValue, auditLimit],
    queryFn: () => listOrgUnitAudit({ orgCode: orgCodeValue, limit: auditLimit })
  })

  const refreshAfterWrite = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['org-units'] })
  }, [queryClient])

  function openAction(type: OrgActionType) {
    const details = detailQuery.data?.org_unit
    const form = emptyActionForm(effectiveDate)
    form.orgCode = orgCodeValue
    form.name = details?.name ?? ''
    form.parentOrgCode = details?.parent_org_code ?? ''
    form.managerPernr = details?.manager_pernr ?? ''
    form.isBusinessUnit = details?.is_business_unit ?? false

    if (type === 'rescind_record') {
      form.effectiveDate = effectiveDate
    }

    setActionErrorMessage('')
    setActionForm(form)
    setActionState({ type, targetCode: orgCodeValue.length > 0 ? orgCodeValue : null })
  }

  const actionMutation = useMutation({
    mutationFn: async () => {
      if (!actionState) {
        return
      }

      const type = actionState.type
      const targetCode = actionForm.orgCode.trim()
      if (!targetCode) {
        throw new Error(t('org_action_target_required'))
      }

      const effectiveDateValue = actionForm.effectiveDate.trim() || effectiveDate

      switch (type) {
        case 'rename':
          await renameOrgUnit({
            org_code: targetCode,
            new_name: actionForm.name.trim(),
            effective_date: effectiveDateValue
          })
          return
        case 'move':
          await moveOrgUnit({
            org_code: targetCode,
            new_parent_org_code: actionForm.parentOrgCode.trim(),
            effective_date: effectiveDateValue
          })
          return
        case 'set_business_unit':
          await setOrgUnitBusinessUnit({
            org_code: targetCode,
            effective_date: effectiveDateValue,
            is_business_unit: actionForm.isBusinessUnit,
            request_code: actionForm.requestCode.trim()
          })
          return
        case 'enable':
          await enableOrgUnit({ org_code: targetCode, effective_date: effectiveDateValue })
          return
        case 'disable':
          await disableOrgUnit({ org_code: targetCode, effective_date: effectiveDateValue })
          return
        case 'correct': {
          const patch: {
            effective_date?: string
            name?: string
            parent_org_code?: string
            manager_pernr?: string
            is_business_unit?: boolean
          } = {}
          if (actionForm.correctedEffectiveDate.trim().length > 0) {
            patch.effective_date = actionForm.correctedEffectiveDate.trim()
          }
          if (actionForm.name.trim().length > 0) {
            patch.name = actionForm.name.trim()
          }
          if (actionForm.parentOrgCode.trim().length > 0) {
            patch.parent_org_code = actionForm.parentOrgCode.trim()
          }
          if (actionForm.managerPernr.trim().length > 0) {
            patch.manager_pernr = actionForm.managerPernr.trim()
          }
          patch.is_business_unit = actionForm.isBusinessUnit

          await correctOrgUnit({
            org_code: targetCode,
            effective_date: effectiveDateValue,
            request_id: actionForm.requestId.trim(),
            patch
          })
          return
        }
        case 'rescind_record':
          await rescindOrgUnitRecord({
            org_code: targetCode,
            effective_date: effectiveDateValue,
            request_id: actionForm.requestId.trim(),
            reason: actionForm.reason.trim()
          })
          return
        case 'rescind_org':
          await rescindOrgUnit({
            org_code: targetCode,
            request_id: actionForm.requestId.trim(),
            reason: actionForm.reason.trim()
          })
          return
      }
    },
    onSuccess: async () => {
      await refreshAfterWrite()
      setActionState(null)
      setToast({ message: t('common_action_done'), severity: 'success' })
    },
    onError: (error) => {
      setActionErrorMessage(getErrorMessage(error))
    }
  })

  const titleLabel = useMemo(() => {
    const details = detailQuery.data?.org_unit
    if (!details) {
      return orgCodeValue.length > 0 ? `${orgCodeValue} · ${t('common_detail')}` : t('common_detail')
    }
    return `${details.name} · ${t('org_detail_title_suffix')}`
  }, [detailQuery.data, orgCodeValue, t])

  const breadcrumbCurrentLabel = useMemo(() => {
    const details = detailQuery.data?.org_unit
    if (details) {
      return `${details.name} (${details.org_code})`
    }
    return orgCodeValue.length > 0 ? orgCodeValue : t('common_detail')
  }, [detailQuery.data, orgCodeValue, t])

  const listLinkSearch = useMemo(() => {
    const params = new URLSearchParams()
    params.set('as_of', asOf)
    if (includeDisabled) {
      params.set('include_disabled', '1')
    }
    const value = params.toString()
    return value.length > 0 ? `?${value}` : ''
  }, [asOf, includeDisabled])

  const handleBack = useCallback(() => {
    if (window.history.length <= 1) {
      navigate({ pathname: '/org/units', search: listLinkSearch })
      return
    }
    navigate(-1)
  }, [listLinkSearch, navigate])

  const statusLabel = useMemo(() => {
    const raw = detailQuery.data?.org_unit?.status ?? ''
    return parseOrgStatus(raw) === 'active' ? t('status_active_short') : t('status_inactive_short')
  }, [detailQuery.data, t])

  if (orgCodeValue.length === 0) {
    return (
      <Alert severity='error'>
        {t('org_action_target_required')}
      </Alert>
    )
  }

  return (
    <>
      <Breadcrumbs sx={{ mb: 1 }}>
        <Link component={RouterLink} to={{ pathname: '/org/units', search: listLinkSearch }} underline='hover' color='inherit'>
          {t('nav_org_units')}
        </Link>
        <Typography color='text.primary'>{breadcrumbCurrentLabel}</Typography>
      </Breadcrumbs>

      <PageHeader
        title={titleLabel}
        actions={
          <>
            <Button disabled={!canWrite} onClick={() => openAction('rename')} size='small' variant='outlined'>
              {t('org_action_rename')}
            </Button>
            <Button disabled={!canWrite} onClick={() => openAction('move')} size='small' variant='outlined'>
              {t('org_action_move')}
            </Button>
            <Button disabled={!canWrite} onClick={() => openAction('set_business_unit')} size='small' variant='outlined'>
              {t('org_action_set_business_unit')}
            </Button>
          </>
        }
      />

      <Tabs
        onChange={(_, value: DetailTab) => updateSearch({ tab: value })}
        sx={{ mb: 1 }}
        value={detailTab}
      >
        <Tab label={t('org_tab_profile')} value='profile' />
        <Tab label={t('org_tab_records')} value='records' />
        <Tab label={t('org_tab_audit')} value='audit' />
      </Tabs>

      {detailQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
      {detailQuery.error ? <Alert severity='error'>{getErrorMessage(detailQuery.error)}</Alert> : null}

      {detailQuery.data && detailTab === 'profile' ? (
        <Stack spacing={1.2}>
          <Typography>{t('org_column_code')}：{detailQuery.data.org_unit.org_code}</Typography>
          <Typography>{t('org_column_name')}：{detailQuery.data.org_unit.name}</Typography>
          <Typography>{t('org_column_parent')}：{detailQuery.data.org_unit.parent_org_code || '-'}</Typography>
          <Typography>{t('org_column_manager')}：{detailQuery.data.org_unit.manager_pernr || '-'}</Typography>
          <Typography>{t('org_column_is_business_unit')}：{detailQuery.data.org_unit.is_business_unit ? t('common_yes') : t('common_no')}</Typography>
          <Typography>
            {t('text_status')}：{statusLabel}
          </Typography>
          <Stack direction='row' flexWrap='wrap' spacing={1}>
            <Button disabled={!canWrite} onClick={() => openAction('enable')} size='small' variant='outlined'>
              {t('org_action_enable')}
            </Button>
            <Button disabled={!canWrite} onClick={() => openAction('disable')} size='small' variant='outlined'>
              {t('org_action_disable')}
            </Button>
            <Button disabled={!canWrite} onClick={() => openAction('correct')} size='small' variant='outlined'>
              {t('org_action_correct')}
            </Button>
            <Button disabled={!canWrite} onClick={() => openAction('rescind_record')} size='small' variant='outlined'>
              {t('org_action_rescind_record')}
            </Button>
            <Button color='error' disabled={!canWrite} onClick={() => openAction('rescind_org')} size='small' variant='outlined'>
              {t('org_action_rescind_org')}
            </Button>
          </Stack>
        </Stack>
      ) : null}

      {detailTab === 'records' ? (
        <Stack spacing={1}>
          {versionsQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
          {versionsQuery.error ? <Alert severity='error'>{getErrorMessage(versionsQuery.error)}</Alert> : null}
          {versionsQuery.data ? (
            <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 320, overflow: 'auto' }}>
              {versionsQuery.data.versions.map((version) => (
                <ListItemButton
                  key={`${version.event_id}-${version.effective_date}`}
                  onClick={() => updateSearch({ effectiveDate: version.effective_date, tab: 'records' })}
                  selected={effectiveDate === version.effective_date}
                >
                  <ListItemText
                    primary={version.effective_date}
                    secondary={`${t('org_version_event_type')}：${version.event_type}`}
                  />
                </ListItemButton>
              ))}
            </List>
          ) : null}
        </Stack>
      ) : null}

      {detailTab === 'audit' ? (
        <Stack spacing={1}>
          {auditQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
          {auditQuery.error ? <Alert severity='error'>{getErrorMessage(auditQuery.error)}</Alert> : null}
          {auditQuery.data ? (
            <>
              <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 360, overflow: 'auto' }}>
                {auditQuery.data.events.map((event) => (
                  <ListItemButton key={event.event_id}>
                    <ListItemText
                      primary={`${event.effective_date} · ${event.event_type}`}
                      secondary={`${event.initiator_name || '-'} / ${event.request_code || '-'}`}
                    />
                  </ListItemButton>
                ))}
              </List>
              {auditQuery.data.has_more ? (
                <Button
                  onClick={() =>
                    setAuditLimitByOrg((previous) => ({
                      ...previous,
                      [orgCodeValue]: (previous[orgCodeValue] ?? 20) + 20
                    }))
                  }
                  size='small'
                  variant='outlined'
                >
                  {t('org_audit_load_more')}
                </Button>
              ) : null}
            </>
          ) : null}
        </Stack>
      ) : null}

      <Box sx={{ mt: 3 }}>
        <Button onClick={handleBack} variant='outlined'>
          {t('common_back')}
        </Button>
      </Box>

      <Dialog onClose={() => setActionState(null)} open={actionState !== null} fullWidth maxWidth='sm'>
        <DialogTitle>
          {actionState ? actionLabel(actionState.type, t) : ''}
        </DialogTitle>
        <DialogContent>
          {actionErrorMessage.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {actionErrorMessage}
            </Alert>
          ) : null}
          {actionState ? (
            <Stack spacing={2} sx={{ mt: 0.5 }}>
              <TextField disabled label={t('org_column_code')} value={actionForm.orgCode} />

              {actionState.type === 'rename' || actionState.type === 'correct' ? (
                <TextField
                  label={t('org_column_name')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, name: event.target.value }))}
                  value={actionForm.name}
                />
              ) : null}

              {actionState.type === 'move' || actionState.type === 'correct' ? (
                <TextField
                  label={t('org_column_parent')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, parentOrgCode: event.target.value }))}
                  value={actionForm.parentOrgCode}
                />
              ) : null}

              {actionState.type === 'correct' ? (
                <TextField
                  label={t('org_column_manager')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
                  value={actionForm.managerPernr}
                />
              ) : null}

              {actionState.type === 'set_business_unit' || actionState.type === 'correct' ? (
                <FormControlLabel
                  control={
                    <Switch
                      checked={actionForm.isBusinessUnit}
                      onChange={(event) => setActionForm((previous) => ({ ...previous, isBusinessUnit: event.target.checked }))}
                    />
                  }
                  label={t('org_column_is_business_unit')}
                />
              ) : null}

              {actionState.type !== 'rescind_org' ? (
                <TextField
                  InputLabelProps={{ shrink: true }}
                  label={t('org_column_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, effectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.effectiveDate}
                />
              ) : null}

              {actionState.type === 'correct' ? (
                <TextField
                  InputLabelProps={{ shrink: true }}
                  label={t('org_corrected_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, correctedEffectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.correctedEffectiveDate}
                />
              ) : null}

              {actionState.type === 'set_business_unit' ? (
                <TextField
                  label={t('org_request_code')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, requestCode: event.target.value }))}
                  value={actionForm.requestCode}
                />
              ) : null}

              {actionState.type === 'correct' || actionState.type === 'rescind_record' || actionState.type === 'rescind_org' ? (
                <TextField
                  label={t('org_request_id')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, requestId: event.target.value }))}
                  value={actionForm.requestId}
                />
              ) : null}

              {actionState.type === 'rescind_record' || actionState.type === 'rescind_org' ? (
                <TextField
                  label={t('org_reason')}
                  multiline
                  onChange={(event) => setActionForm((previous) => ({ ...previous, reason: event.target.value }))}
                  rows={3}
                  value={actionForm.reason}
                />
              ) : null}
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setActionState(null)}>{t('common_cancel')}</Button>
          <Button onClick={() => actionMutation.mutate()} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar autoHideDuration={2800} onClose={() => setToast(null)} open={toast !== null}>
        <Alert severity={toast?.severity ?? 'success'} variant='filled'>
          {toast?.message ?? ''}
        </Alert>
      </Snackbar>
    </>
  )
}
