import { useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  FormControl,
  IconButton,
  InputAdornment,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import SearchIcon from '@mui/icons-material/Search'
import { useQuery } from '@tanstack/react-query'
import type { GridColDef } from '@mui/x-data-grid'
import {
  listAuthzAPICatalog,
  listAuthzCapabilities,
  type AuthzAPICatalogEntry,
  type AuthzCapabilityOption
} from '../../api/authz'
import { type AuthzCapabilityKey } from '../../authz/capabilities'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'

interface CapabilityRow {
  id: AuthzCapabilityKey
  resourceLabel: string
  actionLabel: string
  authzCapabilityKey: AuthzCapabilityKey
  ownerModule: string
  scopeDimension: string
  covered: boolean
  status: string
  assignable: boolean
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function yesNo(value: boolean, t: ReturnType<typeof useAppPreferences>['t']): string {
  return value ? t('common_yes') : t('common_no')
}

function scopeLabel(value: string, t: ReturnType<typeof useAppPreferences>['t']): string {
  return value === 'organization' ? t('authz_scope_organization') : t('authz_scope_none')
}

function statusLabel(value: string, t: ReturnType<typeof useAppPreferences>['t']): string {
  return value === 'enabled' ? t('authz_status_enabled') : value
}

function accessControlLabel(value: string, t: ReturnType<typeof useAppPreferences>['t']): string {
  switch (value) {
    case 'protected':
      return t('authz_access_protected')
    case 'internal_api':
      return t('authz_access_internal_api')
    case 'public_api':
      return t('authz_access_public_api')
    case 'authn':
      return t('authz_access_authn')
    case 'ops':
      return t('authz_access_ops')
    case 'static':
      return t('authz_access_static')
    default:
      return value
  }
}

function methodColor(method: string): 'default' | 'primary' | 'warning' {
  return method === 'GET' ? 'primary' : method === 'POST' || method === 'PATCH' || method === 'PUT' || method === 'DELETE' ? 'warning' : 'default'
}

function AuthzPageHeader(props: {
  title: string
  subtitle: string
}) {
  return (
    <Paper sx={{ borderColor: '#DDE4E6', borderRadius: 1, p: 2 }} variant='outlined'>
      <Typography component='h2' fontWeight={800} sx={{ mb: 0.5 }} variant='h5'>
        {props.title}
      </Typography>
      <Typography color='text.secondary' variant='body2'>
        {props.subtitle}
      </Typography>
    </Paper>
  )
}

function CapabilityFilters(props: {
  query: string
  ownerModule: string
  scopeDimension: string
  moduleOptions: string[]
  scopeOptions: string[]
  onQueryChange: (value: string) => void
  onOwnerModuleChange: (value: string) => void
  onScopeDimensionChange: (value: string) => void
}) {
  const { t } = useAppPreferences()
  return (
    <Paper sx={{ borderColor: '#DDE4E6', borderRadius: 1, mb: 1.5, p: 1.5 }} variant='outlined'>
      <Stack direction={{ md: 'row', xs: 'column' }} spacing={1.5}>
        <TextField
          fullWidth
          InputProps={{
            startAdornment: (
              <InputAdornment position='start'>
                <SearchIcon fontSize='small' />
              </InputAdornment>
            )
          }}
          label={t('authz_capability_search')}
          onChange={(event) => props.onQueryChange(event.target.value)}
          size='small'
          value={props.query}
        />
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel id='authz-capability-owner-module-label'>{t('authz_filter_module')}</InputLabel>
          <Select
            label={t('authz_filter_module')}
            labelId='authz-capability-owner-module-label'
            onChange={(event) => props.onOwnerModuleChange(String(event.target.value))}
            value={props.ownerModule}
          >
            <MenuItem value=''>{t('authz_filter_all_modules')}</MenuItem>
            {props.moduleOptions.map((item) => (
              <MenuItem key={item} value={item}>
                {item}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel id='authz-capability-scope-label'>{t('authz_filter_scope')}</InputLabel>
          <Select
            label={t('authz_filter_scope')}
            labelId='authz-capability-scope-label'
            onChange={(event) => props.onScopeDimensionChange(String(event.target.value))}
            value={props.scopeDimension}
          >
            <MenuItem value=''>{t('authz_filter_all_scopes')}</MenuItem>
            {props.scopeOptions.map((item) => (
              <MenuItem key={item} value={item}>
                {scopeLabel(item, t)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>
    </Paper>
  )
}

function AssociatedAPIDialog(props: {
  open: boolean
  capabilityKey: AuthzCapabilityKey | null
  onClose: () => void
}) {
  const { t } = useAppPreferences()
  const query = useQuery({
    enabled: props.open && !!props.capabilityKey,
    queryKey: ['authz-api-catalog', 'capability', props.capabilityKey],
    queryFn: () => listAuthzAPICatalog({ authzCapabilityKey: props.capabilityKey ?? undefined })
  })
  const rows = useMemo(() => query.data?.api_entries.map(apiEntryToRow) ?? [], [query.data])

  return (
    <Dialog fullWidth maxWidth='md' onClose={props.onClose} open={props.open}>
      <DialogTitle sx={{ alignItems: 'center', display: 'flex', justifyContent: 'space-between', pb: 1.5 }}>
        <Box>
          <Typography component='div' fontWeight={700} variant='h6'>
            {t('authz_associated_api_title')}
          </Typography>
          {props.capabilityKey ? (
            <Typography color='text.secondary' variant='body2'>
              {props.capabilityKey}
            </Typography>
          ) : null}
        </Box>
        <IconButton aria-label={t('common_cancel')} onClick={props.onClose} size='small'>
          <CloseIcon fontSize='small' />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers sx={{ p: 2 }}>
        {query.error ? <Alert severity='error' sx={{ mb: 1.5 }}>{getErrorMessage(query.error)}</Alert> : null}
        <CompactAPITable loading={query.isFetching} rows={rows} />
      </DialogContent>
    </Dialog>
  )
}

function CompactAPITable(props: {
  loading: boolean
  rows: ReturnType<typeof apiEntryToRow>[]
}) {
  const { t } = useAppPreferences()
  if (props.loading) {
    return (
      <Box sx={{ p: 3, textAlign: 'center' }}>
        <Typography color='text.secondary' variant='body2'>
          {t('text_loading')}
        </Typography>
      </Box>
    )
  }
  if (props.rows.length === 0) {
    return (
      <Box sx={{ p: 3, textAlign: 'center' }}>
        <Typography color='text.secondary' variant='body2'>
          {t('authz_associated_api_empty')}
        </Typography>
      </Box>
    )
  }
  return (
    <Box sx={{ border: 1, borderColor: '#DDE4E6', borderRadius: 1, overflowX: 'auto' }}>
      <Stack
        direction='row'
        spacing={1.5}
        sx={{ bgcolor: '#F6F8F9', minWidth: 760, px: 1.5, py: 1.25 }}
      >
        <Typography color='text.secondary' fontWeight={700} sx={{ width: 90 }} variant='caption'>
          {t('authz_api_method')}
        </Typography>
        <Typography color='text.secondary' fontWeight={700} sx={{ width: 330 }} variant='caption'>
          {t('authz_api_path')}
        </Typography>
        <Typography color='text.secondary' fontWeight={700} sx={{ width: 120 }} variant='caption'>
          {t('authz_api_access_control')}
        </Typography>
        <Typography color='text.secondary' fontWeight={700} sx={{ width: 110 }} variant='caption'>
          {t('authz_owner_module')}
        </Typography>
        <Typography color='text.secondary' fontWeight={700} sx={{ width: 110 }} variant='caption'>
          {t('authz_cubebox_callable')}
        </Typography>
      </Stack>
      {props.rows.map((row) => (
        <Stack
          direction='row'
          key={row.id}
          spacing={1.5}
          sx={{ borderTop: 1, borderColor: '#EEF2F3', minWidth: 760, px: 1.5, py: 1.25 }}
        >
          <Box sx={{ width: 90 }}>
            <Chip color={methodColor(row.method)} label={row.method} size='small' variant='outlined' />
          </Box>
          <Typography fontWeight={700} sx={{ width: 330 }} variant='body2'>
            {row.path}
          </Typography>
          <Box sx={{ width: 120 }}>
            <Chip label={accessControlLabel(row.accessControl, t)} size='small' />
          </Box>
          <Typography color='text.secondary' sx={{ width: 110 }} variant='body2'>
            {row.ownerModule}
          </Typography>
          <Typography color={row.cubeboxCallable ? 'primary.main' : 'text.secondary'} fontWeight={700} sx={{ width: 110 }} variant='body2'>
            {yesNo(row.cubeboxCallable, t)}
          </Typography>
        </Stack>
      ))}
    </Box>
  )
}

function capabilityToRow(item: AuthzCapabilityOption): CapabilityRow {
  return {
    id: item.authz_capability_key,
    resourceLabel: item.resource_label,
    actionLabel: item.action_label,
    authzCapabilityKey: item.authz_capability_key,
    ownerModule: item.owner_module,
    scopeDimension: item.scope_dimension,
    covered: item.covered,
    status: item.status,
    assignable: item.assignable
  }
}

function apiEntryToRow(item: AuthzAPICatalogEntry) {
  return {
    id: `${item.method} ${item.path}`,
    method: item.method,
    path: item.path,
    accessControl: item.access_control,
    resourceLabel: item.resource_label ?? '',
    resourceObject: item.resource_object ?? '',
    action: item.action ?? '',
    authzCapabilityKey: item.authz_capability_key ?? '',
    ownerModule: item.owner_module,
    cubeboxCallable: item.cubebox_callable
  }
}

export function CapabilityAuthorizationsPage() {
  const { t, tenantId } = useAppPreferences()
  const [query, setQuery] = useState('')
  const [ownerModule, setOwnerModule] = useState('')
  const [scopeDimension, setScopeDimension] = useState('')
  const [selectedCapabilityKey, setSelectedCapabilityKey] = useState<AuthzCapabilityKey | null>(null)

  const capabilitiesQuery = useQuery({
    queryKey: ['authz-capabilities', query, ownerModule, scopeDimension],
    queryFn: () => listAuthzCapabilities({ ownerModule, q: query, scopeDimension }),
    staleTime: 30_000
  })

  const rows = useMemo(() => capabilitiesQuery.data?.capabilities.map(capabilityToRow) ?? [], [capabilitiesQuery.data])
  const allCapabilitiesQuery = useQuery({
    queryKey: ['authz-capabilities', 'filters'],
    queryFn: () => listAuthzCapabilities(),
    staleTime: 60_000
  })
  const filterSource = allCapabilitiesQuery.data?.capabilities ?? capabilitiesQuery.data?.capabilities ?? []
  const moduleOptions = useMemo(() => uniqueSorted(filterSource.map((item) => item.owner_module)), [filterSource])
  const scopeOptions = useMemo(() => uniqueSorted(filterSource.map((item) => item.scope_dimension)), [filterSource])

  const columns = useMemo<GridColDef<CapabilityRow>[]>(
    () => [
      { field: 'resourceLabel', flex: 0.9, headerName: t('authz_resource_label'), minWidth: 150 },
      { field: 'actionLabel', headerName: t('authz_action_label'), minWidth: 90 },
      {
        field: 'authzCapabilityKey',
        flex: 1.2,
        headerName: t('authz_capability_key'),
        minWidth: 230,
	        renderCell: (params) => (
	          <Button
	            onClick={() => setSelectedCapabilityKey(params.row.authzCapabilityKey)}
	            size='small'
	            sx={{ fontWeight: 800, justifyContent: 'flex-start', px: 0, textTransform: 'none' }}
	          >
	            {params.row.authzCapabilityKey}
	          </Button>
	        )
      },
      { field: 'ownerModule', headerName: t('authz_owner_module'), minWidth: 120 },
      {
        field: 'scopeDimension',
        headerName: t('authz_scope_dimension'),
        minWidth: 130,
        renderCell: (params) => <Chip label={scopeLabel(String(params.value), t)} size='small' />
      },
      {
        field: 'covered',
        headerName: t('authz_current_coverage'),
        minWidth: 120,
        renderCell: (params) => <Chip color='primary' label={yesNo(Boolean(params.value), t)} size='small' variant='outlined' />
      },
      {
        field: 'status',
        headerName: t('text_status'),
        minWidth: 110,
        renderCell: (params) => <Chip color='success' label={statusLabel(String(params.value), t)} size='small' />
      }
    ],
    [t]
  )

  return (
    <Stack spacing={1.75}>
      <AuthzPageHeader subtitle={t('page_authz_capabilities_subtitle')} title={t('page_authz_capabilities_title')} />
      <Box>
        <CapabilityFilters
          moduleOptions={moduleOptions}
          onOwnerModuleChange={setOwnerModule}
          onQueryChange={setQuery}
          onScopeDimensionChange={setScopeDimension}
          ownerModule={ownerModule}
          query={query}
          scopeDimension={scopeDimension}
          scopeOptions={scopeOptions}
        />
        {capabilitiesQuery.error ? <Alert severity='error' sx={{ mb: 1.5 }}>{getErrorMessage(capabilitiesQuery.error)}</Alert> : null}
	        <DataGridPage
	          columns={columns}
	          gridProps={{
	            getRowClassName: (params) => (params.row.authzCapabilityKey === selectedCapabilityKey ? 'authz-selected-row' : ''),
	            sx: {
	              '& .MuiDataGrid-columnHeaders': { bgcolor: '#F6F8F9' },
	              '& .authz-selected-row': { bgcolor: '#E9FBFA' }
            }
          }}
          loading={capabilitiesQuery.isFetching}
          loadingLabel={t('text_loading')}
          noRowsLabel={query.trim() || ownerModule || scopeDimension ? t('authz_capability_no_match') : t('authz_capability_empty')}
          rows={rows}
          storageKey={`authz-capabilities/${tenantId}`}
        />
      </Box>
      <AssociatedAPIDialog
        capabilityKey={selectedCapabilityKey}
        onClose={() => setSelectedCapabilityKey(null)}
        open={selectedCapabilityKey !== null}
      />
    </Stack>
  )
}

export function APIAuthorizationCatalogPage() {
  const { t, tenantId } = useAppPreferences()
  const [query, setQuery] = useState('')
  const [method, setMethod] = useState('')
  const [accessControl, setAccessControl] = useState('')
  const [ownerModule, setOwnerModule] = useState('')
  const [resourceObject, setResourceObject] = useState('')

  const catalogQuery = useQuery({
    queryKey: ['authz-api-catalog', query, method, accessControl, ownerModule, resourceObject],
    queryFn: () => listAuthzAPICatalog({ accessControl, method, ownerModule, q: query, resourceObject }),
    staleTime: 30_000
  })
  const allCatalogQuery = useQuery({
    queryKey: ['authz-api-catalog', 'filters'],
    queryFn: () => listAuthzAPICatalog(),
    staleTime: 60_000
  })

  const rows = useMemo(() => catalogQuery.data?.api_entries.map(apiEntryToRow) ?? [], [catalogQuery.data])
  const filterSource = allCatalogQuery.data?.api_entries ?? catalogQuery.data?.api_entries ?? []
  const methodOptions = useMemo(() => uniqueSorted(filterSource.map((item) => item.method)), [filterSource])
  const accessControlOptions = useMemo(() => uniqueSorted(filterSource.map((item) => item.access_control)), [filterSource])
  const ownerModuleOptions = useMemo(() => uniqueSorted(filterSource.map((item) => item.owner_module)), [filterSource])
  const resourceObjectOptions = useMemo(() => uniqueSorted(filterSource.map((item) => item.resource_object ?? '')), [filterSource])

  const columns = useMemo<GridColDef[]>(
    () => [
      {
        field: 'method',
        headerName: t('authz_api_method'),
        minWidth: 90,
        renderCell: (params) => <Chip color={methodColor(String(params.value))} label={String(params.value)} size='small' variant='outlined' />
      },
      { field: 'path', flex: 1.4, headerName: t('authz_api_path'), minWidth: 280 },
      {
        field: 'accessControl',
        headerName: t('authz_api_access_control'),
        minWidth: 130,
        renderCell: (params) => <Chip label={accessControlLabel(String(params.value), t)} size='small' />
      },
      { field: 'resourceLabel', headerName: t('authz_resource_label'), minWidth: 130, valueFormatter: (value) => String(value || '-') },
      { field: 'resourceObject', headerName: t('authz_resource_object'), minWidth: 170, valueFormatter: (value) => String(value || '-') },
      { field: 'action', headerName: t('authz_action'), minWidth: 90, valueFormatter: (value) => String(value || '-') },
      {
        field: 'authzCapabilityKey',
        flex: 1,
        headerName: t('authz_capability_key'),
        minWidth: 200,
        renderCell: (params) => (
          <Typography color={params.value ? 'primary.main' : 'text.secondary'} fontWeight={params.value ? 800 : 600} variant='body2'>
            {String(params.value || '-')}
          </Typography>
        )
      },
      { field: 'ownerModule', headerName: t('authz_owner_module'), minWidth: 120 },
      {
        field: 'cubeboxCallable',
        headerName: t('authz_cubebox_callable'),
        minWidth: 120,
        renderCell: (params) => (
          <Typography color={params.value ? 'primary.main' : 'text.secondary'} fontWeight={700} variant='body2'>
            {yesNo(Boolean(params.value), t)}
          </Typography>
        )
      }
    ],
    [t]
  )

  return (
    <Stack spacing={1.75}>
      <AuthzPageHeader subtitle={t('page_authz_api_catalog_subtitle')} title={t('page_authz_api_catalog_title')} />
      <Box>
        <APICatalogFilters
          accessControl={accessControl}
          accessControlOptions={accessControlOptions}
          method={method}
          methodOptions={methodOptions}
          onAccessControlChange={setAccessControl}
          onMethodChange={setMethod}
          onOwnerModuleChange={setOwnerModule}
          onQueryChange={setQuery}
          onResourceObjectChange={setResourceObject}
          ownerModule={ownerModule}
          ownerModuleOptions={ownerModuleOptions}
          query={query}
          resourceObject={resourceObject}
          resourceObjectOptions={resourceObjectOptions}
        />
        {catalogQuery.error ? <Alert severity='error' sx={{ mb: 1.5 }}>{getErrorMessage(catalogQuery.error)}</Alert> : null}
	        <DataGridPage
	          columns={columns}
	          gridProps={{
	            sx: {
	              '& .MuiDataGrid-columnHeaders': { bgcolor: '#F6F8F9' }
	            }
          }}
          loading={catalogQuery.isFetching}
          loadingLabel={t('text_loading')}
          noRowsLabel={t('authz_api_catalog_empty')}
          rows={rows}
          storageKey={`authz-api-catalog/${tenantId}`}
        />
      </Box>
    </Stack>
  )
}

function APICatalogFilters(props: {
  query: string
  method: string
  accessControl: string
  ownerModule: string
  resourceObject: string
  methodOptions: string[]
  accessControlOptions: string[]
  ownerModuleOptions: string[]
  resourceObjectOptions: string[]
  onQueryChange: (value: string) => void
  onMethodChange: (value: string) => void
  onAccessControlChange: (value: string) => void
  onOwnerModuleChange: (value: string) => void
  onResourceObjectChange: (value: string) => void
}) {
  const { t } = useAppPreferences()
  return (
    <Paper sx={{ borderColor: '#DDE4E6', borderRadius: 1, mb: 1.5, p: 1.5 }} variant='outlined'>
      <Stack direction={{ md: 'row', xs: 'column' }} spacing={1}>
        <TextField
          fullWidth
          InputProps={{
            startAdornment: (
              <InputAdornment position='start'>
                <SearchIcon fontSize='small' />
              </InputAdornment>
            )
          }}
          label={t('authz_api_catalog_search')}
          onChange={(event) => props.onQueryChange(event.target.value)}
          size='small'
          value={props.query}
        />
        <FormControl size='small' sx={{ minWidth: 130 }}>
          <InputLabel id='authz-api-method-label'>{t('authz_filter_method')}</InputLabel>
          <Select
            label={t('authz_filter_method')}
            labelId='authz-api-method-label'
            onChange={(event) => props.onMethodChange(String(event.target.value))}
            value={props.method}
          >
            <MenuItem value=''>{t('authz_filter_all_methods')}</MenuItem>
            {props.methodOptions.map((item) => (
              <MenuItem key={item} value={item}>
                {item}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel id='authz-api-access-label'>{t('authz_filter_access_control')}</InputLabel>
          <Select
            label={t('authz_filter_access_control')}
            labelId='authz-api-access-label'
            onChange={(event) => props.onAccessControlChange(String(event.target.value))}
            value={props.accessControl}
          >
            <MenuItem value=''>{t('authz_filter_all_access_controls')}</MenuItem>
            {props.accessControlOptions.map((item) => (
              <MenuItem key={item} value={item}>
                {accessControlLabel(item, t)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 140 }}>
          <InputLabel id='authz-api-owner-module-label'>{t('authz_filter_module')}</InputLabel>
          <Select
            label={t('authz_filter_module')}
            labelId='authz-api-owner-module-label'
            onChange={(event) => props.onOwnerModuleChange(String(event.target.value))}
            value={props.ownerModule}
          >
            <MenuItem value=''>{t('authz_filter_all_modules')}</MenuItem>
            {props.ownerModuleOptions.map((item) => (
              <MenuItem key={item} value={item}>
                {item}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 160 }}>
          <InputLabel id='authz-api-resource-object-label'>{t('authz_filter_resource')}</InputLabel>
          <Select
            label={t('authz_filter_resource')}
            labelId='authz-api-resource-object-label'
            onChange={(event) => props.onResourceObjectChange(String(event.target.value))}
            value={props.resourceObject}
          >
            <MenuItem value=''>{t('authz_filter_all_resources')}</MenuItem>
            {props.resourceObjectOptions.map((item) => (
              <MenuItem key={item} value={item}>
                {item}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>
    </Paper>
  )
}

function uniqueSorted(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter((value) => value.length > 0))].sort()
}
