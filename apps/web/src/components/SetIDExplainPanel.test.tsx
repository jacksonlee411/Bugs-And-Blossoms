import type { ComponentProps } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SetIDExplainPanel } from './SetIDExplainPanel'

const setIDApiMocks = vi.hoisted(() => ({
  getSetIDExplain: vi.fn(),
  listSetIDBindings: vi.fn(),
  listSetIDStrategyRegistry: vi.fn(),
  listSetIDs: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../api/setids', () => setIDApiMocks)
vi.mock('../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../utils/readViewState', async () => {
  const actual = await vi.importActual<typeof import('../utils/readViewState')>('../utils/readViewState')
  return {
    ...actual,
    todayISODate: () => '2026-04-08'
  }
})

function renderPanel(props?: Partial<ComponentProps<typeof SetIDExplainPanel>>) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <SetIDExplainPanel
        title='Explain'
        initialAsOf='2026-03-01'
        initialCapabilityKey='org.orgunit_write.field_policy'
        initialFieldKey='cost_center'
        initialBusinessUnitOrgCode='BU-001'
        {...props}
      />
    </QueryClientProvider>
  )
}

describe('SetIDExplainPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'zh',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasPermission: () => true,
      t: (key: string) =>
        ({
          setid_explain_as_of_label: '解释时点',
          setid_explain_as_of_hint: '该时间只用于本次命中解释，不会改写宿主页浏览日期。'
        })[key] ?? key
    })

    setIDApiMocks.listSetIDs.mockResolvedValue({ setids: [] })
    setIDApiMocks.listSetIDBindings.mockResolvedValue({ bindings: [] })
    setIDApiMocks.listSetIDStrategyRegistry.mockResolvedValue({ items: [] })
    setIDApiMocks.getSetIDExplain.mockResolvedValue({
      trace_id: 'trace-1',
      request_id: 'req-1',
      capability_key: 'org.orgunit_write.field_policy',
      business_unit_org_code: 'BU-001',
      as_of: '2026-03-01',
      resolved_setid: 'SHARE',
      decision: 'allow',
      level: 'brief',
      field_decisions: []
    })
  })

  it('uses task-oriented default as-of label instead of raw as_of', async () => {
    renderPanel()

    expect(screen.getByText('解释时点')).toBeInTheDocument()
    expect(screen.getByDisplayValue('2026-03-01')).toBeInTheDocument()
    expect(screen.queryByText('as_of')).not.toBeInTheDocument()
  }, 20000)

  it('submits initialAsOf as explain request parameter', async () => {
    renderPanel()

    fireEvent.click(screen.getByRole('button', { name: '获取命中解释' }))

    await waitFor(() => expect(setIDApiMocks.getSetIDExplain).toHaveBeenCalled())
    expect(setIDApiMocks.getSetIDExplain.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        asOf: '2026-03-01',
        businessUnitOrgCode: 'BU-001'
      })
    )
  }, 20000)

  it('renders custom task-time label and hint from host page props', () => {
    renderPanel({
      asOfLabel: '查看日期 / View As Of',
      asOfHint: '只影响当前 explain 请求。'
    })

    expect(screen.getByText('查看日期 / View As Of')).toBeInTheDocument()
    expect(screen.getByText('只影响当前 explain 请求。')).toBeInTheDocument()
    expect(screen.queryByText('解释时点')).not.toBeInTheDocument()
  }, 20000)
})
