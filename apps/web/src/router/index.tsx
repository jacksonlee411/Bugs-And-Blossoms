import { Navigate, createBrowserRouter } from 'react-router-dom'
import { AppShell } from '../layout/AppShell'
import { navItems } from '../navigation/config'
import { ApprovalsInboxPage } from '../pages/approvals/ApprovalsInboxPage'
import { FoundationDemoPage } from '../pages/FoundationDemoPage'
import { DictConfigsPage } from '../pages/dicts/DictConfigsPage'
import { DictValueDetailsPage } from '../pages/dicts/DictValueDetailsPage'
import { LoginPage } from '../pages/LoginPage'
import { OrgUnitFieldConfigsPage } from '../pages/org/OrgUnitFieldConfigsPage'
import { OrgUnitDetailsPage } from '../pages/org/OrgUnitDetailsPage'
import { OrgUnitsPage } from '../pages/org/OrgUnitsPage'
import { RequirePermission } from './RequirePermission'
import { RouteErrorPage } from './RouteErrorPage'

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />
  },
  {
    path: '/',
    element: <AppShell navItems={navItems} />,
    errorElement: <RouteErrorPage />,
    children: [
      {
        index: true,
        element: (
          <RequirePermission permissionKey='foundation.read'>
            <FoundationDemoPage />
          </RequirePermission>
        )
      },
      {
        path: 'home',
        element: <Navigate replace to='/' />
      },
      {
        path: 'org/units/field-configs',
        element: (
          <RequirePermission permissionKey='orgunit.admin'>
            <OrgUnitFieldConfigsPage />
          </RequirePermission>
        )
      },
      {
        path: 'dicts',
        element: (
          <RequirePermission permissionKey='dict.admin'>
            <DictConfigsPage />
          </RequirePermission>
        )
      },
      {
        path: 'dicts/:dictCode/values/:code',
        element: (
          <RequirePermission permissionKey='dict.admin'>
            <DictValueDetailsPage />
          </RequirePermission>
        )
      },
      {
        path: 'org/units/:orgCode',
        element: (
          <RequirePermission permissionKey='orgunit.read'>
            <OrgUnitDetailsPage />
          </RequirePermission>
        )
      },
      {
        path: 'org/units',
        element: (
          <RequirePermission permissionKey='orgunit.read'>
            <OrgUnitsPage />
          </RequirePermission>
        )
      },
      {
        path: 'approvals',
        element: (
          <RequirePermission permissionKey='approval.read'>
            <ApprovalsInboxPage />
          </RequirePermission>
        )
      },
      {
        path: '*',
        element: <Navigate replace to='/' />
      }
    ]
  }
], {
  basename: '/app'
})
