import { createBrowserRouter } from 'react-router-dom'
import { AppShell } from '../layout/AppShell'
import { navItems } from '../navigation/config'
import { ComingSoonPage } from '../pages/ComingSoonPage'
import { FoundationDemoPage } from '../pages/FoundationDemoPage'
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
        path: 'org/units',
        element: (
          <RequirePermission permissionKey='orgunit.read'>
            <ComingSoonPage moduleNameKey='nav_org_units' />
          </RequirePermission>
        )
      },
      {
        path: 'people',
        element: (
          <RequirePermission permissionKey='person.read'>
            <ComingSoonPage moduleNameKey='nav_people' />
          </RequirePermission>
        )
      },
      {
        path: 'approvals',
        element: (
          <RequirePermission permissionKey='approval.read'>
            <ComingSoonPage moduleNameKey='nav_approvals' />
          </RequirePermission>
        )
      }
    ]
  }
])
