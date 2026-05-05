import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiClientError } from '../../api/errors'
import { OrgUnitsPage } from './OrgUnitsPage'

const orgUnitApiMocks = vi.hoisted(() => ({
  listOrgUnits: vi.fn(),
  listOrgUnitsPage: vi.fn(),
  listOrgUnitFieldConfigs: vi.fn(),
  searchOrgUnit: vi.fn(),
  writeOrgUnit: vi.fn(),
  getOrgUnitFieldOptions: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/orgUnits', () => orgUnitApiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../../observability/tracker', () => ({
  trackUiEvent: vi.fn()
}))
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
vi.mock('../../components/DataGridPage', () => ({
  DataGridPage: ({ rows }: { rows: Array<{ id: string }> }) => <div data-testid='grid-row-count'>{rows.length}</div>
}))
vi.mock('../../components/TreePanel', () => ({
  TreePanel: () => <div data-testid='tree-panel'>tree</div>
}))
vi.mock('../../components/StatusChip', () => ({
  StatusChip: ({ label }: { label: string }) => <span>{label}</span>
}))
vi.mock('../../components/OrgUnitTreeSelector', () => ({
  OrgUnitTreeField: ({
    label,
    onChange,
    value
  }: {
    label: string
    onChange: (value: { org_code: string; org_node_key: string; name: string; status: 'active'; has_visible_children: boolean }) => void
    value?: { org_code: string; name: string } | null
  }) => (
    <button
      aria-label={label}
      type='button'
      onClick={() =>
        onChange({
          org_code: 'DEEP',
          org_node_key: '10000099',
          name: 'Deep Parent',
          status: 'active',
          has_visible_children: false
        })
      }
    >
      {value ? `${value.name} (${value.org_code})` : label}
    </button>
  )
}))

function LocationProbe() {
  const location = useLocation()
  return <div data-testid='location-search'>{location.search}</div>
}

function renderPage(initialEntry = '/org/units') {
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
            path='/org/units'
            element={
              <>
                <OrgUnitsPage />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('OrgUnitsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'en',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasRequiredCapability: () => false,
      t: (key: string) =>
        ({
          page_org_title: 'Organization Units',
          page_org_subtitle: 'Org subtitle',
          org_filter_keyword: 'Keyword',
          org_filter_status: 'Status',
          status_all: 'All',
          status_active: 'Active',
          status_inactive: 'Inactive',
          org_filter_as_of: 'As Of Date',
          common_view_current_label: 'Viewing current data by default',
          common_view_history: 'View History',
          common_view_current: 'View Current',
          org_filter_include_disabled: 'Include Disabled',
          common_confirm: 'Confirm',
          org_action_create: 'Create',
          org_column_code: 'Code',
          org_column_name: 'Name',
          org_column_parent: 'Parent',
          org_column_status: 'Status',
          org_status_active_short: 'Active',
          org_status_inactive_short: 'Inactive',
          org_column_manager: 'Manager',
          org_column_effective_date: 'Effective Date',
          org_column_is_business_unit: 'Business Unit',
          action_apply_filters: 'Apply Filters',
          org_tree_title: 'Tree',
          org_search_label: 'Search in tree',
          org_search_action: 'Locate',
          org_search_query_required: 'Please input a search query',
          org_search_ambiguous_prefix: 'Found multiple matching organization units:',
          text_loading: 'Loading',
          text_no_data: 'No data'
        })[key] ?? key
    })
    orgUnitApiMocks.listOrgUnits.mockResolvedValue({
      as_of: '2026-04-08',
      org_units: [
        {
          org_code: 'ROOT',
          name: 'Root Unit',
          status: 'active',
          has_children: false,
          is_business_unit: true
        }
      ]
    })
    orgUnitApiMocks.listOrgUnitsPage.mockResolvedValue({
      as_of: '2026-04-08',
      total: 1,
      org_units: [
        {
          org_code: 'ROOT',
          name: 'Root Unit',
          status: 'active',
          has_children: false,
          is_business_unit: true
        }
      ]
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('defaults to current mode and hides as_of input', async () => {
    renderPage()

    await waitFor(() => expect(orgUnitApiMocks.listOrgUnits).toHaveBeenCalled())
    await waitFor(() => expect(orgUnitApiMocks.listOrgUnitsPage).toHaveBeenCalled())

    expect(screen.getByText('Viewing current data by default')).toBeInTheDocument()
    expect(screen.queryByLabelText('As Of Date')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'View History' })).toBeInTheDocument()
    expect(screen.getByTestId('location-search')).toHaveTextContent('')
    expect(orgUnitApiMocks.listOrgUnits).toHaveBeenCalledWith({ asOf: expect.any(String), includeDisabled: false })
  }, 20000)

  it('switches to history mode and writes as_of to search params on apply', async () => {
    renderPage()

    await waitFor(() => expect(orgUnitApiMocks.listOrgUnits).toHaveBeenCalled())

    fireEvent.click(screen.getByRole('button', { name: 'View History' }))

    const asOfInput = await screen.findByLabelText('As Of Date')
    fireEvent.change(asOfInput, { target: { value: '2026-03-01' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply Filters' }))

    await waitFor(() =>
      expect(screen.getByTestId('location-search').textContent).toContain('as_of=2026-03-01')
    )
    await waitFor(() =>
      expect(orgUnitApiMocks.listOrgUnits).toHaveBeenLastCalledWith({ asOf: '2026-03-01', includeDisabled: false })
    )
  }, 20000)

  it('shows ambiguous tree search candidates from the API response', async () => {
    orgUnitApiMocks.searchOrgUnit.mockRejectedValue(
      new ApiClientError('api tool http status 409', 'UNKNOWN_ERROR', 409, undefined, {
        error_code: 'org_unit_search_ambiguous',
        candidates: [
          { org_code: 'A001', name: 'East Sales Center' },
          { org_code: 'A002', name: 'East Operations Center' }
        ]
      })
    )

    renderPage()
    await waitFor(() => expect(orgUnitApiMocks.listOrgUnits).toHaveBeenCalled())

    fireEvent.change(screen.getByLabelText('Search in tree'), { target: { value: 'East' } })
    fireEvent.click(screen.getByRole('button', { name: 'Locate' }))

    await waitFor(() =>
      expect(screen.getByText(/East Sales Center \(A001\).*East Operations Center \(A002\)/)).toBeInTheDocument()
    )
  }, 20000)

  it('keeps create parent selector label for nodes outside the already loaded page tree', async () => {
    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'en',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasRequiredCapability: () => true,
      t: (key: string) =>
        ({
          page_org_title: 'Organization Units',
          page_org_subtitle: 'Org subtitle',
          nav_org_field_configs: 'Field Configs',
          org_filter_keyword: 'Keyword',
          org_filter_status: 'Status',
          status_all: 'All',
          status_active: 'Active',
          status_inactive: 'Inactive',
          org_filter_as_of: 'As Of Date',
          common_view_current_label: 'Viewing current data by default',
          common_view_history: 'View History',
          common_view_current: 'View Current',
          common_confirm: 'Confirm',
          common_cancel: 'Cancel',
          common_action_done: 'Done',
          org_filter_include_disabled: 'Include Disabled',
          org_action_create: 'Create',
          org_column_code: 'Code',
          org_column_name: 'Name',
          org_column_parent: 'Parent',
          org_column_status: 'Status',
          org_status_active_short: 'Active',
          org_status_inactive_short: 'Inactive',
          org_column_manager: 'Manager',
          org_column_effective_date: 'Effective Date',
          org_column_is_business_unit: 'Business Unit',
          action_apply_filters: 'Apply Filters',
          org_tree_title: 'Tree',
          org_search_label: 'Search in tree',
          org_search_action: 'Locate',
          org_search_query_required: 'Please input a search query',
          org_search_ambiguous_prefix: 'Found multiple matching organization units:',
          text_loading: 'Loading',
          text_no_data: 'No data'
        })[key] ?? key
    })
    orgUnitApiMocks.writeOrgUnit.mockResolvedValue({})
    orgUnitApiMocks.listOrgUnitFieldConfigs.mockResolvedValue({ field_configs: [] })

    renderPage()

    await waitFor(() => expect(orgUnitApiMocks.listOrgUnits).toHaveBeenCalled())
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    fireEvent.change(screen.getByLabelText('Code'), { target: { value: 'NEW' } })
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Org' } })
    fireEvent.click(screen.getByRole('button', { name: 'Parent' }))

    expect(screen.getByRole('button', { name: 'Parent' })).toHaveTextContent('Deep Parent (DEEP)')

    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() =>
      expect(orgUnitApiMocks.writeOrgUnit).toHaveBeenCalledWith(
        expect.objectContaining({
          intent: 'create_org',
          org_code: 'NEW',
          patch: expect.objectContaining({
            parent_org_code: 'DEEP'
          })
        })
      )
    )
  }, 20000)
})
