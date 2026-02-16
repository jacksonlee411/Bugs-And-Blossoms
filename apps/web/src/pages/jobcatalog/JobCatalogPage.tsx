import { useCallback, useEffect, useMemo, useState } from 'react'
import MoreVertIcon from '@mui/icons-material/MoreVert'
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  IconButton,
  InputLabel,
  List,
  ListItemButton,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Select,
  Snackbar,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography
} from '@mui/material'
import { DatePicker } from '@mui/x-date-pickers/DatePicker'
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { format, isValid, parseISO } from 'date-fns'
import { useSearchParams } from 'react-router-dom'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import {
  applyJobCatalogAction,
  getJobCatalog,
  listOwnedJobCatalogPackages
} from '../../api/jobCatalog'
import { DataGridPage } from '../../components/DataGridPage'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'

type JobCatalogTab = 'groups' | 'families' | 'levels' | 'profiles'

interface GroupRow {
  id: string
  code: string
  name: string
  effectiveDay: string
  isActive: boolean
}

interface FamilyRow {
  id: string
  code: string
  name: string
  groupCode: string
  effectiveDay: string
  isActive: boolean
}

interface LevelRow {
  id: string
  code: string
  name: string
  effectiveDay: string
  isActive: boolean
}

interface ProfileRow {
  id: string
  code: string
  name: string
  familyCodesCSV: string
  primaryFamilyCode: string
  effectiveDay: string
  isActive: boolean
}

interface RowActionItem {
  key: string
  label: string
  onClick: () => void
  disabled?: boolean
}

interface DetailDialogState {
  title: string
  rows: Array<{ label: string; value: string }>
}

const TAB_VALUES: JobCatalogTab[] = ['groups', 'families', 'levels', 'profiles']
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

function todayISO(): string {
  return format(new Date(), 'yyyy-MM-dd')
}

function parseDateOrDefault(raw: string | null, fallback: string): string {
  if (!raw) {
    return fallback
  }
  const value = raw.trim()
  if (!DATE_PATTERN.test(value)) {
    return fallback
  }
  return value
}

function parseOptionalValue(raw: string | null): string {
  if (!raw) {
    return ''
  }
  return raw.trim()
}

function parseTabOrDefault(raw: string | null): JobCatalogTab {
  if (!raw) {
    return 'groups'
  }
  return TAB_VALUES.includes(raw as JobCatalogTab) ? (raw as JobCatalogTab) : 'groups'
}

function toDateValue(raw: string): Date | null {
  if (!DATE_PATTERN.test(raw)) {
    return null
  }
  const parsed = parseISO(raw)
  return isValid(parsed) ? parsed : null
}

function formatDateValue(value: Date | null): string | null {
  if (!value || !isValid(value)) {
    return null
  }
  return format(value, 'yyyy-MM-dd')
}

function normalizeCSV(raw: string): string[] {
  return raw
    .split(',')
    .map((part) => part.trim())
    .filter((part) => part.length > 0)
}

function includesKeyword(keyword: string, values: string[]): boolean {
  const normalized = keyword.trim().toLowerCase()
  if (normalized.length === 0) {
    return true
  }
  return values.some((value) => value.toLowerCase().includes(normalized))
}

function resolveErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function RowActionMenu({ buttonLabel, items }: { buttonLabel: string; items: RowActionItem[] }) {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const open = Boolean(anchorEl)

  return (
    <>
      <Tooltip title={buttonLabel}>
        <IconButton
          aria-label={buttonLabel}
          onClick={(event) => {
            event.stopPropagation()
            setAnchorEl(event.currentTarget)
          }}
          size='small'
        >
          <MoreVertIcon fontSize='small' />
        </IconButton>
      </Tooltip>
      <Menu
        anchorEl={anchorEl}
        onClick={(event) => event.stopPropagation()}
        onClose={() => setAnchorEl(null)}
        open={open}
      >
        {items.map((item) => (
          <MenuItem
            disabled={item.disabled}
            key={item.key}
            onClick={() => {
              setAnchorEl(null)
              item.onClick()
            }}
          >
            {item.label}
          </MenuItem>
        ))}
      </Menu>
    </>
  )
}

export function JobCatalogPage() {
  const { t } = useAppPreferences()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => todayISO(), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const packageCode = parseOptionalValue(searchParams.get('package_code'))
  const selectedTab = parseTabOrDefault(searchParams.get('tab'))
  const selectedGroupCode = parseOptionalValue(searchParams.get('group_code'))

  const [asOfInput, setAsOfInput] = useState<Date | null>(toDateValue(asOf))
  const [packageCodeInput, setPackageCodeInput] = useState(packageCode)
  const [groupKeywordInput, setGroupKeywordInput] = useState('')
  const [listKeywordInput, setListKeywordInput] = useState('')
  const [pageError, setPageError] = useState<string | null>(null)
  const [dialogError, setDialogError] = useState<string | null>(null)
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const [detailDialog, setDetailDialog] = useState<DetailDialogState | null>(null)

  const [createGroupDialog, setCreateGroupDialog] = useState({
    open: false,
    code: '',
    name: '',
    effectiveDate: null as Date | null
  })
  const [createFamilyDialog, setCreateFamilyDialog] = useState({
    open: false,
    code: '',
    name: '',
    groupCode: '',
    effectiveDate: null as Date | null
  })
  const [moveFamilyDialog, setMoveFamilyDialog] = useState({
    open: false,
    familyCode: '',
    groupCode: '',
    effectiveDate: null as Date | null
  })
  const [createLevelDialog, setCreateLevelDialog] = useState({
    open: false,
    code: '',
    name: '',
    effectiveDate: null as Date | null
  })
  const [createProfileDialog, setCreateProfileDialog] = useState({
    open: false,
    code: '',
    name: '',
    familyCodes: [] as string[],
    primaryFamilyCode: '',
    effectiveDate: null as Date | null
  })

  useEffect(() => {
    setAsOfInput(toDateValue(asOf))
  }, [asOf])

  useEffect(() => {
    setPackageCodeInput(packageCode)
  }, [packageCode])

  const updateQuery = useCallback(
    (patch: { asOf?: string; packageCode?: string | null; tab?: JobCatalogTab; groupCode?: string | null }) => {
      const nextParams = new URLSearchParams(searchParams)

      if (Object.hasOwn(patch, 'asOf') && patch.asOf) {
        nextParams.set('as_of', patch.asOf)
      }
      if (Object.hasOwn(patch, 'packageCode')) {
        if (patch.packageCode && patch.packageCode.trim().length > 0) {
          nextParams.set('package_code', patch.packageCode.trim())
        } else {
          nextParams.delete('package_code')
        }
      }
      if (Object.hasOwn(patch, 'tab') && patch.tab) {
        nextParams.set('tab', patch.tab)
      }
      if (Object.hasOwn(patch, 'groupCode')) {
        if (patch.groupCode && patch.groupCode.trim().length > 0) {
          nextParams.set('group_code', patch.groupCode.trim())
        } else {
          nextParams.delete('group_code')
        }
      }

      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  const packagesQuery = useQuery({
    queryKey: ['jobcatalog', 'owned-packages', asOf],
    queryFn: async () => listOwnedJobCatalogPackages({ asOf })
  })

  const catalogQuery = useQuery({
    enabled: packageCode.length > 0,
    queryKey: ['jobcatalog', 'catalog', asOf, packageCode],
    queryFn: async () => getJobCatalog({ asOf, packageCode })
  })

  const actionMutation = useMutation({
    mutationFn: applyJobCatalogAction,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['jobcatalog', 'catalog'] }),
        queryClient.invalidateQueries({ queryKey: ['jobcatalog', 'owned-packages'] })
      ])
    }
  })

  const ownedPackages = packagesQuery.data ?? []
  const catalog = catalogQuery.data ?? null
  const selectedOwnedPackage = ownedPackages.find((item) => item.package_code === packageCode) ?? null
  const ownerSetID = catalog?.view.owner_setid ?? selectedOwnedPackage?.owner_setid ?? '-'
  const isReadOnly = catalog?.view.read_only ?? false
  const disableWriteActions = isReadOnly || packageCode.trim().length === 0

  const groupRows = useMemo<GroupRow[]>(
    () =>
      (catalog?.job_family_groups ?? []).map((item) => ({
        id: item.job_family_group_uuid,
        code: item.job_family_group_code,
        name: item.name,
        effectiveDay: item.effective_day,
        isActive: item.is_active
      })),
    [catalog]
  )

  const familyRows = useMemo<FamilyRow[]>(
    () =>
      (catalog?.job_families ?? []).map((item) => ({
        id: item.job_family_uuid,
        code: item.job_family_code,
        name: item.name,
        groupCode: item.job_family_group_code,
        effectiveDay: item.effective_day,
        isActive: item.is_active
      })),
    [catalog]
  )

  const levelRows = useMemo<LevelRow[]>(
    () =>
      (catalog?.job_levels ?? []).map((item) => ({
        id: item.job_level_uuid,
        code: item.job_level_code,
        name: item.name,
        effectiveDay: item.effective_day,
        isActive: item.is_active
      })),
    [catalog]
  )

  const profileRows = useMemo<ProfileRow[]>(
    () =>
      (catalog?.job_profiles ?? []).map((item) => ({
        id: item.job_profile_uuid,
        code: item.job_profile_code,
        name: item.name,
        familyCodesCSV: item.family_codes_csv,
        primaryFamilyCode: item.primary_family_code,
        effectiveDay: item.effective_day,
        isActive: item.is_active
      })),
    [catalog]
  )

  const groupRowsForPanel = useMemo(
    () =>
      groupRows.filter((row) => includesKeyword(groupKeywordInput, [row.code, row.name, row.effectiveDay])),
    [groupKeywordInput, groupRows]
  )

  const selectedGroup = useMemo(
    () => groupRows.find((row) => row.code === selectedGroupCode) ?? null,
    [groupRows, selectedGroupCode]
  )

  useEffect(() => {
    if (selectedGroupCode.length > 0 && !selectedGroup) {
      updateQuery({ groupCode: null })
    }
  }, [selectedGroup, selectedGroupCode, updateQuery])

  const selectedGroupFamilyCodes = useMemo(() => {
    if (!selectedGroup) {
      return new Set<string>()
    }
    return new Set(
      familyRows.filter((row) => row.groupCode === selectedGroup.code).map((row) => row.code)
    )
  }, [familyRows, selectedGroup])

  const filteredGroupRows = useMemo(
    () => groupRows.filter((row) => includesKeyword(listKeywordInput, [row.code, row.name, row.effectiveDay])),
    [groupRows, listKeywordInput]
  )

  const filteredFamilyRows = useMemo(() => {
    if (!selectedGroup) {
      return [] as FamilyRow[]
    }
    return familyRows.filter((row) => {
      if (row.groupCode !== selectedGroup.code) {
        return false
      }
      return includesKeyword(listKeywordInput, [row.code, row.name, row.groupCode, row.effectiveDay])
    })
  }, [familyRows, listKeywordInput, selectedGroup])

  const filteredLevelRows = useMemo(
    () => levelRows.filter((row) => includesKeyword(listKeywordInput, [row.code, row.name, row.effectiveDay])),
    [levelRows, listKeywordInput]
  )

  const filteredProfileRows = useMemo(() => {
    if (!selectedGroup) {
      return [] as ProfileRow[]
    }
    return profileRows.filter((row) => {
      const familyCodes = normalizeCSV(row.familyCodesCSV)
      const hasMatchedFamily = familyCodes.some((code) => selectedGroupFamilyCodes.has(code))
      const hasPrimaryFamily = selectedGroupFamilyCodes.has(row.primaryFamilyCode)
      if (!hasMatchedFamily && !hasPrimaryFamily) {
        return false
      }
      return includesKeyword(listKeywordInput, [
        row.code,
        row.name,
        row.primaryFamilyCode,
        row.familyCodesCSV,
        row.effectiveDay
      ])
    })
  }, [listKeywordInput, profileRows, selectedGroup, selectedGroupFamilyCodes])

  const isCatalogLoading = catalogQuery.isLoading || catalogQuery.isFetching

  const groupColumns = useMemo<GridColDef<GroupRow>[]>(
    () => [
      { field: 'code', headerName: t('jobcatalog_column_code'), minWidth: 140, flex: 1 },
      { field: 'name', headerName: t('jobcatalog_column_name'), minWidth: 180, flex: 1.3 },
      { field: 'effectiveDay', headerName: t('jobcatalog_column_effective_day'), minWidth: 140, flex: 0.8 },
      {
        field: 'status',
        headerName: t('jobcatalog_column_status'),
        minWidth: 120,
        flex: 0.7,
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<GroupRow>) => (
          <StatusChip
            color={params.row.isActive ? 'success' : 'warning'}
            label={params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')}
          />
        )
      },
      {
        field: 'actions',
        headerName: t('jobcatalog_column_actions'),
        minWidth: 90,
        flex: 0.5,
        sortable: false,
        filterable: false,
        disableColumnMenu: true,
        renderCell: (params: GridRenderCellParams<GroupRow>) => (
          <RowActionMenu
            buttonLabel={t('jobcatalog_action_menu')}
            items={[
              {
                key: 'select-group',
                label: t('jobcatalog_action_select_group'),
                onClick: () => updateQuery({ groupCode: params.row.code })
              },
              {
                key: 'open-families',
                label: t('jobcatalog_action_open_families'),
                onClick: () => updateQuery({ tab: 'families', groupCode: params.row.code })
              }
            ]}
          />
        )
      }
    ],
    [t, updateQuery]
  )

  const familyColumns = useMemo<GridColDef<FamilyRow>[]>(
    () => [
      { field: 'code', headerName: t('jobcatalog_column_code'), minWidth: 140, flex: 1 },
      { field: 'name', headerName: t('jobcatalog_column_name'), minWidth: 180, flex: 1.3 },
      { field: 'groupCode', headerName: t('jobcatalog_column_group_code'), minWidth: 140, flex: 0.9 },
      { field: 'effectiveDay', headerName: t('jobcatalog_column_effective_day'), minWidth: 140, flex: 0.8 },
      {
        field: 'status',
        headerName: t('jobcatalog_column_status'),
        minWidth: 120,
        flex: 0.7,
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<FamilyRow>) => (
          <StatusChip
            color={params.row.isActive ? 'success' : 'warning'}
            label={params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')}
          />
        )
      },
      {
        field: 'actions',
        headerName: t('jobcatalog_column_actions'),
        minWidth: 90,
        flex: 0.5,
        sortable: false,
        filterable: false,
        disableColumnMenu: true,
        renderCell: (params: GridRenderCellParams<FamilyRow>) => (
          <RowActionMenu
            buttonLabel={t('jobcatalog_action_menu')}
            items={[
              {
                key: 'move-family',
                label: t('jobcatalog_action_move_family'),
                disabled: disableWriteActions,
                onClick: () => {
                  setDialogError(null)
                  setMoveFamilyDialog({
                    open: true,
                    familyCode: params.row.code,
                    groupCode: params.row.groupCode,
                    effectiveDate: toDateValue(asOf)
                  })
                }
              },
              {
                key: 'family-detail',
                label: t('common_detail'),
                onClick: () =>
                  setDetailDialog({
                    title: `${params.row.code} · ${params.row.name}`,
                    rows: [
                      { label: t('jobcatalog_detail_label_code'), value: params.row.code },
                      { label: t('jobcatalog_detail_label_name'), value: params.row.name },
                      { label: t('jobcatalog_detail_label_group_code'), value: params.row.groupCode },
                      { label: t('jobcatalog_detail_label_effective_day'), value: params.row.effectiveDay },
                      {
                        label: t('jobcatalog_detail_label_status'),
                        value: params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')
                      }
                    ]
                  })
              }
            ]}
          />
        )
      }
    ],
    [asOf, disableWriteActions, t]
  )

  const levelColumns = useMemo<GridColDef<LevelRow>[]>(
    () => [
      { field: 'code', headerName: t('jobcatalog_column_code'), minWidth: 140, flex: 1 },
      { field: 'name', headerName: t('jobcatalog_column_name'), minWidth: 180, flex: 1.3 },
      { field: 'effectiveDay', headerName: t('jobcatalog_column_effective_day'), minWidth: 140, flex: 0.8 },
      {
        field: 'status',
        headerName: t('jobcatalog_column_status'),
        minWidth: 120,
        flex: 0.7,
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<LevelRow>) => (
          <StatusChip
            color={params.row.isActive ? 'success' : 'warning'}
            label={params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')}
          />
        )
      },
      {
        field: 'actions',
        headerName: t('jobcatalog_column_actions'),
        minWidth: 90,
        flex: 0.5,
        sortable: false,
        filterable: false,
        disableColumnMenu: true,
        renderCell: (params: GridRenderCellParams<LevelRow>) => (
          <RowActionMenu
            buttonLabel={t('jobcatalog_action_menu')}
            items={[
              {
                key: 'level-detail',
                label: t('common_detail'),
                onClick: () =>
                  setDetailDialog({
                    title: `${params.row.code} · ${params.row.name}`,
                    rows: [
                      { label: t('jobcatalog_detail_label_code'), value: params.row.code },
                      { label: t('jobcatalog_detail_label_name'), value: params.row.name },
                      { label: t('jobcatalog_detail_label_effective_day'), value: params.row.effectiveDay },
                      {
                        label: t('jobcatalog_detail_label_status'),
                        value: params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')
                      }
                    ]
                  })
              }
            ]}
          />
        )
      }
    ],
    [t]
  )

  const profileColumns = useMemo<GridColDef<ProfileRow>[]>(
    () => [
      { field: 'code', headerName: t('jobcatalog_column_code'), minWidth: 140, flex: 1 },
      { field: 'name', headerName: t('jobcatalog_column_name'), minWidth: 180, flex: 1.2 },
      {
        field: 'primaryFamilyCode',
        headerName: t('jobcatalog_column_primary_family'),
        minWidth: 150,
        flex: 0.9
      },
      {
        field: 'familyCodesCSV',
        headerName: t('jobcatalog_column_family_codes'),
        minWidth: 220,
        flex: 1.2
      },
      { field: 'effectiveDay', headerName: t('jobcatalog_column_effective_day'), minWidth: 140, flex: 0.8 },
      {
        field: 'status',
        headerName: t('jobcatalog_column_status'),
        minWidth: 120,
        flex: 0.7,
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<ProfileRow>) => (
          <StatusChip
            color={params.row.isActive ? 'success' : 'warning'}
            label={params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')}
          />
        )
      },
      {
        field: 'actions',
        headerName: t('jobcatalog_column_actions'),
        minWidth: 90,
        flex: 0.5,
        sortable: false,
        filterable: false,
        disableColumnMenu: true,
        renderCell: (params: GridRenderCellParams<ProfileRow>) => (
          <RowActionMenu
            buttonLabel={t('jobcatalog_action_menu')}
            items={[
              {
                key: 'profile-detail',
                label: t('common_detail'),
                onClick: () =>
                  setDetailDialog({
                    title: `${params.row.code} · ${params.row.name}`,
                    rows: [
                      { label: t('jobcatalog_detail_label_code'), value: params.row.code },
                      { label: t('jobcatalog_detail_label_name'), value: params.row.name },
                      { label: t('jobcatalog_detail_label_primary_family'), value: params.row.primaryFamilyCode || '-' },
                      { label: t('jobcatalog_detail_label_family_codes'), value: params.row.familyCodesCSV || '-' },
                      { label: t('jobcatalog_detail_label_effective_day'), value: params.row.effectiveDay },
                      {
                        label: t('jobcatalog_detail_label_status'),
                        value: params.row.isActive ? t('jobcatalog_state_active') : t('jobcatalog_state_inactive')
                      }
                    ]
                  })
              }
            ]}
          />
        )
      }
    ],
    [t]
  )

  const familyOptionMap = useMemo(() => {
    const map = new Map<string, FamilyRow>()
    familyRows.forEach((row) => map.set(row.code, row))
    return map
  }, [familyRows])

  const selectedProfileFamilyOptions = useMemo(
    () => createProfileDialog.familyCodes.map((code) => familyOptionMap.get(code)).filter((row): row is FamilyRow => !!row),
    [createProfileDialog.familyCodes, familyOptionMap]
  )

  const profileFamilyOptions = useMemo(() => {
    if (selectedGroup) {
      return familyRows.filter((row) => row.groupCode === selectedGroup.code)
    }
    return familyRows
  }, [familyRows, selectedGroup])

  const applyContext = useCallback(() => {
    const nextAsOf = formatDateValue(asOfInput)
    if (!nextAsOf) {
      setPageError(t('jobcatalog_error_invalid_date'))
      return
    }
    setPageError(null)
    updateQuery({
      asOf: nextAsOf,
      packageCode: packageCodeInput,
      groupCode: selectedGroupCode.length > 0 ? selectedGroupCode : null
    })
  }, [asOfInput, packageCodeInput, selectedGroupCode, t, updateQuery])

  const resetContext = useCallback(() => {
    const defaultAsOf = todayISO()
    setAsOfInput(toDateValue(defaultAsOf))
    setPackageCodeInput('')
    setGroupKeywordInput('')
    setListKeywordInput('')
    setPageError(null)
    updateQuery({
      asOf: defaultAsOf,
      packageCode: null,
      groupCode: null
    })
  }, [updateQuery])

  async function submitAction(
    payload: Omit<Parameters<typeof applyJobCatalogAction>[0], 'package_code'>,
    successMessage: string
  ): Promise<{ ok: true } | { ok: false; message: string }> {
    if (packageCode.trim().length === 0) {
      const message = t('jobcatalog_error_package_required')
      setPageError(message)
      return { ok: false, message }
    }
    setPageError(null)
    try {
      await actionMutation.mutateAsync({
        ...payload,
        package_code: packageCode
      })
      setToastMessage(successMessage)
      return { ok: true }
    } catch (error) {
      const message = resolveErrorMessage(error)
      setPageError(message)
      return { ok: false, message }
    }
  }

  async function onSubmitCreateGroup() {
    if (createGroupDialog.code.trim().length === 0 || createGroupDialog.name.trim().length === 0) {
      setDialogError(t('jobcatalog_error_required_fields'))
      return
    }
    const effectiveDate = formatDateValue(createGroupDialog.effectiveDate)
    if (!effectiveDate) {
      setDialogError(t('jobcatalog_error_invalid_date'))
      return
    }
    const result = await submitAction(
      {
        action: 'create_job_family_group',
        effective_date: effectiveDate,
        code: createGroupDialog.code.trim(),
        name: createGroupDialog.name.trim()
      },
      t('jobcatalog_toast_create_group_success')
    )
    if (!result.ok) {
      setDialogError(result.message)
      return
    }
    setDialogError(null)
    setCreateGroupDialog({ open: false, code: '', name: '', effectiveDate: null })
  }

  async function onSubmitCreateFamily() {
    if (
      createFamilyDialog.code.trim().length === 0 ||
      createFamilyDialog.name.trim().length === 0 ||
      createFamilyDialog.groupCode.trim().length === 0
    ) {
      setDialogError(t('jobcatalog_error_required_fields'))
      return
    }
    const effectiveDate = formatDateValue(createFamilyDialog.effectiveDate)
    if (!effectiveDate) {
      setDialogError(t('jobcatalog_error_invalid_date'))
      return
    }
    const result = await submitAction(
      {
        action: 'create_job_family',
        effective_date: effectiveDate,
        code: createFamilyDialog.code.trim(),
        name: createFamilyDialog.name.trim(),
        group_code: createFamilyDialog.groupCode.trim()
      },
      t('jobcatalog_toast_create_family_success')
    )
    if (!result.ok) {
      setDialogError(result.message)
      return
    }
    setDialogError(null)
    setCreateFamilyDialog({ open: false, code: '', name: '', groupCode: '', effectiveDate: null })
  }

  async function onSubmitMoveFamily() {
    if (moveFamilyDialog.familyCode.trim().length === 0 || moveFamilyDialog.groupCode.trim().length === 0) {
      setDialogError(t('jobcatalog_error_required_fields'))
      return
    }
    const effectiveDate = formatDateValue(moveFamilyDialog.effectiveDate)
    if (!effectiveDate) {
      setDialogError(t('jobcatalog_error_invalid_date'))
      return
    }
    const result = await submitAction(
      {
        action: 'update_job_family_group',
        effective_date: effectiveDate,
        code: moveFamilyDialog.familyCode.trim(),
        group_code: moveFamilyDialog.groupCode.trim()
      },
      t('jobcatalog_toast_move_family_success')
    )
    if (!result.ok) {
      setDialogError(result.message)
      return
    }
    setDialogError(null)
    setMoveFamilyDialog({ open: false, familyCode: '', groupCode: '', effectiveDate: null })
  }

  async function onSubmitCreateLevel() {
    if (createLevelDialog.code.trim().length === 0 || createLevelDialog.name.trim().length === 0) {
      setDialogError(t('jobcatalog_error_required_fields'))
      return
    }
    const effectiveDate = formatDateValue(createLevelDialog.effectiveDate)
    if (!effectiveDate) {
      setDialogError(t('jobcatalog_error_invalid_date'))
      return
    }
    const result = await submitAction(
      {
        action: 'create_job_level',
        effective_date: effectiveDate,
        code: createLevelDialog.code.trim(),
        name: createLevelDialog.name.trim()
      },
      t('jobcatalog_toast_create_level_success')
    )
    if (!result.ok) {
      setDialogError(result.message)
      return
    }
    setDialogError(null)
    setCreateLevelDialog({ open: false, code: '', name: '', effectiveDate: null })
  }

  async function onSubmitCreateProfile() {
    if (createProfileDialog.code.trim().length === 0 || createProfileDialog.name.trim().length === 0) {
      setDialogError(t('jobcatalog_error_required_fields'))
      return
    }
    if (createProfileDialog.familyCodes.length === 0 || createProfileDialog.primaryFamilyCode.trim().length === 0) {
      setDialogError(t('jobcatalog_primary_family_required'))
      return
    }
    const effectiveDate = formatDateValue(createProfileDialog.effectiveDate)
    if (!effectiveDate) {
      setDialogError(t('jobcatalog_error_invalid_date'))
      return
    }
    const result = await submitAction(
      {
        action: 'create_job_profile',
        effective_date: effectiveDate,
        code: createProfileDialog.code.trim(),
        name: createProfileDialog.name.trim(),
        family_codes_csv: createProfileDialog.familyCodes.join(','),
        primary_family_code: createProfileDialog.primaryFamilyCode.trim()
      },
      t('jobcatalog_toast_create_profile_success')
    )
    if (!result.ok) {
      setDialogError(result.message)
      return
    }
    setDialogError(null)
    setCreateProfileDialog({
      open: false,
      code: '',
      name: '',
      familyCodes: [],
      primaryFamilyCode: '',
      effectiveDate: null
    })
  }

  const pendingStartIcon = actionMutation.isPending ? <CircularProgress color='inherit' size={14} /> : undefined

  const currentRowsCount =
    selectedTab === 'groups'
      ? filteredGroupRows.length
      : selectedTab === 'families'
        ? filteredFamilyRows.length
        : selectedTab === 'levels'
          ? filteredLevelRows.length
          : filteredProfileRows.length

  return (
    <Box>
      <PageHeader title={t('jobcatalog_page_title')} subtitle={t('jobcatalog_page_subtitle')} />

      <Stack spacing={2}>
        {pageError ? <Alert severity='error'>{pageError}</Alert> : null}
        {packagesQuery.isError ? <Alert severity='error'>{t('jobcatalog_error_owned_packages_load')}</Alert> : null}
        {catalogQuery.isError ? <Alert severity='error'>{t('jobcatalog_error_catalog_load')}</Alert> : null}

        <Box sx={{ position: 'sticky', top: 0, zIndex: 5 }}>
          <FilterBar>
            <DatePicker
              label={t('jobcatalog_filter_as_of')}
              onChange={(value) => setAsOfInput(value)}
              slotProps={{
                textField: {
                  fullWidth: true,
                  size: 'small'
                }
              }}
              value={asOfInput}
            />
            <Autocomplete
              freeSolo
              onChange={(_, value) => setPackageCodeInput(value ?? '')}
              onInputChange={(_, value) => setPackageCodeInput(value)}
              options={ownedPackages.map((item) => item.package_code)}
              renderInput={(params) => (
                <TextField
                  {...params}
                  fullWidth
                  label={t('jobcatalog_filter_package_code')}
                  size='small'
                />
              )}
              value={packageCodeInput}
            />
            <Button onClick={applyContext} variant='contained'>
              {t('jobcatalog_filter_apply_context')}
            </Button>
            <Button onClick={resetContext} variant='outlined'>
              {t('jobcatalog_filter_reset_context')}
            </Button>
          </FilterBar>

          <Paper sx={{ mb: 2, p: 1.5 }} variant='outlined'>
            <Stack direction='row' flexWrap='wrap' gap={1}>
              <Chip
                color='primary'
                label={`${t('jobcatalog_filter_as_of')}: ${asOf}`}
                size='small'
                variant='outlined'
              />
              <Chip
                color='primary'
                label={`${t('jobcatalog_filter_package_code')}: ${packageCode || t('jobcatalog_context_no_package')}`}
                size='small'
                variant='outlined'
              />
              <Chip
                label={`${t('jobcatalog_context_owner_setid')}: ${ownerSetID}`}
                size='small'
                variant='outlined'
              />
              <Chip
                color={isReadOnly ? 'warning' : 'success'}
                label={`${t('jobcatalog_context_read_only')}: ${
                  isReadOnly ? t('jobcatalog_context_read_only_true') : t('jobcatalog_context_read_only_false')
                }`}
                size='small'
                variant='outlined'
              />
            </Stack>
          </Paper>
        </Box>

        {packageCode.length === 0 ? (
          <Alert severity='info'>{t('jobcatalog_info_select_package')}</Alert>
        ) : (
          <>
            <Tabs onChange={(_, value: JobCatalogTab) => updateQuery({ tab: value })} value={selectedTab}>
              <Tab label={t('jobcatalog_tab_groups')} value='groups' />
              <Tab label={t('jobcatalog_tab_families')} value='families' />
              <Tab label={t('jobcatalog_tab_levels')} value='levels' />
              <Tab label={t('jobcatalog_tab_profiles')} value='profiles' />
            </Tabs>

            {(selectedTab === 'families' || selectedTab === 'profiles') ? (
              <Box
                sx={{
                  display: 'grid',
                  gap: 2,
                  gridTemplateColumns: {
                    xs: '1fr',
                    md: '280px minmax(0, 1fr)'
                  }
                }}
              >
                <Paper sx={{ p: 1.5 }} variant='outlined'>
                  <Stack spacing={1}>
                    <Typography variant='subtitle2'>{t('jobcatalog_group_panel_title')}</Typography>
                    <TextField
                      fullWidth
                      label={t('jobcatalog_filter_group_keyword')}
                      onChange={(event) => setGroupKeywordInput(event.target.value)}
                      size='small'
                      value={groupKeywordInput}
                    />
                    <Button
                      onClick={() => updateQuery({ groupCode: null })}
                      size='small'
                      variant='text'
                    >
                      {t('jobcatalog_group_panel_clear')}
                    </Button>
                    {groupRowsForPanel.length > 0 ? (
                      <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 440, overflow: 'auto', p: 0.5 }}>
                        {groupRowsForPanel.map((row) => (
                          <ListItemButton
                            key={row.id}
                            onClick={() => updateQuery({ groupCode: row.code })}
                            selected={selectedGroupCode === row.code}
                            sx={{ borderRadius: 1, mb: 0.5 }}
                          >
                            <ListItemText
                              primary={`${row.code} · ${row.name}`}
                              primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
                              secondary={row.effectiveDay}
                              secondaryTypographyProps={{ variant: 'caption' }}
                            />
                          </ListItemButton>
                        ))}
                      </List>
                    ) : (
                      <Typography color='text.secondary' variant='body2'>
                        {t('jobcatalog_group_panel_none')}
                      </Typography>
                    )}
                  </Stack>
                </Paper>

                <Stack spacing={1.5}>
                  <Paper sx={{ p: 1.5 }} variant='outlined'>
                    <Stack
                      alignItems={{ md: 'center', xs: 'flex-start' }}
                      direction={{ md: 'row', xs: 'column' }}
                      justifyContent='space-between'
                      spacing={1.5}
                    >
                      <Box>
                        <Typography variant='subtitle1'>
                          {selectedTab === 'families' ? t('jobcatalog_toolbar_families_title') : t('jobcatalog_toolbar_profiles_title')}
                          {selectedGroup ? ` · ${selectedGroup.code}` : ''}
                        </Typography>
                        <Typography color='text.secondary' variant='body2'>
                          {t('jobcatalog_toolbar_count', { count: currentRowsCount })}
                        </Typography>
                      </Box>
                      <Stack direction={{ sm: 'row', xs: 'column' }} spacing={1} sx={{ width: { sm: 'auto', xs: '100%' } }}>
                        <TextField
                          fullWidth
                          label={t('jobcatalog_filter_list_keyword')}
                          onChange={(event) => setListKeywordInput(event.target.value)}
                          size='small'
                          value={listKeywordInput}
                        />
                        <Button
                          disabled={disableWriteActions}
                          onClick={() => {
                            setDialogError(null)
                            if (selectedTab === 'families') {
                              setCreateFamilyDialog({
                                open: true,
                                code: '',
                                name: '',
                                groupCode: selectedGroupCode,
                                effectiveDate: toDateValue(asOf)
                              })
                              return
                            }
                            setCreateProfileDialog({
                              open: true,
                              code: '',
                              name: '',
                              familyCodes: [],
                              primaryFamilyCode: '',
                              effectiveDate: toDateValue(asOf)
                            })
                          }}
                          variant='contained'
                        >
                          {selectedTab === 'families' ? t('jobcatalog_action_create_family') : t('jobcatalog_action_create_profile')}
                        </Button>
                      </Stack>
                    </Stack>
                  </Paper>

                  {!selectedGroup ? (
                    <Alert severity='info'>{t('jobcatalog_info_select_group')}</Alert>
                  ) : (
                    <DataGridPage
                      columns={selectedTab === 'families' ? familyColumns : profileColumns}
                      gridProps={{ initialState: { density: 'compact' } }}
                      loading={isCatalogLoading}
                      noRowsLabel={selectedTab === 'families' ? t('jobcatalog_empty_families') : t('jobcatalog_empty_profiles')}
                      rows={selectedTab === 'families' ? filteredFamilyRows : filteredProfileRows}
                      storageKey={`jobcatalog-${selectedTab}-grid`}
                    />
                  )}
                </Stack>
              </Box>
            ) : (
              <Stack spacing={1.5}>
                <Paper sx={{ p: 1.5 }} variant='outlined'>
                  <Stack
                    alignItems={{ md: 'center', xs: 'flex-start' }}
                    direction={{ md: 'row', xs: 'column' }}
                    justifyContent='space-between'
                    spacing={1.5}
                  >
                    <Box>
                      <Typography variant='subtitle1'>
                        {selectedTab === 'groups' ? t('jobcatalog_toolbar_groups_title') : t('jobcatalog_toolbar_levels_title')}
                      </Typography>
                      <Typography color='text.secondary' variant='body2'>
                        {t('jobcatalog_toolbar_count', { count: currentRowsCount })}
                      </Typography>
                    </Box>
                    <Stack direction={{ sm: 'row', xs: 'column' }} spacing={1} sx={{ width: { sm: 'auto', xs: '100%' } }}>
                      <TextField
                        fullWidth
                        label={t('jobcatalog_filter_list_keyword')}
                        onChange={(event) => setListKeywordInput(event.target.value)}
                        size='small'
                        value={listKeywordInput}
                      />
                      <Button
                        disabled={disableWriteActions}
                        onClick={() => {
                          setDialogError(null)
                          if (selectedTab === 'groups') {
                            setCreateGroupDialog({
                              open: true,
                              code: '',
                              name: '',
                              effectiveDate: toDateValue(asOf)
                            })
                            return
                          }
                          setCreateLevelDialog({
                            open: true,
                            code: '',
                            name: '',
                            effectiveDate: toDateValue(asOf)
                          })
                        }}
                        variant='contained'
                      >
                        {selectedTab === 'groups' ? t('jobcatalog_action_create_group') : t('jobcatalog_action_create_level')}
                      </Button>
                    </Stack>
                  </Stack>
                </Paper>

                <DataGridPage
                  columns={selectedTab === 'groups' ? groupColumns : levelColumns}
                  gridProps={{ initialState: { density: 'compact' } }}
                  loading={isCatalogLoading}
                  noRowsLabel={selectedTab === 'groups' ? t('jobcatalog_empty_groups') : t('jobcatalog_empty_levels')}
                  rows={selectedTab === 'groups' ? filteredGroupRows : filteredLevelRows}
                  storageKey={`jobcatalog-${selectedTab}-grid`}
                />
              </Stack>
            )}
          </>
        )}
      </Stack>

      <Dialog
        fullWidth
        maxWidth='sm'
        onClose={() => {
          setCreateGroupDialog((previous) => ({ ...previous, open: false }))
          setDialogError(null)
        }}
        open={createGroupDialog.open}
      >
        <DialogTitle>{t('jobcatalog_dialog_create_group_title')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1.5} sx={{ mt: 0.5 }}>
            {dialogError ? <Alert severity='error'>{dialogError}</Alert> : null}
            <Typography color='text.secondary' variant='body2'>
              {t('jobcatalog_dialog_package_readonly', { packageCode: packageCode || '-' })}
            </Typography>
            <TextField
              fullWidth
              label={t('jobcatalog_form_code')}
              onChange={(event) =>
                setCreateGroupDialog((previous) => ({
                  ...previous,
                  code: event.target.value
                }))}
              value={createGroupDialog.code}
            />
            <TextField
              fullWidth
              label={t('jobcatalog_form_name')}
              onChange={(event) =>
                setCreateGroupDialog((previous) => ({
                  ...previous,
                  name: event.target.value
                }))}
              value={createGroupDialog.name}
            />
            <DatePicker
              label={t('jobcatalog_form_effective_date')}
              onChange={(value) =>
                setCreateGroupDialog((previous) => ({
                  ...previous,
                  effectiveDate: value
                }))}
              slotProps={{ textField: { fullWidth: true } }}
              value={createGroupDialog.effectiveDate}
            />
            <Alert severity='info'>
              {t('jobcatalog_dialog_effective_hint', {
                date: formatDateValue(createGroupDialog.effectiveDate) ?? '-'
              })}
            </Alert>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateGroupDialog((previous) => ({ ...previous, open: false }))} variant='text'>
            {t('common_cancel')}
          </Button>
          <Button
            disabled={actionMutation.isPending || disableWriteActions}
            onClick={() => void onSubmitCreateGroup()}
            startIcon={pendingStartIcon}
            variant='contained'
          >
            {t('jobcatalog_submit_create')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        fullWidth
        maxWidth='sm'
        onClose={() => {
          setCreateFamilyDialog((previous) => ({ ...previous, open: false }))
          setDialogError(null)
        }}
        open={createFamilyDialog.open}
      >
        <DialogTitle>{t('jobcatalog_dialog_create_family_title')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1.5} sx={{ mt: 0.5 }}>
            {dialogError ? <Alert severity='error'>{dialogError}</Alert> : null}
            <Typography color='text.secondary' variant='body2'>
              {t('jobcatalog_dialog_package_readonly', { packageCode: packageCode || '-' })}
            </Typography>
            <TextField
              fullWidth
              label={t('jobcatalog_form_code')}
              onChange={(event) =>
                setCreateFamilyDialog((previous) => ({
                  ...previous,
                  code: event.target.value
                }))}
              value={createFamilyDialog.code}
            />
            <TextField
              fullWidth
              label={t('jobcatalog_form_name')}
              onChange={(event) =>
                setCreateFamilyDialog((previous) => ({
                  ...previous,
                  name: event.target.value
                }))}
              value={createFamilyDialog.name}
            />
            <FormControl fullWidth>
              <InputLabel id='create-family-group-code'>{t('jobcatalog_form_group_code')}</InputLabel>
              <Select
                label={t('jobcatalog_form_group_code')}
                labelId='create-family-group-code'
                onChange={(event) =>
                  setCreateFamilyDialog((previous) => ({
                    ...previous,
                    groupCode: String(event.target.value)
                  }))}
                value={createFamilyDialog.groupCode}
              >
                {groupRows.map((row) => (
                  <MenuItem key={row.id} value={row.code}>
                    {`${row.code} · ${row.name}`}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <DatePicker
              label={t('jobcatalog_form_effective_date')}
              onChange={(value) =>
                setCreateFamilyDialog((previous) => ({
                  ...previous,
                  effectiveDate: value
                }))}
              slotProps={{ textField: { fullWidth: true } }}
              value={createFamilyDialog.effectiveDate}
            />
            <Alert severity='info'>
              {t('jobcatalog_dialog_effective_hint', {
                date: formatDateValue(createFamilyDialog.effectiveDate) ?? '-'
              })}
            </Alert>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateFamilyDialog((previous) => ({ ...previous, open: false }))} variant='text'>
            {t('common_cancel')}
          </Button>
          <Button
            disabled={actionMutation.isPending || disableWriteActions}
            onClick={() => void onSubmitCreateFamily()}
            startIcon={pendingStartIcon}
            variant='contained'
          >
            {t('jobcatalog_submit_create')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        fullWidth
        maxWidth='sm'
        onClose={() => {
          setMoveFamilyDialog((previous) => ({ ...previous, open: false }))
          setDialogError(null)
        }}
        open={moveFamilyDialog.open}
      >
        <DialogTitle>{t('jobcatalog_dialog_move_family_title')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1.5} sx={{ mt: 0.5 }}>
            {dialogError ? <Alert severity='error'>{dialogError}</Alert> : null}
            <Typography color='text.secondary' variant='body2'>
              {t('jobcatalog_dialog_package_readonly', { packageCode: packageCode || '-' })}
            </Typography>
            <TextField
              fullWidth
              label={t('jobcatalog_form_code')}
              slotProps={{ input: { readOnly: true } }}
              value={moveFamilyDialog.familyCode}
            />
            <FormControl fullWidth>
              <InputLabel id='move-family-group-code'>{t('jobcatalog_form_group_code')}</InputLabel>
              <Select
                label={t('jobcatalog_form_group_code')}
                labelId='move-family-group-code'
                onChange={(event) =>
                  setMoveFamilyDialog((previous) => ({
                    ...previous,
                    groupCode: String(event.target.value)
                  }))}
                value={moveFamilyDialog.groupCode}
              >
                {groupRows.map((row) => (
                  <MenuItem key={row.id} value={row.code}>
                    {`${row.code} · ${row.name}`}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <DatePicker
              label={t('jobcatalog_form_effective_date')}
              onChange={(value) =>
                setMoveFamilyDialog((previous) => ({
                  ...previous,
                  effectiveDate: value
                }))}
              slotProps={{ textField: { fullWidth: true } }}
              value={moveFamilyDialog.effectiveDate}
            />
            <Alert severity='info'>
              {t('jobcatalog_dialog_effective_hint', {
                date: formatDateValue(moveFamilyDialog.effectiveDate) ?? '-'
              })}
            </Alert>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setMoveFamilyDialog((previous) => ({ ...previous, open: false }))} variant='text'>
            {t('common_cancel')}
          </Button>
          <Button
            disabled={actionMutation.isPending || disableWriteActions}
            onClick={() => void onSubmitMoveFamily()}
            startIcon={pendingStartIcon}
            variant='contained'
          >
            {t('jobcatalog_submit_move')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        fullWidth
        maxWidth='sm'
        onClose={() => {
          setCreateLevelDialog((previous) => ({ ...previous, open: false }))
          setDialogError(null)
        }}
        open={createLevelDialog.open}
      >
        <DialogTitle>{t('jobcatalog_dialog_create_level_title')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1.5} sx={{ mt: 0.5 }}>
            {dialogError ? <Alert severity='error'>{dialogError}</Alert> : null}
            <Typography color='text.secondary' variant='body2'>
              {t('jobcatalog_dialog_package_readonly', { packageCode: packageCode || '-' })}
            </Typography>
            <TextField
              fullWidth
              label={t('jobcatalog_form_code')}
              onChange={(event) =>
                setCreateLevelDialog((previous) => ({
                  ...previous,
                  code: event.target.value
                }))}
              value={createLevelDialog.code}
            />
            <TextField
              fullWidth
              label={t('jobcatalog_form_name')}
              onChange={(event) =>
                setCreateLevelDialog((previous) => ({
                  ...previous,
                  name: event.target.value
                }))}
              value={createLevelDialog.name}
            />
            <DatePicker
              label={t('jobcatalog_form_effective_date')}
              onChange={(value) =>
                setCreateLevelDialog((previous) => ({
                  ...previous,
                  effectiveDate: value
                }))}
              slotProps={{ textField: { fullWidth: true } }}
              value={createLevelDialog.effectiveDate}
            />
            <Alert severity='info'>
              {t('jobcatalog_dialog_effective_hint', {
                date: formatDateValue(createLevelDialog.effectiveDate) ?? '-'
              })}
            </Alert>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateLevelDialog((previous) => ({ ...previous, open: false }))} variant='text'>
            {t('common_cancel')}
          </Button>
          <Button
            disabled={actionMutation.isPending || disableWriteActions}
            onClick={() => void onSubmitCreateLevel()}
            startIcon={pendingStartIcon}
            variant='contained'
          >
            {t('jobcatalog_submit_create')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        fullWidth
        maxWidth='sm'
        onClose={() => {
          setCreateProfileDialog((previous) => ({ ...previous, open: false }))
          setDialogError(null)
        }}
        open={createProfileDialog.open}
      >
        <DialogTitle>{t('jobcatalog_dialog_create_profile_title')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1.5} sx={{ mt: 0.5 }}>
            {dialogError ? <Alert severity='error'>{dialogError}</Alert> : null}
            <Typography color='text.secondary' variant='body2'>
              {t('jobcatalog_dialog_package_readonly', { packageCode: packageCode || '-' })}
            </Typography>
            <TextField
              fullWidth
              label={t('jobcatalog_form_code')}
              onChange={(event) =>
                setCreateProfileDialog((previous) => ({
                  ...previous,
                  code: event.target.value
                }))}
              value={createProfileDialog.code}
            />
            <TextField
              fullWidth
              label={t('jobcatalog_form_name')}
              onChange={(event) =>
                setCreateProfileDialog((previous) => ({
                  ...previous,
                  name: event.target.value
                }))}
              value={createProfileDialog.name}
            />
            <Autocomplete
              multiple
              onChange={(_, nextOptions) => {
                const familyCodes = nextOptions.map((item) => item.code)
                setCreateProfileDialog((previous) => {
                  const nextPrimary =
                    familyCodes.includes(previous.primaryFamilyCode) && previous.primaryFamilyCode.length > 0
                      ? previous.primaryFamilyCode
                      : familyCodes[0] ?? ''
                  return {
                    ...previous,
                    familyCodes,
                    primaryFamilyCode: nextPrimary
                  }
                })
              }}
              options={profileFamilyOptions}
              renderInput={(params) => (
                <TextField
                  {...params}
                  fullWidth
                  label={t('jobcatalog_form_family_codes')}
                  placeholder={t('jobcatalog_profile_family_placeholder')}
                />
              )}
              value={selectedProfileFamilyOptions}
            />
            <FormControl fullWidth>
              <InputLabel id='create-profile-primary-family'>
                {t('jobcatalog_form_primary_family_code')}
              </InputLabel>
              <Select
                label={t('jobcatalog_form_primary_family_code')}
                labelId='create-profile-primary-family'
                onChange={(event) =>
                  setCreateProfileDialog((previous) => ({
                    ...previous,
                    primaryFamilyCode: String(event.target.value)
                  }))}
                value={createProfileDialog.primaryFamilyCode}
              >
                {createProfileDialog.familyCodes.map((code) => (
                  <MenuItem key={code} value={code}>
                    {code}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <DatePicker
              label={t('jobcatalog_form_effective_date')}
              onChange={(value) =>
                setCreateProfileDialog((previous) => ({
                  ...previous,
                  effectiveDate: value
                }))}
              slotProps={{ textField: { fullWidth: true } }}
              value={createProfileDialog.effectiveDate}
            />
            <Alert severity='info'>
              {t('jobcatalog_dialog_effective_hint', {
                date: formatDateValue(createProfileDialog.effectiveDate) ?? '-'
              })}
            </Alert>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateProfileDialog((previous) => ({ ...previous, open: false }))} variant='text'>
            {t('common_cancel')}
          </Button>
          <Button
            disabled={actionMutation.isPending || disableWriteActions}
            onClick={() => void onSubmitCreateProfile()}
            startIcon={pendingStartIcon}
            variant='contained'
          >
            {t('jobcatalog_submit_create')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        fullWidth
        maxWidth='sm'
        onClose={() => setDetailDialog(null)}
        open={detailDialog !== null}
      >
        <DialogTitle>{detailDialog?.title ?? t('jobcatalog_dialog_detail_title')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1}>
            {detailDialog?.rows.map((row) => (
              <Typography key={row.label} variant='body2'>
                {row.label}
                ：
                {row.value}
              </Typography>
            ))}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDetailDialog(null)} variant='text'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        autoHideDuration={2500}
        onClose={() => setToastMessage(null)}
        open={toastMessage !== null}
      >
        <Alert onClose={() => setToastMessage(null)} severity='success' variant='filled'>
          {toastMessage}
        </Alert>
      </Snackbar>
    </Box>
  )
}
