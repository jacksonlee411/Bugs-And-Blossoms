import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { OrgUnitDetailsPage } from './OrgUnitDetailsPage'

const orgUnitApiMocks = vi.hoisted(() => ({
  getOrgUnitFieldOptions: vi.fn(),
  getOrgUnitDetails: vi.fn(),
  listOrgUnitAudit: vi.fn(),
  listOrgUnitVersions: vi.fn(),
  rescindOrgUnit: vi.fn(),
  rescindOrgUnitRecord: vi.fn(),
  writeOrgUnit: vi.fn()
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

function renderPage(initialEntry: string) {
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
            path='/org/units/:orgCode'
            element={
              <>
                <OrgUnitDetailsPage />
                <LocationProbe />
              </>
            }
          />
          <Route path='/org/units' element={<LocationProbe />} />
          <Route path='/org/units/field-configs' element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('OrgUnitDetailsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'en',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasRequiredCapability: () => true,
      t: (key: string, vars?: Record<string, string>) =>
        ({
          common_detail: 'Detail',
          common_view_current: 'View Current',
          common_yes: 'Yes',
          common_no: 'No',
          common_action_done: 'Done',
          common_confirm: 'Confirm',
          nav_org_units: 'Organization Units',
          nav_org_field_configs: 'Field Configs',
          org_action_create: 'Create',
          org_action_add_version: 'Add Version',
          org_action_insert_version: 'Insert Version',
          org_action_correct: 'Correct',
          org_action_delete: 'Delete',
          org_tab_profile: 'Basic Info',
          org_tab_audit: 'Audit',
          org_detail_title_suffix: 'Details',
          org_detail_selected_version: 'Version Details',
          org_view_history_context: `Viewing history as of ${vars?.date ?? ''}`,
          org_column_effective_date: 'Effective Date',
          org_column_code: 'Code',
          org_column_name: 'Name',
          org_column_status: 'Status',
          org_column_parent: 'Parent',
          org_column_manager: 'Manager',
          org_column_is_business_unit: 'Business Unit',
          org_column_full_name_path: 'Full Name Path',
          org_status_active_short: 'Active',
          org_status_inactive_short: 'Inactive',
          org_corrected_effective_date: 'Corrected Effective Date',
          text_loading: 'Loading',
          text_no_data: 'No data'
        })[key] ?? key
    })

    orgUnitApiMocks.listOrgUnitVersions.mockResolvedValue({
      org_code: 'ROOT',
      versions: [
        { event_id: 1, event_uuid: 'evt-1', effective_date: '2026-03-01', event_type: 'created' },
        { event_id: 2, event_uuid: 'evt-2', effective_date: '2026-04-01', event_type: 'updated' }
      ]
    })
    orgUnitApiMocks.getOrgUnitDetails.mockImplementation(async ({ asOf }: { asOf: string }) => ({
      as_of: asOf,
      org_unit: {
        org_id: 1,
        org_code: 'ROOT',
        name: asOf === '2026-04-01' ? 'Root Unit V2' : 'Root Unit',
        status: 'active',
        parent_org_code: '',
        parent_name: '',
        is_business_unit: true,
        manager_pernr: '0001',
        manager_name: 'Alice',
        full_name_path: 'Root Unit',
        created_at: '2026-03-01T00:00:00Z',
        updated_at: '2026-04-01T00:00:00Z',
        event_uuid: asOf === '2026-04-01' ? 'evt-2' : 'evt-1'
      },
      ext_fields: []
    }))
    orgUnitApiMocks.listOrgUnitAudit.mockResolvedValue({
      org_code: 'ROOT',
      limit: 20,
      has_more: false,
      events: []
    })
    orgUnitApiMocks.writeOrgUnit.mockResolvedValue({
      org_code: 'ROOT',
      effective_date: '2026-03-01',
      event_type: 'orgunit_corrected',
      event_uuid: 'evt-3'
    })
  })

  it('normalizes legacy history as_of into a single effective_date anchor', async () => {
    renderPage('/org/units/ROOT?as_of=2026-03-15')

    await waitFor(() =>
      expect(orgUnitApiMocks.getOrgUnitDetails).toHaveBeenCalledWith({
        orgCode: 'ROOT',
        asOf: '2026-03-01',
        includeDisabled: false
      })
    )
    expect(screen.getByText('Viewing history as of 2026-03-01')).toBeInTheDocument()
    expect(screen.getByTestId('location-state').textContent).toContain('/org/units/ROOT')
  }, 20000)

  it('keeps action effective date sticky when switching selected version', async () => {
    renderPage('/org/units/ROOT?effective_date=2026-03-01')

    const correctButton = await screen.findByRole('button', { name: 'Correct' })
    await waitFor(() => expect(correctButton).not.toBeDisabled())

    fireEvent.click(correctButton)
    const dialog = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(within(dialog).getAllByDisplayValue('2026-03-01').length).toBeGreaterThan(0)
    )

    fireEvent.click(screen.getByTestId('org-version-2026-04-01'))

    await waitFor(() =>
      expect(screen.getByTestId('location-state').textContent).toBe('/org/units/ROOT?effective_date=2026-04-01&tab=profile')
    )
    expect(within(dialog).getAllByDisplayValue('2026-03-01').length).toBeGreaterThan(0)
  }, 20000)

  it('does not auto-jump read state after successful write', async () => {
    renderPage('/org/units/ROOT?effective_date=2026-03-01')

    const correctButton = await screen.findByRole('button', { name: 'Correct' })
    await waitFor(() => expect(correctButton).not.toBeDisabled())

    fireEvent.click(correctButton)
    const dialog = await screen.findByRole('dialog')
    const nameInput = await within(dialog).findByLabelText('Name')
    fireEvent.change(nameInput, { target: { value: 'Root Unit Updated' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Confirm' }))

    await waitFor(() =>
        expect(orgUnitApiMocks.writeOrgUnit).toHaveBeenCalledWith(
          expect.objectContaining({
            intent: 'correct',
            org_code: 'ROOT',
            effective_date: '2026-03-01',
            target_effective_date: '2026-03-01'
          })
        )
      )
    expect(screen.getByTestId('location-state').textContent).toBe('/org/units/ROOT?effective_date=2026-03-01')
  }, 20000)
})
