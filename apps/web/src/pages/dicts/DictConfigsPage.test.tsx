import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DictConfigsPage } from './DictConfigsPage'

const currentUTCDate = () => new Date().toISOString().slice(0, 10)

const dictApiMocks = vi.hoisted(() => ({
  createDict: vi.fn(),
  createDictValue: vi.fn(),
  disableDict: vi.fn(),
  executeDictRelease: vi.fn(),
  listDicts: vi.fn(),
  listDictValues: vi.fn(),
  previewDictRelease: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../../api/dicts', () => dictApiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../../components/PageHeader', () => ({
  PageHeader: ({ title, actions }: { title: string; actions?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      <div>{actions}</div>
    </div>
  )
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
  return <div data-testid='location-state'>{`${location.pathname}${location.search}`}</div>
}

function findReleaseDateInput(value = ''): HTMLInputElement {
  const input = document.querySelector(`input[type="date"][value="${value}"]`)
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`release date input with value ${value} not found`)
  }
  return input
}

function renderPage(initialEntry = '/dicts') {
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
            path='/dicts'
            element={
              <>
                <DictConfigsPage />
                <LocationProbe />
              </>
            }
          />
          <Route path='/dicts/:dictCode/values/:code' element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('DictConfigsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'zh',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasRequiredCapability: () => true,
      t: (key: string) =>
        ({
          dict_release_title: '发布',
          dict_release_subtitle: '发布说明',
          dict_release_task_time_hint: '发布时点只作用于本次发布任务，不会改写列表浏览日期。',
          dict_release_stage_idle: '空闲',
          dict_release_no_permission: '无权限',
          dict_release_field_source_tenant_id: '源租户',
          dict_release_field_as_of: '发布时点',
          dict_release_field_release_id: '发布批次',
          dict_release_field_request_id: '请求号',
          dict_release_field_max_conflicts: '冲突上限',
          dict_release_action_preview: '预览',
          dict_release_action_release: '发布',
          dict_release_action_reset: '重置'
        })[key] ?? key
    })

    dictApiMocks.listDicts.mockResolvedValue({
      dicts: [{ dict_code: 'cost_center', name: '成本中心', status: 'active' }]
    })
    dictApiMocks.listDictValues.mockResolvedValue({
      values: [
        {
          dict_code: 'cost_center',
          code: 'A1',
          label: '默认值',
          status: 'active',
          enabled_on: '2026-04-01',
          disabled_on: null,
          updated_at: '2026-04-08T00:00:00Z'
        }
      ]
    })
    dictApiMocks.previewDictRelease.mockResolvedValue({
      source_dict_count: 1,
      target_dict_count: 1,
      source_value_count: 1,
      target_value_count: 1,
      missing_dict_count: 0,
      dict_name_mismatch_count: 0,
      missing_value_count: 0,
      value_label_mismatch_count: 0,
      conflicts: []
    })
  })

  it('defaults to current browsing and omits as_of when opening details from current mode', async () => {
    renderPage()

    await waitFor(() => expect(dictApiMocks.listDicts).toHaveBeenCalledWith(currentUTCDate()))
    await waitFor(() =>
      expect(dictApiMocks.listDictValues).toHaveBeenCalledWith({
        dictCode: 'cost_center',
        asOf: currentUTCDate(),
        q: '',
        status: 'all',
        limit: 50
      })
    )

    expect(screen.queryByLabelText('查看日期')).not.toBeInTheDocument()
    expect(screen.getByText('默认显示当前数据')).toBeInTheDocument()
    expect(screen.getByText('发布时点')).toBeInTheDocument()
    expect(screen.getByText('发布时点只作用于本次发布任务，不会改写列表浏览日期。')).toBeInTheDocument()

    fireEvent.click(await screen.findByText('默认值'))

    await waitFor(() => expect(screen.getByTestId('location-state')).toHaveTextContent('/dicts/cost_center/values/A1'))
    expect(screen.getByTestId('location-state')).not.toHaveTextContent('as_of=')
  }, 20000)

  it('carries as_of to details only in history mode', async () => {
    renderPage()

    await waitFor(() => expect(dictApiMocks.listDicts).toHaveBeenCalled())

    fireEvent.click(screen.getByRole('button', { name: '查看历史' }))
    fireEvent.change(await screen.findByLabelText('查看日期'), { target: { value: '2026-03-01' } })
    fireEvent.click(screen.getByRole('button', { name: '应用筛选' }))

    await waitFor(() => expect(screen.getByTestId('location-state')).toHaveTextContent('/dicts?as_of=2026-03-01'))

    fireEvent.click(await screen.findByText('默认值'))

    await waitFor(() =>
      expect(screen.getByTestId('location-state')).toHaveTextContent('/dicts/cost_center/values/A1?as_of=2026-03-01')
    )
  }, 20000)

  it('does not push browsing as_of when editing release time', async () => {
    renderPage()

    await waitFor(() => expect(dictApiMocks.listDicts).toHaveBeenCalled())

    fireEvent.change(findReleaseDateInput(''), { target: { value: '2026-02-01' } })

    expect(screen.getByTestId('location-state')).toHaveTextContent('/dicts')
    expect(screen.getByTestId('location-state')).not.toHaveTextContent('as_of=')
  }, 20000)
})
