import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
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
      hasPermission: () => false,
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
          action_apply_filters: 'Apply Filters',
          org_tree_title: 'Tree',
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
})
