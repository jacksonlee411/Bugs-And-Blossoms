import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GridColDef } from '@mui/x-data-grid'
import { OrgUnitFieldConfigsPage } from './OrgUnitFieldConfigsPage'

const orgUnitApiMocks = vi.hoisted(() => ({
  disableOrgUnitFieldConfig: vi.fn(),
  enableOrgUnitFieldConfig: vi.fn(),
  listOrgUnitFieldConfigEnableCandidates: vi.fn(),
  listOrgUnitFieldConfigs: vi.fn(),
  listOrgUnitFieldDefinitions: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/orgUnits', () => orgUnitApiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../../components/PageHeader', () => ({
  PageHeader: ({ title, actions }: { title: string; actions?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      <div>{actions}</div>
    </div>
  )
}))
vi.mock('../../components/FilterBar', () => ({
  FilterBar: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))
vi.mock('../../components/StatusChip', () => ({
  StatusChip: ({ label }: { label: string }) => <span>{label}</span>
}))
vi.mock('../../components/DataGridPage', () => ({
  DataGridPage: ({
    columns,
    rows
  }: {
    columns: GridColDef[]
    rows: Array<Record<string, unknown>>
  }) => (
    <div data-testid='field-config-grid'>
      {rows.map((row) => (
        <div data-testid={`field-config-row-${String(row.id)}`} key={String(row.id)}>
          {columns.map((column) => {
            const key = `${String(row.id)}:${String(column.field)}`
            if (typeof column.renderCell === 'function') {
              return (
                <div key={key}>
                  {column.renderCell({
                    row,
                    value: row[column.field as string],
                    field: String(column.field),
                    id: row.id
                  } as never)}
                </div>
              )
            }
            return <div key={key}>{String(row[column.field as string] ?? '')}</div>
          })}
        </div>
      ))}
    </div>
  )
}))
vi.mock('./readViewState', async () => {
  const actual = await vi.importActual<typeof import('./readViewState')>('./readViewState')
  return {
    ...actual,
    todayISODate: () => '2026-04-08'
  }
})

function LocationProbe() {
  const location = useLocation()
  return <div data-testid='location-state'>{`${location.pathname}${location.search}`}</div>
}

function renderPage(initialEntry = '/org/units/field-configs') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path='/org/units/field-configs'
            element={
              <>
                <OrgUnitFieldConfigsPage />
                <LocationProbe />
              </>
            }
          />
          <Route path='/org/setid/registry' element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('OrgUnitFieldConfigsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'en',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasPermission: () => true,
      t: (key: string) =>
        ({
          nav_org_units: 'Organization Units',
          org_field_configs_title: 'Field Configs',
          org_field_configs_subtitle: 'Field Config subtitle',
          org_field_configs_filter_as_of: 'As Of Date',
          org_field_configs_filter_status: 'Status',
          org_field_configs_filter_disabled_state: 'Disabled State',
          org_field_configs_filter_keyword: 'Keyword',
          org_field_configs_state_enabled: 'Enabled',
          org_field_configs_state_disabled_bucket: 'Disabled Bucket',
          org_field_configs_state_pending: 'Pending',
          org_field_configs_state_disabled: 'Disabled',
          common_view_current_label: 'Viewing current data by default',
          common_view_history: 'View History',
          common_view_current: 'View Current',
          action_apply_filters: 'Apply Filters',
          status_all: 'All',
          text_status: 'Status',
          text_loading: 'Loading',
          text_no_data: 'No data',
          common_yes: 'Yes',
          common_no: 'No',
          common_detail: 'Detail',
          nav_setid: 'SetID',
          org_field_configs_column_label: 'Label',
          org_field_configs_column_key: 'Key',
          org_field_configs_column_field_class: 'Field Class',
          org_field_configs_column_value_type: 'Value Type',
          org_field_configs_column_data_source_type: 'Data Source Type',
          org_field_configs_column_data_source_config: 'Data Source Config',
          org_field_configs_column_maintainable: 'Maintainable',
          org_field_configs_column_default_value: 'Default Value',
          org_field_configs_column_policy_scope: 'Policy Scope',
          org_field_configs_column_enabled_on: 'Enabled On',
          org_field_configs_column_disabled_on: 'Disabled On',
          org_field_configs_column_physical_col: 'Physical Col',
          org_field_configs_column_updated_at: 'Updated At',
          org_field_configs_column_actions: 'Actions',
          org_field_configs_action_enable: 'Enable Field',
          org_field_configs_action_disable: 'Disable',
          org_field_configs_action_postpone: 'Postpone',
          org_field_configs_empty: 'No field configs'
        })[key] ?? key
    })

    orgUnitApiMocks.listOrgUnitFieldDefinitions.mockResolvedValue({
      fields: [
        {
          field_key: 'x_cost_center',
          label_i18n_key: '',
          data_source_type: 'PLAIN'
        }
      ]
    })
    orgUnitApiMocks.listOrgUnitFieldConfigs.mockResolvedValue({
      field_configs: [
        {
          field_key: 'x_cost_center',
          field_class: 'EXT',
          label_i18n_key: '',
          label: 'Cost Center',
          value_type: 'text',
          data_source_type: 'PLAIN',
          data_source_config: {},
          physical_col: 'ext_text_01',
          enabled_on: '2026-04-01',
          disabled_on: null,
          updated_at: '2026-04-08T00:00:00Z',
          maintainable: true,
          default_mode: 'NONE',
          default_rule_expr: '',
          policy_scope_type: 'FORM',
          policy_scope_key: 'orgunit.create_dialog'
        }
      ]
    })
  })

  it('defaults to current mode and does not show as_of input', async () => {
    renderPage()

    await waitFor(() => expect(orgUnitApiMocks.listOrgUnitFieldDefinitions).toHaveBeenCalled())
    await waitFor(() =>
      expect(orgUnitApiMocks.listOrgUnitFieldConfigs).toHaveBeenCalledWith({ asOf: '2026-04-08', status: 'all' })
    )

    expect(screen.queryByLabelText('As Of Date')).not.toBeInTheDocument()
    expect(screen.getByText('Viewing current data by default')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'View History' })).toBeInTheDocument()
  }, 20000)

  it('omits as_of when opening setid registry from current mode', async () => {
    renderPage()

    await waitFor(() => expect(screen.getByRole('button', { name: 'SetID' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'SetID' }))
    await waitFor(() =>
      expect(screen.getByTestId('location-state').textContent).toBe(
        '/org/setid/registry?registry_view=editor&capability_key=org.orgunit_write.field_policy&field_key=x_cost_center'
      )
    )
  }, 20000)

  it('passes as_of to setid registry in history mode', async () => {
    cleanup()

    renderPage('/org/units/field-configs')
    await waitFor(() => expect(screen.getByRole('button', { name: 'View History' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'View History' }))
    const asOfInput = await screen.findByLabelText('As Of Date')
    fireEvent.change(asOfInput, { target: { value: '2026-03-01' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply Filters' }))

    await waitFor(() =>
      expect(screen.getByTestId('location-state').textContent).toContain('/org/units/field-configs?as_of=2026-03-01')
    )
    await waitFor(() =>
      expect(orgUnitApiMocks.listOrgUnitFieldConfigs).toHaveBeenLastCalledWith({ asOf: '2026-03-01', status: 'all' })
    )

    fireEvent.click(screen.getByRole('button', { name: 'SetID' }))
    await waitFor(() =>
      expect(screen.getByTestId('location-state').textContent).toBe(
        '/org/setid/registry?as_of=2026-03-01&registry_view=editor&capability_key=org.orgunit_write.field_policy&field_key=x_cost_center'
      )
    )
  }, 20000)
})
