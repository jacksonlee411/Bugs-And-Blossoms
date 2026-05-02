import { useCallback, useMemo, useState } from 'react'
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  Snackbar,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import SaveIcon from '@mui/icons-material/Save'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef } from '@mui/x-data-grid'
import {
  createAuthzRole,
  getPrincipalAuthzAssignment,
  listAuthzCapabilities,
  listAuthzRoles,
  listPrincipalAssignmentCandidates,
  replacePrincipalAuthzAssignment,
  updateAuthzRole,
  type AuthzCapabilityOption,
  type AuthzRoleDefinition,
  type PrincipalAssignmentCandidate,
  type PrincipalAuthzAssignmentResponse,
  type PrincipalOrgScope
} from '../../api/authz'
import { ApiClientError } from '../../api/errors'
import { listOrgUnits, type OrgUnitAPIItem } from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import type { AuthzCapabilityKey } from '../../authz/capabilities'
import { DataGridPage } from '../../components/DataGridPage'
import { PageHeader } from '../../components/PageHeader'
import { todayISODate } from '../../utils/readViewState'

const EMPTY_CAPABILITIES: AuthzCapabilityOption[] = []
const EMPTY_ROLES: AuthzRoleDefinition[] = []
const EMPTY_PRINCIPALS: PrincipalAssignmentCandidate[] = []
const EMPTY_ORG_UNITS: OrgUnitAPIItem[] = []

interface RoleDraft {
  mode: 'create' | 'edit'
  originalRoleSlug: string
  roleSlug: string
  name: string
  description: string
  revision: number
  systemManaged: boolean
  capabilityKeys: AuthzCapabilityKey[]
}

interface RoleRow {
  id: AuthzCapabilityKey
  selected: boolean
  resourceLabel: string
  actionLabel: string
  authzCapabilityKey: AuthzCapabilityKey
  description: string
  scopeDimension: string
}

interface AssignmentRoleRow {
  id: string
  roleSlug: string
}

interface AssignmentOrgScopeRow {
  id: string
  orgCode: string
  orgName: string
  includeDescendants: boolean
}

interface AssignmentDraft {
  revision: number
  roleRows: AssignmentRoleRow[]
  scopeRows: AssignmentOrgScopeRow[]
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function apiErrorCode(error: unknown): string {
  if (error instanceof ApiClientError && error.details && typeof error.details === 'object') {
    const details = error.details as { code?: unknown }
    return typeof details.code === 'string' ? details.code : ''
  }
  return ''
}

function emptyRoleDraft(): RoleDraft {
  return {
    mode: 'create',
    originalRoleSlug: '',
    roleSlug: '',
    name: '',
    description: '',
    revision: 0,
    systemManaged: false,
    capabilityKeys: []
  }
}

function draftFromRole(role: AuthzRoleDefinition): RoleDraft {
  return {
    mode: 'edit',
    originalRoleSlug: role.role_slug,
    roleSlug: role.role_slug,
    name: role.name,
    description: role.description,
    revision: role.revision,
    systemManaged: role.system_managed,
    capabilityKeys: [...role.authz_capability_keys]
  }
}

function roleSummary(role: AuthzRoleDefinition): string {
  const count = role.authz_capability_keys.length
  return `${role.role_slug} · ${count} 项功能权限`
}

function roleDescription(role: AuthzRoleDefinition, capabilitiesByKey: Map<string, AuthzCapabilityOption>): string {
  const labels = role.authz_capability_keys
    .map((key) => capabilitiesByKey.get(key))
    .filter((item): item is AuthzCapabilityOption => Boolean(item))
    .map((item) => `${item.resource_label}${item.action_label}`)
  if (labels.length > 0) {
    return labels.join('、')
  }
  return role.description || role.role_slug
}

function roleOptionLabel(role: AuthzRoleDefinition): string {
  return `${role.name} · ${role.role_slug}`
}

function principalOptionLabel(principal: PrincipalAssignmentCandidate): string {
  const displayName = principal.display_name?.trim()
  return displayName ? `${principal.email} · ${displayName}` : principal.email
}

function orgOptionLabel(org: OrgUnitAPIItem): string {
  return `${org.name} (${org.org_code})`
}

function selectedRoleSlugs(rows: AssignmentRoleRow[]): string[] {
  return rows.map((row) => row.roleSlug.trim()).filter((value) => value.length > 0)
}

function scopeRowsFromAssignment(assignment: PrincipalAuthzAssignmentResponse | undefined): AssignmentOrgScopeRow[] {
  return (assignment?.org_scopes ?? []).map((scope) => ({
    id: scope.org_code ?? '',
    orgCode: scope.org_code ?? '',
    orgName: scope.org_name ?? '',
    includeDescendants: scope.include_descendants
  }))
}

function roleRowsFromAssignment(assignment: PrincipalAuthzAssignmentResponse | undefined): AssignmentRoleRow[] {
  return (assignment?.roles ?? []).map((role) => ({
    id: role.role_slug,
    roleSlug: role.role_slug
  }))
}

function assignmentDraftFromResponse(assignment: PrincipalAuthzAssignmentResponse | undefined): AssignmentDraft {
  return {
    revision: assignment?.revision ?? 0,
    roleRows: roleRowsFromAssignment(assignment),
    scopeRows: scopeRowsFromAssignment(assignment)
  }
}

function buildScopePayload(rows: AssignmentOrgScopeRow[]): PrincipalOrgScope[] {
  return rows
    .map((row) => ({
      org_code: row.orgCode.trim(),
      org_name: row.orgName.trim() || undefined,
      include_descendants: row.includeDescendants
    }))
    .filter((row) => row.org_code.length > 0)
}

export function RoleManagementPage() {
  const queryClient = useQueryClient()
  const { t, tenantId } = useAppPreferences()
  const [selectedRoleSlug, setSelectedRoleSlug] = useState('')
  const [draft, setDraft] = useState<RoleDraft | null>(null)
  const [errorMessage, setErrorMessage] = useState('')
  const [toastOpen, setToastOpen] = useState(false)

  const rolesQuery = useQuery({
    queryKey: ['authz-roles'],
    queryFn: () => listAuthzRoles()
  })
  const capabilitiesQuery = useQuery({
    queryKey: ['authz-capabilities', 'role-management'],
    queryFn: () => listAuthzCapabilities()
  })

  const capabilities = capabilitiesQuery.data?.capabilities ?? EMPTY_CAPABILITIES
  const capabilitiesByKey = useMemo(
    () => new Map(capabilities.map((item) => [item.authz_capability_key, item])),
    [capabilities]
  )
  const roles = rolesQuery.data?.roles ?? EMPTY_ROLES
  const selectedRole = draft?.mode === 'create'
    ? null
    : roles.find((role) => role.role_slug === selectedRoleSlug) ?? roles.at(0) ?? null
  const activeDraft = useMemo(
    () => draft ?? (selectedRole ? draftFromRole(selectedRole) : emptyRoleDraft()),
    [draft, selectedRole]
  )

  const capabilityRows = useMemo<RoleRow[]>(() => {
    const selectedKeys = new Set(activeDraft.capabilityKeys)
    return capabilities.map((item) => ({
      id: item.authz_capability_key,
      selected: selectedKeys.has(item.authz_capability_key),
      resourceLabel: item.resource_label,
      actionLabel: item.action_label,
      authzCapabilityKey: item.authz_capability_key,
      description: item.label,
      scopeDimension: item.scope_dimension
    }))
  }, [capabilities, activeDraft.capabilityKeys])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const request = {
        role_slug: activeDraft.roleSlug.trim(),
        name: activeDraft.name.trim(),
        description: activeDraft.description.trim(),
        revision: activeDraft.revision,
        authz_capability_keys: [...activeDraft.capabilityKeys]
      }
      if (activeDraft.mode === 'create') {
        return createAuthzRole(request)
      }
      return updateAuthzRole(activeDraft.originalRoleSlug, request)
    },
    onSuccess: async (response) => {
      setErrorMessage('')
      setSelectedRoleSlug(response.role.role_slug)
      setDraft(draftFromRole(response.role))
      await queryClient.invalidateQueries({ queryKey: ['authz-roles'] })
      setToastOpen(true)
    },
    onError: (error) => {
      setErrorMessage(getErrorMessage(error))
    }
  })

  const roleColumns = useMemo<GridColDef<RoleRow>[]>(() => [
    {
      field: 'selected',
      headerName: '选择',
      minWidth: 80,
      sortable: false,
      renderCell: (params) => (
        <Checkbox
          checked={params.row.selected}
          disabled={activeDraft.systemManaged || saveMutation.isPending}
          onChange={(event) => {
            const key = params.row.authzCapabilityKey
                setDraft((previous) => {
                  const base = previous ?? activeDraft
                  const keys = new Set(base.capabilityKeys)
                  if (event.target.checked) {
                    keys.add(key)
                  } else {
                    keys.delete(key)
                  }
                  return { ...base, capabilityKeys: [...keys].sort() }
                })
              }}
            />
      )
    },
    { field: 'resourceLabel', headerName: '资源', minWidth: 150, flex: 0.8 },
    { field: 'actionLabel', headerName: '操作', minWidth: 90 },
    { field: 'authzCapabilityKey', headerName: '授权项标识', minWidth: 220, flex: 1 },
    { field: 'description', headerName: '权限说明', minWidth: 260, flex: 1.2 },
    {
      field: 'scopeDimension',
      headerName: '组织范围',
      minWidth: 110,
      renderCell: (params) => <Chip label={params.value === 'organization' ? '需要' : '不需要'} size='small' />
    }
  ], [activeDraft, saveMutation.isPending])

  const saveDisabled =
    saveMutation.isPending ||
    activeDraft.systemManaged ||
    activeDraft.name.trim().length === 0 ||
    activeDraft.roleSlug.trim().length === 0 ||
    activeDraft.capabilityKeys.length === 0

  return (
    <>
      <PageHeader
        title='角色管理'
        subtitle='定义角色基础信息与可分配功能权限，保存后立即生效'
        actions={
          <>
            <Button onClick={() => {
              setSelectedRoleSlug('')
              setDraft(emptyRoleDraft())
              setErrorMessage('')
            }} startIcon={<AddIcon />} variant='outlined'>
              新建角色
            </Button>
            <Button disabled={saveDisabled} onClick={() => saveMutation.mutate()} startIcon={<SaveIcon />} variant='contained'>
              保存
            </Button>
          </>
        }
      />

      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <Box
          sx={{
            bgcolor: 'background.paper',
            border: 1,
            borderColor: 'divider',
            borderRadius: 1,
            flex: '0 0 320px',
            minWidth: 0,
            p: 1.5
          }}
        >
          <Typography sx={{ px: 1, py: 0.5 }} variant='subtitle2'>
            角色
          </Typography>
          {rolesQuery.isLoading ? (
            <Stack alignItems='center' sx={{ py: 4 }}>
              <CircularProgress size={20} />
            </Stack>
          ) : (
            <List dense disablePadding>
              {roles.map((role) => (
                <ListItemButton
                  key={role.role_slug}
                  onClick={() => {
                    setSelectedRoleSlug(role.role_slug)
                    setDraft(draftFromRole(role))
                    setErrorMessage('')
                  }}
                  selected={role.role_slug === selectedRole?.role_slug}
                  sx={{ borderRadius: 1, mb: 0.5 }}
                >
	                  <ListItemText
	                    primary={role.name}
	                    secondary={
	                      <Stack spacing={0.5}>
	                        <Typography color='text.secondary' variant='caption'>{roleSummary(role)}</Typography>
	                        <Typography color='text.secondary' variant='caption'>{roleDescription(role, capabilitiesByKey)}</Typography>
	                      </Stack>
	                    }
	                    secondaryTypographyProps={{ component: 'div' }}
	                  />
                </ListItemButton>
              ))}
            </List>
          )}
        </Box>

        <Box sx={{ flex: 1, minWidth: 0 }}>
          {rolesQuery.error || capabilitiesQuery.error ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {getErrorMessage(rolesQuery.error ?? capabilitiesQuery.error)}
            </Alert>
          ) : null}
          {errorMessage ? <Alert severity='error' sx={{ mb: 2 }}>{errorMessage}</Alert> : null}
          {activeDraft.systemManaged ? <Alert severity='info' sx={{ mb: 2 }}>系统内置角色不可修改。</Alert> : null}

          <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 1, p: 2, mb: 2 }}>
            <Typography variant='h6'>
              {activeDraft.mode === 'create' ? '新建角色' : `编辑角色：${activeDraft.name || activeDraft.roleSlug}`}
            </Typography>
            <Typography color='text.secondary' variant='body2'>
              角色只维护基础信息和功能权限；组织范围在用户授权中配置
            </Typography>
            <Divider sx={{ my: 2 }} />
            <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
              <TextField
                disabled={activeDraft.systemManaged}
                fullWidth
                label='角色名称'
                onChange={(event) => setDraft((previous) => ({ ...(previous ?? activeDraft), name: event.target.value }))}
                value={activeDraft.name}
              />
              <TextField
                disabled={activeDraft.mode === 'edit' || activeDraft.systemManaged}
                fullWidth
                label='角色标识'
                onChange={(event) => setDraft((previous) => ({ ...(previous ?? activeDraft), roleSlug: event.target.value }))}
                value={activeDraft.roleSlug}
              />
            </Stack>
            <TextField
              disabled={activeDraft.systemManaged}
              fullWidth
              label='角色描述'
              multiline
              minRows={2}
              onChange={(event) => setDraft((previous) => ({ ...(previous ?? activeDraft), description: event.target.value }))}
              sx={{ mt: 2 }}
              value={activeDraft.description}
            />
          </Box>

          <Typography sx={{ mb: 1 }} variant='h6'>功能权限分配</Typography>
          <DataGridPage
            columns={roleColumns}
            gridProps={{
              hideFooter: true,
              sx: { minHeight: 520 }
            }}
            loading={capabilitiesQuery.isFetching}
            loadingLabel={t('text_loading')}
            noRowsLabel='暂无可分配功能授权项'
            rows={capabilityRows}
            storageKey={`authz-role-definition/${tenantId}`}
          />
        </Box>
      </Stack>

      <Snackbar autoHideDuration={2800} onClose={() => setToastOpen(false)} open={toastOpen}>
        <Alert severity='success' variant='filled'>{t('common_action_done')}</Alert>
      </Snackbar>
    </>
  )
}

export function UserAuthorizationPage() {
  const queryClient = useQueryClient()
  const { t, tenantId } = useAppPreferences()
  const currentDate = useMemo(() => todayISODate(), [])
  const [selectedPrincipalID, setSelectedPrincipalID] = useState('')
  const [tab, setTab] = useState<'roles' | 'org'>('roles')
  const [draft, setDraft] = useState<AssignmentDraft | null>(null)
  const [errorMessage, setErrorMessage] = useState('')
  const [toastOpen, setToastOpen] = useState(false)

  const principalsQuery = useQuery({
    queryKey: ['authz-user-assignment-principals'],
    queryFn: () => listPrincipalAssignmentCandidates()
  })
  const rolesQuery = useQuery({
    queryKey: ['authz-roles'],
    queryFn: () => listAuthzRoles()
  })
  const orgUnitsQuery = useQuery({
    queryKey: ['org-units', 'authz-scope-options', currentDate],
    queryFn: () => listOrgUnits({ asOf: currentDate, includeDisabled: false }),
    staleTime: 60_000
  })

  const principals = principalsQuery.data?.principals ?? EMPTY_PRINCIPALS
  const roles = rolesQuery.data?.roles ?? EMPTY_ROLES
  const orgUnits = orgUnitsQuery.data?.org_units ?? EMPTY_ORG_UNITS
  const rolesBySlug = useMemo(() => new Map(roles.map((role) => [role.role_slug, role])), [roles])
  const activePrincipalID = selectedPrincipalID || principals.at(0)?.principal_id || ''
  const assignmentQueryKey = ['authz-user-assignment', activePrincipalID] as const
  const assignmentQuery = useQuery({
    enabled: activePrincipalID.trim().length > 0,
    queryKey: assignmentQueryKey,
    queryFn: () => getPrincipalAuthzAssignment(activePrincipalID)
  })
  const selectedPrincipal = principals.find((item) => item.principal_id === activePrincipalID) ?? null
  const assignment = assignmentQuery.data
  const baseDraft = useMemo(() => assignmentDraftFromResponse(assignment), [assignment])
  const activeDraft = draft ?? baseDraft
  const roleRows = activeDraft.roleRows
  const scopeRows = activeDraft.scopeRows

  const saveMutation = useMutation({
    mutationFn: () => replacePrincipalAuthzAssignment(activePrincipalID, {
      roles: selectedRoleSlugs(roleRows).map((roleSlug) => ({ role_slug: roleSlug })),
      org_scopes: buildScopePayload(scopeRows),
      revision: activeDraft.revision
    }),
    onSuccess: (response) => {
      setErrorMessage('')
      queryClient.setQueryData(assignmentQueryKey, response)
      setDraft(null)
      setToastOpen(true)
    },
    onError: (error) => {
      setErrorMessage(getErrorMessage(error))
      if (apiErrorCode(error) === 'authz_org_scope_required') {
        setTab('org')
      }
    }
  })

  const updateDraft = useCallback((updater: (current: AssignmentDraft) => AssignmentDraft) => {
    setDraft((previous) => updater(previous ?? baseDraft))
  }, [baseDraft])

  const addRoleRow = useCallback(() => {
    const used = new Set(selectedRoleSlugs(roleRows))
    const nextRole = roles.find((role) => !used.has(role.role_slug))
    updateDraft((current) => ({
      ...current,
      roleRows: [
        ...current.roleRows,
        { id: `role-${Date.now()}`, roleSlug: nextRole?.role_slug ?? '' }
      ]
    }))
  }, [roleRows, roles, updateDraft])

  const addScopeRow = useCallback(() => {
    const used = new Set(scopeRows.map((row) => row.orgCode).filter((value) => value.length > 0))
    const nextOrg = orgUnits.find((org) => !used.has(org.org_code))
    updateDraft((current) => ({
      ...current,
      scopeRows: [
        ...current.scopeRows,
        {
          id: `scope-${Date.now()}`,
          orgCode: nextOrg?.org_code ?? '',
          orgName: nextOrg?.name ?? '',
          includeDescendants: true
        }
      ]
    }))
  }, [orgUnits, scopeRows, updateDraft])

  const roleColumns = useMemo<GridColDef<AssignmentRoleRow>[]>(() => [
    {
      field: 'roleSlug',
      headerName: '授权角色',
      minWidth: 260,
      flex: 1,
      renderCell: (params) => {
        const value = rolesBySlug.get(params.row.roleSlug) ?? null
        return (
          <Autocomplete
            fullWidth
            getOptionLabel={roleOptionLabel}
            isOptionEqualToValue={(option, selected) => option.role_slug === selected.role_slug}
            onChange={(_, option) => {
              updateDraft((current) => ({
                ...current,
                roleRows: current.roleRows.map((row) => (
                  row.id === params.row.id ? { ...row, roleSlug: option?.role_slug ?? '' } : row
                ))
              }))
            }}
            options={roles}
            renderInput={(inputParams) => <TextField {...inputParams} label='授权角色' size='small' />}
            value={value}
          />
        )
      }
    },
    {
      field: 'description',
      headerName: '角色说明',
      minWidth: 300,
      flex: 1.2,
      valueGetter: (_, row) => {
        const role = rolesBySlug.get(row.roleSlug)
        return role?.description || role?.name || ''
      }
    },
    {
      field: 'actions',
      headerName: '操作',
      minWidth: 90,
      sortable: false,
      renderCell: (params) => (
        <IconButton
          aria-label='移除角色'
          onClick={() => {
            updateDraft((current) => ({
              ...current,
              roleRows: current.roleRows.filter((row) => row.id !== params.row.id)
            }))
          }}
          size='small'
        >
          <DeleteIcon fontSize='small' />
        </IconButton>
      )
    }
  ], [roleRows, roles, rolesBySlug])

  const scopeColumns = useMemo<GridColDef<AssignmentOrgScopeRow>[]>(() => [
    {
      field: 'orgCode',
      headerName: '组织',
      minWidth: 280,
      flex: 1,
      renderCell: (params) => {
        const value = orgUnits.find((org) => org.org_code === params.row.orgCode) ?? null
        return (
          <Autocomplete
            fullWidth
            getOptionLabel={orgOptionLabel}
            isOptionEqualToValue={(option, selected) => option.org_code === selected.org_code}
            loading={orgUnitsQuery.isFetching}
            onChange={(_, option) => {
              updateDraft((current) => ({
                ...current,
                scopeRows: current.scopeRows.map((row) => (
                  row.id === params.row.id
                    ? { ...row, orgCode: option?.org_code ?? '', orgName: option?.name ?? '' }
                    : row
                ))
              }))
            }}
            options={orgUnits}
            renderInput={(inputParams) => <TextField {...inputParams} label='组织' size='small' />}
            value={value}
          />
        )
      }
    },
    {
      field: 'includeDescendants',
      headerName: '包含下级组织',
      minWidth: 150,
      renderCell: (params) => (
        <FormControlLabel
          control={
            <Checkbox
              checked={params.row.includeDescendants}
              onChange={(event) => {
                updateDraft((current) => ({
                  ...current,
                  scopeRows: current.scopeRows.map((row) => (
                    row.id === params.row.id ? { ...row, includeDescendants: event.target.checked } : row
                  ))
                }))
              }}
            />
          }
          label=''
        />
      )
    },
    {
      field: 'actions',
      headerName: '操作',
      minWidth: 90,
      sortable: false,
      renderCell: (params) => (
        <IconButton
          aria-label='移除组织范围'
          onClick={() => {
            updateDraft((current) => ({
              ...current,
              scopeRows: current.scopeRows.filter((row) => row.id !== params.row.id)
            }))
          }}
          size='small'
        >
          <DeleteIcon fontSize='small' />
        </IconButton>
      )
    }
  ], [orgUnits, orgUnitsQuery.isFetching, scopeRows])

  const saveDisabled =
    !activePrincipalID ||
    assignmentQuery.isFetching ||
    saveMutation.isPending ||
    selectedRoleSlugs(roleRows).length === 0

  return (
    <>
      <PageHeader
        title='用户授权'
        subtitle={selectedPrincipal ? `用户：${principalOptionLabel(selectedPrincipal)}` : '选择用户后维护角色与组织范围'}
        actions={
          <>
            <Button
              disabled={assignmentQuery.isFetching || saveMutation.isPending}
              onClick={() => {
                setDraft(null)
                setErrorMessage('')
                void assignmentQuery.refetch()
              }}
              variant='outlined'
            >
              取消
            </Button>
            <Button disabled={saveDisabled} onClick={() => saveMutation.mutate()} startIcon={<SaveIcon />} variant='contained'>
              保存
            </Button>
          </>
        }
      />

      <Stack spacing={2}>
        <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 1, p: 2 }}>
          <Autocomplete
            getOptionLabel={principalOptionLabel}
            isOptionEqualToValue={(option, value) => option.principal_id === value.principal_id}
            loading={principalsQuery.isFetching}
            onChange={(_, option) => {
              setSelectedPrincipalID(option?.principal_id ?? '')
              setDraft(null)
              setErrorMessage('')
            }}
            options={principals}
            renderInput={(params) => <TextField {...params} label='用户' />}
            value={selectedPrincipal}
          />
        </Box>

        {principalsQuery.error || rolesQuery.error || assignmentQuery.error || orgUnitsQuery.error ? (
          <Alert severity='error'>
            {getErrorMessage(principalsQuery.error ?? rolesQuery.error ?? assignmentQuery.error ?? orgUnitsQuery.error)}
          </Alert>
        ) : null}
        {errorMessage ? <Alert severity='error'>{errorMessage}</Alert> : null}

        <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden' }}>
          <Tabs onChange={(_, value: 'roles' | 'org') => setTab(value)} value={tab}>
            <Tab label='角色' value='roles' />
            <Tab label='组织范围' value='org' />
          </Tabs>
          <Divider />
          <Box sx={{ p: 2 }}>
            {tab === 'roles' ? (
              <Stack spacing={1.5}>
                <Stack alignItems='center' direction='row' justifyContent='space-between'>
                  <Typography variant='h6'>授权角色</Typography>
                  <Button onClick={addRoleRow} startIcon={<AddIcon />} variant='outlined'>添加行</Button>
                </Stack>
                <DataGridPage
                  columns={roleColumns}
                  gridProps={{ hideFooter: true, sx: { minHeight: 360 } }}
                  loading={rolesQuery.isFetching || assignmentQuery.isFetching}
                  loadingLabel={t('text_loading')}
                  noRowsLabel='暂无授权角色'
                  rows={roleRows}
                  storageKey={`authz-user-roles/${tenantId}`}
                />
              </Stack>
            ) : (
              <Stack spacing={1.5}>
                <Stack alignItems='center' direction='row' justifyContent='space-between'>
                  <Typography variant='h6'>组织范围</Typography>
                  <Button onClick={addScopeRow} startIcon={<AddIcon />} variant='outlined'>添加行</Button>
                </Stack>
                <DataGridPage
                  columns={scopeColumns}
                  gridProps={{ hideFooter: true, sx: { minHeight: 360 } }}
                  loading={orgUnitsQuery.isFetching || assignmentQuery.isFetching}
                  loadingLabel={t('text_loading')}
                  noRowsLabel='暂无组织范围'
                  rows={scopeRows}
                  storageKey={`authz-user-org-scopes/${tenantId}`}
                />
              </Stack>
            )}
          </Box>
        </Box>
      </Stack>

      <Snackbar autoHideDuration={2800} onClose={() => setToastOpen(false)} open={toastOpen}>
        <Alert severity='success' variant='filled'>{t('common_action_done')}</Alert>
      </Snackbar>
    </>
  )
}
