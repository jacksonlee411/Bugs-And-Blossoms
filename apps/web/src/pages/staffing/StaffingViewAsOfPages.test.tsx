import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AssignmentsPage } from './AssignmentsPage'
import { PositionsPage } from './PositionsPage'

const assignmentApiMocks = vi.hoisted(() => ({
  listAssignments: vi.fn(),
  upsertAssignment: vi.fn()
}))

const personApiMocks = vi.hoisted(() => ({
  getPersonByPernr: vi.fn()
}))

const positionApiMocks = vi.hoisted(() => ({
  listPositions: vi.fn(),
  getPositionOptions: vi.fn(),
  upsertPosition: vi.fn()
}))

vi.mock('../../api/assignments', () => assignmentApiMocks)
vi.mock('../../api/persons', () => personApiMocks)
vi.mock('../../api/positions', () => positionApiMocks)
vi.mock('../../components/PageHeader', () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>
}))
vi.mock('../../components/SetIDExplainPanel', () => ({
  SetIDExplainPanel: () => <div data-testid='setid-explain-panel'>explain</div>
}))
vi.mock('../org/readViewState', async () => {
  const actual = await vi.importActual<typeof import('../org/readViewState')>('../org/readViewState')
  return {
    ...actual,
    todayISODate: () => '2026-04-08'
  }
})

function LocationProbe() {
  const location = useLocation()
  return <div data-testid='location-search'>{location.search}</div>
}

function renderAssignmentsPage(initialEntry = '/staffing/assignments') {
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
            path='/staffing/assignments'
            element={
              <>
                <AssignmentsPage />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

function renderPositionsPage(initialEntry = '/staffing/positions') {
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
            path='/staffing/positions'
            element={
              <>
                <PositionsPage />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('Staffing view-as-of pages', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    personApiMocks.getPersonByPernr.mockResolvedValue({
      person_uuid: 'person-1',
      display_name: 'Jane Doe'
    })
    positionApiMocks.listPositions.mockResolvedValue({
      positions: [
        {
          position_uuid: 'position-1',
          name: 'Analyst',
          job_profile_code: 'JP-1',
          org_code: 'ROOT',
          effective_date: '2026-04-08',
          jobcatalog_setid: 'SET-1'
        }
      ]
    })
    assignmentApiMocks.listAssignments.mockResolvedValue({
      assignments: [
        {
          assignment_uuid: 'assignment-1',
          position_uuid: 'position-1',
          effective_date: '2026-04-08',
          status: 'active'
        }
      ]
    })
    positionApiMocks.getPositionOptions.mockResolvedValue({
      jobcatalog_setid: 'SET-1',
      job_profiles: [{ job_profile_uuid: 'profile-1', job_profile_code: 'JP-1', name: 'Analyst Profile' }]
    })
  })

  it('AssignmentsPage defaults to current mode and does not let history overwrite effective_date', async () => {
    renderAssignmentsPage()

    await waitFor(() => expect(positionApiMocks.listPositions).toHaveBeenCalledWith({ asOf: '2026-04-08' }))

    expect(screen.queryByLabelText('as_of')).not.toBeInTheDocument()
    expect(screen.getAllByText('Viewing current data by default').length).toBeGreaterThan(0)
    expect(screen.getByTestId('location-search')).toHaveTextContent('')

    const effectiveDateInput = screen.getByLabelText('effective_date') as HTMLInputElement
    fireEvent.change(effectiveDateInput, { target: { value: '2026-06-10' } })

    fireEvent.click(screen.getByRole('button', { name: 'View History' }))
    fireEvent.change(await screen.findByLabelText('as_of'), { target: { value: '2026-03-01' } })
    fireEvent.click(screen.getByRole('button', { name: 'Load' }))

    await waitFor(() => expect(screen.getByTestId('location-search')).toHaveTextContent('as_of=2026-03-01'))
    expect((screen.getByLabelText('effective_date') as HTMLInputElement).value).toBe('2026-06-10')
  })

  it('PositionsPage defaults to current mode and does not let history overwrite effective_date', async () => {
    renderPositionsPage()

    await waitFor(() => expect(positionApiMocks.listPositions).toHaveBeenCalledWith({ asOf: '2026-04-08' }))

    expect(screen.queryByLabelText('as_of')).not.toBeInTheDocument()
    expect(screen.getAllByText('Viewing current data by default').length).toBeGreaterThan(0)
    expect(screen.getByTestId('location-search')).toHaveTextContent('')

    const effectiveDateInput = screen.getByLabelText('effective_date') as HTMLInputElement
    fireEvent.change(effectiveDateInput, { target: { value: '2026-07-15' } })

    fireEvent.click(screen.getByRole('button', { name: 'View History' }))
    fireEvent.change(await screen.findByLabelText('as_of'), { target: { value: '2026-03-01' } })
    fireEvent.click(screen.getByRole('button', { name: 'Load' }))

    await waitFor(() => expect(screen.getByTestId('location-search')).toHaveTextContent('as_of=2026-03-01'))
    expect((screen.getByLabelText('effective_date') as HTMLInputElement).value).toBe('2026-07-15')
  })
})
