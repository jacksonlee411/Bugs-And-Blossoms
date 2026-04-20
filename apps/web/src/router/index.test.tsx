import { describe, expect, it, vi } from 'vitest'

vi.mock('../layout/AppShell', () => ({
  AppShell: () => null
}))
vi.mock('../pages/approvals/ApprovalsInboxPage', () => ({
  ApprovalsInboxPage: () => null
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

describe('app router', () => {
  it('registers current primary business routes', () => {
    const rootRoute = router.routes.find((route) => route.path === '/')
    expect(rootRoute).toBeTruthy()

    const children = rootRoute?.children ?? []
    const routePaths = new Set(children.map((route) => route.path))

    expect(routePaths.has('org/units')).toBe(true)
    expect(routePaths.has('jobcatalog')).toBe(true)
    expect(routePaths.has('staffing/positions')).toBe(true)
    expect(routePaths.has('person/persons')).toBe(true)
    expect(routePaths.has('staffing/assignments')).toBe(true)
  })
})
