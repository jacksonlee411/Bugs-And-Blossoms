import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppShell } from './AppShell'
import type { NavItem } from '../types/navigation'

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('../pages/cubebox/CubeBoxProvider', () => ({
  CubeBoxProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))
vi.mock('../pages/cubebox/CubeBoxPanel', () => ({
  CubeBoxPanel: () => <div data-testid='cubebox-panel'>CubeBox panel</div>
}))
vi.mock('../observability/tracker', () => ({
  trackUiEvent: vi.fn()
}))

const navItems: NavItem[] = [
  {
    key: 'home',
    path: '/',
    labelKey: 'nav_foundation_demo',
    icon: <span />,
    order: 1,
    permissionKey: 'foundation.read',
    keywords: ['home']
  }
]

function renderShell() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route element={<AppShell navItems={navItems} />} path='/'>
          <Route element={<button>Left action</button>} index />
        </Route>
      </Routes>
    </MemoryRouter>
  )
}

function setViewport(width: number) {
  window.innerWidth = width
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: matchesMinWidth(query, width),
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn()
  }))
}

function matchesMinWidth(query: string, width: number) {
  const match = /min-width:\s*(\d+(?:\.\d+)?)px/.exec(query)
  if (!match) {
    return false
  }
  return width >= Number(match[1])
}

describe('AppShell CubeBox shell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'zh',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasPermission: vi.fn().mockReturnValue(true),
      t: (key: string) =>
        (
          {
            app_title: 'Bugs & Blossoms',
            action_logout: '退出登录',
            cubebox_open_drawer: '打开 CubeBox 抽屉',
            cubebox_close_drawer: '关闭 CubeBox 抽屉',
            page_cubebox_title: 'CubeBox',
            global_search: '全局搜索',
            global_search_placeholder: '搜索',
            global_search_empty: '无结果',
            language_zh: '中文',
            language_en: 'English',
            nav_foundation_demo: '基座示例',
            nav_debug_mode: '导航调试',
            search_source_navigation: '导航',
            search_source_common: '常用',
            theme_dark: '深色',
            theme_light: '浅色'
          } as Record<string, string>
        )[key] ?? key
    })
  })

  it('uses wide non-modal shell and reserves main content on lg screens', () => {
    setViewport(1280)
    renderShell()

    fireEvent.click(screen.getByRole('button', { name: '打开 CubeBox 抽屉' }))

    const root = screen.getByTestId('cubebox-panel').closest('[data-cubebox-shell-mode]')
    expect(root).toHaveAttribute('data-cubebox-shell-mode', 'wide')
    expect(screen.getByRole('main')).toHaveAttribute('data-cubebox-main-reserves-panel', 'true')
    expect(screen.getByRole('complementary', { name: 'CubeBox' })).toHaveAttribute('data-cubebox-non-modal', 'true')
    expect(screen.getByRole('button', { name: 'Left action' })).toBeInTheDocument()
  })

  it('uses medium non-modal overlay shell on md screens', () => {
    setViewport(960)
    renderShell()

    fireEvent.click(screen.getByRole('button', { name: '打开 CubeBox 抽屉' }))

    const root = screen.getByTestId('cubebox-panel').closest('[data-cubebox-shell-mode]')
    expect(root).toHaveAttribute('data-cubebox-shell-mode', 'medium')
    expect(screen.getByRole('main')).toHaveAttribute('data-cubebox-main-reserves-panel', 'false')
    expect(screen.getByRole('complementary', { name: 'CubeBox' })).toHaveAttribute('data-cubebox-non-modal', 'true')
  })

  it('uses compact modal shell on small screens and closes with Escape', () => {
    setViewport(640)
    renderShell()

    fireEvent.click(screen.getByRole('button', { name: '打开 CubeBox 抽屉' }))

    const root = screen.getByTestId('cubebox-panel').closest('[data-cubebox-shell-mode]')
    expect(root).toHaveAttribute('data-cubebox-shell-mode', 'compact')
    expect(screen.getByRole('complementary', { name: 'CubeBox' })).toHaveAttribute('data-cubebox-non-modal', 'false')

    fireEvent.keyDown(window, { key: 'Escape' })

    expect(screen.getByRole('button', { name: '打开 CubeBox 抽屉' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('hides CubeBox entry when conversation permissions are missing', () => {
    setViewport(1280)
    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'zh',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasPermission: vi.fn().mockImplementation((permissionKey?: string) => permissionKey === 'foundation.read'),
      t: (key: string) =>
        (
          {
            app_title: 'Bugs & Blossoms',
            action_logout: '退出登录',
            cubebox_open_drawer: '打开 CubeBox 抽屉',
            cubebox_close_drawer: '关闭 CubeBox 抽屉',
            page_cubebox_title: 'CubeBox',
            global_search: '全局搜索',
            global_search_placeholder: '搜索',
            global_search_empty: '无结果',
            language_zh: '中文',
            language_en: 'English',
            nav_foundation_demo: '基座示例',
            nav_debug_mode: '导航调试',
            search_source_navigation: '导航',
            search_source_common: '常用',
            theme_dark: '深色',
            theme_light: '浅色'
          } as Record<string, string>
        )[key] ?? key
    })

    renderShell()

    expect(screen.queryByRole('button', { name: '打开 CubeBox 抽屉' })).not.toBeInTheDocument()
  })
})
