import ApartmentIcon from '@mui/icons-material/Apartment'
import AssignmentIndIcon from '@mui/icons-material/AssignmentInd'
import CategoryIcon from '@mui/icons-material/Category'
import GroupsIcon from '@mui/icons-material/Groups'
import HubIcon from '@mui/icons-material/Hub'
import HomeWorkIcon from '@mui/icons-material/HomeWork'
import PendingActionsIcon from '@mui/icons-material/PendingActions'
import WorkOutlineIcon from '@mui/icons-material/WorkOutline'
import type { NavItem, SearchEntry } from '../types/navigation'

export const navItems: NavItem[] = [
  {
    key: 'foundation-demo',
    path: '/',
    labelKey: 'nav_foundation_demo',
    icon: <HomeWorkIcon fontSize='small' />,
    order: 10,
    permissionKey: 'foundation.read',
    keywords: ['foundation', 'demo', 'mui', '基座', '示例']
  },
  {
    key: 'org-units',
    path: '/org/units',
    labelKey: 'nav_org_units',
    icon: <ApartmentIcon fontSize='small' />,
    order: 20,
    permissionKey: 'orgunit.read',
    keywords: ['org', 'unit', 'department', '组织', '部门']
  },
  {
    key: 'org-setid',
    path: '/org/setid',
    labelKey: 'nav_setid',
    icon: <HubIcon fontSize='small' />,
    order: 25,
    permissionKey: 'orgunit.read',
    keywords: ['setid', 'scope', 'package', '治理', '共享', '只读']
  },
  {
    key: 'jobcatalog',
    path: '/jobcatalog',
    labelKey: 'nav_jobcatalog',
    icon: <CategoryIcon fontSize='small' />,
    order: 30,
    permissionKey: 'jobcatalog.read',
    keywords: ['jobcatalog', 'job', 'catalog', '职位族', '职位', '分类']
  },
  {
    key: 'staffing-positions',
    path: '/staffing/positions',
    labelKey: 'nav_staffing_positions',
    icon: <WorkOutlineIcon fontSize='small' />,
    order: 40,
    permissionKey: 'staffing.positions.read',
    keywords: ['staffing', 'position', '职位', '岗位']
  },
  {
    key: 'staffing-assignments',
    path: '/staffing/assignments',
    labelKey: 'nav_staffing_assignments',
    icon: <AssignmentIndIcon fontSize='small' />,
    order: 50,
    permissionKey: 'staffing.assignments.read',
    keywords: ['staffing', 'assignment', '任职', '用工']
  },
  {
    key: 'people-directory',
    path: '/person/persons',
    labelKey: 'nav_people',
    icon: <GroupsIcon fontSize='small' />,
    order: 60,
    permissionKey: 'person.read',
    keywords: ['people', 'person', 'employee', '人员', '员工']
  },
  {
    key: 'approval-inbox',
    path: '/approvals',
    labelKey: 'nav_approvals',
    icon: <PendingActionsIcon fontSize='small' />,
    order: 70,
    permissionKey: 'approval.read',
    keywords: ['approval', 'workflow', 'task', '审批', '待办']
  }
]

export const commonSearchEntries: SearchEntry[] = [
  {
    key: 'common-my-tasks',
    labelKey: 'common_my_tasks',
    path: '/approvals',
    source: 'common',
    keywords: ['task', 'todo', 'approval', '任务', '待办']
  },
  {
    key: 'common-recent-changes',
    labelKey: 'common_recent_changes',
    path: '/org/units',
    source: 'common',
    keywords: ['change', 'history', 'audit', '变更', '审计']
  },
  {
    key: 'common-foundation-demo',
    labelKey: 'nav_foundation_demo',
    path: '/',
    source: 'common',
    keywords: ['demo', 'example', '示例']
  }
]

export function buildNavigationSearchEntries(items: NavItem[]): SearchEntry[] {
  return items.map((item) => ({
    key: `nav-${item.key}`,
    labelKey: item.labelKey,
    path: item.path,
    source: 'navigation',
    keywords: item.keywords
  }))
}
