import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GridColDef } from '@mui/x-data-grid'
import { APIAuthorizationCatalogPage, CapabilityAuthorizationsPage } from './AuthzCatalogPage'

const authzApiMocks = vi.hoisted(() => ({
  listAuthzAPICatalog: vi.fn(),
  listAuthzCapabilities: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/authz', () => authzApiMocks)
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
    <div data-testid='authz-grid'>
      {rows.length === 0 ? <div>{noRowsLabel}</div> : null}
      {rows.map((row) => (
        <div data-testid={`authz-row-${String(row.id)}`} key={String(row.id)}>
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

describe('AuthzCatalogPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    appPreferencesMocks.useAppPreferences.mockReturnValue({
      hasRequiredCapability: () => true,
      locale: 'zh',
      navDebugMode: false,
      setLocale: vi.fn(),
      t: (key: string) =>
        ({
          authz_access_internal_api: '内部 API',
          authz_access_ops: 'health',
          authz_access_protected: '受保护',
          authz_access_public_api: '公开 API',
          authz_access_authn: '登录握手',
          authz_access_static: '静态资源',
          authz_action: '操作',
          authz_action_label: '操作',
          authz_api_access_control: '访问控制',
          authz_api_catalog_empty: '暂无 API 授权目录项',
          authz_api_catalog_search: '搜索 API 路径 / 授权项标识 / 资源',
          authz_api_method: '方法',
          authz_api_path: 'API 路径',
          authz_associated_api_empty: '暂无关联 API',
          authz_associated_api_title: '关联 API',
          authz_capability_empty: '暂无可分配功能授权项',
          authz_capability_key: '授权项标识',
          authz_capability_no_match: '没有匹配的功能授权项',
          authz_capability_search: '搜索标识 / 资源 / 模块',
          authz_cubebox_callable: '丘宝可调用',
          authz_current_coverage: '当前覆盖',
          authz_filter_access_control: '访问控制',
          authz_filter_all_access_controls: '全部访问控制',
          authz_filter_all_methods: '全部方法',
          authz_filter_all_modules: '全部模块',
          authz_filter_all_resources: '资源：全部',
          authz_filter_all_scopes: '全部范围',
          authz_filter_method: '方法',
          authz_filter_module: '模块',
          authz_filter_resource: '资源',
          authz_filter_scope: '范围',
          authz_owner_module: '归属模块',
          authz_resource_label: '资源',
          authz_resource_object: '资源对象',
          authz_scope_dimension: '组织范围',
          authz_scope_none: '不需要',
          authz_scope_organization: '需要',
          authz_status_enabled: '可用',
          common_cancel: '取消',
          common_no: '否',
          common_yes: '是',
          page_authz_api_catalog_subtitle: '查看 API 路径与授权资源、操作、授权项标识的绑定关系',
          page_authz_api_catalog_title: 'API 授权目录',
          page_authz_capabilities_subtitle: '管理角色可选择的功能授权项及其授权维度',
          page_authz_capabilities_title: '功能授权项',
          text_loading: '加载中',
          text_status: '状态',
          theme_dark: '深色',
          theme_light: '浅色'
        })[key] ?? key,
      tenantId: 'tenant-a',
      themeMode: 'light',
      toggleThemeMode: vi.fn()
    })

    authzApiMocks.listAuthzCapabilities.mockResolvedValue({
      capabilities: [
        {
          action: 'read',
          action_label: '查看',
          assignable: true,
          authz_capability_key: 'orgunit.orgunits:read',
          covered: true,
          label: '组织单元 · 查看',
          object: 'orgunit.orgunits',
          owner_module: 'orgunit',
          resource_label: '组织单元',
          scope_dimension: 'organization',
          sort_order: 10,
          status: 'enabled',
          surface: 'tenant_api'
        }
      ],
      registry_rev: 'rev'
    })
    authzApiMocks.listAuthzAPICatalog.mockResolvedValue({
      api_entries: [
        {
          access_control: 'protected',
          action: 'read',
          assignable: true,
          authz_capability_key: 'orgunit.orgunits:read',
          capability_status: 'enabled',
          cubebox_callable: true,
          method: 'GET',
          owner_module: 'orgunit',
          path: '/org/api/org-units',
          resource_label: '组织单元',
          resource_object: 'orgunit.orgunits'
        }
      ]
    })
  })

  it('shows capability rows without persistent API method and path columns', async () => {
    renderWithQueryClient(<CapabilityAuthorizationsPage />)

    const row = await screen.findByTestId('authz-row-orgunit.orgunits:read')
    expect(within(row).getByText('组织单元')).toBeInTheDocument()
    expect(within(row).getByText('orgunit.orgunits:read')).toBeInTheDocument()
    expect(within(row).queryByText('GET')).not.toBeInTheDocument()
    expect(within(row).queryByText('/org/api/org-units')).not.toBeInTheDocument()
  })

  it('loads associated APIs through the API catalog facade when a capability key is opened', async () => {
    renderWithQueryClient(<CapabilityAuthorizationsPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'orgunit.orgunits:read' }))

    await waitFor(() => {
      expect(authzApiMocks.listAuthzAPICatalog).toHaveBeenCalledWith({
        authzCapabilityKey: 'orgunit.orgunits:read'
      })
    })
    expect(await screen.findByText('关联 API')).toBeInTheDocument()
    expect(await screen.findByText('/org/api/org-units')).toBeInTheDocument()
    expect(screen.getByText('GET')).toBeInTheDocument()
  })

  it('renders the API catalog as the forward API view', async () => {
    renderWithQueryClient(<APIAuthorizationCatalogPage />)

    const row = await screen.findByTestId('authz-row-GET /org/api/org-units')
    expect(within(row).getByText('/org/api/org-units')).toBeInTheDocument()
    expect(within(row).getByText('orgunit.orgunits:read')).toBeInTheDocument()
    expect(within(row).getByText('是')).toBeInTheDocument()
  })
})
