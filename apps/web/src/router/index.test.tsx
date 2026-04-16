import { describe, expect, it, vi } from 'vitest'

vi.mock('../layout/AppShell', () => ({
  AppShell: () => null
}))
vi.mock('../pages/approvals/ApprovalsInboxPage', () => ({
  ApprovalsInboxPage: () => null
}))
vi.mock('../pages/cubebox/CubeBoxFilesPage', () => ({
  CubeBoxFilesPage: () => null
}))
vi.mock('../pages/cubebox/CubeBoxModelsPage', () => ({
  CubeBoxModelsPage: () => null
}))
vi.mock('../pages/cubebox/CubeBoxPage', () => ({
  CubeBoxPage: () => null
}))
vi.mock('../pages/FoundationDemoPage', () => ({
  FoundationDemoPage: () => null
}))
vi.mock('../pages/dicts/DictConfigsPage', () => ({
  DictConfigsPage: () => null
}))
vi.mock('../pages/dicts/DictValueDetailsPage', () => ({
  DictValueDetailsPage: () => null
}))
vi.mock('../pages/jobcatalog/JobCatalogPage', () => ({
  JobCatalogPage: () => null
}))
vi.mock('../pages/LoginPage', () => ({
  LoginPage: () => null
}))
vi.mock('../pages/org/OrgUnitFieldConfigsPage', () => ({
  OrgUnitFieldConfigsPage: () => null
}))
vi.mock('../pages/org/SetIDGovernancePage', () => ({
  SetIDGovernancePage: () => null
}))
vi.mock('../pages/org/OrgUnitDetailsPage', () => ({
  OrgUnitDetailsPage: () => null
}))
vi.mock('../pages/org/OrgUnitsPage', () => ({
  OrgUnitsPage: () => null
}))
vi.mock('../pages/person/PersonsPage', () => ({
  PersonsPage: () => null
}))
vi.mock('../pages/staffing/AssignmentsPage', () => ({
  AssignmentsPage: () => null
}))
vi.mock('../pages/staffing/PositionsPage', () => ({
  PositionsPage: () => null
}))
vi.mock('./RequirePermission', () => ({
  RequirePermission: ({ children }: { children: unknown }) => children
}))
vi.mock('./RouteErrorPage', () => ({
  RouteErrorPage: () => null
}))

import { router } from './index'

function routeElement(route: unknown): { props?: { to?: string; replace?: boolean; permissionKey?: string } } | null {
  if (!route || typeof route !== 'object' || !('element' in route)) {
    return null
  }

  const element = (route as { element?: unknown }).element
  if (!element || typeof element !== 'object') {
    return null
  }

  return element as { props?: { to?: string; replace?: boolean; permissionKey?: string } }
}

describe('cubebox router aliases', () => {
  it('keeps assistant aliases as redirect-only entries', () => {
    const rootRoute = router.routes.find((route) => route.path === '/')
    expect(rootRoute).toBeTruthy()

    const children = rootRoute?.children ?? []
    const assistant = children.find((route) => route.path === 'assistant')
    const assistantModels = children.find((route) => route.path === 'assistant/models')
    const cubeboxModels = children.find((route) => route.path === 'cubebox/models')

    expect(routeElement(assistant)).toBeTruthy()
    expect(routeElement(assistantModels)).toBeTruthy()
    expect(routeElement(assistant)?.props?.to).toBe('/cubebox')
    expect(routeElement(assistantModels)?.props?.to).toBe('/cubebox/models')
    expect(routeElement(assistant)?.props?.replace).toBe(true)
    expect(routeElement(assistantModels)?.props?.replace).toBe(true)
    expect(routeElement(cubeboxModels)?.props?.permissionKey).toBe('orgunit.read')
    expect(children.find((route) => route.path === 'assistant/librechat')).toBeUndefined()
  })
})
