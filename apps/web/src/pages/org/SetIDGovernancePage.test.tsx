import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SetIDGovernancePage } from './SetIDGovernancePage'

const setIDApiMocks = vi.hoisted(() => ({
  activatePolicyVersion: vi.fn(),
  bindSetID: vi.fn(),
  createSetID: vi.fn(),
  disableSetIDStrategyRegistry: vi.fn(),
  getPolicyActivationState: vi.fn(),
  listCapabilityCatalogByIntent: vi.fn(),
  listSetIDBindings: vi.fn(),
  listFunctionalAreaState: vi.fn(),
  listSetIDStrategyRegistry: vi.fn(),
  listSetIDs: vi.fn(),
  rollbackPolicyVersion: vi.fn(),
  setPolicyDraft: vi.fn(),
  switchFunctionalAreaState: vi.fn(),
  upsertSetIDStrategyRegistry: vi.fn()
}))

const orgUnitApiMocks = vi.hoisted(() => ({
  listOrgUnits: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/setids', () => setIDApiMocks)
vi.mock('../../api/orgUnits', () => orgUnitApiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../../components/PageHeader', () => ({
  PageHeader: ({ title, subtitle }: { title: string; subtitle?: string }) => (
    <div>
      <h1>{title}</h1>
      <div>{subtitle}</div>
    </div>
  )
}))
vi.mock('../../components/DataGridPage', () => ({
  DataGridPage: () => <div data-testid='setid-grid'>grid</div>
}))
vi.mock('../../components/SetIDExplainPanel', () => ({
  SetIDExplainPanel: ({
    asOfHint,
    asOfLabel
  }: {
    asOfHint?: string
    asOfLabel?: string
  }) => (
    <div data-testid='setid-explain'>
      explain|label={asOfLabel ?? '-'}|hint={asOfHint ?? '-'}
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
  return <div data-testid='location-search'>{location.search}</div>
}

function renderPage(
  initialEntry = '/org/setid/registry?registry_view=editor',
  section: 'base' | 'registry' | 'explain' = 'registry'
) {
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
            path='/org/setid/registry'
            element={
                <>
                <SetIDGovernancePage section={section} />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

function findDateInputByValue(value: string): HTMLInputElement {
  const input = document.querySelector(`input[type="date"][value="${value}"]`)
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`date input with value ${value} not found`)
  }
  return input
}

describe('SetIDGovernancePage', () => {
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
      t: (key: string, vars?: Record<string, string>) =>
        ({
          nav_configuration_policy: 'Configuration Policy',
          nav_configuration_policy_registry: 'Registry',
          org_filter_as_of: 'As Of Date',
          common_view_current_label: 'Viewing current data by default',
          common_view_history: 'View History',
          common_view_current: 'View Current',
          org_view_history_context: `Viewing history as of ${vars?.date ?? ''}`,
          setid_explain_as_of_hint: 'This time is only used for the current explain request and will not change the host page browsing date.',
          setid_registry_effective_date: 'Rule Effective Date',
          setid_registry_disable_as_of: 'Disable Date'
        })[key] ?? key
    })

    setIDApiMocks.listSetIDs.mockResolvedValue({ setids: [] })
    setIDApiMocks.listSetIDBindings.mockResolvedValue({ bindings: [] })
    setIDApiMocks.listCapabilityCatalogByIntent.mockResolvedValue({
      items: [
        {
          capability_key: 'orgunit.orgunit.api_write.write_all',
          owner_module: 'orgunit',
          target_object: 'orgunit',
          surface: 'api_write',
          intent: 'write_all'
        }
      ]
    })
    setIDApiMocks.listSetIDStrategyRegistry.mockResolvedValue({ items: [] })
    setIDApiMocks.listFunctionalAreaState.mockResolvedValue({ items: [] })
    setIDApiMocks.getPolicyActivationState.mockResolvedValue({
      capability_key: 'org.policy_activation.manage',
      active_policy_version: 'v1',
      draft_policy_version: 'v1'
    })
    orgUnitApiMocks.listOrgUnits.mockResolvedValue({
      as_of: '2026-04-08',
      org_units: []
    })
  })

  it('keeps registry effective_date on current date by default', async () => {
    renderPage()

    await waitFor(() => expect(findDateInputByValue('2026-04-08')).toBeInTheDocument(), {
      timeout: 10000
    })
    expect(screen.queryByLabelText('As Of Date')).not.toBeInTheDocument()
    expect(screen.getAllByText('Viewing current data by default').length).toBeGreaterThan(0)
  }, 15000)

  it('does not sync registry effective_date when browsing history as_of changes', async () => {
    renderPage()

    await waitFor(() => expect(findDateInputByValue('2026-04-08')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'View History' }))

    const asOfInput = await screen.findByLabelText('As Of Date')
    fireEvent.change(asOfInput, { target: { value: '2026-03-01' } })

    await waitFor(() =>
      expect(screen.getByTestId('location-search').textContent).toContain('as_of=2026-03-01')
    )
    expect(findDateInputByValue('2026-04-08')).toBeInTheDocument()
    expect(screen.getByText('Viewing history as of 2026-03-01')).toBeInTheDocument()
  }, 20000)

  it('passes task-time hint into explain tooling section', async () => {
    renderPage('/org/setid/registry', 'explain')

    await waitFor(() => expect(screen.getByTestId('setid-explain')).toBeInTheDocument())
    expect(screen.getByTestId('setid-explain')).toHaveTextContent(
      'hint=This time is only used for the current explain request and will not change the host page browsing date.'
    )
  }, 20000)

  it('uses task-oriented labels for registry editor dates', async () => {
    renderPage()

    await waitFor(() => expect(screen.getByText('Rule Effective Date')).toBeInTheDocument())
    expect(screen.queryByText('effective_date')).not.toBeInTheDocument()
  }, 20000)

  it('renders bindings with org_code-only API payloads', async () => {
    setIDApiMocks.listSetIDBindings.mockResolvedValue({
      bindings: [
        {
          org_code: 'ROOT',
          setid: 'S2601',
          valid_from: '2026-01-01',
          valid_to: ''
        }
      ]
    })

    renderPage('/org/setid/registry?base_view=bindings', 'base')

    await waitFor(() => expect(screen.getByText('Bindings')).toBeInTheDocument())
    expect(screen.getByRole('columnheader', { name: 'org_code' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('ROOT')).toBeInTheDocument())
  }, 20000)
})
