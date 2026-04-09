import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { JobCatalogPage } from './JobCatalogPage'

const currentUTCDate = () => new Date().toISOString().slice(0, 10)

const jobCatalogApiMocks = vi.hoisted(() => ({
  applyJobCatalogAction: vi.fn(),
  getJobCatalog: vi.fn()
}))

const setidApiMocks = vi.hoisted(() => ({
  listSetIDs: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/jobCatalog', () => jobCatalogApiMocks)
vi.mock('../../api/setids', () => setidApiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../../components/PageHeader', () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>
}))
vi.mock('../../components/FilterBar', () => ({
  FilterBar: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))
vi.mock('../../components/DataGridPage', () => ({
  DataGridPage: ({ rows }: { rows: Array<{ id: string }> }) => <div data-testid='grid-row-count'>{rows.length}</div>
}))
vi.mock('../../components/StatusChip', () => ({
  StatusChip: ({ label }: { label: string }) => <span>{label}</span>
}))
vi.mock('../../components/SetIDExplainPanel', () => ({
  SetIDExplainPanel: () => <div data-testid='setid-explain-panel'>explain</div>
}))
vi.mock('@mui/x-date-pickers/DatePicker', () => ({
  DatePicker: ({
    label,
    value,
    onChange
  }: {
    label: string
    value: Date | null
    onChange: (value: Date | null) => void
  }) => {
    const dateValue = value
      ? `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
      : ''

    return (
      <input
        aria-label={label}
        type='date'
        value={dateValue}
        onChange={(event) => onChange(event.target.value ? new Date(`${event.target.value}T00:00:00`) : null)}
      />
    )
  }
}))
vi.mock('../../utils/readViewState', async () => {
  const actual = await vi.importActual<typeof import('../../utils/readViewState')>('../../utils/readViewState')
  return {
    ...actual,
    todayISODate: () => new Date().toISOString().slice(0, 10)
  }
})

function LocationProbe() {
  const location = useLocation()
  return <div data-testid='location-search'>{location.search}</div>
}

function renderPage(initialEntry = '/jobcatalog') {
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
            path='/jobcatalog'
            element={
              <>
                <JobCatalogPage />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('JobCatalogPage', () => {
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
      t: (key: string, vars?: Record<string, unknown>) =>
        (
          {
            jobcatalog_page_title: 'Job Catalog',
            jobcatalog_page_subtitle: 'Catalog subtitle',
            jobcatalog_filter_as_of: 'As Of Date',
            jobcatalog_filter_package_code: 'Package Code',
            jobcatalog_filter_apply_context: 'Apply Context',
            jobcatalog_filter_reset_context: 'Reset',
            jobcatalog_context_no_package: 'No package',
            jobcatalog_context_owner_setid: 'Owner SetID',
            jobcatalog_context_read_only: 'Read Only',
            jobcatalog_context_read_only_true: 'Yes',
            jobcatalog_context_read_only_false: 'No',
            jobcatalog_info_select_package: 'Select a package',
            jobcatalog_tab_groups: 'Groups',
            jobcatalog_tab_families: 'Families',
            jobcatalog_tab_levels: 'Levels',
            jobcatalog_tab_profiles: 'Profiles',
            jobcatalog_toolbar_groups_title: 'Groups',
            jobcatalog_toolbar_levels_title: 'Levels',
            jobcatalog_toolbar_families_title: 'Families',
            jobcatalog_toolbar_profiles_title: 'Profiles',
            jobcatalog_toolbar_count: `Count ${vars?.count ?? ''}`,
            jobcatalog_filter_list_keyword: 'Keyword',
            jobcatalog_action_create_group: 'Create Group',
            jobcatalog_action_create_level: 'Create Level',
            jobcatalog_empty_groups: 'No groups',
            jobcatalog_empty_levels: 'No levels',
            common_view_current_label: 'Viewing current data by default',
            common_view_history: 'View History',
            common_view_current: 'View Current',
            jobcatalog_form_group_code: 'Group Code',
            jobcatalog_form_group_name: 'Group Name',
            jobcatalog_form_effective_date: 'Effective Date',
            common_cancel: 'Cancel',
            common_submit: 'Submit',
            jobcatalog_action_menu: 'Actions',
            common_detail: 'Detail'
          } as Record<string, string>
        )[key] ?? key
    })

    setidApiMocks.listSetIDs.mockResolvedValue({
      setids: [{ setid: 'SET-1' }]
    })
    jobCatalogApiMocks.getJobCatalog.mockResolvedValue({
      view: { owner_setid: 'SET-1', read_only: false },
      job_family_groups: [
        {
          job_family_group_uuid: 'group-1',
          job_family_group_code: 'GRP',
          name: 'Group',
          effective_day: '2026-04-08',
          is_active: true
        }
      ],
      job_families: [],
      job_levels: [],
      job_profiles: []
    })
    jobCatalogApiMocks.applyJobCatalogAction.mockResolvedValue({})
  })

  it('defaults to current mode and only enters history when requested', async () => {
    renderPage('/jobcatalog?setid=SET-1')

    await waitFor(() =>
      expect(jobCatalogApiMocks.getJobCatalog).toHaveBeenCalledWith({ asOf: currentUTCDate(), setid: 'SET-1' })
    )

    expect(screen.queryByLabelText('As Of Date')).not.toBeInTheDocument()
    expect(screen.getAllByText('Viewing current data by default').length).toBeGreaterThan(0)

    fireEvent.click(screen.getByRole('button', { name: 'View History' }))
    fireEvent.change(await screen.findByLabelText('As Of Date'), { target: { value: '2026-03-01' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply Context' }))

    await waitFor(() => expect(screen.getByTestId('location-search')).toHaveTextContent('as_of=2026-03-01'))
  }, 20000)

  it('does not seed create dialog effective date from history as_of', async () => {
    renderPage('/jobcatalog?setid=SET-1&as_of=2026-03-01')

    await waitFor(() => expect(jobCatalogApiMocks.getJobCatalog).toHaveBeenCalledWith({ asOf: '2026-03-01', setid: 'SET-1' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create Group' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Create Group' }))

    expect(await screen.findByLabelText('Effective Date')).toHaveValue(currentUTCDate())
  }, 20000)
})
