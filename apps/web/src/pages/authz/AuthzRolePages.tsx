import { useCallback, useMemo, useState } from 'react'
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
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
import type { OrgUnitSelectorNode } from '../../api/orgUnitSelector'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import type { AuthzCapabilityKey } from '../../authz/capabilities'
import { DataGridPage } from '../../components/DataGridPage'
import { OrgUnitTreeField } from '../../components/OrgUnitTreeSelector'
import { PageHeader } from '../../components/PageHeader'
import { todayISODate } from '../../utils/readViewState'

const EMPTY_CAPABILITIES: AuthzCapabilityOption[] = []
const EMPTY_ROLES: AuthzRoleDefinition[] = []
const EMPTY_PRINCIPALS: PrincipalAssignmentCandidate[] = []

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
}

interface AssignmentRoleRow {
  id: string
  roleSlug: string
}

interface AssignmentOrgScopeRow {
  id: string
  orgNodeKey: string
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

function roleSummary(role: AuthzRoleDefinition, capabilityCountLabel: string): string {
  return `${role.role_slug} · ${capabilityCountLabel}`
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

function selectedRoleSlugs(rows: AssignmentRoleRow[]): string[] {
  return rows.map((row) => row.roleSlug.trim()).filter((value) => value.length > 0)
}

function scopeRowsFromAssignment(assignment: PrincipalAuthzAssignmentResponse | undefined): AssignmentOrgScopeRow[] {
  return (assignment?.org_scopes ?? []).map((scope, index) => ({
    id: scope.org_code ?? scope.org_node_key ?? `scope-${index}`,
    orgNodeKey: scope.org_node_key ?? '',
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
      org_node_key: row.orgNodeKey.trim() || undefined,
      org_code: row.orgCode.trim(),
      org_name: row.orgName.trim() || undefined,
      include_descendants: row.includeDescendants
    }))
    .filter((row) => (row.org_code?.length ?? 0) > 0 || (row.org_node_key?.length ?? 0) > 0)
}

function orgScopeRowSelectorValue(row: AssignmentOrgScopeRow): OrgUnitSelectorNode | null {
  const orgCode = row.orgCode.trim()
  if (!orgCode) {
    return null
  }
  const orgName = row.orgName.trim() || orgCode
  return {
    org_code: orgCode,
    org_node_key: row.orgNodeKey.trim(),
    name: orgName,
    status: 'active',
    has_visible_children: false
  }
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
      description: item.label
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
      headerName: t('authz_role_column_select'),
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
    { field: 'resourceLabel', headerName: t('authz_role_column_resource'), minWidth: 150, flex: 0.8 },
    { field: 'actionLabel', headerName: t('authz_role_column_action'), minWidth: 90 },
    { field: 'authzCapabilityKey', headerName: t('authz_role_column_key'), minWidth: 220, flex: 1 },
    { field: 'description', headerName: t('authz_role_column_description'), minWidth: 260, flex: 1.2 }
  ], [activeDraft, saveMutation.isPending, t])

  const saveDisabled =
    saveMutation.isPending ||
    activeDraft.systemManaged ||
    activeDraft.name.trim().length === 0 ||
    activeDraft.roleSlug.trim().length === 0 ||
    activeDraft.capabilityKeys.length === 0

  return (
    <>
      <PageHeader
        title={t('page_authz_roles_title')}
        subtitle={t('page_authz_roles_subtitle')}
        actions={
          <>
            <Button onClick={() => {
              setSelectedRoleSlug('')
              setDraft(emptyRoleDraft())
              setErrorMessage('')
            }} startIcon={<AddIcon />} variant='outlined'>
              {t('authz_role_create')}
            </Button>
            <Button disabled={saveDisabled} onClick={() => saveMutation.mutate()} startIcon={<SaveIcon />} variant='contained'>
              {t('common_save')}
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
            {t('authz_role_list_title')}
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
                        <Typography color='text.secondary' variant='caption'>
                          {roleSummary(role, t('authz_role_capability_count', { count: role.authz_capability_keys.length }))}
                        </Typography>
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
          {activeDraft.systemManaged ? <Alert severity='info' sx={{ mb: 2 }}>{t('authz_role_system_readonly')}</Alert> : null}

          <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 1, p: 2, mb: 2 }}>
            <Typography variant='h6'>
              {activeDraft.mode === 'create'
                ? t('authz_role_create_title')
                : t('authz_role_edit_title', { name: activeDraft.name || activeDraft.roleSlug })}
            </Typography>
            <Typography color='text.secondary' variant='body2'>
              {t('authz_role_form_hint')}
            </Typography>
            <Divider sx={{ my: 2 }} />
            <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
              <TextField
                disabled={activeDraft.systemManaged}
                fullWidth
                label={t('authz_role_name')}
                onChange={(event) => setDraft((previous) => ({ ...(previous ?? activeDraft), name: event.target.value }))}
                value={activeDraft.name}
              />
              <TextField
                disabled={activeDraft.mode === 'edit' || activeDraft.systemManaged}
                fullWidth
                label={t('authz_role_slug')}
                onChange={(event) => setDraft((previous) => ({ ...(previous ?? activeDraft), roleSlug: event.target.value }))}
                value={activeDraft.roleSlug}
              />
            </Stack>
            <TextField
              disabled={activeDraft.systemManaged}
              fullWidth
              label={t('authz_role_description')}
              multiline
              minRows={2}
              onChange={(event) => setDraft((previous) => ({ ...(previous ?? activeDraft), description: event.target.value }))}
              sx={{ mt: 2 }}
              value={activeDraft.description}
            />
          </Box>

          <Typography sx={{ mb: 1 }} variant='h6'>{t('authz_role_capability_section')}</Typography>
          <DataGridPage
            columns={roleColumns}
            gridProps={{
              hideFooter: true,
              sx: { minHeight: 520 }
            }}
            loading={capabilitiesQuery.isFetching}
            loadingLabel={t('text_loading')}
            noRowsLabel={t('authz_role_capability_empty')}
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
  const principals = principalsQuery.data?.principals ?? EMPTY_PRINCIPALS
  const roles = rolesQuery.data?.roles ?? EMPTY_ROLES
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
    updateDraft((current) => ({
      ...current,
      scopeRows: [
        ...current.scopeRows,
        {
          id: `scope-${Date.now()}`,
          orgCode: '',
          orgNodeKey: '',
          orgName: '',
          includeDescendants: true
        }
      ]
    }))
  }, [updateDraft])

  const roleColumns = useMemo<GridColDef<AssignmentRoleRow>[]>(() => [
    {
      field: 'roleSlug',
      headerName: t('authz_assignment_role'),
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
            renderInput={(inputParams) => <TextField {...inputParams} label={t('authz_assignment_role')} size='small' />}
            value={value}
          />
        )
      }
    },
    {
      field: 'description',
      headerName: t('authz_assignment_role_description'),
      minWidth: 300,
      flex: 1.2,
      valueGetter: (_, row) => {
        const role = rolesBySlug.get(row.roleSlug)
        return role?.description || role?.name || ''
      }
    },
    {
      field: 'actions',
      headerName: t('authz_assignment_actions'),
      minWidth: 90,
      sortable: false,
      renderCell: (params) => (
        <IconButton
          aria-label={t('authz_assignment_remove_role')}
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
  ], [roles, rolesBySlug, t, updateDraft])

  const scopeColumns = useMemo<GridColDef<AssignmentOrgScopeRow>[]>(() => [
    {
      field: 'orgCode',
      headerName: t('authz_assignment_org'),
      minWidth: 280,
      flex: 1,
      renderCell: (params) => {
        return (
          <OrgUnitTreeField
            asOf={currentDate}
            label={t('authz_assignment_org')}
            onChange={(option) => {
              updateDraft((current) => ({
                ...current,
                scopeRows: current.scopeRows.map((row) => (
                  row.id === params.row.id
                    ? {
                        ...row,
                        orgCode: option.org_code,
                        orgNodeKey: option.org_node_key,
                        orgName: option.name
                      }
                    : row
                ))
              }))
            }}
            value={orgScopeRowSelectorValue(params.row)}
          />
        )
      }
    },
    {
      field: 'includeDescendants',
      headerName: t('authz_assignment_include_descendants'),
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
      headerName: t('authz_assignment_actions'),
      minWidth: 90,
      sortable: false,
      renderCell: (params) => (
        <IconButton
          aria-label={t('authz_assignment_remove_org_scope')}
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
  ], [currentDate, t, updateDraft])

  const saveDisabled =
    !activePrincipalID ||
    assignmentQuery.isFetching ||
    saveMutation.isPending ||
    selectedRoleSlugs(roleRows).length === 0

  return (
    <>
      <PageHeader
        title={t('page_authz_user_assignments_title')}
        subtitle={selectedPrincipal
          ? t('page_authz_user_assignments_subtitle_selected', { user: principalOptionLabel(selectedPrincipal) })
          : t('page_authz_user_assignments_subtitle_empty')}
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
              {t('common_cancel')}
            </Button>
            <Button disabled={saveDisabled} onClick={() => saveMutation.mutate()} startIcon={<SaveIcon />} variant='contained'>
              {t('common_save')}
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
            renderInput={(params) => <TextField {...params} label={t('authz_assignment_user')} />}
            value={selectedPrincipal}
          />
        </Box>

        {principalsQuery.error || rolesQuery.error || assignmentQuery.error ? (
          <Alert severity='error'>
            {getErrorMessage(principalsQuery.error ?? rolesQuery.error ?? assignmentQuery.error)}
          </Alert>
        ) : null}
        {errorMessage ? <Alert severity='error'>{errorMessage}</Alert> : null}

        <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden' }}>
          <Tabs onChange={(_, value: 'roles' | 'org') => setTab(value)} value={tab}>
            <Tab label={t('authz_assignment_tab_roles')} value='roles' />
            <Tab label={t('authz_assignment_tab_org_scopes')} value='org' />
          </Tabs>
          <Divider />
          <Box sx={{ p: 2 }}>
            {tab === 'roles' ? (
              <Stack spacing={1.5}>
                <Stack alignItems='center' direction='row' justifyContent='space-between'>
                  <Typography variant='h6'>{t('authz_assignment_roles_title')}</Typography>
                  <Button onClick={addRoleRow} startIcon={<AddIcon />} variant='outlined'>{t('common_add_row')}</Button>
                </Stack>
                <DataGridPage
                  columns={roleColumns}
                  gridProps={{ hideFooter: true, sx: { minHeight: 360 } }}
                  loading={rolesQuery.isFetching || assignmentQuery.isFetching}
                  loadingLabel={t('text_loading')}
                  noRowsLabel={t('authz_assignment_roles_empty')}
                  rows={roleRows}
                  storageKey={`authz-user-roles/${tenantId}`}
                />
              </Stack>
            ) : (
              <Stack spacing={1.5}>
                <Stack alignItems='center' direction='row' justifyContent='space-between'>
                  <Typography variant='h6'>{t('authz_assignment_org_scopes_title')}</Typography>
                  <Button onClick={addScopeRow} startIcon={<AddIcon />} variant='outlined'>{t('common_add_row')}</Button>
                </Stack>
                <DataGridPage
                  columns={scopeColumns}
                  gridProps={{ hideFooter: true, sx: { minHeight: 360 } }}
                  loading={assignmentQuery.isFetching}
                  loadingLabel={t('text_loading')}
                  noRowsLabel={t('authz_assignment_org_scopes_empty')}
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
