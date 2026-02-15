import { useCallback, useMemo, useState } from 'react'
import { Link as RouterLink, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Breadcrumbs,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  Link,
  MenuItem,
  Paper,
  Select,
  Snackbar,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef } from '@mui/x-data-grid'
import {
  disableOrgUnitFieldConfig,
  enableOrgUnitFieldConfig,
  listOrgUnitFieldConfigs,
  listOrgUnitFieldDefinitions,
  type OrgUnitFieldDefinition,
  type OrgUnitTenantFieldConfig
} from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { isMessageKey } from '../../i18n/messages'

type FieldConfigListStatus = 'all' | 'enabled' | 'disabled'
type DisabledState = 'all' | 'pending' | 'disabled'
type RowState = 'enabled' | 'pending' | 'disabled'

interface FieldConfigRow {
  id: string
  fieldKey: string
  fieldLabel: string
  valueType: string
  dataSourceType: string
  dataSourceConfig: Record<string, unknown>
  physicalCol: string
  enabledOn: string
  disabledOn: string | null
  updatedAt: string
  state: RowState
}

interface EnableFormState {
  fieldKey: string
  enabledOn: string
  dataSourceConfigOption: string
}

interface DisableFormState {
  disabledOn: string
}

interface SelectedConfigState {
  mode: 'disable' | 'postpone'
  row: FieldConfigRow
}

function FieldConfigsFilterBar(props: {
  asOf: string
  status: FieldConfigListStatus
  disabledState: DisabledState
  keyword: string
  t: ReturnType<typeof useAppPreferences>['t']
  onApply: (value: { asOf: string; status: FieldConfigListStatus; disabledState: DisabledState; keyword: string }) => void
}) {
  const { t } = props
  const [asOfInput, setAsOfInput] = useState(props.asOf)
  const [statusInput, setStatusInput] = useState<FieldConfigListStatus>(props.status)
  const [disabledStateInput, setDisabledStateInput] = useState<DisabledState>(props.disabledState)
  const [keywordInput, setKeywordInput] = useState(props.keyword)

  return (
    <FilterBar>
      <TextField
        InputLabelProps={{ shrink: true }}
        label={t('org_field_configs_filter_as_of')}
        onChange={(event) => setAsOfInput(event.target.value)}
        type='date'
        value={asOfInput}
      />
      <FormControl sx={{ minWidth: 160 }}>
        <InputLabel id='org-field-configs-status-filter'>{t('org_field_configs_filter_status')}</InputLabel>
        <Select
          label={t('org_field_configs_filter_status')}
          labelId='org-field-configs-status-filter'
          onChange={(event) => {
            const next = String(event.target.value) as FieldConfigListStatus
            setStatusInput(next)
            if (next !== 'disabled') {
              setDisabledStateInput('all')
            }
          }}
          value={statusInput}
        >
          <MenuItem value='all'>{t('status_all')}</MenuItem>
          <MenuItem value='enabled'>{t('org_field_configs_state_enabled')}</MenuItem>
          <MenuItem value='disabled'>{t('org_field_configs_state_disabled_bucket')}</MenuItem>
        </Select>
      </FormControl>
      {statusInput === 'disabled' ? (
        <FormControl sx={{ minWidth: 160 }}>
          <InputLabel id='org-field-configs-disabled-state-filter'>{t('org_field_configs_filter_disabled_state')}</InputLabel>
          <Select
            label={t('org_field_configs_filter_disabled_state')}
            labelId='org-field-configs-disabled-state-filter'
            onChange={(event) => setDisabledStateInput(String(event.target.value) as DisabledState)}
            value={disabledStateInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='pending'>{t('org_field_configs_state_pending')}</MenuItem>
            <MenuItem value='disabled'>{t('org_field_configs_state_disabled')}</MenuItem>
          </Select>
        </FormControl>
      ) : null}
      <TextField
        fullWidth
        label={t('org_field_configs_filter_keyword')}
        onChange={(event) => setKeywordInput(event.target.value)}
        value={keywordInput}
      />
      <Button
        onClick={() =>
          props.onApply({
            asOf: asOfInput,
            status: statusInput,
            disabledState: statusInput === 'disabled' ? disabledStateInput : 'all',
            keyword: keywordInput
          })
        }
        variant='contained'
      >
        {t('action_apply_filters')}
      </Button>
    </FilterBar>
  )
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

function parseListStatus(raw: string | null): FieldConfigListStatus {
  const value = (raw ?? '').trim().toLowerCase()
  if (value === 'enabled' || value === 'disabled') {
    return value
  }
  return 'all'
}

function parseDisabledState(raw: string | null): DisabledState {
  const value = (raw ?? '').trim().toLowerCase()
  if (value === 'pending' || value === 'disabled') {
    return value
  }
  return 'all'
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function maxDay(a: string, b: string): string {
  return a > b ? a : b
}

function addUtcDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

function newRequestCode(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `req-${Date.now()}`
}

function toRowState(config: OrgUnitTenantFieldConfig, asOf: string): RowState {
  if (asOf < config.enabled_on) {
    return 'pending'
  }
  if (config.disabled_on && config.disabled_on <= asOf) {
    return 'disabled'
  }
  return 'enabled'
}

function formatDataSourceConfigSummary(config: FieldConfigRow): string {
  const cfg = config.dataSourceConfig ?? {}
  if (config.dataSourceType === 'DICT') {
    const code = typeof cfg.dict_code === 'string' ? cfg.dict_code : ''
    return code.length > 0 ? `dict:${code}` : '-'
  }
  if (config.dataSourceType === 'ENTITY') {
    const entity = typeof cfg.entity === 'string' ? cfg.entity : ''
    const idKind = typeof cfg.id_kind === 'string' ? cfg.id_kind : ''
    const left = entity.length > 0 ? entity : '-'
    const right = idKind.length > 0 ? idKind : '-'
    return `entity:${left}/${right}`
  }
  if (config.dataSourceType === 'PLAIN') {
    return '{}'
  }
  return '-'
}

function resolveFieldLabel(
  t: ReturnType<typeof useAppPreferences>['t'],
  defByKey: Map<string, OrgUnitFieldDefinition>,
  fieldKey: string
): string {
  const def = defByKey.get(fieldKey)
  if (!def) {
    return fieldKey
  }
  const key = def.label_i18n_key?.trim() ?? ''
  if (key.length === 0 || !isMessageKey(key)) {
    return fieldKey
  }
  return t(key)
}

export function OrgUnitFieldConfigsPage() {
  const queryClient = useQueryClient()
  const { t, tenantId } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => formatAsOfDate(new Date()), [])
  const todayUtc = useMemo(() => formatAsOfDate(new Date()), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const status = parseListStatus(searchParams.get('status'))
  const disabledState = parseDisabledState(searchParams.get('disabled_state'))
  const keyword = (searchParams.get('keyword') ?? '').trim()

  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)

  const [enableOpen, setEnableOpen] = useState(false)
  const [enableForm, setEnableForm] = useState<EnableFormState>(() => ({
    fieldKey: '',
    enabledOn: todayUtc,
    dataSourceConfigOption: ''
  }))
  const [enableError, setEnableError] = useState('')
  const [enableRequestCode, setEnableRequestCode] = useState(() => newRequestCode())

  const [selectedConfig, setSelectedConfig] = useState<SelectedConfigState | null>(null)
  const [disableForm, setDisableForm] = useState<DisableFormState>({ disabledOn: todayUtc })
  const [disableError, setDisableError] = useState('')
  const [disableRequestCode, setDisableRequestCode] = useState(() => newRequestCode())

  const [viewRow, setViewRow] = useState<FieldConfigRow | null>(null)

  const updateSearch = useCallback(
    (options: {
      asOf?: string | null
      status?: FieldConfigListStatus | null
      disabledState?: DisabledState | null
      keyword?: string | null
    }) => {
      const nextParams = new URLSearchParams(searchParams)

      if (Object.hasOwn(options, 'asOf')) {
        const value = (options.asOf ?? '').trim()
        if (value.length > 0) {
          nextParams.set('as_of', value)
        } else {
          nextParams.delete('as_of')
        }
      }

      if (Object.hasOwn(options, 'status')) {
        const value = options.status ?? 'all'
        if (value === 'all') {
          nextParams.delete('status')
        } else {
          nextParams.set('status', value)
        }
      }

      if (Object.hasOwn(options, 'disabledState')) {
        const value = options.disabledState ?? 'all'
        if (value === 'all') {
          nextParams.delete('disabled_state')
        } else {
          nextParams.set('disabled_state', value)
        }
      }

      if (Object.hasOwn(options, 'keyword')) {
        const value = (options.keyword ?? '').trim()
        if (value.length > 0) {
          nextParams.set('keyword', value)
        } else {
          nextParams.delete('keyword')
        }
      }

      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  const fieldDefinitionsQuery = useQuery({
    queryKey: ['org-field-definitions'],
    queryFn: () => listOrgUnitFieldDefinitions(),
    staleTime: 60_000
  })

  const fieldConfigsQuery = useQuery({
    queryKey: ['org-field-configs', asOf],
    queryFn: () => listOrgUnitFieldConfigs({ asOf, status: 'all' }),
    staleTime: 10_000
  })

  const fieldDefinitions = useMemo(() => fieldDefinitionsQuery.data?.fields ?? [], [fieldDefinitionsQuery.data])
  const fieldConfigs = useMemo(() => fieldConfigsQuery.data?.field_configs ?? [], [fieldConfigsQuery.data])

  const fieldDefinitionByKey = useMemo(() => {
    const map = new Map<string, OrgUnitFieldDefinition>()
    fieldDefinitions.forEach((def) => {
      map.set(def.field_key, def)
    })
    return map
  }, [fieldDefinitions])

  const rowsAll = useMemo<FieldConfigRow[]>(() => {
    return fieldConfigs.map((cfg) => {
      const label = resolveFieldLabel(t, fieldDefinitionByKey, cfg.field_key)
      return {
        id: cfg.field_key,
        fieldKey: cfg.field_key,
        fieldLabel: label,
        valueType: cfg.value_type,
        dataSourceType: cfg.data_source_type,
        dataSourceConfig: cfg.data_source_config ?? {},
        physicalCol: cfg.physical_col,
        enabledOn: cfg.enabled_on,
        disabledOn: cfg.disabled_on,
        updatedAt: cfg.updated_at,
        state: toRowState(cfg, asOf)
      }
    })
  }, [asOf, fieldConfigs, fieldDefinitionByKey, t])

  const rowsFiltered = useMemo(() => {
    let next = rowsAll
    if (status === 'enabled') {
      next = next.filter((row) => row.state === 'enabled')
    } else if (status === 'disabled') {
      next = next.filter((row) => row.state === 'pending' || row.state === 'disabled')
      if (disabledState === 'pending') {
        next = next.filter((row) => row.state === 'pending')
      } else if (disabledState === 'disabled') {
        next = next.filter((row) => row.state === 'disabled')
      }
    }

    const q = keyword.trim().toLowerCase()
    if (q.length > 0) {
      next = next.filter((row) => {
        if (row.fieldKey.toLowerCase().includes(q)) {
          return true
        }
        if (row.fieldLabel.toLowerCase().includes(q)) {
          return true
        }
        return false
      })
    }
    return next
  }, [disabledState, keyword, rowsAll, status])

  const existingFieldKeys = useMemo(() => new Set(rowsAll.map((row) => row.fieldKey)), [rowsAll])

  const availableDefinitions = useMemo(() => {
    return fieldDefinitions.filter((def) => !existingFieldKeys.has(def.field_key))
  }, [existingFieldKeys, fieldDefinitions])

  const enableMutation = useMutation({
    mutationFn: (req: { field_key: string; enabled_on: string; request_code: string; data_source_config?: Record<string, unknown> }) =>
      enableOrgUnitFieldConfig(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['org-field-configs'] })
    }
  })

  const disableMutation = useMutation({
    mutationFn: (req: { field_key: string; disabled_on: string; request_code: string }) => disableOrgUnitFieldConfig(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['org-field-configs'] })
    }
  })

  const requestErrorMessage = fieldDefinitionsQuery.error
    ? getErrorMessage(fieldDefinitionsQuery.error)
    : fieldConfigsQuery.error
    ? getErrorMessage(fieldConfigsQuery.error)
    : ''

  const tableLoading = fieldDefinitionsQuery.isLoading || fieldConfigsQuery.isFetching

  const openDisableDialog = useCallback(
    (mode: 'disable' | 'postpone', row: FieldConfigRow) => {
      setDisableError('')
      setSelectedConfig({ mode, row })
      const base = maxDay(todayUtc, asOf)
      if (mode === 'disable') {
        setDisableForm({ disabledOn: maxDay(base, row.enabledOn) })
      } else {
        const old = row.disabledOn ?? base
        const next = maxDay(maxDay(addUtcDays(old, 1), base), row.enabledOn)
        setDisableForm({ disabledOn: next })
      }
      setDisableRequestCode(newRequestCode())
    },
    [asOf, todayUtc]
  )

  const columns = useMemo<GridColDef<FieldConfigRow>[]>(() => {
    return [
      {
        field: 'fieldLabel',
        headerName: t('org_field_configs_column_label'),
        minWidth: 200,
        flex: 1.2
      },
      {
        field: 'fieldKey',
        headerName: t('org_field_configs_column_key'),
        minWidth: 160,
        flex: 1
      },
      {
        field: 'valueType',
        headerName: t('org_field_configs_column_value_type'),
        minWidth: 120,
        flex: 0.7
      },
      {
        field: 'dataSourceType',
        headerName: t('org_field_configs_column_data_source_type'),
        minWidth: 120,
        flex: 0.8
      },
      {
        field: 'dataSourceConfig',
        headerName: t('org_field_configs_column_data_source_config'),
        minWidth: 200,
        flex: 1.1,
        sortable: false,
        renderCell: (params) => {
          return (
            <Typography component='span' sx={{ fontFamily: 'monospace', fontSize: 12 }}>
              {formatDataSourceConfigSummary(params.row)}
            </Typography>
          )
        }
      },
      {
        field: 'state',
        headerName: t('text_status'),
        minWidth: 140,
        flex: 0.8,
        sortable: false,
        renderCell: (params) => {
          const state = params.row.state
          if (state === 'enabled') {
            return <StatusChip color='success' label={t('org_field_configs_state_enabled')} />
          }
          if (state === 'pending') {
            return <StatusChip color='info' label={t('org_field_configs_state_pending')} />
          }
          return <StatusChip color='warning' label={t('org_field_configs_state_disabled')} />
        }
      },
      {
        field: 'enabledOn',
        headerName: t('org_field_configs_column_enabled_on'),
        minWidth: 120,
        flex: 0.8
      },
      {
        field: 'disabledOn',
        headerName: t('org_field_configs_column_disabled_on'),
        minWidth: 120,
        flex: 0.8,
        renderCell: (params) => params.row.disabledOn ?? '-'
      },
      {
        field: 'physicalCol',
        headerName: t('org_field_configs_column_physical_col'),
        minWidth: 140,
        flex: 0.9
      },
      {
        field: 'updatedAt',
        headerName: t('org_field_configs_column_updated_at'),
        minWidth: 180,
        flex: 1,
        renderCell: (params) => params.row.updatedAt ?? '-'
      },
      {
        field: 'actions',
        headerName: t('org_field_configs_column_actions'),
        minWidth: 220,
        flex: 1,
        sortable: false,
        filterable: false,
        disableColumnMenu: true,
        renderCell: (params) => {
          const row = params.row
          const canDisable = row.disabledOn == null
          const canPostpone = row.disabledOn != null && todayUtc < row.disabledOn
          return (
            <Stack direction='row' spacing={1}>
              <Button onClick={() => setViewRow(row)} size='small' variant='text'>
                {t('common_detail')}
              </Button>
              <Button
                disabled={!canDisable}
                onClick={() => openDisableDialog('disable', row)}
                size='small'
                variant='text'
              >
                {t('org_field_configs_action_disable')}
              </Button>
              <Button
                disabled={!canPostpone}
                onClick={() => openDisableDialog('postpone', row)}
                size='small'
                variant='text'
              >
                {t('org_field_configs_action_postpone')}
              </Button>
            </Stack>
          )
        }
      }
    ]
  }, [openDisableDialog, t, todayUtc])

  function openEnableDialog() {
    setEnableError('')
    setEnableOpen(true)
    setEnableForm({
      fieldKey: '',
      enabledOn: maxDay(todayUtc, asOf),
      dataSourceConfigOption: ''
    })
    setEnableRequestCode(newRequestCode())
  }

  function closeEnableDialog() {
    setEnableOpen(false)
    setEnableError('')
  }

  const selectedDefinition = useMemo(() => {
    const key = enableForm.fieldKey.trim()
    if (key.length === 0) {
      return null
    }
    return fieldDefinitionByKey.get(key) ?? null
  }, [enableForm.fieldKey, fieldDefinitionByKey])

  async function submitEnable() {
    setEnableError('')
    const fieldKey = enableForm.fieldKey.trim()
    const enabledOn = enableForm.enabledOn.trim()
    if (fieldKey.length === 0 || enabledOn.length === 0) {
      setEnableError(t('org_field_configs_error_required'))
      return
    }
    if (!/^\d{4}-\d{2}-\d{2}$/.test(enabledOn)) {
      setEnableError(t('org_field_configs_error_invalid_date'))
      return
    }
    const def = fieldDefinitionByKey.get(fieldKey)
    if (!def) {
      setEnableError(t('org_field_configs_error_definition_missing'))
      return
    }
    if (existingFieldKeys.has(fieldKey)) {
      setEnableError(t('org_field_configs_error_already_exists'))
      return
    }

    const dataSourceType = String(def.data_source_type ?? '').toUpperCase()
    let dataSourceConfig: Record<string, unknown> | undefined
    if (dataSourceType === 'DICT' || dataSourceType === 'ENTITY') {
      const raw = enableForm.dataSourceConfigOption.trim()
      if (raw.length === 0) {
        setEnableError(t('org_field_configs_error_data_source_config_required'))
        return
      }
      try {
        dataSourceConfig = JSON.parse(raw) as Record<string, unknown>
      } catch {
        setEnableError(t('org_field_configs_error_data_source_config_required'))
        return
      }
    }

    try {
      await enableMutation.mutateAsync({
        field_key: fieldKey,
        enabled_on: enabledOn,
        request_code: enableRequestCode,
        data_source_config: dataSourceConfig
      })
      setToast({ message: t('org_field_configs_toast_enable_success'), severity: 'success' })
      closeEnableDialog()
    } catch (error) {
      setEnableError(getErrorMessage(error))
    }
  }

  function closeDisableDialog() {
    setSelectedConfig(null)
    setDisableError('')
  }

  async function submitDisable() {
    if (!selectedConfig) {
      return
    }
    setDisableError('')
    const disabledOn = disableForm.disabledOn.trim()
    if (!/^\d{4}-\d{2}-\d{2}$/.test(disabledOn)) {
      setDisableError(t('org_field_configs_error_invalid_date'))
      return
    }

    const row = selectedConfig.row
    if (disabledOn < todayUtc) {
      setDisableError(t('org_field_configs_error_disabled_on_backdated'))
      return
    }
    if (disabledOn < row.enabledOn) {
      setDisableError(t('org_field_configs_error_disabled_on_before_enabled_on'))
      return
    }

    if (selectedConfig.mode === 'postpone') {
      const old = row.disabledOn
      if (!old) {
        setDisableError(t('org_field_configs_error_postpone_requires_old'))
        return
      }
      if (!(todayUtc < old)) {
        setDisableError(t('org_field_configs_error_postpone_already_effective'))
        return
      }
      if (disabledOn <= old) {
        setDisableError(t('org_field_configs_error_postpone_not_later'))
        return
      }
    }

    try {
      await disableMutation.mutateAsync({
        field_key: row.fieldKey,
        disabled_on: disabledOn,
        request_code: disableRequestCode
      })
      setToast({ message: t('org_field_configs_toast_disable_success'), severity: 'success' })
      closeDisableDialog()
    } catch (error) {
      setDisableError(getErrorMessage(error))
    }
  }

  const listLinkSearch = useMemo(() => {
    const params = new URLSearchParams()
    params.set('as_of', asOf)
    return params.toString().length > 0 ? `?${params.toString()}` : ''
  }, [asOf])

  const pageSubtitle = t('org_field_configs_subtitle')

  return (
    <>
      <Breadcrumbs sx={{ mb: 1 }}>
        <Link component={RouterLink} to={{ pathname: '/org/units', search: listLinkSearch }} underline='hover' color='inherit'>
          {t('nav_org_units')}
        </Link>
        <Typography color='text.primary'>{t('org_field_configs_title')}</Typography>
      </Breadcrumbs>

      <PageHeader
        title={t('org_field_configs_title')}
        subtitle={pageSubtitle}
        actions={
          <Button onClick={openEnableDialog} size='small' variant='contained'>
            {t('org_field_configs_action_enable')}
          </Button>
        }
      />

      <FieldConfigsFilterBar
        asOf={asOf}
        disabledState={disabledState}
        keyword={keyword}
        key={`${asOf}|${status}|${disabledState}|${keyword}`}
        onApply={(next) => updateSearch(next)}
        status={status}
        t={t}
      />

      {requestErrorMessage.length > 0 ? (
        <Alert severity='error' sx={{ mb: 2 }}>
          {requestErrorMessage}
        </Alert>
      ) : null}

      {!tableLoading && requestErrorMessage.length === 0 && rowsFiltered.length === 0 ? (
        <Paper sx={{ mb: 2, p: 2 }} variant='outlined'>
          <Stack direction={{ md: 'row', xs: 'column' }} spacing={2} alignItems={{ md: 'center', xs: 'flex-start' }}>
            <Typography color='text.secondary' variant='body2'>
              {t('org_field_configs_empty')}
            </Typography>
            <Button onClick={openEnableDialog} size='small' variant='contained'>
              {t('org_field_configs_action_enable')}
            </Button>
          </Stack>
        </Paper>
      ) : null}

      <DataGridPage
        columns={columns}
        gridProps={{
          onRowDoubleClick: (params) => {
            setViewRow(params.row as FieldConfigRow)
          },
          pageSizeOptions: [10, 20, 50],
          pagination: true,
          initialState: { pagination: { paginationModel: { page: 0, pageSize: 20 } } },
          sx: { minHeight: 560 }
        }}
        loading={tableLoading}
        loadingLabel={t('text_loading')}
        noRowsLabel={t('text_no_data')}
        rows={rowsFiltered}
        storageKey={`org-field-configs-grid/${tenantId}`}
      />

      <Dialog onClose={closeEnableDialog} open={enableOpen} fullWidth maxWidth='sm'>
        <DialogTitle>{t('org_field_configs_action_enable')}</DialogTitle>
        <DialogContent>
          {enableError.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {enableError}
            </Alert>
          ) : null}
          <Stack spacing={2} sx={{ mt: 0.5 }}>
            <FormControl>
              <InputLabel id='org-field-key-select-label'>{t('org_field_configs_form_field_key')}</InputLabel>
              <Select
                label={t('org_field_configs_form_field_key')}
                labelId='org-field-key-select-label'
                onChange={(event) => {
                  const nextFieldKey = String(event.target.value)
                  setEnableRequestCode(newRequestCode())
                  const def = fieldDefinitionByKey.get(nextFieldKey)
                  const dataSourceType = String(def?.data_source_type ?? '').toUpperCase()
                  let nextOption = ''
                  const options = def?.data_source_config_options ?? []
                  if ((dataSourceType === 'DICT' || dataSourceType === 'ENTITY') && options.length === 1) {
                    nextOption = JSON.stringify(options[0] ?? {})
                  }
                  setEnableForm((previous) => ({
                    ...previous,
                    fieldKey: nextFieldKey,
                    dataSourceConfigOption: nextOption
                  }))
                }}
                value={enableForm.fieldKey}
              >
                {availableDefinitions.map((def) => (
                  <MenuItem key={def.field_key} value={def.field_key}>
                    {resolveFieldLabel(t, fieldDefinitionByKey, def.field_key)} ({def.field_key})
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              InputLabelProps={{ shrink: true }}
              label={t('org_field_configs_form_enabled_on')}
              onChange={(event) => {
                setEnableRequestCode(newRequestCode())
                setEnableForm((previous) => ({ ...previous, enabledOn: event.target.value }))
              }}
              type='date'
              value={enableForm.enabledOn}
            />
            {selectedDefinition && (String(selectedDefinition.data_source_type ?? '').toUpperCase() === 'DICT' || String(selectedDefinition.data_source_type ?? '').toUpperCase() === 'ENTITY') ? (
              <FormControl>
                <InputLabel id='org-field-configs-data-source-config-label'>{t('org_field_configs_form_data_source_config')}</InputLabel>
                <Select
                  label={t('org_field_configs_form_data_source_config')}
                  labelId='org-field-configs-data-source-config-label'
                  onChange={(event) => {
                    setEnableRequestCode(newRequestCode())
                    setEnableForm((previous) => ({ ...previous, dataSourceConfigOption: String(event.target.value) }))
                  }}
                  value={enableForm.dataSourceConfigOption}
                >
                  {(selectedDefinition.data_source_config_options ?? []).map((opt, idx) => {
                    const raw = JSON.stringify(opt ?? {})
                    return (
                      <MenuItem key={`${raw}-${idx}`} value={raw}>
                        <Typography component='span' sx={{ fontFamily: 'monospace', fontSize: 13 }}>
                          {raw}
                        </Typography>
                      </MenuItem>
                    )
                  })}
                </Select>
              </FormControl>
            ) : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeEnableDialog}>{t('common_cancel')}</Button>
          <Button disabled={enableMutation.isPending} onClick={() => void submitEnable()} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog onClose={closeDisableDialog} open={selectedConfig != null} fullWidth maxWidth='sm'>
        <DialogTitle>
          {selectedConfig?.mode === 'postpone' ? t('org_field_configs_action_postpone') : t('org_field_configs_action_disable')}
        </DialogTitle>
        <DialogContent>
          {disableError.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {disableError}
            </Alert>
          ) : null}
          {selectedConfig ? (
            <Stack spacing={2} sx={{ mt: 0.5 }}>
              <Alert severity='warning'>
                {t('org_field_configs_disable_warning')}
              </Alert>
              <Typography variant='body2'>
                <strong>{selectedConfig.row.fieldLabel}</strong> ({selectedConfig.row.fieldKey}) · {selectedConfig.row.physicalCol}
              </Typography>
              {selectedConfig.mode === 'postpone' && selectedConfig.row.disabledOn ? (
                <Typography color='text.secondary' variant='body2'>
                  {t('org_field_configs_current_disabled_on', { date: selectedConfig.row.disabledOn })}
                </Typography>
              ) : null}
              <TextField
                InputLabelProps={{ shrink: true }}
                label={t('org_field_configs_form_disabled_on')}
                onChange={(event) => {
                  setDisableRequestCode(newRequestCode())
                  setDisableForm({ disabledOn: event.target.value })
                }}
                type='date'
                value={disableForm.disabledOn}
              />
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDisableDialog}>{t('common_cancel')}</Button>
          <Button disabled={disableMutation.isPending} onClick={() => void submitDisable()} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog onClose={() => setViewRow(null)} open={viewRow != null} fullWidth maxWidth='md'>
        <DialogTitle>{t('common_detail')}</DialogTitle>
        <DialogContent>
          {viewRow ? (
            <Stack spacing={1.5} sx={{ mt: 0.5 }}>
              <Typography variant='body2'>
                <strong>{viewRow.fieldLabel}</strong> ({viewRow.fieldKey})
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_value_type')}: <code>{viewRow.valueType}</code> · {t('org_field_configs_column_data_source_type')}: <code>{viewRow.dataSourceType}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_physical_col')}: <code>{viewRow.physicalCol}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_enabled_on')}: <code>{viewRow.enabledOn}</code> · {t('org_field_configs_column_disabled_on')}: <code>{viewRow.disabledOn ?? '-'}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_data_source_config')}:
              </Typography>
              <Paper sx={{ p: 1.5 }} variant='outlined'>
                <Typography component='pre' sx={{ fontFamily: 'monospace', fontSize: 12, m: 0, whiteSpace: 'pre-wrap' }}>
                  {JSON.stringify(viewRow.dataSourceConfig ?? {}, null, 2)}
                </Typography>
              </Paper>
              <Typography variant='body2'>
                {t('org_field_configs_column_updated_at')}: <code>{viewRow.updatedAt}</code>
              </Typography>
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setViewRow(null)}>{t('common_confirm')}</Button>
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
