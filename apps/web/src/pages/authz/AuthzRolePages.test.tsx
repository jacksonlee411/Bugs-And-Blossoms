import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GridColDef } from '@mui/x-data-grid'
import { ApiClientError } from '../../api/errors'
import { RoleManagementPage, UserAuthorizationPage } from './AuthzRolePages'

const authzApiMocks = vi.hoisted(() => ({
  createAuthzRole: vi.fn(),
  getPrincipalAuthzAssignment: vi.fn(),
  listAuthzCapabilities: vi.fn(),
  listAuthzRoles: vi.fn(),
  listPrincipalAssignmentCandidates: vi.fn(),
  replacePrincipalAuthzAssignment: vi.fn(),
  updateAuthzRole: vi.fn()
}))

const orgUnitsApiMocks = vi.hoisted(() => ({
  listOrgUnits: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/authz', () => authzApiMocks)
vi.mock('../../api/orgUnits', () => orgUnitsApiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../../components/DataGridPage', () => ({
  DataGridPage: ({
    columns,
    noRowsLabel,
    rows
  }: {
    columns: GridColDef[]
    noRowsLabel?: string
    rows: Array<Record<string, unknown>>
  }) => (
    <div data-testid='grid'>
      {rows.length === 0 ? <div>{noRowsLabel}</div> : null}
      {rows.map((row) => (
        <div data-testid={`row-${String(row.id)}`} key={String(row.id)}>
          {columns.map((column) => {
            const key = `${String(row.id)}:${String(column.field)}`
            if (typeof column.renderCell === 'function') {
              return (
                <div data-field={String(column.field)} key={key}>
                  {column.renderCell({
                    field: String(column.field),
                    id: row.id,
                    row,
                    value: row[column.field as string]
                  } as never)}
                </div>
              )
            }
            if (typeof column.valueGetter === 'function') {
              const getValue = column.valueGetter as (
                value: unknown,
                row: Record<string, unknown>,
                column: GridColDef,
                apiRef: unknown
              ) => unknown
              return (
                <div data-field={String(column.field)} key={key}>
                  {String(getValue(row[column.field as string], row, column, {}) ?? '')}
                </div>
              )
            }
            return (
              <div data-field={String(column.field)} key={key}>
                {String(row[column.field as string] ?? '')}
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}))

function renderWithQueryClient(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false }
    }
  })

  return render(<QueryClientProvider client={queryClient}>{element}</QueryClientProvider>)
}

const capabilities = [
  {
    action: 'read',
    action_label: '查看',
    assignable: true,
    authz_capability_key: 'orgunit.orgunits:read',
    covered: true,
    label: '可查看组织树、详情、审计摘要与搜索入口',
    object: 'orgunit.orgunits',
    owner_module: 'orgunit',
    resource_label: '组织管理',
    scope_dimension: 'organization',
    sort_order: 10,
    status: 'enabled',
    surface: 'tenant_api'
  },
  {
    action: 'use',
    action_label: '使用',
    assignable: true,
    authz_capability_key: 'cubebox.conversations:use',
    covered: true,
    label: '允许进入对话入口，不代表业务 API 权限',
    object: 'cubebox.conversations',
    owner_module: 'cubebox',
    resource_label: 'CubeBox',
    scope_dimension: 'none',
    sort_order: 20,
    status: 'enabled',
    surface: 'tenant_api'
  }
]

const roles = [
  {
    role_slug: 'flower-hr',
    name: '鲜花公司HR',
    description: '查看组织',
    system_managed: false,
    revision: 3,
    authz_capability_keys: ['orgunit.orgunits:read'],
    requires_org_scope: true
  },
  {
    role_slug: 'dict-reader',
    name: '字典只读',
    description: '查看字典',
    system_managed: false,
    revision: 1,
    authz_capability_keys: ['cubebox.conversations:use'],
    requires_org_scope: false
  }
]

describe('AuthzRolePages', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appPreferencesMocks.useAppPreferences.mockReturnValue({
      hasRequiredCapability: () => true,
      locale: 'zh',
      navDebugMode: false,
      setLocale: vi.fn(),
      t: (key: string) =>
        ({
          common_action_done: '操作已完成',
          text_loading: '加载中'
        })[key] ?? key,
      tenantId: 'tenant-a',
      themeMode: 'light',
      toggleThemeMode: vi.fn()
    })
    authzApiMocks.listAuthzCapabilities.mockResolvedValue({ capabilities, registry_rev: 'rev' })
    authzApiMocks.listAuthzRoles.mockResolvedValue({ roles })
    authzApiMocks.listPrincipalAssignmentCandidates.mockResolvedValue({
      principals: [{ principal_id: 'principal-b', email: 'user-b@example.invalid', display_name: '王小花' }]
    })
    authzApiMocks.getPrincipalAuthzAssignment.mockResolvedValue({
      principal_id: 'principal-b',
      roles: [{ role_slug: 'flower-hr', display_name: '鲜花公司HR', description: '查看组织', requires_org_scope: true }],
      org_scopes: [],
      revision: 5
    })
    orgUnitsApiMocks.listOrgUnits.mockResolvedValue({
      as_of: '2026-05-02',
      org_units: [{ org_code: 'FLOWERS', name: '鲜花事业部', status: 'active', has_children: true }]
    })
  })

  it('saves the selected role definition through the 487 API', async () => {
    const initialRole = roles.at(0)
    expect(initialRole).toBeDefined()
    authzApiMocks.updateAuthzRole.mockResolvedValue({
      role: { ...initialRole!, name: '鲜花公司HR', revision: 4 }
    })

	    renderWithQueryClient(<RoleManagementPage />)

	    await screen.findByText('编辑角色：鲜花公司HR')
	    fireEvent.change(screen.getByLabelText('角色描述'), { target: { value: '查看组织范围内组织' } })
	    const saveButton = screen.getByRole('button', { name: '保存' })
	    await waitFor(() => expect(saveButton).toBeEnabled())
	    fireEvent.click(saveButton)

    await waitFor(() => {
      expect(authzApiMocks.updateAuthzRole).toHaveBeenCalledWith('flower-hr', {
        role_slug: 'flower-hr',
        name: '鲜花公司HR',
        description: '查看组织范围内组织',
        revision: 3,
        authz_capability_keys: ['orgunit.orgunits:read']
      })
    })
  })

  it('switches to organization scope tab when user authorization save requires scope', async () => {
    authzApiMocks.replacePrincipalAuthzAssignment.mockRejectedValue(
      new ApiClientError('该角色需要选择组织范围。', 'UNKNOWN_ERROR', 422, undefined, { code: 'authz_org_scope_required' })
    )

	    renderWithQueryClient(<UserAuthorizationPage />)

	    await screen.findByText('用户：user-b@example.invalid · 王小花')
	    await screen.findByTestId('row-flower-hr')
	    const saveButton = screen.getByRole('button', { name: '保存' })
	    await waitFor(() => expect(saveButton).toBeEnabled())
	    fireEvent.click(saveButton)

    await waitFor(() => {
      expect(authzApiMocks.replacePrincipalAuthzAssignment).toHaveBeenCalledWith('principal-b', {
        roles: [{ role_slug: 'flower-hr' }],
        org_scopes: [],
        revision: 5
      })
    })
    expect(await screen.findByText('该角色需要选择组织范围。')).toBeInTheDocument()
    const selectedTab = screen.getByRole('tab', { name: '组织范围' })
    expect(selectedTab).toHaveAttribute('aria-selected', 'true')
    expect(within(screen.getByTestId('grid')).getByText('暂无组织范围')).toBeInTheDocument()
  })

  it('preserves org scopes when saving role-only edits', async () => {
    authzApiMocks.getPrincipalAuthzAssignment.mockResolvedValue({
      principal_id: 'principal-b',
      roles: [{ role_slug: 'flower-hr', display_name: '鲜花公司HR', description: '查看组织', requires_org_scope: true }],
      org_scopes: [{ org_code: 'FLOWERS', org_name: '鲜花事业部', include_descendants: true }],
      revision: 5
    })
    authzApiMocks.replacePrincipalAuthzAssignment.mockResolvedValue({
      principal_id: 'principal-b',
      roles: [{ role_slug: 'dict-reader', display_name: '字典只读', description: '查看字典', requires_org_scope: false }],
      org_scopes: [{ org_code: 'FLOWERS', org_name: '鲜花事业部', include_descendants: true }],
      revision: 6
    })

    renderWithQueryClient(<UserAuthorizationPage />)

    await screen.findByText('用户：user-b@example.invalid · 王小花')
    await screen.findByTestId('row-flower-hr')
    fireEvent.click(screen.getByRole('button', { name: '添加行' }))
    await screen.findByTestId(/row-role-/)
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(authzApiMocks.replacePrincipalAuthzAssignment).toHaveBeenCalledWith('principal-b', {
        roles: [{ role_slug: 'flower-hr' }, { role_slug: 'dict-reader' }],
        org_scopes: [{ org_code: 'FLOWERS', org_name: '鲜花事业部', include_descendants: true }],
        revision: 5
      })
    })
  })

  it('preserves roles when saving org-scope-only edits', async () => {
    authzApiMocks.getPrincipalAuthzAssignment.mockResolvedValue({
      principal_id: 'principal-b',
      roles: [{ role_slug: 'flower-hr', display_name: '鲜花公司HR', description: '查看组织', requires_org_scope: true }],
      org_scopes: [{ org_code: 'FLOWERS', org_name: '鲜花事业部', include_descendants: true }],
      revision: 5
    })
    authzApiMocks.replacePrincipalAuthzAssignment.mockResolvedValue({
      principal_id: 'principal-b',
      roles: [{ role_slug: 'flower-hr', display_name: '鲜花公司HR', description: '查看组织', requires_org_scope: true }],
      org_scopes: [],
      revision: 6
    })

    renderWithQueryClient(<UserAuthorizationPage />)

    await screen.findByText('用户：user-b@example.invalid · 王小花')
    fireEvent.click(screen.getByRole('tab', { name: '组织范围' }))
    await screen.findByTestId('row-FLOWERS')
    fireEvent.click(screen.getByLabelText('移除组织范围'))
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(authzApiMocks.replacePrincipalAuthzAssignment).toHaveBeenCalledWith('principal-b', {
        roles: [{ role_slug: 'flower-hr' }],
        org_scopes: [],
        revision: 5
      })
    })
  })
})
