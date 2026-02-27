import ApartmentIcon from '@mui/icons-material/Apartment'
import AssignmentIndIcon from '@mui/icons-material/AssignmentInd'
import CategoryIcon from '@mui/icons-material/Category'
import GroupsIcon from '@mui/icons-material/Groups'
import HubIcon from '@mui/icons-material/Hub'
import HomeWorkIcon from '@mui/icons-material/HomeWork'
import MenuBookIcon from '@mui/icons-material/MenuBook'
import PendingActionsIcon from '@mui/icons-material/PendingActions'
import TuneIcon from '@mui/icons-material/Tune'
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
    key: 'org-field-configs',
    path: '/org/units/field-configs',
    labelKey: 'nav_org_field_configs',
    icon: <TuneIcon fontSize='small' />,
    order: 21,
    permissionKey: 'orgunit.admin',
    keywords: ['field', 'config', 'metadata', 'orgunit', '字段', '配置', '元数据']
  },
  {
    key: 'dict-configs',
    path: '/dicts',
    labelKey: 'nav_dicts',
    icon: <MenuBookIcon fontSize='small' />,
    order: 22,
    permissionKey: 'dict.admin',
    keywords: ['dict', 'dictionary', 'value', '配置', '字典', '编码']
  },
  {
    key: 'configuration-policy',
    path: '/org/setid',
    labelKey: 'nav_configuration_policy',
    icon: <HubIcon fontSize='small' />,
    order: 25,
    permissionKey: 'orgunit.read',
    keywords: ['setid', 'configuration', 'policy', '集合', '策略', '治理']
  },
  {
    key: 'configuration-policy-base',
    path: '/org/setid/base',
    labelKey: 'nav_configuration_policy_base',
    icon: <HubIcon fontSize='small' />,
    order: 251,
    parentKey: 'configuration-policy',
    permissionKey: 'orgunit.read',
    keywords: ['setid', 'binding', '集合', '绑定']
  },
  {
    key: 'configuration-policy-registry',
    path: '/org/setid/registry',
    labelKey: 'nav_configuration_policy_registry',
    icon: <HubIcon fontSize='small' />,
    order: 252,
    parentKey: 'configuration-policy',
    permissionKey: 'orgunit.read',
    keywords: ['policy', 'registry', 'capability', '策略', '规则']
  },
  {
    key: 'configuration-policy-explain',
    path: '/org/setid/explain',
    labelKey: 'nav_configuration_policy_explain',
    icon: <HubIcon fontSize='small' />,
    order: 253,
    parentKey: 'configuration-policy',
    permissionKey: 'orgunit.read',
    keywords: ['explain', 'trace', '命中', '解释']
  },
  {
    key: 'configuration-policy-ops',
    path: '/org/setid/ops',
    labelKey: 'nav_configuration_policy_ops',
    icon: <HubIcon fontSize='small' />,
    order: 254,
    parentKey: 'configuration-policy',
    permissionKey: 'orgunit.read',
    keywords: ['activation', 'functional', '运维', '激活']
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
