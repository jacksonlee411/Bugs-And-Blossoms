import { Navigate, createBrowserRouter } from 'react-router-dom'
import { AppShell } from '../layout/AppShell'
import { navItems } from '../navigation/config'
import { ApprovalsInboxPage } from '../pages/approvals/ApprovalsInboxPage'
import { FoundationDemoPage } from '../pages/FoundationDemoPage'
import { OrgUnitsPage } from '../pages/org/OrgUnitsPage'
import { PeopleAssignmentsPage } from '../pages/people/PeopleAssignmentsPage'
import { RequirePermission } from './RequirePermission'

export const router = createBrowserRouter([
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
        path: 'org/units',
        element: (
          <RequirePermission permissionKey='orgunit.read'>
            <OrgUnitsPage />
          </RequirePermission>
        )
      },
      {
        path: 'people',
        element: (
          <RequirePermission permissionKey='person.read'>
            <PeopleAssignmentsPage />
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
