import { Navigate, createBrowserRouter } from 'react-router-dom'
import { AUTHZ_CAPABILITY_KEYS } from '../authz/capabilities'
import { AppShell } from '../layout/AppShell'
import { navItems } from '../navigation/config'
import { APIAuthorizationCatalogPage, CapabilityAuthorizationsPage } from '../pages/authz/AuthzCatalogPage'
import { RoleManagementPage, UserAuthorizationPage } from '../pages/authz/AuthzRolePages'
import { ApprovalsInboxPage } from '../pages/approvals/ApprovalsInboxPage'
import { FoundationDemoPage } from '../pages/FoundationDemoPage'
import { DictConfigsPage } from '../pages/dicts/DictConfigsPage'
import { DictValueDetailsPage } from '../pages/dicts/DictValueDetailsPage'
import { LoginPage } from '../pages/LoginPage'
import { OrgUnitFieldConfigsPage } from '../pages/org/OrgUnitFieldConfigsPage'
import { OrgUnitDetailsPage } from '../pages/org/OrgUnitDetailsPage'
import { OrgUnitsPage } from '../pages/org/OrgUnitsPage'
import { RequireCapability } from './RequireCapability'
import { RouteErrorPage } from './RouteErrorPage'
import { APP_ROUTER_BASENAME } from './paths'

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
        element: <FoundationDemoPage />
      },
      {
        path: 'home',
        element: <Navigate replace to='/' />
      },
      {
        path: 'org/units/field-configs',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsAdmin}>
            <OrgUnitFieldConfigsPage />
          </RequireCapability>
        )
      },
      {
        path: 'dicts',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.iamDictsAdmin}>
            <DictConfigsPage />
          </RequireCapability>
        )
      },
      {
        path: 'dicts/:dictCode/values/:code',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.iamDictsAdmin}>
            <DictValueDetailsPage />
          </RequireCapability>
        )
      },
      {
        path: 'org/units/:orgCode',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsRead}>
            <OrgUnitDetailsPage />
          </RequireCapability>
        )
      },
      {
        path: 'org/units',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsRead}>
            <OrgUnitsPage />
          </RequireCapability>
        )
      },
      {
        path: 'authz/roles',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.iamAuthzAdmin}>
            <RoleManagementPage />
          </RequireCapability>
        )
      },
      {
        path: 'authz/user-assignments',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.iamAuthzAdmin}>
            <UserAuthorizationPage />
          </RequireCapability>
        )
      },
      {
        path: 'authz/capabilities',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.iamAuthzRead}>
            <CapabilityAuthorizationsPage />
          </RequireCapability>
        )
      },
      {
        path: 'authz/api-catalog',
        element: (
          <RequireCapability requiredCapabilityKey={AUTHZ_CAPABILITY_KEYS.iamAuthzRead}>
            <APIAuthorizationCatalogPage />
          </RequireCapability>
        )
      },
      {
        path: 'approvals',
        element: <ApprovalsInboxPage />
      },
      {
        path: '*',
        element: <Navigate replace to='/' />
      }
    ]
  }
], {
  basename: APP_ROUTER_BASENAME
})
