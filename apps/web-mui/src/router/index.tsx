import { Navigate, createBrowserRouter } from 'react-router-dom'
import { AppShell } from '../layout/AppShell'
import { navItems } from '../navigation/config'
import { ApprovalsInboxPage } from '../pages/approvals/ApprovalsInboxPage'
import { FoundationDemoPage } from '../pages/FoundationDemoPage'
import { JobCatalogPage } from '../pages/jobcatalog/JobCatalogPage'
import { LoginPage } from '../pages/LoginPage'
import { OrgUnitFieldConfigsPage } from '../pages/org/OrgUnitFieldConfigsPage'
import { SetIDGovernancePage } from '../pages/org/SetIDGovernancePage'
import { OrgUnitDetailsPage } from '../pages/org/OrgUnitDetailsPage'
import { OrgUnitsPage } from '../pages/org/OrgUnitsPage'
import { PersonsPage } from '../pages/person/PersonsPage'
import { AssignmentsPage } from '../pages/staffing/AssignmentsPage'
import { PositionsPage } from '../pages/staffing/PositionsPage'
import { RequirePermission } from './RequirePermission'

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />
  },
  {
    path: '/',
    element: <AppShell navItems={navItems} />,
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
        path: 'people',
        element: <Navigate replace to='/person/persons' />
      },
      {
        path: 'person/persons',
        element: (
          <RequirePermission permissionKey='person.read'>
            <PersonsPage />
          </RequirePermission>
        )
      },
      {
        path: 'org/setid',
        element: (
          <RequirePermission permissionKey='orgunit.read'>
            <SetIDGovernancePage />
          </RequirePermission>
        )
      },
      {
        path: 'jobcatalog',
        element: (
          <RequirePermission permissionKey='jobcatalog.read'>
            <JobCatalogPage />
          </RequirePermission>
        )
      },
      {
        path: 'staffing/positions',
        element: (
          <RequirePermission permissionKey='staffing.positions.read'>
            <PositionsPage />
          </RequirePermission>
        )
      },
      {
        path: 'staffing/assignments',
        element: (
          <RequirePermission permissionKey='staffing.assignments.read'>
            <AssignmentsPage />
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
